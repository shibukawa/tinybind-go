package httpbind

import (
	"errors"
	"net/http"
	"sync"
)

// typeMarker is used only for registry dispatch identity — not for field
// walking. A zero-size generic struct gives each T a distinct comparable `any`
// key without pulling reflect into the runtime. Generated binders/writers
// perform all field mapping without reflect.
type typeMarker[T any] struct{}

func typeKey[T any]() any {
	return typeMarker[T]{}
}

// The registries hold the generated function itself, not a closure that
// launders T through `any`. A func value is pointer-shaped, so storing one in
// an interface is free, while passing T through `any` would box (and copy) the
// bound struct on every request.
var (
	binders sync.Map // typeMarker[T] -> func(*http.Request) (T, error)
	writers sync.Map // typeMarker[T] -> func(http.ResponseWriter, *http.Request, T) error
)

// RegisterBind registers a generated binder for T.
// Call from generated init(); field mapping lives entirely inside fn.
func RegisterBind[T any](fn func(*http.Request) (T, error)) {
	binders.Store(typeKey[T](), any(fn))
}

// RegisterWrite registers a generated writer for T.
func RegisterWrite[T any](fn func(http.ResponseWriter, *http.Request, T) error) {
	writers.Store(typeKey[T](), any(fn))
}

func lookupBinder[T any]() (func(*http.Request) (T, error), bool) {
	v, ok := binders.Load(typeKey[T]())
	if !ok {
		return nil, false
	}
	fn, ok := v.(func(*http.Request) (T, error))
	return fn, ok
}

func lookupWriter[T any]() (func(http.ResponseWriter, *http.Request, T) error, bool) {
	v, ok := writers.Load(typeKey[T]())
	if !ok {
		return nil, false
	}
	fn, ok := v.(func(http.ResponseWriter, *http.Request, T) error)
	return fn, ok
}

func missingBinderError() error {
	return Internal(errors.New("httpbind: no binder registered for request type (missing generated init?)"))
}

func missingWriterError() error {
	return Internal(errors.New("httpbind: no writer registered for response type (missing generated init?)"))
}
