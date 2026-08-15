package httpbind

import "github.com/shibukawa/tinybind-go/internal/bindcore"

// The error model lives in bindcore so both transport runtimes share one set of
// types: an error built by one surface has to match when the other inspects it,
// and a duplicated HTTPError would silently stop matching. These are aliases,
// not wrappers, so *HTTPError is the same type on either side.

// Problem is an application error payload carried by status helpers.
type Problem = bindcore.Problem

// FieldError describes a single field-level validation failure.
type FieldError = bindcore.FieldError

// HTTPError is an HTTP-mapped error with optional RFC 9457 details and cause.
type HTTPError = bindcore.HTTPError

// Field builds a field-level validation error.
func Field(field, location, message string) FieldError {
	return bindcore.Field(field, location, message)
}

// BadRequest returns a 400 Bad Request error.
func BadRequest(problem Problem, cause ...error) error {
	return bindcore.BadRequest(problem, cause...)
}

// Unauthorized returns a 401 Unauthorized error.
func Unauthorized(problem Problem, cause ...error) error {
	return bindcore.Unauthorized(problem, cause...)
}

// Forbidden returns a 403 Forbidden error.
func Forbidden(problem Problem, cause ...error) error {
	return bindcore.Forbidden(problem, cause...)
}

// NotFound returns a 404 Not Found error.
func NotFound(problem Problem, cause ...error) error {
	return bindcore.NotFound(problem, cause...)
}

// Redirect returns a value that sends the browser to target, travelling the
// error return because a caller returning values holds no ResponseWriter.
//
// [WriteError] recognizes it and emits the status with a Location header
// instead of a problem document. The status defaults to 303; pass one of 301,
// 302, 307, or 308 to choose another.
//
// It is an ordinary error value, so a page function, a handler, and a template's
// failing external all express a redirect the same way.
func Redirect(target string, status ...int) error {
	return bindcore.Redirect(target, status...)
}

// Conflict returns a 409 Conflict error.
func Conflict(problem Problem, cause ...error) error {
	return bindcore.Conflict(problem, cause...)
}

// PayloadTooLarge returns a 413 Payload Too Large error.
func PayloadTooLarge(problem Problem, cause ...error) error {
	return bindcore.PayloadTooLarge(problem, cause...)
}

// Internal returns a 500 Internal Server Error that wraps err.
func Internal(err error) error {
	return bindcore.Internal(err)
}

// Validation returns a 400 Bad Request validation error with field details.
func Validation(fields ...FieldError) error {
	return bindcore.Validation(fields...)
}

// AsHTTPError extracts *HTTPError from err if present.
func AsHTTPError(err error) (*HTTPError, bool) {
	return bindcore.AsHTTPError(err)
}

// BindError is returned when binding fails for a specific field/source.
func BindError(field, location, message string) error {
	return bindcore.BindError(field, location, message)
}
