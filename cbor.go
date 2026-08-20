package httpbind

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

// CBOR negotiation helpers used by generated binders and writers when the
// generator ran with EnableCBORHTTP. Nothing here names a driver type: the
// codec calls live in generated code, so a build whose generation left the
// option off carries none of this — the functions are unreferenced and the
// linker drops them along with the whole encoding path.

// DefaultMaxCBORBodyBytes is the default cap for CBOR body reads (1 MiB).
const DefaultMaxCBORBodyBytes = bindcore.DefaultMaxCBORBodyBytes

// SetMaxCBORBodyBytes changes the process-wide CBOR body limit. A non-positive
// value restores DefaultMaxCBORBodyBytes. Both transport runtimes honour the
// same value.
func SetMaxCBORBodyBytes(n int64) {
	bindcore.SetMaxCBORBodyBytes(n)
}

// MaxCBORBodyBytes returns the effective CBOR body limit.
func MaxCBORBodyBytes() int64 {
	return bindcore.MaxCBORBodyBytes()
}

// IsCBORRequest reports whether the request body should be treated as CBOR.
// Matches application/cbor and *+cbor types (RFC 6839).
func IsCBORRequest(r *http.Request) bool {
	return bindcore.IsCBORMediaType(mediaType(r.Header.Get("Content-Type")))
}

// AcceptsCBOR reports whether the client asked for a CBOR response. Only an
// explicit application/cbor entry in Accept counts; wildcards keep the JSON
// default, so a browser's */* never flips the response format.
func AcceptsCBOR(r *http.Request) bool {
	return bindcore.AcceptsCBOR(r.Header.Get("Accept"))
}

// VaryAccept records that the response body depends on the Accept header, so
// a shared cache keys the entry on it. Generated writers call it before
// negotiating; without it a cache could hand a CBOR body to a JSON client.
func VaryAccept(w http.ResponseWriter) {
	w.Header().Add("Vary", "Accept")
}

// ReadCBORBody reads the whole request body, bounded by MaxCBORBodyBytes.
// Decoding is the generated caller's: the bytes handed back are unparsed, and
// the decoder bounds its own walk by the same limit this read enforced.
func ReadCBORBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	limit := MaxCBORBodyBytes()
	if r.ContentLength > limit {
		return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "CBOR body too large"}, jsonbind.ErrBodyTooLarge)
	}
	data, err := jsonbind.ReadLimitHint(r.Body, limit, r.ContentLength)
	if err != nil {
		if err == jsonbind.ErrBodyTooLarge {
			return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "CBOR body too large"}, err)
		}
		return nil, BadRequest(Problem{Code: "body_read", Message: "failed to read body"}, err)
	}
	return data, nil
}

// WriteCBORBytes writes an already-encoded CBOR document, the WriteJSONBytes
// twin. Generated writers build the body into a pooled buffer and hand it
// over here.
func WriteCBORBytes(w http.ResponseWriter, status int, data []byte) error {
	w.Header().Set("Content-Type", bindcore.CBORContentType)
	w.WriteHeader(status)
	_, err := w.Write(data)
	return err
}
