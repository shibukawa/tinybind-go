package jsonbind

import (
	"fmt"
	"io"
	"reflect"
	"sync"
)

func typeKey[T any]() reflect.Type { return reflect.TypeFor[T]() }

// Registries hold the generated function itself rather than a closure that
// launders T through `any`. A func value is pointer-shaped, so storing one in
// an interface costs nothing, while returning T through `any` would box (and
// copy) the whole decoded struct on every call.
var decoders sync.Map // reflect.Type -> func([]byte) (T, error)
var encoders sync.Map // reflect.Type -> func(io.Writer, T) error

// RegisterDecode registers a generated JSON document decoder for T.
func RegisterDecode[T any](fn func([]byte) (T, error)) {
	decoders.Store(typeKey[T](), any(fn))
}

// RegisterEncode registers a generated compact JSON encoder for T.
func RegisterEncode[T any](fn func(io.Writer, T) error) {
	encoders.Store(typeKey[T](), any(fn))
}

func lookupDecoder[T any]() (func([]byte) (T, error), bool) {
	v, ok := decoders.Load(typeKey[T]())
	if !ok {
		return nil, false
	}
	fn, ok := v.(func([]byte) (T, error))
	return fn, ok
}

func lookupEncoder[T any]() (func(io.Writer, T) error, bool) {
	v, ok := encoders.Load(typeKey[T]())
	if !ok {
		return nil, false
	}
	fn, ok := v.(func(io.Writer, T) error)
	return fn, ok
}

func missingDecoderError(t reflect.Type) error {
	return newError("missing_codec", fmt.Sprintf("jsonbind: no JSON decoder registered for %s", t), nil)
}

func missingEncoderError(t reflect.Type) error {
	return newError("missing_codec", fmt.Sprintf("jsonbind: no JSON encoder registered for %s", t), nil)
}
