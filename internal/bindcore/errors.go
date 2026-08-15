// Package bindcore holds the declarations both transport runtimes need and
// neither should own: the error model, the uploaded-file value, and the check
// helpers. Keeping them here is what lets an error cross between the net/http
// and fasthttp surfaces and still match, and what keeps a model struct naming
// File from pulling a transport in behind it.
//
// It imports no transport package.
package bindcore

import "strconv"

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
	// Location carries a redirect target. It is empty for every ordinary error,
	// and set only by Redirect, whose value travels the error return because a
	// redirect and an error both end the normal response before it starts.
	Location string
	cause    error
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
	return StatusText(e.Status)
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// StatusText mirrors net/http.StatusText for the codes this package can
// produce and the ordinary ones beside them, and returns "" for anything else
// exactly as the standard library does. It exists so the error model links no
// transport: every constructor below sets Title, so this is reached only by a
// hand-built HTTPError.
func StatusText(status int) string {
	switch status {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 202:
		return "Accepted"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 406:
		return "Not Acceptable"
	case 409:
		return "Conflict"
	case 410:
		return "Gone"
	case 413:
		return "Request Entity Too Large"
	case 415:
		return "Unsupported Media Type"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 501:
		return "Not Implemented"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	case 504:
		return "Gateway Timeout"
	}
	return ""
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
	return statusError(400, "Bad Request", problem, cause...)
}

// Unauthorized returns a 401 Unauthorized error.
func Unauthorized(problem Problem, cause ...error) error {
	return statusError(401, "Unauthorized", problem, cause...)
}

// Forbidden returns a 403 Forbidden error.
func Forbidden(problem Problem, cause ...error) error {
	return statusError(403, "Forbidden", problem, cause...)
}

// NotFound returns a 404 Not Found error.
func NotFound(problem Problem, cause ...error) error {
	return statusError(404, "Not Found", problem, cause...)
}

// Redirect returns a value that sends the browser to target. It travels the
// error return because a caller that returns values rather than holding a
// ResponseWriter has no other channel, and because a redirect and an error both
// end the normal response before it starts.
//
// The value is an ordinary error: nothing panics and no control-flow exception
// is thrown, which is the difference from how a server-function ecosystem built
// on exceptions expresses this.
//
// status defaults to 303, which is what a page wants after a POST. Pass one of
// 301, 302, 307, or 308 to choose another; anything else is refused here rather
// than emitted as a status no client will follow.
func Redirect(target string, status ...int) error {
	code := 303
	if len(status) > 0 {
		code = status[0]
	}
	switch code {
	case 301, 302, 303, 307, 308:
	default:
		return statusError(500, "Internal Server Error",
			Problem{Code: "invalid_redirect", Message: "redirect status " + strconv.Itoa(code) + " is not a redirect"})
	}
	return &HTTPError{
		Status:   code,
		Title:    StatusText(code),
		Problem:  Problem{Code: "redirect", Message: "redirect to " + target},
		Location: target,
	}
}

// RedirectTarget reports the location a redirect value carries.
func RedirectTarget(err error) (string, int, bool) {
	he, ok := AsHTTPError(err)
	if !ok || he.Location == "" {
		return "", 0, false
	}
	return he.Location, he.Status, true
}

// Conflict returns a 409 Conflict error.
func Conflict(problem Problem, cause ...error) error {
	return statusError(409, "Conflict", problem, cause...)
}

// PayloadTooLarge returns a 413 Payload Too Large error.
func PayloadTooLarge(problem Problem, cause ...error) error {
	return statusError(413, "Payload Too Large", problem, cause...)
}

// Internal returns a 500 Internal Server Error that wraps err.
func Internal(err error) error {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	return &HTTPError{
		Status:  500,
		Title:   "Internal Server Error",
		Problem: Problem{Code: "internal", Message: msg},
		cause:   err,
	}
}

// Validation returns a 400 Bad Request validation error with field details.
func Validation(fields ...FieldError) error {
	return &HTTPError{
		Status:  400,
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
