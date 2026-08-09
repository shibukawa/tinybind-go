package fasthttpbind

import (
	"github.com/shibukawa/tinybind-go/jsonbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Bind maps a fasthttp request into a typed request value.
// Dispatch uses a registry of generated binders; field mapping does not use reflect.
//
// Every field the binder fills is copied out of the pooled request, so the
// returned value stays valid after the handler returns.
func Bind[T any](ctx *fasthttp.RequestCtx) (T, error) {
	var zero T
	fn, ok := lookupBinder[T]()
	if !ok {
		return zero, missingBinderError()
	}
	out, err := fn(ctx)
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
