package httpbind

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// ReadActionBody reads the JSON payload of a typed server action call, under
// the configured body limit.
//
// A generated wrapper calls this rather than reading the body itself, so the
// limit and the error mapping live in one place instead of being written into
// every emitted entry point.
func ReadActionBody(r *http.Request) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, nil
	}
	// The Content-Length is passed as the hint so a body of known size lands in
	// one allocation, which is what that parameter exists for and what an
	// action call, being a small JSON document, always has.
	data, err := jsonbind.ReadLimitHint(r.Body, jsonbind.MaxJSONBodyBytes(), r.ContentLength)
	if err != nil {
		if err == jsonbind.ErrBodyTooLarge {
			return nil, PayloadTooLarge(Problem{Code: "payload_too_large", Message: "request body too large"}, err)
		}
		return nil, BadRequest(Problem{Code: "body_read", Message: "cannot read request body"}, err)
	}
	return data, nil
}

// Declaration is what [ServerAction] returns. It carries nothing: the value
// exists only so the annotation can be written as a package-level declaration,
// which is where generation reads it.
type Declaration struct{}

// ServerAction declares that fn is a server action reachable from client
// script, whatever its signature.
//
// A handler-shaped function is an action by existing, because that shape is
// unambiguous: an exported function taking the transport types and returning
// nothing is nothing else. An arbitrary signature distinguishes nothing, since
// every function has one, so something outside the signature has to say which
// functions are actions. That is what this declaration is for, and it is the
// only thing it does.
//
// Write it at package level, beside the function:
//
//	var _ = httpbind.ServerAction(GetUser)
//
// fn is taken as a symbol rather than as a name string, so a declaration naming
// something that does not exist fails to compile before generation reads it.
//
// The optional name is the identifier client script calls through. Without one
// it is derived from the Go name in initialism-aware lowerCamelCase, so GetUser
// is reached as getUser and URLFor as urlFor. Supply one for a name the
// derivation reads wrong, or for a published name a Go rename must not move:
// like a struct tag, it is a wire name rather than a second identity.
//
// The call runs at init and does nothing. The declaration is the point.
func ServerAction(fn any, name ...string) Declaration {
	_, _ = fn, name
	return Declaration{}
}
