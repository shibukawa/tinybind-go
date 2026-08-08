// Package jsonbind provides generated, reflection-free JSON document codecs.
package jsonbind

import "io"

// DecodeJSON decodes one JSON value from r into T using a generated codec.
// It does not inspect HTTP headers or use reflection on T's fields.
func DecodeJSON[T any](r io.Reader) (T, error) {
	return decodeJSON[T](r, MaxJSONBodyBytes(), 0)
}

// DecodeJSONLimit is DecodeJSON with a per-call byte limit. A non-positive
// limit uses MaxJSONBodyBytes.
func DecodeJSONLimit[T any](r io.Reader, limit int64) (T, error) {
	if limit <= 0 {
		limit = MaxJSONBodyBytes()
	}
	return decodeJSON[T](r, limit, 0)
}

// DecodeJSONHint is DecodeJSONLimit with an expected document size. A caller
// that knows the length up front — an HTTP handler with a Content-Length, say
// — lets the whole body land in one allocation; see ReadLimitHint.
func DecodeJSONHint[T any](r io.Reader, limit, hint int64) (T, error) {
	if limit <= 0 {
		limit = MaxJSONBodyBytes()
	}
	return decodeJSON[T](r, limit, hint)
}

func decodeJSON[T any](r io.Reader, limit, hint int64) (T, error) {
	var zero T
	fn, ok := lookupDecoder[T]()
	if !ok {
		return zero, missingDecoderError()
	}
	if r == nil {
		return zero, newError("json_parse", "nil reader", nil)
	}
	data, err := ReadLimitHint(r, limit, hint)
	if err != nil {
		if err == ErrBodyTooLarge {
			return zero, newError("payload_too_large", "JSON body too large", err)
		}
		return zero, newError("body_read", "failed to read JSON", err)
	}
	return fn(data)
}

// EncodeJSON encodes v as compact JSON to w using a generated codec.
// It does not set HTTP headers or status.
func EncodeJSON[T any](w io.Writer, v T) error {
	fn, ok := lookupEncoder[T]()
	if !ok {
		return missingEncoderError()
	}
	if w == nil {
		return newError("internal", "jsonbind: nil writer", nil)
	}
	return fn(w, v)
}
