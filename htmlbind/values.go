package htmlbind

import (
	"strconv"
	"strings"
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
	const hex = "0123456789abcdef"
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range string(value) {
		switch r {
		case '"':
			out.WriteString("\\\"")
		case '\\':
			out.WriteString("\\\\")
		case '\b':
			out.WriteString("\\b")
		case '\f':
			out.WriteString("\\f")
		case '\n':
			out.WriteString("\\n")
		case '\r':
			out.WriteString("\\r")
		case '\t':
			out.WriteString("\\t")
		case '<':
			out.WriteString("\\u003c")
		case '>':
			out.WriteString("\\u003e")
		case '&':
			out.WriteString("\\u0026")
		case ' ':
			out.WriteString("\\u2028")
		case ' ':
			out.WriteString("\\u2029")
		default:
			if r < 0x20 {
				out.WriteString("\\u00")
				out.WriteByte(hex[byte(r)>>4])
				out.WriteByte(hex[byte(r)&15])
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
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
