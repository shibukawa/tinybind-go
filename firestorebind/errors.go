package firestorebind

import "github.com/shibukawa/tinygodriver/nosql/datastore"

// Error describes an entity mapping failure. Property names the property that
// failed, which is the Datastore name rather than the Go field name, because
// that is the name the stored data uses.
type Error struct {
	// Property is the property that failed, or "" for a whole-entity failure.
	Property string
	// Expected and Got name value kinds, such as "string" or "integer".
	Expected string
	Got      string
	Message  string
	cause    error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	out := e.Message
	if e.Property != "" {
		out = "property " + e.Property + ": " + out
	}
	return "firestorebind: " + out
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// TypeError reports a property whose stored kind is not the one the field needs.
// Generated decoders call it.
//
// An integer stored where a float is expected is a type error rather than a
// conversion: Datastore orders and compares integerValue and doubleValue
// separately, so coercing one to the other would produce a value the query that
// found it can no longer find.
func TypeError(property, expected string, got datastore.Value) error {
	name := got.Kind().String()
	return &Error{
		Property: property,
		Expected: expected,
		Got:      name,
		Message:  "expected " + expected + ", got " + name,
	}
}

// ValueError reports a property whose kind is right but whose value cannot be
// represented by the field, such as an integer too large for the Go type.
// Generated decoders call it.
func ValueError(property, message string, cause error) error {
	return &Error{Property: property, Message: message, cause: cause}
}

// KeyError reports a key that cannot be used for the operation, such as an
// incomplete key passed to a read.
func KeyError(message string) error {
	return &Error{Message: message}
}

// AsError finds a firestorebind Error in a chain without errors.As, which needs
// reflection. Whether reflect is linked at all is the driver's business, not
// this package's.
func AsError(err error) (*Error, bool) {
	for err != nil {
		if fe, ok := err.(*Error); ok {
			return fe, true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil, false
		}
		err = u.Unwrap()
	}
	return nil, false
}
