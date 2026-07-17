package httpbinder

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
)

// DefaultMaxJSONBodyBytes is the default cap for JSON document reads (1 MiB).
const DefaultMaxJSONBodyBytes int64 = 1 << 20

var maxJSONBodyBytes atomic.Int64
var errJSONBodyTooLarge = errors.New("httpbinder: JSON body too large")

// SetMaxJSONBodyBytes changes the process-wide JSON body limit. A non-positive
// value restores DefaultMaxJSONBodyBytes.
func SetMaxJSONBodyBytes(n int64) {
	if n <= 0 {
		maxJSONBodyBytes.Store(0)
	} else {
		maxJSONBodyBytes.Store(n)
	}
}

// MaxJSONBodyBytes returns the effective JSON body limit.
func MaxJSONBodyBytes() int64 {
	if n := maxJSONBodyBytes.Load(); n > 0 {
		return n
	}
	return DefaultMaxJSONBodyBytes
}

func readJSONBytes(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = DefaultMaxJSONBodyBytes
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errJSONBodyTooLarge
	}
	return data, nil
}

// DefaultMultipartMaxMemory is the maxMemory argument passed to
// http.Request.ParseMultipartForm (how much of the form stays in RAM before
// spilling file parts to temp files). This is not a body size cap; see
// DefaultMaxMultipartBodyBytes.
const DefaultMultipartMaxMemory int64 = 32 << 20

// DefaultMaxMultipartBodyBytes is the default cap on multipart request bodies
// enforced by ParseMultipartMap (1 MiB). Override with SetMaxMultipartBodyBytes.
// Without this, io.ReadAll / unrestricted ParseMultipartForm would accept
// arbitrarily large bodies inside httpbind-go alone.
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
// The request body is capped at MaxMultipartBodyBytes() so httpbind-go itself
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
var errFileTooLarge = errors.New("httpbinder: multipart file too large")

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

// RawJSONMap decodes a JSON object RawMessage into a map of raw fields.
func RawJSONMap(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid JSON object"}, err)
	}
	if m == nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "JSON value must be an object"}, nil)
	}
	return m, nil
}

// BytesJSONMap decodes a full JSON document (bytes) as an object map.
func BytesJSONMap(data []byte) (map[string]json.RawMessage, error) {
	return RawJSONMap(json.RawMessage(data))
}

// RawJSONArray decodes a JSON array RawMessage into element raw values.
func RawJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid JSON array"}, err)
	}
	return arr, nil
}

// DecodeJSONMapStringString decodes a JSON object with string values.
func DecodeJSONMapStringString(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid string map"}, err)
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

// DecodeJSONStringSlice decodes a JSON array of strings.
func DecodeJSONStringSlice(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid string array"}, err)
	}
	return s, nil
}

// DecodeJSONIntSlice decodes a JSON array of ints.
func DecodeJSONIntSlice(raw json.RawMessage) ([]int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s []int
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid int array"}, err)
	}
	return s, nil
}

// DecodeJSONInt64Slice decodes a JSON array of int64.
func DecodeJSONInt64Slice(raw json.RawMessage) ([]int64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s []int64
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid int64 array"}, err)
	}
	return s, nil
}

// DecodeJSONBoolSlice decodes a JSON array of bools.
func DecodeJSONBoolSlice(raw json.RawMessage) ([]bool, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s []bool
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid bool array"}, err)
	}
	return s, nil
}

// DecodeJSONFloat64Slice decodes a JSON array of float64.
func DecodeJSONFloat64Slice(raw json.RawMessage) ([]float64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s []float64
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid float64 array"}, err)
	}
	return s, nil
}

// ReadJSONMap decodes a JSON object body into a map of raw messages.
// Used by generated binders so they can pick named fields without reflect on T.
// Non-object JSON (arrays, scalars) fails with 400 — required when payload:"*" rest maps are used.
func ReadJSONMap(r *http.Request) (map[string]json.RawMessage, error) {
	if r.Body == nil {
		return map[string]json.RawMessage{}, nil
	}
	defer r.Body.Close()
	limit := MaxJSONBodyBytes()
	if r.ContentLength > limit {
		return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "JSON body too large"}, errJSONBodyTooLarge)
	}
	data, err := readJSONBytes(r.Body, limit)
	if err != nil {
		if err == errJSONBodyTooLarge {
			return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "JSON body too large"}, err)
		}
		return nil, BadRequest(Problem{Code: "body_read", Message: "failed to read body"}, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid JSON body"}, err)
	}
	// encoding/json decodes JSON null into a nil map. Null (and non-objects)
	// must not be treated as an empty object — payload:"*" rest binding and
	// object-shaped models require a real JSON object (or empty body).
	if m == nil {
		return nil, BadRequest(Problem{Code: "json_parse", Message: "JSON body must be an object"}, nil)
	}
	return m, nil
}

// RestJSONAny builds map[string]any from leftover JSON object keys not in exclude.
// Nested JSON values are decoded into any (objects/arrays/numbers/bools/strings/null).
// Prefer non-nil empty map when nothing remains.
func RestJSONAny(jsonBody map[string]json.RawMessage, exclude []string) (map[string]any, error) {
	out := make(map[string]any)
	if jsonBody == nil {
		return out, nil
	}
	skip := excludeSet(exclude)
	for k, raw := range jsonBody {
		if skip[k] {
			continue
		}
		var v any
		if len(raw) == 0 || string(raw) == "null" {
			out[k] = nil
			continue
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, BadRequest(Problem{Code: "json_parse", Message: "invalid JSON rest value"}, err)
		}
		out[k] = v
	}
	return out, nil
}

// RestJSONRaw builds map[string]json.RawMessage from leftover JSON object keys not in exclude.
func RestJSONRaw(jsonBody map[string]json.RawMessage, exclude []string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage)
	if jsonBody == nil {
		return out
	}
	skip := excludeSet(exclude)
	for k, raw := range jsonBody {
		if skip[k] {
			continue
		}
		// Copy bytes so callers can mutate independently.
		cp := make(json.RawMessage, len(raw))
		copy(cp, raw)
		out[k] = cp
	}
	return out
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
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

// DecodeJSONInt unmarshals a JSON raw value as int.
func DecodeJSONInt(raw json.RawMessage) (int, error) {
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	return n, nil
}

// DecodeJSONInt64 unmarshals a JSON raw value as int64.
func DecodeJSONInt64(raw json.RawMessage) (int64, error) {
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	return n, nil
}

// DecodeJSONBool unmarshals a JSON raw value as bool.
func DecodeJSONBool(raw json.RawMessage) (bool, error) {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, err
	}
	return b, nil
}

// DecodeJSONFloat64 unmarshals a JSON raw value as float64.
func DecodeJSONFloat64(raw json.RawMessage) (float64, error) {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, err
	}
	return f, nil
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
