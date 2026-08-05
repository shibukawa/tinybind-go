package jsonbind

import (
	"errors"
	"io"
	"sync/atomic"
)

// DefaultMaxJSONBodyBytes is the default JSON document limit (1 MiB).
const DefaultMaxJSONBodyBytes int64 = 1 << 20

var maxJSONBodyBytes atomic.Int64

// ErrBodyTooLarge reports that a JSON document exceeded its configured limit.
var ErrBodyTooLarge = errors.New("jsonbind: JSON body too large")

// SetMaxJSONBodyBytes changes the process-wide JSON document limit.
func SetMaxJSONBodyBytes(n int64) {
	if n <= 0 {
		maxJSONBodyBytes.Store(0)
		return
	}
	maxJSONBodyBytes.Store(n)
}

// MaxJSONBodyBytes returns the effective JSON document limit.
func MaxJSONBodyBytes() int64 {
	if n := maxJSONBodyBytes.Load(); n > 0 {
		return n
	}
	return DefaultMaxJSONBodyBytes
}

// ReadLimit reads at most limit bytes from r.
func ReadLimit(r io.Reader, limit int64) ([]byte, error) {
	return ReadLimitHint(r, limit, 0)
}

// ReadLimitHint is ReadLimit with an expected size. A caller that knows the
// length up front — an HTTP handler with a Content-Length, say — lets the whole
// body land in one allocation instead of the repeated grow-and-copy io.ReadAll
// performs. A wrong hint costs nothing but the usual growth.
func ReadLimitHint(r io.Reader, limit, hint int64) ([]byte, error) {
	if limit <= 0 {
		limit = DefaultMaxJSONBodyBytes
	}
	if hint <= 0 {
		hint = sizeOf(r)
	}
	if hint < 0 || hint > limit {
		hint = 0
	}
	// One extra byte distinguishes "exactly at the limit" from "over it".
	// Without a hint, a small floor keeps tiny bodies off the growth path; with
	// one, padding past it would only waste the allocation the hint bought.
	capacity := hint + 1
	if hint == 0 {
		capacity = 512
	}
	buf := make([]byte, 0, capacity)
	for {
		if len(buf) == cap(buf) {
			buf = append(buf, 0)[:len(buf)]
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if int64(len(buf)) > limit {
			return nil, ErrBodyTooLarge
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return nil, err
		}
	}
}

// sizeOf reports the remaining length of readers that can say, such as
// *bytes.Reader, *bytes.Buffer and *strings.Reader.
func sizeOf(r io.Reader) int64 {
	if l, ok := r.(interface{ Len() int }); ok {
		return int64(l.Len())
	}
	return 0
}

func readJSONBytes(r io.Reader, limit int64) ([]byte, error) { return ReadLimit(r, limit) }

// IsBlank reports whether data holds no JSON document at all. Generated
// decoders treat that as the zero value rather than a parse error, which is how
// an absent body and an empty config file have always behaved.
func IsBlank(data []byte) bool {
	for _, c := range data {
		switch c {
		case ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}

// DecodeJSONMapStringString decodes a JSON object of strings.
func DecodeJSONMapStringString(raw []byte) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	p := NewParser(raw)
	if p.IsNull() {
		return map[string]string{}, nil
	}
	if _, err := p.ObjectStart(); err != nil {
		return nil, fieldTypeError("invalid string map", err)
	}
	out := map[string]string{}
	for n := 0; ; n++ {
		key, ok, err := p.ObjectKey(n)
		if err != nil {
			return nil, fieldTypeError("invalid string map", err)
		}
		if !ok {
			return out, nil
		}
		name := string(key)
		v, err := p.String()
		if err != nil {
			return nil, fieldTypeError("invalid string map", err)
		}
		out[name] = v
	}
}

// DecodeJSONMapStringInt decodes a JSON object of ints.
func DecodeJSONMapStringInt(raw []byte) (map[string]int, error) {
	return decodeScalarMap(raw, "invalid int map", (*Parser).Int)
}

// DecodeJSONMapStringInt64 decodes a JSON object of int64s.
func DecodeJSONMapStringInt64(raw []byte) (map[string]int64, error) {
	return decodeScalarMap(raw, "invalid int64 map", (*Parser).Int64)
}

// DecodeJSONMapStringBool decodes a JSON object of booleans.
func DecodeJSONMapStringBool(raw []byte) (map[string]bool, error) {
	return decodeScalarMap(raw, "invalid bool map", (*Parser).Bool)
}

// DecodeJSONMapStringFloat64 decodes a JSON object of floats.
func DecodeJSONMapStringFloat64(raw []byte) (map[string]float64, error) {
	return decodeScalarMap(raw, "invalid float64 map", (*Parser).Float64)
}

func decodeScalarMap[T any](raw []byte, message string, read func(*Parser) (T, error)) (map[string]T, error) {
	if len(raw) == 0 {
		return map[string]T{}, nil
	}
	p := NewParser(raw)
	if p.IsNull() {
		return map[string]T{}, nil
	}
	if _, err := p.ObjectStart(); err != nil {
		return nil, fieldTypeError(message, err)
	}
	out := map[string]T{}
	for n := 0; ; n++ {
		key, ok, err := p.ObjectKey(n)
		if err != nil {
			return nil, fieldTypeError(message, err)
		}
		if !ok {
			return out, nil
		}
		name := string(key)
		v, err := read(p)
		if err != nil {
			return nil, fieldTypeError(message, err)
		}
		out[name] = v
	}
}

// DecodeJSONStringSlice decodes a JSON array of strings.
func DecodeJSONStringSlice(raw []byte) ([]string, error) {
	return decodeScalarSlice(raw, "invalid string array", (*Parser).String)
}

// DecodeJSONIntSlice decodes a JSON array of ints.
func DecodeJSONIntSlice(raw []byte) ([]int, error) {
	return decodeScalarSlice(raw, "invalid int array", (*Parser).Int)
}

// DecodeJSONInt64Slice decodes a JSON array of int64s.
func DecodeJSONInt64Slice(raw []byte) ([]int64, error) {
	return decodeScalarSlice(raw, "invalid int64 array", (*Parser).Int64)
}

// DecodeJSONBoolSlice decodes a JSON array of booleans.
func DecodeJSONBoolSlice(raw []byte) ([]bool, error) {
	return decodeScalarSlice(raw, "invalid bool array", (*Parser).Bool)
}

// DecodeJSONFloat64Slice decodes a JSON array of floats.
func DecodeJSONFloat64Slice(raw []byte) ([]float64, error) {
	return decodeScalarSlice(raw, "invalid float64 array", (*Parser).Float64)
}

func decodeScalarSlice[T any](raw []byte, message string, read func(*Parser) (T, error)) ([]T, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	p := NewParser(raw)
	null, err := p.ArrayStart()
	if err != nil {
		return nil, fieldTypeError(message, err)
	}
	if null {
		return nil, nil
	}
	out := []T{}
	for i := 0; ; i++ {
		more, err := p.ArrayNext(i)
		if err != nil {
			return nil, fieldTypeError(message, err)
		}
		if !more {
			return out, nil
		}
		if i == 0 {
			out = make([]T, 0, 8)
		}
		v, err := read(p)
		if err != nil {
			return nil, fieldTypeError(message, err)
		}
		out = append(out, v)
	}
}

// DecodeJSONString decodes a JSON string value.
func DecodeJSONString(raw []byte) (string, error) {
	return decodeScalar(raw, "invalid string", (*Parser).String)
}

// DecodeJSONInt decodes a JSON number as int.
func DecodeJSONInt(raw []byte) (int, error) {
	return decodeScalar(raw, "invalid int", (*Parser).Int)
}

// DecodeJSONInt64 decodes a JSON number as int64.
func DecodeJSONInt64(raw []byte) (int64, error) {
	return decodeScalar(raw, "invalid int64", (*Parser).Int64)
}

// DecodeJSONBool decodes a JSON boolean.
func DecodeJSONBool(raw []byte) (bool, error) {
	return decodeScalar(raw, "invalid bool", (*Parser).Bool)
}

// DecodeJSONFloat64 decodes a JSON number as float64.
func DecodeJSONFloat64(raw []byte) (float64, error) {
	return decodeScalar(raw, "invalid float64", (*Parser).Float64)
}

// DecodeJSONAny decodes any JSON value into the Go shapes encoding/json uses
// for an `any` destination.
func DecodeJSONAny(raw []byte) (any, error) {
	return decodeScalar(raw, "invalid JSON value", (*Parser).Any)
}

func decodeScalar[T any](raw []byte, message string, read func(*Parser) (T, error)) (T, error) {
	var zero T
	p := NewParser(raw)
	v, err := read(p)
	if err != nil {
		return zero, fieldTypeError(message, err)
	}
	if err := p.End(); err != nil {
		return zero, fieldTypeError(message, err)
	}
	return v, nil
}

func fieldTypeError(message string, cause error) error {
	return newError("json_parse", message, cause)
}

// RawJSONArray splits a JSON array into its raw elements. Elements alias raw.
func RawJSONArray(raw []byte) ([][]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	p := NewParser(raw)
	null, err := p.ArrayStart()
	if err != nil {
		return nil, fieldTypeError("invalid JSON array", err)
	}
	if null {
		return nil, nil
	}
	var out [][]byte
	for i := 0; ; i++ {
		more, err := p.ArrayNext(i)
		if err != nil {
			return nil, fieldTypeError("invalid JSON array", err)
		}
		if !more {
			return out, nil
		}
		if i == 0 {
			out = make([][]byte, 0, 8)
		}
		el, err := p.RawValue()
		if err != nil {
			return nil, fieldTypeError("invalid JSON array", err)
		}
		out = append(out, el)
	}
}

// RawJSONMap splits a JSON object into raw fields. Values alias raw.
func RawJSONMap(raw []byte) (*Object, error) { return ParseObject(raw) }

// BytesJSONMap splits a complete JSON object document.
func BytesJSONMap(data []byte) (*Object, error) { return ParseObject(data) }

// RestJSONAny returns JSON fields not named in exclude.
func RestJSONAny(body *Object, exclude []string) (map[string]any, error) {
	return body.RestAny(exclude)
}

// RestJSONNames returns the names of JSON fields not named in exclude.
//
// A raw rest field is typed map[string]json.RawMessage in the caller's struct,
// and Go will not convert a map[string][]byte to it however identical the
// element layout is. Handing back names keeps encoding/json — and the
// reflection it drags into a TinyGo binary — out of this package: generated
// code fills its own map with the value copies.
func RestJSONNames(body *Object, exclude []string) []string {
	return body.RestNames(exclude)
}
