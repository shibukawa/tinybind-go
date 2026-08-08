package htmlbind

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// TrustedHTML is markup the template author vouched for. It is written without
// escaping, so it must never carry unvalidated input.
type TrustedHTML string

// TrustedCSS is stylesheet text the template author vouched for.
type TrustedCSS string

// TrustedJavaScript is script text the template author vouched for.
type TrustedJavaScript string

// ScriptJSON is a JSON document destined for a script element. It is produced
// by the JSON encoders below, which escape the characters that would otherwise
// terminate the element.
type ScriptJSON string

// FormatBool renders a bool as template text.
func FormatBool(value bool) string { return strconv.FormatBool(value) }

// FormatInt renders an int as template text.
func FormatInt(value int) string { return strconv.Itoa(value) }

// FormatFloat renders a float64 as template text.
func FormatFloat(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }

// JSONString encodes any string-kinded value as a JSON string, covering plain
// strings, generated enums, and the trusted string types above. It escapes the
// characters that could close a script element or break a JavaScript line, so
// the result is safe to embed in inline script content.
func JSONString[T ~string](value T) string {
	return string(appendJSONString(make([]byte, 0, len(value)+2), string(value)))
}

// appendJSONString appends value to dst under JSONString's rules and returns
// the extended slice. It scans bytes and copies clean runs in bulk, so a value
// needing no escapes costs one copy rather than a write per rune.
func appendJSONString(dst []byte, value string) []byte {
	const hex = "0123456789abcdef"
	if free := cap(dst) - len(dst); free < len(value)+2 {
		grown := make([]byte, len(dst), len(dst)+len(value)+18)
		copy(grown, dst)
		dst = grown
	}
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(value); {
		c := value[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			if start < i {
				dst = append(dst, value[start:i]...)
			}
			switch c {
			case '"':
				dst = append(dst, '\\', '"')
			case '\\':
				dst = append(dst, '\\', '\\')
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			case '<':
				dst = append(dst, '\\', 'u', '0', '0', '3', 'c')
			case '>':
				dst = append(dst, '\\', 'u', '0', '0', '3', 'e')
			case '&':
				dst = append(dst, '\\', 'u', '0', '0', '2', '6')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hex[c>>4], hex[c&15])
			}
			i++
			start = i
			continue
		}
		r, width := utf8.DecodeRuneInString(value[i:])
		switch {
		case r == ' ', r == ' ':
			if start < i {
				dst = append(dst, value[start:i]...)
			}
			if r == ' ' {
				dst = append(dst, '\\', 'u', '2', '0', '2', '8')
			} else {
				dst = append(dst, '\\', 'u', '2', '0', '2', '9')
			}
			i += width
			start = i
		case r == utf8.RuneError && width == 1:
			// An invalid byte becomes the replacement character, exactly as a
			// rune loop would decode it. A genuine U+FFFD is three valid bytes
			// and travels inside the clean run.
			if start < i {
				dst = append(dst, value[start:i]...)
			}
			dst = append(dst, "�"...)
			i += width
			start = i
		default:
			i += width
		}
	}
	if start < len(value) {
		dst = append(dst, value[start:]...)
	}
	return append(dst, '"')
}

// JSONBool encodes a bool as JSON.
func JSONBool(value bool) string { return strconv.FormatBool(value) }

// JSONInt encodes an int as JSON.
func JSONInt(value int) string { return strconv.Itoa(value) }

// JSONFloat encodes a float64 as JSON.
func JSONFloat(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }

// JSONOptional encodes a nil pointer as JSON null and otherwise delegates.
func JSONOptional[T any](value *T, encode func(T) string) string {
	if value == nil {
		return "null"
	}
	return encode(*value)
}

// JSONArray encodes a slice as a JSON array, delegating each element.
func JSONArray[T any](values []T, encode func(T) string) string {
	var out strings.Builder
	out.WriteByte('[')
	for index, item := range values {
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString(encode(item))
	}
	out.WriteByte(']')
	return out.String()
}
