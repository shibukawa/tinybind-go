package fasthttpbind

import (
	"encoding/json"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Write serializes a typed response value via a registered writer.
// Status is always 200 OK; use WriteStatus for other success codes.
//
// There is no separate request parameter: RequestCtx carries both halves, and
// the net/http signature only takes r to reach negotiation it then discards.
func Write[T any](ctx *fasthttp.RequestCtx, value T) error {
	fn, ok := lookupWriter[T]()
	if !ok {
		return missingWriterError()
	}
	return fn(ctx, value)
}

// WriteStatus serializes value with an explicit HTTP status code using the
// registered encoder for T (no field-walking reflection on T).
// For status 204 No Content, the body is not written.
func WriteStatus[T any](ctx *fasthttp.RequestCtx, status int, value T) error {
	if status == 204 {
		ctx.SetStatusCode(204)
		return nil
	}
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(status)
	return jsonbind.EncodeJSON(ctx, value)
}

// WriteError writes err as an RFC 9457 Problem Details response.
// Internal causes are not exposed in the client body.
//
// The document is derived by the same shared code the net/http runtime calls,
// so both transports emit identical bytes for identical errors.
func WriteError(ctx *fasthttp.RequestCtx, err error) {
	status, body, ok := bindcore.ProblemResponse(err)
	if !ok {
		return
	}
	ctx.Response.Header.Set("Content-Type", bindcore.ProblemContentType)
	ctx.SetStatusCode(status)
	_, _ = ctx.Write(body)
}

// WriteJSON is a helper for generated writers: encode a pre-built map/slice
// without reflecting over application structs.
func WriteJSON(ctx *fasthttp.RequestCtx, status int, v any) error {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(status)
	return json.NewEncoder(ctx).Encode(v)
}

// WriteJSONBytes writes an already-encoded document. Generated writers build
// the body into a pooled buffer and hand it over here, so the response path
// never reflects over the value and never allocates an intermediate map.
func WriteJSONBytes(ctx *fasthttp.RequestCtx, status int, data []byte) error {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(status)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		if _, err := ctx.Write(data); err != nil {
			return err
		}
		_, err := ctx.Write(newline)
		return err
	}
	_, err := ctx.Write(data)
	return err
}

var newline = []byte("\n")

// AppendFileJSON appends an uploaded file the way encoding/json rendered it
// before generated encoders stopped going through reflection.
func AppendFileJSON(dst []byte, f File) []byte {
	return bindcore.AppendFileJSON(dst, f)
}
