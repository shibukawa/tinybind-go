// Package raw is the rung 3 route: a page whose Load owns the whole response.
//
// On net/http that signature is func(http.ResponseWriter, *http.Request); here
// it is one value carrying both, which is the whole reason the recognizer takes
// the shape as configuration.
package raw

import (
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Load owns its response, so the registry generates registration and nothing
// else.
func Load(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusTeapot)
	_, _ = ctx.Write([]byte("raw"))
}
