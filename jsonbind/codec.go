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
	if !ok && !carriesDecoder[T]() {
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
	if out, done, err := decodeThroughInterface[T](data); done {
		return out, err
	}
	return fn(data)
}

// carriesDecoder reports whether *T implements [Decoder], which is what lets a
// type this run never planned be read anyway.
//
// The check is on the pointer because a decoder fills its receiver. It runs
// before the body is read so a type that can be decoded by neither route still
// fails without consuming the reader.
//
// The probe uses a typed nil rather than the address of a local: boxing a
// pointer stores it in the interface word directly, so asking costs nothing,
// while &zero would force zero onto the heap on every call.
func carriesDecoder[T any]() bool {
	_, ok := any((*T)(nil)).(Decoder)
	return ok
}

// carriesAppender reports whether T's value method set includes [Appender],
// probed through *T for the same zero-cost reason as carriesDecoder. *T's
// method set is a superset of T's, so a false here proves the value assertion
// below would fail too, and the value is never boxed just to find that out.
func carriesAppender[T any]() bool {
	_, ok := any((*T)(nil)).(Appender)
	return ok
}

// decodeThroughInterface reads data into a T that carries its own decoder, and
// reports whether it did. A false means the type carries none and the registry
// is the remaining route.
func decodeThroughInterface[T any](data []byte) (T, bool, error) {
	if !carriesDecoder[T]() {
		var out T
		return out, false, nil
	}
	var out T
	target := any(&out).(Decoder)
	if err := target.DecodeJSONFrom(data); err != nil {
		return out, true, err
	}
	return out, true, nil
}

// encodeThroughInterface writes v through the codec it carries itself.
//
// The buffer comes from the same pool a generated writer uses, so a type
// reaching this path allocates no more than a planned one does. The method-set
// probe runs on *T first so the common case — a type with no method — skips
// the value boxing that any(v) would cost on every call.
func encodeThroughInterface[T any](w io.Writer, v T) (bool, error) {
	if !carriesAppender[T]() {
		return false, nil
	}
	source, ok := any(v).(Appender)
	if !ok {
		// Appender lives on *T alone; the value cannot reach it.
		return false, nil
	}
	buf := GetBuffer()
	*buf = source.AppendJSONTo((*buf)[:0])
	_, err := w.Write(*buf)
	PutBuffer(buf)
	return true, err
}

// DecodeJSONBytes decodes one JSON value already held in memory into T.
//
// The reader entries above exist because an HTTP body arrives as a stream. A
// caller that already has the whole document — a WebSocket message, say — has
// nothing to read, and going through a reader would allocate one and copy the
// document into a second buffer for every call.
//
// The limit belongs to whoever produced the bytes, so none is applied here.
// A type carrying [Decoder] is read through it, on the terms [EncodeJSON]
// states for the other direction.
func DecodeJSONBytes[T any](data []byte) (T, error) {
	if out, done, err := decodeThroughInterface[T](data); done {
		return out, err
	}
	var zero T
	fn, ok := lookupDecoder[T]()
	if !ok {
		return zero, missingDecoderError()
	}
	return fn(data)
}

// EncodeJSON encodes v as compact JSON to w using a generated codec, or, for a
// type carrying its own, through [Appender].
//
// The interface is tried first. A type this run planned and also carries a
// method is one whose author wrote an encoder, and encoding it through the
// generated one instead would silently produce bytes they did not intend. This
// is how encoding/json resolves the same conflict, letting the method win over
// walking the fields. For a declared codec the two are the same code, since the
// emitted method delegates to the emitted function.
//
// It does not set HTTP headers or status.
func EncodeJSON[T any](w io.Writer, v T) error {
	if w == nil {
		return newError("internal", "jsonbind: nil writer", nil)
	}
	if done, err := encodeThroughInterface(w, v); done {
		return err
	}
	fn, ok := lookupEncoder[T]()
	if !ok {
		return missingEncoderError()
	}
	return fn(w, v)
}
