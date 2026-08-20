package fasthttpbind

import (
	"github.com/shibukawa/tinygodriver/fasthttp"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

// CBOR negotiation helpers, the net/http runtime's twins. Nothing here names
// a driver CBOR type: the codec calls live in generated code, and a build
// generated without EnableCBORHTTP never references these.

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
func IsCBORRequest(ctx *fasthttp.RequestCtx) bool {
	return bindcore.IsCBORMediaType(contentType(ctx))
}

// AcceptsCBOR reports whether the client asked for a CBOR response. Only an
// explicit application/cbor entry in Accept counts; wildcards keep the JSON
// default.
func AcceptsCBOR(ctx *fasthttp.RequestCtx) bool {
	if ctx == nil {
		return false
	}
	return bindcore.AcceptsCBOR(string(ctx.Request.Header.Peek("Accept")))
}

// VaryAccept records that the response body depends on the Accept header, so
// a shared cache keys the entry on it. Generated writers call it before
// negotiating; without it a cache could hand a CBOR body to a JSON client.
func VaryAccept(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Add("Vary", "Accept")
}

// ReadCBORBody hands back the request body, bounded by MaxCBORBodyBytes.
//
// The bytes are copied out of the transport's pooled buffer: a generated
// decoder borrows sub-slices while it walks, and this runtime cannot see
// whether one of them (a captured raw member, say) outlives the request.
func ReadCBORBody(ctx *fasthttp.RequestCtx) ([]byte, error) {
	if ctx == nil {
		return nil, nil
	}
	body := ctx.PostBody()
	limit := MaxCBORBodyBytes()
	if int64(len(body)) > limit {
		return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "CBOR body too large"}, jsonbind.ErrBodyTooLarge)
	}
	if len(body) == 0 {
		return nil, nil
	}
	owned := make([]byte, len(body))
	copy(owned, body)
	return owned, nil
}

// WriteCBORBytes writes an already-encoded CBOR document, the WriteJSONBytes
// twin. Generated writers build the body into a pooled buffer and hand it
// over here.
func WriteCBORBytes(ctx *fasthttp.RequestCtx, status int, data []byte) error {
	ctx.Response.Header.Set("Content-Type", bindcore.CBORContentType)
	ctx.SetStatusCode(status)
	_, err := ctx.Write(data)
	return err
}
