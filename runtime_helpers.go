package httpbind

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

// DefaultMaxJSONBodyBytes is the default cap for JSON document reads (1 MiB).
const DefaultMaxJSONBodyBytes = jsonbind.DefaultMaxJSONBodyBytes

var errJSONBodyTooLarge = jsonbind.ErrBodyTooLarge

// SetMaxJSONBodyBytes changes the process-wide JSON body limit. A non-positive
// value restores DefaultMaxJSONBodyBytes.
func SetMaxJSONBodyBytes(n int64) {
	jsonbind.SetMaxJSONBodyBytes(n)
}

// MaxJSONBodyBytes returns the effective JSON body limit.
func MaxJSONBodyBytes() int64 {
	return jsonbind.MaxJSONBodyBytes()
}

// DefaultMultipartMaxMemory is the maxMemory argument passed to
// http.Request.ParseMultipartForm (how much of the form stays in RAM before
// spilling file parts to temp files). This is not a body size cap; see
// DefaultMaxMultipartBodyBytes.
const DefaultMultipartMaxMemory = bindcore.DefaultMultipartMaxMemory

// DefaultMaxMultipartBodyBytes is the default cap on multipart request bodies
// enforced by ParseMultipartMap (1 MiB). Override with SetMaxMultipartBodyBytes.
// Without this, io.ReadAll / unrestricted ParseMultipartForm would accept
// arbitrarily large bodies inside tinybind-go alone.
const DefaultMaxMultipartBodyBytes = bindcore.DefaultMaxMultipartBodyBytes

// SetMaxMultipartBodyBytes sets the global multipart body size limit used by
// ParseMultipartMap (and generated binders). The limit wraps r.Body with
// http.MaxBytesReader and bounds per-file reads.
//
// The value lives in bindcore, so a process setting it once configures every
// transport runtime rather than only the one it happened to call.
//
//	n > 0  → use n bytes
//	n <= 0 → restore DefaultMaxMultipartBodyBytes (1 MiB)
func SetMaxMultipartBodyBytes(n int64) {
	bindcore.SetMaxMultipartBodyBytes(n)
}

// MaxMultipartBodyBytes returns the effective global multipart body limit.
func MaxMultipartBodyBytes() int64 {
	return bindcore.MaxMultipartBodyBytes()
}

// Content-type helpers and scalar parsers used by generated binders.
// These do not inspect application struct fields via reflect.

func mediaType(ct string) string { return bindcore.MediaType(ct) }

func isJSONMediaType(media string) bool { return bindcore.IsJSONMediaType(media) }

// IsJSONRequest reports whether the request body should be treated as JSON.
// Matches application/json, text/json, and *+json types such as
// application/problem+json (RFC 7807 / RFC 9457).
func IsJSONRequest(r *http.Request) bool {
	return isJSONMediaType(mediaType(r.Header.Get("Content-Type")))
}

// IsFormRequest reports application/x-www-form-urlencoded.
//
// Deprecated: current binders dispatch the form kinds through ReadFormBody.
// This exists for generated code that predates it and is removed once that
// code is regenerated.
func IsFormRequest(r *http.Request) bool {
	return mediaType(r.Header.Get("Content-Type")) == "application/x-www-form-urlencoded"
}

// IsMultipartRequest reports multipart/form-data.
//
// Deprecated: current binders dispatch the form kinds through ReadFormBody.
// This exists for generated code that predates it and is removed once that
// code is regenerated.
func IsMultipartRequest(r *http.Request) bool {
	return mediaType(r.Header.Get("Content-Type")) == "multipart/form-data"
}

// ParseMultipartMap parses a multipart/form-data body into scalar form fields
// (first value wins) and named file parts (first file wins per field name).
//
// The request body is capped at MaxMultipartBodyBytes() so tinybind-go itself
// enforces a size limit (default 1 MiB): Content-Length is checked when known,
// r.Body is wrapped with http.MaxBytesReader, and per-file reads use LimitReader.
// Oversized bodies and oversize file parts map to HTTP 413.
func ParseMultipartMap(r *http.Request) (form map[string]string, files map[string]File, err error) {
	limit := MaxMultipartBodyBytes()
	if limit > 0 {
		if r.ContentLength > limit {
			return nil, nil, PayloadTooLarge(Problem{
				Code:    "payload_too_large",
				Message: "multipart body too large",
			}, nil)
		}
		if r.Body != nil {
			// nil ResponseWriter: MaxBytesReader still enforces the byte cap
			// (covers missing/incorrect Content-Length).
			r.Body = http.MaxBytesReader(nil, r.Body, limit)
		}
	}
	maxMem := DefaultMultipartMaxMemory
	if limit > 0 && limit < maxMem {
		maxMem = limit
	}
	if err := r.ParseMultipartForm(maxMem); err != nil {
		return nil, nil, multipartParseError(err)
	}
	form = make(map[string]string)
	files = make(map[string]File)
	if r.MultipartForm == nil {
		return form, files, nil
	}
	for k, vs := range r.MultipartForm.Value {
		if len(vs) > 0 {
			form[k] = vs[0]
		}
	}
	for k, fhs := range r.MultipartForm.File {
		if len(fhs) == 0 {
			continue
		}
		f, err := fileFromHeader(fhs[0], limit)
		if err != nil {
			if errors.Is(err, errFileTooLarge) || isRequestTooLarge(err) {
				return nil, nil, PayloadTooLarge(Problem{
					Code:    "payload_too_large",
					Message: "multipart file too large",
				}, err)
			}
			return nil, nil, BindError(k, "payload", "unreadable file")
		}
		files[k] = f
	}
	return form, files, nil
}

// errFileTooLarge is returned when a single file part exceeds MaxMultipartBodyBytes.
var errFileTooLarge = bindcore.ErrFileTooLarge

func fileFromHeader(fh *multipart.FileHeader, limit int64) (File, error) {
	return bindcore.FileFromHeader(fh, limit)
}

func multipartParseError(err error) error {
	return bindcore.MultipartParseError(err, isRequestTooLarge(err))
}

// isRequestTooLarge reports body/message size limit errors without errors.As,
// matching AsHTTPError's TinyGo-friendly unwrap style. The net/http-specific
// MaxBytesError is checked here; the transport-neutral cases live in bindcore
// so the other runtime reaches the same verdict for the same body.
func isRequestTooLarge(err error) bool {
	for e := err; e != nil; {
		if _, ok := e.(*http.MaxBytesError); ok {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			break
		}
		e = u.Unwrap()
	}
	return bindcore.IsMessageTooLarge(err)
}

// ReadJSONObject splits a JSON object body into its raw fields.
//
// Generated binders need random access by name, because a field may also come
// from the query string or a form and the tag decides which source wins. The
// returned Object holds subslices of the body, so this costs one pass and one
// slice rather than a map plus a copy of every member.
//
// Non-object JSON (arrays, scalars, null) fails with 400 — required when
// payload:"*" rest maps are used.
//
// Deprecated: current binders walk the raw bytes of ReadJSONBody inline
// instead of splitting them into an Object. This exists for generated code
// that predates the inline walk and is removed once that code is regenerated.
func ReadJSONObject(r *http.Request) (*jsonbind.Object, error) {
	data, err := ReadJSONBody(r)
	if err != nil {
		return nil, err
	}
	if jsonbind.IsBlank(data) {
		return jsonbind.EmptyObject(), nil
	}
	obj, err := jsonbind.ParseObject(data)
	if err != nil {
		return nil, JSONBodyError(err)
	}
	return obj, nil
}

// ReadJSONBody reads the raw JSON body under MaxJSONBodyBytes. Generated
// binders parse the returned bytes in a single inline pass rather than going
// through the member split ReadJSONObject performs.
func ReadJSONBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	limit := MaxJSONBodyBytes()
	if r.ContentLength > limit {
		return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "JSON body too large"}, errJSONBodyTooLarge)
	}
	data, err := jsonbind.ReadLimitHint(r.Body, limit, r.ContentLength)
	if err != nil {
		if err == errJSONBodyTooLarge || err == jsonbind.ErrBodyTooLarge {
			return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "JSON body too large"}, err)
		}
		return nil, BadRequest(Problem{Code: "body_read", Message: "failed to read body"}, err)
	}
	return data, nil
}

// ReadJSONBodyOwned is ReadJSONBody under the name the transport pair needs:
// here the read buffer is always freshly owned, while the fasthttp runtime's
// version copies out of the pooled request. A binder that lets raw body bytes
// outlive the bind — a json.RawMessage field or rest map — is emitted against
// this name on both transports.
func ReadJSONBodyOwned(r *http.Request) ([]byte, error) {
	return ReadJSONBody(r)
}

// JSONBodyError wraps a structural JSON failure from a binder's inline body
// walk in the same 400 problems ReadJSONObject produces.
func JSONBodyError(err error) error {
	return bindcore.JSONBodyError(err)
}

// JSONBodyNotObject is the 400 for a body that decodes to a non-object.
func JSONBodyNotObject() error {
	return bindcore.JSONBodyNotObject()
}

// ReadFormBody is the non-JSON half of ReadBody: it dispatches on the form
// content types alone, for binders that read their JSON body inline.
func ReadFormBody(r *http.Request, wantForm, wantFiles bool) (map[string]string, map[string]File, error) {
	if !wantForm && !wantFiles {
		return nil, nil, nil
	}
	media := mediaType(r.Header.Get("Content-Type"))
	if media == "application/x-www-form-urlencoded" {
		m, err := ParseFormMap(r)
		if err != nil {
			return nil, nil, err
		}
		return m, nil, nil
	}
	if media == "multipart/form-data" {
		m, files, err := ParseMultipartMap(r)
		if err != nil {
			return nil, nil, err
		}
		if !wantFiles {
			files = nil
		}
		return m, files, nil
	}
	return nil, nil, nil
}

// RestFormAny builds map[string]any from leftover form keys not in exclude (string values).
func RestFormAny(formBody map[string]string, exclude []string) map[string]any {
	return bindcore.RestFormAny(formBody, exclude)
}

// RestFormRaw builds map[string]json.RawMessage from leftover form keys (JSON-encoded strings).
func RestFormRaw(formBody map[string]string, exclude []string) map[string]json.RawMessage {
	return bindcore.RestFormRaw(formBody, exclude)
}

// ParseFormMap parses urlencoded form body into a flat map (first value wins).
func ParseFormMap(r *http.Request) (map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, BadRequest(Problem{Code: "form_parse", Message: "invalid form body"}, err)
	}
	out := make(map[string]string, len(r.PostForm))
	for k, vs := range r.PostForm {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out, nil
}

// QueryValue returns the first query parameter value for key. It scans the
// raw query for that one key rather than materializing every pair the way
// r.URL.Query() would.
func QueryValue(r *http.Request, key string) (string, bool) {
	if r.URL == nil {
		return "", false
	}
	return queryScan(r.URL.RawQuery, key)
}

// PathValue returns the path value for key (Go 1.22+ ServeMux).
func PathValue(r *http.Request, key string) string {
	return r.PathValue(key)
}

// HeaderValue returns a request header.
func HeaderValue(r *http.Request, key string) string {
	return r.Header.Get(key)
}

// CookieValue returns a cookie value if present.
func CookieValue(r *http.Request, name string) (string, bool) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", false
	}
	return c.Value, true
}

// ParseInt converts a string to int.
func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// ParseInt64 converts a string to int64.
func ParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ParseBool converts a string to bool.
func ParseBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// ParseFloat64 converts a string to float64.
func ParseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
