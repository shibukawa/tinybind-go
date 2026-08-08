package httpbind

import (
	"net/http"
)

// Problem is an application error payload carried by status helpers.
type Problem struct {
	Code    string
	Message string
}

// FieldError describes a single field-level validation failure.
type FieldError struct {
	Field    string
	Location string
	Message  string
}

// Field builds a field-level validation error.
func Field(field, location, message string) FieldError {
	return FieldError{
		Field:    field,
		Location: location,
		Message:  message,
	}
}

// HTTPError is an HTTP-mapped error with optional RFC 9457 details and cause.
type HTTPError struct {
	Status  int
	Title   string
	Problem Problem
	Fields  []FieldError
	cause   error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Problem.Message != "" {
		return e.Problem.Message
	}
	if e.Title != "" {
		return e.Title
	}
	return http.StatusText(e.Status)
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func firstCause(cause []error) error {
	if len(cause) == 0 {
		return nil
	}
	return cause[0]
}

func statusError(status int, title string, problem Problem, cause ...error) error {
	return &HTTPError{
		Status:  status,
		Title:   title,
		Problem: problem,
		cause:   firstCause(cause),
	}
}

// BadRequest returns a 400 Bad Request error.
func BadRequest(problem Problem, cause ...error) error {
	return statusError(http.StatusBadRequest, "Bad Request", problem, cause...)
}

// Unauthorized returns a 401 Unauthorized error.
func Unauthorized(problem Problem, cause ...error) error {
	return statusError(http.StatusUnauthorized, "Unauthorized", problem, cause...)
}

// Forbidden returns a 403 Forbidden error.
func Forbidden(problem Problem, cause ...error) error {
	return statusError(http.StatusForbidden, "Forbidden", problem, cause...)
}

// NotFound returns a 404 Not Found error.
func NotFound(problem Problem, cause ...error) error {
	return statusError(http.StatusNotFound, "Not Found", problem, cause...)
}

// Conflict returns a 409 Conflict error.
func Conflict(problem Problem, cause ...error) error {
	return statusError(http.StatusConflict, "Conflict", problem, cause...)
}

// PayloadTooLarge returns a 413 Payload Too Large error.
func PayloadTooLarge(problem Problem, cause ...error) error {
	return statusError(http.StatusRequestEntityTooLarge, "Payload Too Large", problem, cause...)
}

// Internal returns a 500 Internal Server Error that wraps err.
func Internal(err error) error {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Title:   "Internal Server Error",
		Problem: Problem{Code: "internal", Message: msg},
		cause:   err,
	}
}

// Validation returns a 400 Bad Request validation error with field details.
func Validation(fields ...FieldError) error {
	return &HTTPError{
		Status:  http.StatusBadRequest,
		Title:   "Validation failed",
		Problem: Problem{Code: "validation_failed", Message: "Validation failed"},
		Fields:  append([]FieldError(nil), fields...),
	}
}

// AsHTTPError extracts *HTTPError from err if present.
// Implemented without errors.As so TinyGo does not require reflect.AssignableTo
// (unimplemented for interfaces in TinyGo 0.40), which otherwise panics when
// Bind's json.RawMessage path is also linked into the same binary.
func AsHTTPError(err error) (*HTTPError, bool) {
	for err != nil {
		if he, ok := err.(*HTTPError); ok {
			return he, true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil, false
		}
		err = u.Unwrap()
	}
	return nil, false
}

// BindError is returned when binding fails for a specific field/source.
func BindError(field, location, message string) error {
	return Validation(Field(field, location, message))
}
