package fasthttpupdate

import (
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Redraw answers a request for one registered component, and reports whether it
// did.
//
// It is the entry a caller branches on inside its own handler, so the address a
// client redraws at is the caller's to choose and this package mounts nothing.
//
// The registry and the components in it are the same values on either
// transport: Reloadable.Render takes a context rather than a request, so one
// registration built by generation serves both.
func (o Options) Redraw(ctx *fasthttp.RequestCtx, reg *Registry, options ...htmlbind.Option) (Response, bool) {
	resp, answered := o.core().Redraw(reader(ctx), reg, options...)
	return fromCore(resp), answered
}
