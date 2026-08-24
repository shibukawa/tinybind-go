package jsonbind

import "errors"

// Error describes a transport-neutral JSON mapping failure.
type Error struct {
	Code    string
	Message string
	Field   string
	cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func newError(code, message string, cause error) error {
	return &Error{Code: code, Message: message, cause: cause}
}

// FieldError annotates a JSON decoding error with its document field.
func FieldError(field, message string, cause error) error {
	return &Error{Code: "json_field", Message: message, Field: field, cause: cause}
}

// AsError finds a JSON Error without using reflection-dependent errors.As.
func AsError(err error) (*Error, bool) {
	for err != nil {
		if je, ok := err.(*Error); ok {
			return je, true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil, false
		}
		err = u.Unwrap()
	}
	return nil, false
}

// ErrArrayTooLong reports a JSON array carrying more elements than the
// fixed-length Go array it decodes into can hold.
//
// It is the cause of the [Error] ParseArray returns, so a caller that wants to
// tell a too-long array from a malformed one asks errors.Is rather than
// matching the message.
var ErrArrayTooLong = errors.New("jsonbind: too many array elements")
