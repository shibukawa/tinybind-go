package fasthttpbind

import (
	"errors"
	"sync"

	"github.com/shibukawa/tinygodriver/fasthttp"
)

// typeMarker is used only for registry dispatch identity — not for field
// walking. A zero-size generic struct gives each T a distinct comparable `any`
// key without pulling reflect into the runtime. Generated binders/writers
// perform all field mapping without reflect.
type typeMarker[T any] struct{}

func typeKey[T any]() any {
	return typeMarker[T]{}
}

// The registries are this package's own: a binder here takes a RequestCtx and
// could not be stored beside a net/http one even if both were linked.
var (
	binders sync.Map // typeMarker[T] -> func(*fasthttp.RequestCtx) (T, error)
	writers sync.Map // typeMarker[T] -> func(*fasthttp.RequestCtx, T) error
)

// RegisterBind registers a generated binder for T.
// Call from generated init(); field mapping lives entirely inside fn.
func RegisterBind[T any](fn func(*fasthttp.RequestCtx) (T, error)) {
	binders.Store(typeKey[T](), any(fn))
}

// RegisterWrite registers a generated writer for T.
func RegisterWrite[T any](fn func(*fasthttp.RequestCtx, T) error) {
	writers.Store(typeKey[T](), any(fn))
}

func lookupBinder[T any]() (func(*fasthttp.RequestCtx) (T, error), bool) {
	v, ok := binders.Load(typeKey[T]())
	if !ok {
		return nil, false
	}
	fn, ok := v.(func(*fasthttp.RequestCtx) (T, error))
	return fn, ok
}

func lookupWriter[T any]() (func(*fasthttp.RequestCtx, T) error, bool) {
	v, ok := writers.Load(typeKey[T]())
	if !ok {
		return nil, false
	}
	fn, ok := v.(func(*fasthttp.RequestCtx, T) error)
	return fn, ok
}

func missingBinderError() error {
	return Internal(errors.New("fasthttpbind: no binder registered for request type (missing generated init?)"))
}

func missingWriterError() error {
	return Internal(errors.New("fasthttpbind: no writer registered for response type (missing generated init?)"))
}
