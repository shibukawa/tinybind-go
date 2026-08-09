package fasthttpupdate

import "github.com/shibukawa/tinygodriver/fasthttp"

// Redirect sends the browser somewhere else, for the branch WantsUpdate exists
// to create. It is the same name and the same meaning as htmlupdate's, over the
// transport that spells a redirect as a method rather than a function.
//
// The status is passed through rather than defaulted, so a handler choosing 303
// after a POST — which is what a form submission wants — gets it on either
// backend.
func Redirect(ctx *fasthttp.RequestCtx, url string, status int) {
	if ctx == nil {
		return
	}
	ctx.Redirect(url, status)
}
