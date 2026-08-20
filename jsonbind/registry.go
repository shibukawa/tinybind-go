package jsonbind

import (
	"io"
	"sync"
)

// typeMarker gives each T a distinct comparable `any` key for registry
// dispatch without pulling reflect into the runtime.
type typeMarker[T any] struct{}

func typeKey[T any]() any { return typeMarker[T]{} }

// Registries hold the generated function itself rather than a closure that
// launders T through `any`. A func value is pointer-shaped, so storing one in
// an interface costs nothing, while returning T through `any` would box (and
// copy) the whole decoded struct on every call.
var decoders sync.Map  // typeMarker[T] -> func([]byte) (T, error)
var encoders sync.Map  // typeMarker[T] -> func(io.Writer, T) error
var appenders sync.Map // typeMarker[T] -> func([]byte, T) []byte

// RegisterDecode registers a generated JSON document decoder for T.
func RegisterDecode[T any](fn func([]byte) (T, error)) {
	decoders.Store(typeKey[T](), any(fn))
}

// RegisterEncode registers a generated compact JSON encoder for T.
func RegisterEncode[T any](fn func(io.Writer, T) error) {
	encoders.Store(typeKey[T](), any(fn))
}

// RegisterAppend registers a generated append-form encoder for T: the function
// a generated writer body is built from, appending one compact JSON value with
// no trailing newline.
//
// The writer form above frames and writes a whole document, which is right for
// a response body but wrong for a caller composing a larger frame — an SSE
// event, a JSON array element — who would pay a second buffer and copy to
// unwrap it. This form hands the bytes over where they are wanted instead.
func RegisterAppend[T any](fn func([]byte, T) []byte) {
	appenders.Store(typeKey[T](), any(fn))
}

// AppendFuncFor returns T's registered append-form encoder. Callers that
// encode the same T repeatedly — a stream writing events — resolve it once
// instead of paying a registry lookup per value.
func AppendFuncFor[T any]() (func([]byte, T) []byte, bool) {
	v, ok := appenders.Load(typeKey[T]())
	if !ok {
		return nil, false
	}
	fn, ok := v.(func([]byte, T) []byte)
	return fn, ok
}

// EncodeFuncFor returns T's registered writer-form encoder, for the same
// resolve-once callers when only that form was registered.
func EncodeFuncFor[T any]() (func(io.Writer, T) error, bool) {
	return lookupEncoder[T]()
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

func missingDecoderError() error {
	return newError("missing_codec", "jsonbind: no JSON decoder registered (missing generated init?)", nil)
}

func missingEncoderError() error {
	return newError("missing_codec", "jsonbind: no JSON encoder registered (missing generated init?)", nil)
}
