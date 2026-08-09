package fasthttpbind

import (
	"bufio"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// StreamFormat is the negotiated on-the-wire format for Stream[T].
type StreamFormat = bindcore.StreamFormat

const (
	// StreamSSE is text/event-stream (data: <json>\n\n).
	StreamSSE = bindcore.StreamSSE
	// StreamNDJSON is application/x-ndjson (one JSON object per line).
	StreamNDJSON = bindcore.StreamNDJSON
	// StreamJSONArray is application/json as one JSON array document.
	StreamJSONArray = bindcore.StreamJSONArray
)

// Stream is a typed incremental response stream. It is the same type the
// net/http runtime uses, so the events produce the same bytes here.
type Stream[T any] = bindcore.Stream[T]

// WriteStream opens a negotiated stream, runs fn against it, and closes it.
//
// The headers and status go out while the handler still owns the context; fn
// itself runs from the body stream writer, after the handler has returned.
// That inversion is why the entry point returns nothing: an error raised in fn
// has no way back to handler code, on this transport or the other, so both
// route it to the handler installed with SetStreamErrorHandler.
//
// Because fn outlives the handler, it must not read ctx. Everything the stream
// needs is captured before WriteStream returns.
func WriteStream[T any](ctx *fasthttp.RequestCtx, fn func(*Stream[T]) error) {
	if ctx == nil {
		return
	}
	format := NegotiateStreamFormat(ctx)
	for _, h := range bindcore.StreamHeaders(format) {
		ctx.Response.Header.Set(h.Name, h.Value)
	}
	ctx.SetStatusCode(200)
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		s := bindcore.NewStream[T](w, format)
		ferr := fn(s)
		if cerr := s.Close(); ferr == nil {
			ferr = cerr
		}
		if werr := w.Flush(); ferr == nil {
			ferr = werr
		}
		if ferr != nil {
			bindcore.ReportStreamError(ferr)
		}
	})
}

// SetStreamErrorHandler installs the destination for stream failures raised
// after the response status has been sent. It is shared with the net/http
// runtime, so installing it once covers both.
func SetStreamErrorHandler(fn func(error)) { bindcore.SetStreamErrorHandler(fn) }

// NegotiateStreamFormat selects SSE, NDJSON, or JSON array using the ?stream=
// query value, then Accept, then User-Agent, defaulting to NDJSON.
func NegotiateStreamFormat(ctx *fasthttp.RequestCtx) StreamFormat {
	if ctx == nil {
		return StreamNDJSON
	}
	return bindcore.NegotiateStream(
		string(ctx.QueryArgs().Peek("stream")),
		string(ctx.Request.Header.Peek("Accept")),
		string(ctx.Request.Header.Peek("User-Agent")),
	)
}
