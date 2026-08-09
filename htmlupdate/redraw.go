package htmlupdate

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Redraw answers a request for one registered component, and reports whether it
// did.
//
// It is the entry a caller branches on inside its own handler, so the address a
// client redraws at is the caller's to choose and this package mounts nothing.
// Usually that is the page the component sits on, where the redraw inherits the
// page's own authorization rather than needing a second path pattern kept in
// step with the one protecting the page.
//
// options reach the component's render, which is what a caller uses when a
// component renders one way inside its page and another in the response that
// replaces it — and one containing an unsafe form does not render at all, since
// [htmlbind.Builder.CSRFField] needs a token. The boundary prefix and the build
// identity are supplied from these Options and do not need passing.
func (o Options) Redraw(r *http.Request, reg *Registry, options ...htmlbind.Option) (Response, bool) {
	resp, answered := o.core().Redraw(reader(r), reg, options...)
	return Response(resp), answered
}
