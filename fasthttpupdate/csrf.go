package fasthttpupdate

import "github.com/shibukawa/tinygodriver/fasthttp"

// CSRFToken reads the token a request carries, header first and form body
// second.
//
// The body channel is the post arguments and the multipart form, never the
// query, which is what net/http's PostFormValue means and what keeps a token
// out of access logs and referrers.
func (o Options) CSRFToken(ctx *fasthttp.RequestCtx) string {
	return o.core().CSRFToken(reader(ctx))
}

// VerifyCSRF compares what a request carries against the session's token.
//
// expected is the caller's to produce, from wherever its session lives. An empty
// expected is refused rather than treated as "nothing to check": a session
// lookup that quietly returned nothing would otherwise disable the whole control
// for exactly the requests that most need it.
func (o Options) VerifyCSRF(ctx *fasthttp.RequestCtx, expected string) error {
	return o.core().VerifyCSRF(reader(ctx), expected)
}
