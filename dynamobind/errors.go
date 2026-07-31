package dynamobind

import "github.com/shibukawa/tinygodriver/nosql/dynamodb"

// Error describes an item mapping failure. Attribute names the attribute that
// failed, which is the DynamoDB name rather than the Go field name, because
// that is the name the stored data uses.
type Error struct {
	// Attribute is the attribute that failed, or "" for a whole-item failure.
	Attribute string
	// Expected and Got name attribute kinds, such as "S" or "N".
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
	if e.Attribute != "" {
		out = "attribute " + e.Attribute + ": " + out
	}
	return "dynamobind: " + out
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// TypeError reports an attribute whose stored kind is not the one the field
// needs. Generated decoders call it.
func TypeError(attribute, expected string, got dynamodb.AttributeValue) error {
	name := kindName(got.Kind())
	return &Error{
		Attribute: attribute,
		Expected:  expected,
		Got:       name,
		Message:   "expected " + expected + ", got " + name,
	}
}

// ValueError reports an attribute whose kind is right but whose value cannot be
// represented by the field, such as a number too large for the Go type.
// Generated decoders call it.
func ValueError(attribute, message string, cause error) error {
	return &Error{Attribute: attribute, Message: message, cause: cause}
}

// AsError finds a dynamobind Error in a chain without errors.As, which needs
// reflection. Whether reflect is linked at all is the driver's business, not
// this package's.
func AsError(err error) (*Error, bool) {
	for err != nil {
		if de, ok := err.(*Error); ok {
			return de, true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil, false
		}
		err = u.Unwrap()
	}
	return nil, false
}

func kindName(k dynamodb.Kind) string {
	switch k {
	case dynamodb.KindString:
		return "S"
	case dynamodb.KindNumber:
		return "N"
	case dynamodb.KindBinary:
		return "B"
	case dynamodb.KindBool:
		return "BOOL"
	case dynamodb.KindNull:
		return "NULL"
	case dynamodb.KindList:
		return "L"
	case dynamodb.KindMap:
		return "M"
	case dynamodb.KindStringSet:
		return "SS"
	case dynamodb.KindNumberSet:
		return "NS"
	case dynamodb.KindBinarySet:
		return "BS"
	default:
		return "none"
	}
}
