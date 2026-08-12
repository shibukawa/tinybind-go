package fasthttpbind

import "github.com/shibukawa/tinygodriver/fasthttp"

// DefaultActionSelectorField is the hidden field a generated form carries to say
// which server function a native submit is for. It matches the net/http half, so
// one generated form works on either backend.
const DefaultActionSelectorField = "_action"

// ActionSelector returns the server function selector a native form submit
// carried, or the empty string when it carried none.
//
// The query is read before the body for the reason the net/http half gives: a
// submit button's formaction carries the selector when one form dispatches to
// several handlers, and that channel has to win rather than merely coexist.
func ActionSelector(ctx *fasthttp.RequestCtx, field string) string {
	if ctx == nil {
		return ""
	}
	if field == "" {
		field = DefaultActionSelectorField
	}
	if value := ctx.QueryArgs().Peek(field); len(value) > 0 {
		return string(value)
	}
	return string(ctx.PostArgs().Peek(field))
}

// DispatchAction runs one server function on the page's own POST route and
// applies the post-redirect-get default, matching the net/http half.
//
// A handler that writes nothing gets a 303 back to the page it was submitted
// from; one that writes a status, a header, or a body keeps that response.
//
// The observation is made by comparing the response before and after rather than
// by wrapping the writer, because fasthttp carries the request and the response
// in one value and there is nothing to wrap.
func DispatchAction(ctx *fasthttp.RequestCtx, handler func(*fasthttp.RequestCtx)) {
	if ctx == nil {
		return
	}
	headers := ctx.Response.Header.Len()
	handler(ctx)
	// fasthttp starts a response at 200, so a status still reading 200 is one
	// the handler did not set rather than one it chose.
	if ctx.Response.StatusCode() != fasthttp.StatusOK ||
		len(ctx.Response.Body()) > 0 ||
		ctx.Response.Header.Len() != headers {
		return
	}
	ctx.Redirect(string(ctx.RequestURI()), fasthttp.StatusSeeOther)
}
