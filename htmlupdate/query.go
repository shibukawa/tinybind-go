package htmlupdate

import (
	"net/url"
	"time"

	"github.com/shibukawa/tinybind-go/internal/updatecore"
)

// The typed query decoders a generated redraw registration calls. Each refuses a
// missing, repeated, or undecodable value rather than yielding a zero one,
// because a redraw's arguments come from the client.

// QueryString decodes one string-kinded value.
func QueryString[T ~string](values url.Values, name string, target *T) error {
	return updatecore.QueryString(values, name, target)
}

// QueryBool decodes one boolean value.
func QueryBool(values url.Values, name string, target *bool) error {
	return updatecore.QueryBool(values, name, target)
}

// QueryInt decodes one integer value.
func QueryInt(values url.Values, name string, target *int) error {
	return updatecore.QueryInt(values, name, target)
}

// QueryFloat decodes one floating-point value.
func QueryFloat(values url.Values, name string, target *float64) error {
	return updatecore.QueryFloat(values, name, target)
}

// QueryURL decodes one URL value.
func QueryURL(values url.Values, name string, target *url.URL) error {
	return updatecore.QueryURL(values, name, target)
}

// QueryTime decodes one date, time, or datetime value.
func QueryTime(values url.Values, name string, target *time.Time) error {
	return updatecore.QueryTime(values, name, target)
}

// QueryOptional decodes a value that may be absent, leaving the target nil when
// it is.
func QueryOptional[T any](values url.Values, name string, target **T) error {
	return updatecore.QueryOptional(values, name, target)
}
