package updatecore

import (
	"crypto/subtle"
	"errors"
)

// A CSRF token reaches this package by two channels, because a browser has two.
// The runtime puts it in a header on everything it fetches; a form carries it in
// the hidden field htmlbind generated, because a form cannot set a header and
// has to submit with scripting disabled.
//
// What is here is the reading half only. Creating the token, storing it in a
// session, and destroying it at logout are the caller's: this package has no
// session and would be claiming the largest thing it has so far declined.

// ErrCSRFMissing reports an unsafe request carrying no token at all.
var ErrCSRFMissing = errors.New("htmlupdate: request carries no CSRF token")

// ErrCSRFMismatch reports a token that is not the session's.
var ErrCSRFMismatch = errors.New("htmlupdate: CSRF token does not match the session")

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
func (o Options) CSRFToken(r Reader) string {
	if token := r.Header(o.CSRFHeader()); token != "" {
		return token
	}
	return r.FormValue(o.CSRFField())
}

// DefaultCSRFFieldName is the hidden field generated forms carry. It matches the
// generator's own default, because the two have to agree: one writes the field
// and the other reads it.
const DefaultCSRFFieldName = "_csrf"

func (o Options) CSRFField() string {
	if o.CSRFFieldName == "" {
		return DefaultCSRFFieldName
	}
	return o.CSRFFieldName
}

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
func (o Options) VerifyCSRF(r Reader, expected string) error {
	if expected == "" {
		return ErrCSRFMissing
	}
	token := o.CSRFToken(r)
	if token == "" {
		return ErrCSRFMissing
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return ErrCSRFMismatch
	}
	return nil
}
