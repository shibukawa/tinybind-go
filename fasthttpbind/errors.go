package fasthttpbind

import "github.com/shibukawa/tinybind-go/internal/bindcore"

// The error model is shared with the net/http runtime rather than duplicated,
// so an *HTTPError built on one surface still matches when the other inspects
// it. These are aliases; the types are the same types.

// Problem is an application error payload carried by status helpers.
type Problem = bindcore.Problem

// FieldError describes a single field-level validation failure.
type FieldError = bindcore.FieldError

// HTTPError is an HTTP-mapped error with optional RFC 9457 details and cause.
type HTTPError = bindcore.HTTPError

// File is an uploaded file bound from a multipart/form-data part.
type File = bindcore.File

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

// Conflict returns a 409 Conflict error.
func Conflict(problem Problem, cause ...error) error {
	return bindcore.Conflict(problem, cause...)
}

// PayloadTooLarge returns a 413 Payload Too Large error.
func PayloadTooLarge(problem Problem, cause ...error) error {
	return bindcore.PayloadTooLarge(problem, cause...)
}

// Internal returns a 500 Internal Server Error that wraps err.
func Internal(err error) error { return bindcore.Internal(err) }

// Validation returns a 400 Bad Request validation error with field details.
func Validation(fields ...FieldError) error { return bindcore.Validation(fields...) }

// AsHTTPError extracts *HTTPError from err if present.
func AsHTTPError(err error) (*HTTPError, bool) { return bindcore.AsHTTPError(err) }

// BindError is returned when binding fails for a specific field/source.
func BindError(field, location, message string) error {
	return bindcore.BindError(field, location, message)
}

// CheckEmail reports whether s is a pragmatic (non-RFC5322) email.
func CheckEmail(s string) bool { return bindcore.CheckEmail(s) }

// CheckUUID reports whether s is a UUID string (8-4-4-4-12 hex with dashes).
func CheckUUID(s string) bool { return bindcore.CheckUUID(s) }

// CheckDate reports whether s is an ISO date (YYYY-MM-DD / time.DateOnly).
func CheckDate(s string) bool { return bindcore.CheckDate(s) }

// CheckTime reports whether s is an ISO time (HH:MM:SS / time.TimeOnly).
func CheckTime(s string) bool { return bindcore.CheckTime(s) }

// CheckDateTime reports whether s is RFC3339 (fractional seconds accepted).
func CheckDateTime(s string) bool { return bindcore.CheckDateTime(s) }
