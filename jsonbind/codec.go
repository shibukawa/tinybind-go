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

// DecodeJSONBytes decodes one JSON value already held in memory into T.
//
// The reader entries above exist because an HTTP body arrives as a stream. A
// caller that already has the whole document — a WebSocket message, say — has
// nothing to read, and going through a reader would allocate one and copy the
// document into a second buffer for every call.
//
// The limit belongs to whoever produced the bytes, so none is applied here.
func DecodeJSONBytes[T any](data []byte) (T, error) {
	var zero T
	fn, ok := lookupDecoder[T]()
	if !ok {
		return zero, missingDecoderError()
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
