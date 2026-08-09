package fasthttpupdate

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/internal/updatecore"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// This package writes bytes and nothing else, exactly as htmlupdate does. What
// it computes and hands over — the Vary axes, the content type, the served mode,
// the entity tag — is the same set on either transport, because it is derived
// from the request and the body rather than from the writer.

// WriteTo sends the response: its headers, its status, and its body.
//
// It is here so the common case is one call and an omission is a decision. A
// caller wanting its own policy sets it on ctx before calling, or copies Header
// itself and writes Body.
func (resp Response) WriteTo(ctx *fasthttp.RequestCtx) (int64, error) {
	if ctx == nil {
		return 0, nil
	}
	ApplyTo(resp.Header, ctx)
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	ctx.SetStatusCode(status)
	written, err := ctx.Write(resp.Body)
	return int64(written), err
}

// ApplyTo copies a computed header set onto a response, adding rather than
// replacing so a caller's own values survive.
//
// Content-Type is set rather than added: fasthttp keeps it as a dedicated field
// with a default already in it, so adding would leave the default beside the
// computed value and let a proxy pick either.
func ApplyTo(header http.Header, ctx *fasthttp.RequestCtx) {
	if ctx == nil {
		return
	}
	for name, values := range header {
		for _, value := range values {
			if http.CanonicalHeaderKey(name) == "Content-Type" {
				ctx.Response.Header.SetContentType(value)
				continue
			}
			ctx.Response.Header.Add(name, value)
		}
	}
}

// NotModified reports whether the request already holds this answer, by
// comparing its If-None-Match against the entity tag this response carries.
//
// Answering it is the caller's: a 304 is a cache policy decision, and this
// package makes none. A response with no entity tag is never a match.
func (resp Response) NotModified(ctx *fasthttp.RequestCtx) bool {
	return updatecore.Response(resp).NotModified(reader(ctx))
}

// Headers is what a response to this request must carry, for the entries that
// write their body directly and therefore need their headers set first.
func (o Options) Headers(ctx *fasthttp.RequestCtx, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.core().Headers(reader(ctx), wrappers, leaf)
}

// RedrawHeaders names the Vary axes a URL that answers redraws depends on,
// whichever way this request turns out.
//
// A page handler declares them before it branches: a page and the redraws of the
// components on it share one URL, so a cache that learned only the page would
// answer a redraw from it.
func (o Options) RedrawHeaders(ctx *fasthttp.RequestCtx) http.Header {
	return o.core().RedrawHeaders(reader(ctx))
}

// StreamHeaders is Headers for a streamed navigation, whose body is a record
// stream rather than one of the buffered shapes.
//
// The entry that writes such a stream is not in this package yet; the headers
// are, because a caller driving SetBodyStreamWriter itself needs exactly these
// and nothing about computing them depends on holding a response open.
func (o Options) StreamHeaders(ctx *fasthttp.RequestCtx, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.core().StreamHeaders(reader(ctx), wrappers, leaf)
}

// LiveHeaders is StreamHeaders for an entry that does hold subscriptions open,
// so a live request keeps the live mode rather than being downgraded.
func (o Options) LiveHeaders(ctx *fasthttp.RequestCtx, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.core().LiveHeaders(reader(ctx), wrappers, leaf)
}
