package htmlupdate

import (
	"net/url"
	"strconv"
	"time"
)

// The decoders below are what generated redraw code calls. A redraw carries
// every argument in the query string, and those arguments come from the caller,
// so a value that does not parse is an error rather than a zero: silently
// substituting one would let a malformed request render as though it were
// valid.

// QueryError is a redraw parameter the decoder refused, naming which one.
//
// The name is what makes a refusal answerable: a failure response reports the
// parameter as a field-level error rather than one line of prose a caller would
// have to parse. The reason never quotes the value, because the value is
// attacker-supplied and the response is not the place to reflect it.
type QueryError struct {
	// Parameter is the declared name the request got wrong.
	Parameter string
	// Reason completes the sentence "redraw parameter <name> …", so it reads
	// the same in a log line and in a problem response.
	Reason string
}

func (e *QueryError) Error() string {
	return "redraw parameter " + e.Parameter + " " + e.Reason
}

func queryError(name, reason string) error {
	return &QueryError{Parameter: name, Reason: reason}
}

// QueryString decodes any string-kinded parameter, covering plain strings,
// decimals, and generated enums.
func QueryString[T ~string](values url.Values, name string, target *T) error {
	raw, err := requireOne(values, name)
	if err != nil {
		return err
	}
	*target = T(raw)
	return nil
}

// QueryBool decodes a bool parameter.
func QueryBool(values url.Values, name string, target *bool) error {
	raw, err := requireOne(values, name)
	if err != nil {
		return err
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return queryError(name, "is not a bool")
	}
	*target = parsed
	return nil
}

// QueryInt decodes an int parameter.
func QueryInt(values url.Values, name string, target *int) error {
	raw, err := requireOne(values, name)
	if err != nil {
		return err
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return queryError(name, "is not an integer")
	}
	*target = parsed
	return nil
}

// QueryFloat decodes a float parameter.
func QueryFloat(values url.Values, name string, target *float64) error {
	raw, err := requireOne(values, name)
	if err != nil {
		return err
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return queryError(name, "is not a number")
	}
	*target = parsed
	return nil
}

// QueryURL decodes a URL parameter.
func QueryURL(values url.Values, name string, target *url.URL) error {
	raw, err := requireOne(values, name)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return queryError(name, "is not a URL")
	}
	*target = *parsed
	return nil
}

// QueryTime decodes an instant, date, or time parameter in RFC 3339 form, which
// is the form CanonTime writes and the one a query string can carry unambiguously.
func QueryTime(values url.Values, name string, target *time.Time) error {
	raw, err := requireOne(values, name)
	if err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return queryError(name, "is not an RFC 3339 timestamp")
	}
	*target = parsed
	return nil
}

// QueryOptional decodes a parameter the template declared optional. An absent
// name is the absent value; a present but undecodable one is still an error.
func QueryOptional[T any](values url.Values, name string, target **T) error {
	if !values.Has(name) {
		*target = nil
		return nil
	}
	var decoded T
	if err := decodeOne(values, name, &decoded); err != nil {
		return err
	}
	*target = &decoded
	return nil
}

// decodeOne dispatches an optional parameter to the decoder for its type.
// Generated code names the concrete type, so this switch covers exactly the
// types checkReloadable admits.
func decodeOne[T any](values url.Values, name string, target *T) error {
	switch typed := any(target).(type) {
	case *string:
		return QueryString(values, name, typed)
	case *bool:
		return QueryBool(values, name, typed)
	case *int:
		return QueryInt(values, name, typed)
	case *float64:
		return QueryFloat(values, name, typed)
	case *url.URL:
		return QueryURL(values, name, typed)
	case *time.Time:
		return QueryTime(values, name, typed)
	default:
		return queryError(name, "has no decoder")
	}
}

// requireOne rejects a missing parameter and a repeated one alike. A repeat is
// ambiguous, and picking the first would let a caller smuggle a second value
// past a check that read only one.
func requireOne(values url.Values, name string) (string, error) {
	found, ok := values[name]
	if !ok || len(found) == 0 {
		return "", queryError(name, "is missing")
	}
	if len(found) > 1 {
		return "", queryError(name, "appears more than once")
	}
	return found[0], nil
}
