package httpbind

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// Bind maps an HTTP request into a typed request value.
// Dispatch uses a registry of generated binders; field mapping does not use reflect.
func Bind[T any](r *http.Request) (T, error) {
	var zero T
	fn, ok := lookupBinder[T]()
	if !ok {
		return zero, missingBinderError(typeKey[T]())
	}
	out, err := fn(r)
	if err != nil {
		return zero, mapJSONError(err)
	}
	return out, nil
}

func mapJSONError(err error) error {
	je, ok := jsonbind.AsError(err)
	if !ok {
		return err
	}
	problem := Problem{Code: je.Code, Message: je.Message}
	if je.Code == "payload_too_large" {
		return PayloadTooLarge(problem, err)
	}
	if je.Field != "" {
		return Validation(Field(je.Field, "payload", je.Message))
	}
	return BadRequest(problem, err)
}
