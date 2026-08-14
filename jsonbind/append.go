package jsonbind

import (
	"strconv"
	"sync"
	"unicode/utf8"
)

// Generated encoders build a document by appending to a byte slice and write it
// once. Escaping matches encoding/json's default encoder byte for byte, so
// switching a codec over does not change the bytes on the wire.

var bufPool = sync.Pool{New: func() any { b := make([]byte, 0, 1024); return &b }}

// GetBuffer borrows an encode buffer. Generated code returns it with
// PutBuffer once the bytes have been written out.
func GetBuffer() *[]byte {
	b := bufPool.Get().(*[]byte)
	*b = (*b)[:0]
	return b
}

// PutBuffer returns an encode buffer to the pool. Oversized buffers are
// dropped so one large document does not pin memory for the process lifetime.
func PutBuffer(b *[]byte) {
	if b == nil || cap(*b) > 1<<20 {
		return
	}
	bufPool.Put(b)
}

const hexDigits = "0123456789abcdef"

// AppendString appends a JSON string literal. Like encoding/json's encoder it
// escapes <, > and & so the result is safe to embed in HTML, and U+2028/U+2029
// so it stays valid JavaScript.
func AppendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			dst = append(dst, s[start:i]...)
			switch c {
			case '"':
				dst = append(dst, '\\', '"')
			case '\\':
				dst = append(dst, '\\', '\\')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', 'f', 'f', 'f', 'd')
			i += size
			start = i
			continue
		}
		if r == ' ' || r == ' ' {
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}

// AppendInt appends a JSON number.
func AppendInt(dst []byte, v int64) []byte { return strconv.AppendInt(dst, v, 10) }

// AppendUint appends a JSON number.
func AppendUint(dst []byte, v uint64) []byte { return strconv.AppendUint(dst, v, 10) }

// AppendBool appends a JSON boolean.
func AppendBool(dst []byte, v bool) []byte { return strconv.AppendBool(dst, v) }

// AppendFloat appends a JSON number using encoding/json's formatting: shortest
// round-trip, switching to exponent form outside [1e-6, 1e21) and trimming the
// exponent's leading zero.
func AppendFloat(dst []byte, v float64) []byte {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	format := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	dst = strconv.AppendFloat(dst, v, format, -1, 64)
	if format == 'e' {
		n := len(dst)
		if n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}
	return dst
}

// AppendRaw appends an already-encoded JSON value, or null when it is empty.
func AppendRaw(dst []byte, raw []byte) []byte {
	if len(raw) == 0 {
		return append(dst, "null"...)
	}
	return append(dst, raw...)
}

// AppendAny appends an arbitrary Go value produced by rest-field decoding.
// It covers the shapes Parser.Any yields plus the common scalar types, and any
// type carrying its own encoder through [Appender]; anything else is written as
// null rather than failing an otherwise valid response.
//
// The Appender arm is what keeps a user type out of that null. Before it a
// value the switch did not name — which is every named struct — reached the
// default and encoded as null, producing a wrong document rather than a
// reported error. It sits after the concrete cases so a builtin shape is still
// matched by identity rather than by method set.
func AppendAny(dst []byte, v any) []byte {
	switch t := v.(type) {
	case nil:
		return append(dst, "null"...)
	case string:
		return AppendString(dst, t)
	case bool:
		return AppendBool(dst, t)
	case float64:
		return AppendFloat(dst, t)
	case float32:
		return AppendFloat(dst, float64(t))
	case int:
		return AppendInt(dst, int64(t))
	case int8:
		return AppendInt(dst, int64(t))
	case int16:
		return AppendInt(dst, int64(t))
	case int32:
		return AppendInt(dst, int64(t))
	case int64:
		return AppendInt(dst, t)
	case uint:
		return AppendUint(dst, uint64(t))
	case uint8:
		return AppendUint(dst, uint64(t))
	case uint16:
		return AppendUint(dst, uint64(t))
	case uint32:
		return AppendUint(dst, uint64(t))
	case uint64:
		return AppendUint(dst, t)
	case []byte:
		return AppendRaw(dst, t)
	case []any:
		dst = append(dst, '[')
		for i, e := range t {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = AppendAny(dst, e)
		}
		return append(dst, ']')
	case map[string]any:
		dst = append(dst, '{')
		for i, k := range sortedKeys(t) {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = AppendString(dst, k)
			dst = append(dst, ':')
			dst = AppendAny(dst, t[k])
		}
		return append(dst, '}')
	case []string:
		dst = append(dst, '[')
		for i, e := range t {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = AppendString(dst, e)
		}
		return append(dst, ']')
	case Appender:
		return t.AppendJSONTo(dst)
	default:
		return append(dst, "null"...)
	}
}

// SortedKeys orders map keys so a map encodes deterministically, matching
// encoding/json's behaviour. Generated encoders call it for every map-typed
// field and for payload:"*" rest maps.
func SortedKeys[V any](m map[string]V) []string { return sortedKeys(m) }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Insertion sort: rest maps are small, and this avoids pulling the sort
	// package (and its reflect-adjacent generic machinery) into TinyGo builds.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
