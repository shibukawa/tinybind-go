package fasthttpupdate

import (
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// WantsUpdate reports whether this request can be answered with the regions an
// action changed, rather than with a redirect. See htmlupdate for the whole
// rule; this is the same entry over the other transport.
func (o Options) WantsUpdate(ctx *fasthttp.RequestCtx) bool {
	return o.core().WantsUpdate(reader(ctx))
}

// WriteUpdate answers a mutating request with the regions it changed, so one
// round trip both performs the action and refreshes the page.
func (o Options) WriteUpdate(ctx *fasthttp.RequestCtx, updates []Update, options ...htmlbind.Option) (Response, error) {
	resp, err := o.core().WriteUpdate(reader(ctx), updates, options...)
	return fromCore(resp), err
}

// WriteUpdateStatus is WriteUpdate with an explicit status, so a failed
// validation can return 422 and still rewrite the form region with its errors.
func (o Options) WriteUpdateStatus(ctx *fasthttp.RequestCtx, status int, updates []Update, options ...htmlbind.Option) (Response, error) {
	resp, err := o.core().WriteUpdateStatus(reader(ctx), status, updates, options...)
	return fromCore(resp), err
}

// WriteNavigate tells the browser to leave the page, which is how an action
// that changed where the user belongs stays correct without guessing which
// regions to rewrite.
func (o Options) WriteNavigate(url string) (Response, error) {
	resp, err := o.core().WriteNavigate(url)
	return fromCore(resp), err
}
