package bindcore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime/multipart"
	"strings"
	"sync/atomic"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// Body-side logic both transport runtimes share. The multipart limit in
// particular has to be one value: a process configuring it once must not find
// that only one of the two surfaces honoured it.
//
// Only real logic lives here. A helper that is a one-line delegation to
// jsonbind or strconv stays with each surface, because routing it through a
// third package would cost an indirection and save nothing.

// DefaultMultipartMaxMemory is how much of a multipart form stays in RAM before
// file parts spill to temp files. This is not a body size cap; see
// DefaultMaxMultipartBodyBytes.
const DefaultMultipartMaxMemory int64 = 32 << 20

// DefaultMaxMultipartBodyBytes is the default cap on multipart request bodies
// (1 MiB). Override with SetMaxMultipartBodyBytes.
const DefaultMaxMultipartBodyBytes int64 = 1 << 20

// maxMultipartBodyBytes holds the process-wide multipart body limit.
// Zero means "use DefaultMaxMultipartBodyBytes".
var maxMultipartBodyBytes atomic.Int64

// SetMaxMultipartBodyBytes sets the global multipart body size limit.
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

// MediaType returns the lowercase type/subtype of a Content-Type header value
// (parameters after ';' are stripped).
func MediaType(ct string) string {
	media, _, _ := strings.Cut(ct, ";")
	return strings.TrimSpace(strings.ToLower(media))
}

// IsJSONMediaType reports whether media is JSON or a +json structured syntax
// suffix type (RFC 6839), e.g. application/json, application/problem+json,
// application/vnd.api+json. text/json is also accepted.
func IsJSONMediaType(media string) bool {
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

// ErrFileTooLarge is returned when a single file part exceeds the limit.
var ErrFileTooLarge = errors.New("httpbind: multipart file too large")

// FileFromHeader reads one multipart part into a File, bounded by limit.
// Both transports reach this: fasthttp's MultipartForm also yields
// *multipart.FileHeader, so the part-reading rule is written once.
func FileFromHeader(fh *multipart.FileHeader, limit int64) (File, error) {
	if limit <= 0 {
		limit = DefaultMaxMultipartBodyBytes
	}
	if limit > 0 && fh.Size > limit {
		return File{}, ErrFileTooLarge
	}
	rc, err := fh.Open()
	if err != nil {
		return File{}, err
	}
	defer rc.Close()

	// Read at most limit+1 bytes so an unknown FileHeader size stays bounded;
	// the header size (when known) sizes the buffer in one allocation.
	data, err := jsonbind.ReadLimitHint(rc, limit, fh.Size)
	if err != nil {
		if err == jsonbind.ErrBodyTooLarge {
			return File{}, ErrFileTooLarge
		}
		return File{}, err
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

// MultipartParseError maps a parse failure to 413 or 400. The caller decides
// tooLarge, because detecting it needs a transport-specific error type on top
// of the shared cases IsMessageTooLarge covers.
func MultipartParseError(err error, tooLarge bool) error {
	if tooLarge {
		return PayloadTooLarge(Problem{Code: "payload_too_large", Message: "multipart body too large"}, err)
	}
	return BadRequest(Problem{Code: "multipart_parse", Message: "invalid multipart body"}, err)
}

// IsMessageTooLarge reports the transport-neutral size-limit failures, without
// errors.As so TinyGo does not need reflect.AssignableTo. A caller adds its own
// transport's error type before consulting this.
func IsMessageTooLarge(err error) bool {
	for err != nil {
		if err == multipart.ErrMessageTooLarge {
			return true
		}
		msg := err.Error()
		if strings.Contains(msg, "request body too large") ||
			strings.Contains(msg, "message too large") {
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

// RestFormAny builds map[string]any from leftover form keys not in exclude.
func RestFormAny(formBody map[string]string, exclude []string) map[string]any {
	out := make(map[string]any)
	if formBody == nil {
		return out
	}
	for k, v := range formBody {
		if isExcluded(exclude, k) {
			continue
		}
		out[k] = v
	}
	return out
}

// RestFormRaw builds map[string]json.RawMessage from leftover form keys.
func RestFormRaw(formBody map[string]string, exclude []string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage)
	if formBody == nil {
		return out
	}
	for k, v := range formBody {
		if isExcluded(exclude, k) {
			continue
		}
		b, _ := json.Marshal(v)
		out[k] = json.RawMessage(b)
	}
	return out
}

func isExcluded(exclude []string, key string) bool {
	for _, k := range exclude {
		if k == key && k != "" && k != "*" {
			return true
		}
	}
	return false
}

// AppendFileJSON appends an uploaded file the way encoding/json rendered it
// before generated encoders stopped going through reflection: exported fields
// in declaration order, with the content base64-encoded.
func AppendFileJSON(dst []byte, f File) []byte {
	dst = append(dst, `{"Filename":`...)
	dst = jsonbind.AppendString(dst, f.Filename)
	dst = append(dst, `,"ContentType":`...)
	dst = jsonbind.AppendString(dst, f.ContentType)
	dst = append(dst, `,"Size":`...)
	dst = jsonbind.AppendInt(dst, f.Size)
	dst = append(dst, `,"Content":`...)
	if f.Content == nil {
		dst = append(dst, "null"...)
	} else {
		dst = append(dst, '"')
		dst = base64.StdEncoding.AppendEncode(dst, f.Content)
		dst = append(dst, '"')
	}
	return append(dst, '}')
}
