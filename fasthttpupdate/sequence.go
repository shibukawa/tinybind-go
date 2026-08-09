package fasthttpupdate

import "github.com/shibukawa/tinygodriver/fasthttp"

// Sequence answers a request for one sequence tree, and reports whether it did.
//
// It is the entry a caller branches on inside its own handler, exactly as Redraw
// is:
//
//	func page(ctx *fasthttp.RequestCtx) {
//		if answer, ok := options.Sequence(ctx); ok {
//			ctx.Response.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
//			_, _ = answer.WriteTo(ctx)
//			return
//		}
//		// ordinary page render
//	}
//
// The cache policy is the caller's, and a sequence is the one answer here that
// may be public and held forever: it is addressed by a digest of its own
// content, so a template edit produces a new address rather than a new body at
// the old one, and nothing needs invalidating.
func (o Options) Sequence(ctx *fasthttp.RequestCtx) (Response, bool) {
	resp, answered := o.core().Sequence(reader(ctx))
	return fromCore(resp), answered
}
