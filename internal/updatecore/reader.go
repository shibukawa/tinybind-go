package updatecore

import (
	"context"
	"net/url"
)

// Reader is everything the update entries read from a request.
//
// It is deliberately six methods long. Each one is here because an entry in
// this package calls it, and the list is the measured answer to what "reads a
// request and writes nothing through it" costs a second transport: a header
// lookup, the method, the query in both its raw and parsed forms, one form
// value, and the context.
//
// It is not a portable request type and no application implements it. A shell
// wraps its own transport in one of these, and the wrapper is a few lines
// because both transports already have every one of these values — what they
// disagree about is only how to spell them.
type Reader interface {
	// Header is the first value of the named request header, or "".
	Header(name string) string
	// Method is the request method, upper case, as both transports report it.
	Method() string
	// RawQuery is the undecoded query string, without the leading "?".
	//
	// It is separate from Query because a redraw bounds the arguments before
	// parsing them: parsing to measure would do the work the bound exists to
	// refuse.
	RawQuery() string
	// Query is the parsed query string.
	Query() url.Values
	// FormValue is the named value of a urlencoded or multipart request body.
	//
	// Only the CSRF entries read it, and only after the header channel came up
	// empty, so an ordinary fetch never pays for a body it does not have.
	FormValue(name string) string
	// Context is the request's context: its cancellation, its deadline, and
	// whatever the caller's middleware put there.
	Context() context.Context
}
