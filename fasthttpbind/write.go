package fasthttpbind

import (
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
		// fasthttp fills in a default Content-Type for a response that names
		// none, and net/http sends none at all for a bodyless 204. The header
		// set is part of what requirement:fasthttpbind-parity-scope compares,
		// so the default is declined rather than left to differ.
		ctx.Response.Header.SetNoDefaultContentType(true)
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
	// A redirect travels the error return, so it is recognized here rather than
	// needing a second channel on every page function. It emits a Location and
	// no problem document, because the browser is being sent somewhere rather
	// than told what went wrong.
	if target, status, ok := bindcore.RedirectTarget(err); ok {
		// No body, so no content type. This transport supplies a default one
		// for any status set without it, which would put text/plain on a
		// response that carries nothing — and make the two surfaces disagree
		// on a header only one of them writes.
		ctx.Response.Header.SetNoDefaultContentType(true)
		ctx.Response.Header.Set("Location", target)
		ctx.SetStatusCode(status)
		return
	}
	status, body, ok := bindcore.ProblemResponse(err)
	if !ok {
		return
	}
	ctx.Response.Header.Set("Content-Type", bindcore.ProblemContentType)
	ctx.SetStatusCode(status)
	_, _ = ctx.Write(body)
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
