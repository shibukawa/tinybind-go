package httpbind

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

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

func readJSONBytes(r io.Reader, limit int64) ([]byte, error) {
	return jsonbind.ReadLimit(r, limit)
}

// DefaultMultipartMaxMemory is the maxMemory argument passed to
// http.Request.ParseMultipartForm (how much of the form stays in RAM before
// spilling file parts to temp files). This is not a body size cap; see
// DefaultMaxMultipartBodyBytes.
const DefaultMultipartMaxMemory int64 = 32 << 20

// DefaultMaxMultipartBodyBytes is the default cap on multipart request bodies
// enforced by ParseMultipartMap (1 MiB). Override with SetMaxMultipartBodyBytes.
// Without this, io.ReadAll / unrestricted ParseMultipartForm would accept
// arbitrarily large bodies inside tinybind-go alone.
const DefaultMaxMultipartBodyBytes int64 = 1 << 20

// maxMultipartBodyBytes holds the process-wide multipart body limit.
// Zero means "use DefaultMaxMultipartBodyBytes".
var maxMultipartBodyBytes atomic.Int64

// SetMaxMultipartBodyBytes sets the global multipart body size limit used by
// ParseMultipartMap (and generated binders). The limit wraps r.Body with
// http.MaxBytesReader and bounds per-file reads.
//
//	n > 0  → use n bytes
//	n <= 0 → restore DefaultMaxMultipartBodyBytes (1 MiB)
func SetMaxMultipartBodyBytes(n int64) {
	if n <= 0 {
		maxMultipartBodyBytes.Store(0)
		return
	}
	maxMultipartBodyBytes.Store(n)
}

// MaxMultipartBodyBytes returns the effective global multipart body limit.
func MaxMultipartBodyBytes() int64 {
	n := maxMultipartBodyBytes.Load()
	if n <= 0 {
		return DefaultMaxMultipartBodyBytes
	}
	return n
}

// Content-type helpers and scalar parsers used by generated binders.
// These do not inspect application struct fields via reflect.

// mediaType returns the lowercase type/subtype of a Content-Type header value
// (parameters after ';' are stripped).
func mediaType(ct string) string {
	media, _, _ := strings.Cut(ct, ";")
	return strings.TrimSpace(strings.ToLower(media))
}

// isJSONMediaType reports whether media is JSON or a +json structured syntax
// suffix type (RFC 6839), e.g. application/json, application/problem+json,
// application/vnd.api+json. text/json is also accepted.
func isJSONMediaType(media string) bool {
	if media == "" {
		return false
	}
	switch media {
	case "application/json", "text/json":
		return true
	}
	// "+json" structured syntax suffix (not "+jsonl", "+json-seq", etc.).
	return strings.HasSuffix(media, "+json")
}

// IsJSONRequest reports whether the request body should be treated as JSON.
// Matches application/json, text/json, and *+json types such as
// application/problem+json (RFC 7807 / RFC 9457).
func IsJSONRequest(r *http.Request) bool {
	return isJSONMediaType(mediaType(r.Header.Get("Content-Type")))
}

// IsFormRequest reports application/x-www-form-urlencoded.
func IsFormRequest(r *http.Request) bool {
	return mediaType(r.Header.Get("Content-Type")) == "application/x-www-form-urlencoded"
}

// IsMultipartRequest reports multipart/form-data.
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
var errFileTooLarge = errors.New("httpbind: multipart file too large")

func fileFromHeader(fh *multipart.FileHeader, limit int64) (File, error) {
	if limit <= 0 {
		limit = DefaultMaxMultipartBodyBytes
	}
	if limit > 0 && fh.Size > limit {
		return File{}, errFileTooLarge
	}
	rc, err := fh.Open()
	if err != nil {
		return File{}, err
	}
	defer rc.Close()

	// Read at most limit+1 bytes so an unknown FileHeader size stays bounded.
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return File{}, err
	}
	if int64(len(data)) > limit {
		return File{}, errFileTooLarge
	}
	ct := fh.Header.Get("Content-Type")
	size := fh.Size
	if size <= 0 {
		size = int64(len(data))
	}
	return File{
		Filename:    fh.Filename,
		ContentType: ct,
		Size:        size,
		Content:     data,
	}, nil
}

func multipartParseError(err error) error {
	if isRequestTooLarge(err) {
		return PayloadTooLarge(Problem{Code: "payload_too_large", Message: "multipart body too large"}, err)
	}
	return BadRequest(Problem{Code: "multipart_parse", Message: "invalid multipart body"}, err)
}

// isRequestTooLarge reports body/message size limit errors without errors.As,
// matching AsHTTPError's TinyGo-friendly unwrap style.
func isRequestTooLarge(err error) bool {
	for err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			return true
		}
		if err == multipart.ErrMessageTooLarge {
			return true
		}
		msg := err.Error()
		if strings.Contains(msg, "request body too large") ||
			strings.Contains(msg, "message too large") ||
			strings.Contains(msg, "http: request body too large") {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// RawJSONMap splits a JSON object value into its raw fields.
func RawJSONMap(raw []byte) (*jsonbind.Object, error) {
	return jsonbind.RawJSONMap(raw)
}

// BytesJSONMap splits a full JSON object document into its raw fields.
func BytesJSONMap(data []byte) (*jsonbind.Object, error) {
	return jsonbind.BytesJSONMap(data)
}

// RawJSONArray splits a JSON array into its raw element values.
func RawJSONArray(raw []byte) ([][]byte, error) {
	return jsonbind.RawJSONArray(raw)
}

// DecodeJSONMapStringString decodes a JSON object with string values.
func DecodeJSONMapStringString(raw []byte) (map[string]string, error) {
	return jsonbind.DecodeJSONMapStringString(raw)
}

// DecodeJSONStringSlice decodes a JSON array of strings.
func DecodeJSONStringSlice(raw []byte) ([]string, error) {
	return jsonbind.DecodeJSONStringSlice(raw)
}

// DecodeJSONIntSlice decodes a JSON array of ints.
func DecodeJSONIntSlice(raw []byte) ([]int, error) {
	return jsonbind.DecodeJSONIntSlice(raw)
}

// DecodeJSONInt64Slice decodes a JSON array of int64.
func DecodeJSONInt64Slice(raw []byte) ([]int64, error) {
	return jsonbind.DecodeJSONInt64Slice(raw)
}

// DecodeJSONBoolSlice decodes a JSON array of bools.
func DecodeJSONBoolSlice(raw []byte) ([]bool, error) {
	return jsonbind.DecodeJSONBoolSlice(raw)
}

// DecodeJSONFloat64Slice decodes a JSON array of float64.
func DecodeJSONFloat64Slice(raw []byte) ([]float64, error) {
	return jsonbind.DecodeJSONFloat64Slice(raw)
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
func ReadJSONObject(r *http.Request) (*jsonbind.Object, error) {
	if r.Body == nil {
		return jsonbind.EmptyObject(), nil
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
	if len(strings.TrimSpace(string(data))) == 0 {
		return jsonbind.EmptyObject(), nil
	}
	obj, err := jsonbind.ParseObject(data)
	if err != nil {
		if je, ok := jsonbind.AsError(err); ok && je.Message == "JSON value must be an object" {
			return nil, BadRequest(Problem{Code: "json_parse", Message: "JSON body must be an object"}, err)
		}
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid JSON body"}, err)
	}
	return obj, nil
}

// RestJSONAny builds map[string]any from leftover JSON object keys not in exclude.
// Nested JSON values are decoded into any (objects/arrays/numbers/bools/strings/null).
// Prefer non-nil empty map when nothing remains.
func RestJSONAny(jsonBody *jsonbind.Object, exclude []string) (map[string]any, error) {
	return jsonbind.RestJSONAny(jsonBody, exclude)
}

// RestJSONNames lists leftover JSON object keys not in exclude, so generated
// code can fill a map[string]json.RawMessage without this package converting
// between map types.
func RestJSONNames(jsonBody *jsonbind.Object, exclude []string) []string {
	return jsonbind.RestJSONNames(jsonBody, exclude)
}

// RestFormAny builds map[string]any from leftover form keys not in exclude (string values).
func RestFormAny(formBody map[string]string, exclude []string) map[string]any {
	out := make(map[string]any)
	if formBody == nil {
		return out
	}
	skip := excludeSet(exclude)
	for k, v := range formBody {
		if skip[k] {
			continue
		}
		out[k] = v
	}
	return out
}

// RestFormRaw builds map[string]json.RawMessage from leftover form keys (JSON-encoded strings).
func RestFormRaw(formBody map[string]string, exclude []string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage)
	if formBody == nil {
		return out
	}
	skip := excludeSet(exclude)
	for k, v := range formBody {
		if skip[k] {
			continue
		}
		b, _ := json.Marshal(v)
		out[k] = json.RawMessage(b)
	}
	return out
}

func excludeSet(exclude []string) map[string]bool {
	skip := make(map[string]bool, len(exclude))
	for _, k := range exclude {
		if k != "" && k != "*" {
			skip[k] = true
		}
	}
	return skip
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

// QueryValue returns the first query parameter value for key.
func QueryValue(r *http.Request, key string) (string, bool) {
	if r.URL == nil {
		return "", false
	}
	vs := r.URL.Query()[key]
	if len(vs) == 0 {
		return "", false
	}
	return vs[0], true
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

// DecodeJSONString unmarshals a JSON raw value as string.
func DecodeJSONString(raw json.RawMessage) (string, error) {
	return jsonbind.DecodeJSONString(raw)
}

// DecodeJSONInt unmarshals a JSON raw value as int.
func DecodeJSONInt(raw json.RawMessage) (int, error) {
	return jsonbind.DecodeJSONInt(raw)
}

// DecodeJSONInt64 unmarshals a JSON raw value as int64.
func DecodeJSONInt64(raw json.RawMessage) (int64, error) {
	return jsonbind.DecodeJSONInt64(raw)
}

// DecodeJSONBool unmarshals a JSON raw value as bool.
func DecodeJSONBool(raw json.RawMessage) (bool, error) {
	return jsonbind.DecodeJSONBool(raw)
}

// DecodeJSONFloat64 unmarshals a JSON raw value as float64.
func DecodeJSONFloat64(raw json.RawMessage) (float64, error) {
	return jsonbind.DecodeJSONFloat64(raw)
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
