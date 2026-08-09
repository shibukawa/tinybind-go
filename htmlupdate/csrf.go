package htmlupdate

import "net/http"

// CSRFToken reads the token a request carries, header first and form body
// second.
//
// The order is not arbitrary. The header is what the runtime sends and the only
// channel a non-form body has; the field is the fallback for a submission made
// without script. Reading the header first also means an ordinary fetch never
// pays for parsing a body it does not have.
//
// Reading the field consumes the request body through ParseForm, as any handler
// reading a form does.
func (o Options) CSRFToken(r *http.Request) string { return o.core().CSRFToken(reader(r)) }

// VerifyCSRF compares what a request carries against the session's token.
//
// expected is the caller's to produce, from wherever its session lives. An empty
// expected is refused rather than treated as "nothing to check": a session
// lookup that quietly returned nothing would otherwise disable the whole control
// for exactly the requests that most need it.
//
// This is a token check and nothing else. Origin and Fetch Metadata validation
// belong to middleware that sees the request before any of this — Go's own
// http.CrossOriginProtection is what wraps a handler with them — and they are
// worth having: the two defenses fail for unrelated reasons, which is the point
// of running both.
func (o Options) VerifyCSRF(r *http.Request, expected string) error {
	return o.core().VerifyCSRF(reader(r), expected)
}
