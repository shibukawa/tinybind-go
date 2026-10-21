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
	return isJSONMediaType(mediaType(canonicalHeader(r, "Content-Type")))
}

// canonicalHeader reads one request header by a key already in canonical
// form. net/http canonicalizes every key it parses off the wire and Header.Set
// canonicalizes every key a test stores, so indexing the map with the
// canonical spelling finds exactly what Header.Get would — without Get
// re-canonicalizing a constant on every request. The key must be a literal
// in canonical form; a name from a struct tag still goes through HeaderValue.
func canonicalHeader(r *http.Request, key string) string {
	if v := r.Header[key]; len(v) > 0 {
		return v[0]
	}
	return ""
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

// isRequestTooLarge reports body/message size limit errors. The
// net/http-specific MaxBytesError is checked here; the transport-neutral cases
// live in bindcore so the other runtime reaches the same verdict for the same
// body.
func isRequestTooLarge(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge) || bindcore.IsMessageTooLarge(err)
}

// ReadJSONBody reads the raw JSON body under MaxJSONBodyBytes. Generated
// binders parse the returned bytes in a single inline pass.
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
// walk as a 400 problem.
func JSONBodyError(err error) error {
	return bindcore.JSONBodyError(err)
}

// JSONBodyNotObject is the 400 for a body that decodes to a non-object.
func JSONBodyNotObject() error {
	return bindcore.JSONBodyNotObject()
}

// FormValues is a form body read once; see bindcore.FormValues. Generated
// binders ask it for their fields through FormLookup.
type FormValues = bindcore.FormValues

// ReadFormBody dispatches on the form content types alone, for binders that
// read their JSON body inline through ReadJSONBody.
//
// An urlencoded body is read once and split into key=value spans, unescaped
// only where a field is looked up, rather than parsed into url.Values and
// copied into a map: the same trade the query string makes. A multipart body
// goes through net/http's parser and keeps the map it built.
func ReadFormBody(r *http.Request, wantForm, wantFiles bool) (FormValues, map[string]File, error) {
	if !wantForm && !wantFiles {
		return FormValues{}, nil, nil
	}
	media := mediaType(canonicalHeader(r, "Content-Type"))
	if media == "application/x-www-form-urlencoded" {
		body, err := readFormBody(r)
		if err != nil {
			return FormValues{}, nil, err
		}
		return bindcore.ReadFormValues(body), nil, nil
	}
	if media == "multipart/form-data" {
		m, files, err := ParseMultipartMap(r)
		if err != nil {
			return FormValues{}, nil, err
		}
		if !wantFiles {
			files = nil
		}
		return bindcore.FormFromMap(m), files, nil
	}
	return FormValues{}, nil, nil
}

// readFormBody reads an urlencoded body under the bound ParseForm applied,
// as one string the pairs then alias.
//
// The read is sized by the Content-Length the way the JSON body's is, so a
// body of known size lands in one buffer rather than growing through
// io.ReadAll's, and the string it becomes is the only other allocation. A
// declared length over the bound is refused before any byte is read.
func readFormBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	if r.ContentLength > bindcore.MaxFormBodyBytes {
		return "", BadRequest(Problem{Code: "form_parse", Message: "form body too large"}, nil)
	}
	data, err := jsonbind.ReadLimitHint(r.Body, bindcore.MaxFormBodyBytes, r.ContentLength)
	if err != nil {
		if err == jsonbind.ErrBodyTooLarge {
			return "", BadRequest(Problem{Code: "form_parse", Message: "form body too large"}, nil)
		}
		return "", BadRequest(Problem{Code: "form_parse", Message: "invalid form body"}, err)
	}
	return string(data), nil
}

// FormLookup returns the first value for key in a form body.
func FormLookup(f FormValues, key string) (string, bool) { return f.Lookup(key) }

// RestFormAny builds map[string]any from the form keys not in exclude (string values).
func RestFormAny(formBody FormValues, exclude []string) map[string]any {
	return bindcore.RestFormAny(formBody, exclude)
}

// RestFormRaw builds map[string]json.RawMessage from the form keys not in exclude (JSON-encoded strings).
func RestFormRaw(formBody FormValues, exclude []string) map[string]json.RawMessage {
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
	return bindcore.ScanQuery(r.URL.RawQuery, key)
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

// ParseIntBits converts a string to a signed integer of the given width,
// reporting a value the width cannot hold rather than truncating it. bits is
// 0 for platform-width int.
//
// The generated binder calls this for every signed width beside int and
// int64, which keep the two helpers they already had so their output does not
// move.
func ParseIntBits(s string, bits int) (int64, error) {
	return strconv.ParseInt(s, 10, bits)
}

// ParseUintBits is the unsigned twin of ParseIntBits. A leading minus is an
// error, so a negative query value cannot arrive as a large positive one.
func ParseUintBits(s string, bits int) (uint64, error) {
	return strconv.ParseUint(s, 10, bits)
}

// ParseBytes decodes a base64 query, path, header or cookie value into a byte
// field. Both alphabets are accepted, padded or not, since a value that
// travelled through a URL may have been written either way; the encoder always
// produces padded standard base64.
func ParseBytes(s string) ([]byte, error) { return bindcore.ParseBytes(s) }
