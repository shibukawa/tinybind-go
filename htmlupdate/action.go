package htmlupdate

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// WantsUpdate reports whether this request can be answered with the regions an
// action changed, rather than with a redirect.
//
// An ordinary form submission cannot, so a handler branches on this and
// redirects instead, which is what keeps a page working without JavaScript.
func (o Options) WantsUpdate(r *http.Request) bool { return o.core().WantsUpdate(reader(r)) }

// WriteUpdate answers a mutating request with the regions it changed, so one
// round trip both performs the action and refreshes the page.
//
// The body is the same shape a redraw returns, so the browser applies it with
// the same code. Unlike a redraw this request is not idempotent: it carries
// ambient credentials, so it needs CSRF protection, and its response is never
// cacheable.
//
// options reach every region's render. The token one matters most here: a region
// holding an unsafe form emits a CSRF field, and without a token that render
// fails outright — which is this entry's own headline case, since rewriting a
// form with its validation errors is what it exists for.
func (o Options) WriteUpdate(r *http.Request, updates []Update, options ...htmlbind.Option) (Response, error) {
	resp, err := o.core().WriteUpdate(reader(r), updates, options...)
	return Response(resp), err
}

// WriteUpdateStatus is WriteUpdate with an explicit status, so a failed
// validation can return 422 and still rewrite the form region with its errors.
//
// The browser applies an update response whatever the status says, because
// rendering the failure is the point.
func (o Options) WriteUpdateStatus(r *http.Request, status int, updates []Update, options ...htmlbind.Option) (Response, error) {
	resp, err := o.core().WriteUpdateStatus(reader(r), status, updates, options...)
	return Response(resp), err
}

// WriteNavigate tells the browser to leave the page, which is how an action
// that changed where the user belongs stays correct without guessing which
// regions to rewrite.
func (o Options) WriteNavigate(url string) (Response, error) {
	resp, err := o.core().WriteNavigate(url)
	return Response(resp), err
}
