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
// The conversion to a plain string is free, and it keeps every ~string caller
// on one instantiation of the escaper below rather than one apiece.
func JSONString[T ~string](value T) string {
	return string(AppendJSONString(make([]byte, 0, len(value)+2), string(value)))
}

// AppendJSONString appends value to dst under JSONString's rules and returns
// the extended slice. It scans plain ASCII a word at a time and copies clean
// runs in bulk, so a value needing no escapes costs one copy rather than a
// write per rune; a run of multibyte runes stays with the rune decoder
// rather than returning to the word scan at every rune.
//
// It is exported because the framing around a record is the caller's, as
// Content.AppendJSON says: a caller that assembles its own record shape still
// has to reach the escaping the module uses, and JSONString offers it only
// through a freshly allocated string. Accepting []byte is the other half of
// that. A rendered fragment is bytes, so a caller holding one would convert it
// before the escaper even ran.
func AppendJSONString[T ~string | ~[]byte](dst []byte, value T) []byte {
	const hex = "0123456789abcdef"
	if free := cap(dst) - len(dst); free < len(value)+2 {
		grown := make([]byte, len(dst), len(dst)+len(value)+18)
		copy(grown, dst)
		dst = grown
	}
	dst = append(dst, '"')
	start := 0
	i := 0
	for i < len(value) {
		i = jsonRun(value, i)
		if i >= len(value) {
			break
		}
		c := value[i]
		if c < utf8.RuneSelf {
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
		for i < len(value) && value[i] >= utf8.RuneSelf {
			// Decoding through a window of one rune at most keeps this call
			// free for a []byte value: a conversion this short stays on the
			// stack, where converting the whole remaining tail would copy it.
			end := min(i+utf8.UTFMax, len(value))
			r, width := utf8.DecodeRuneInString(string(value[i:end]))
			switch {
			case r == '\u2028', r == '\u2029':
				if start < i {
					dst = append(dst, value[start:i]...)
				}
				if r == '\u2028' {
					dst = append(dst, '\\', 'u', '2', '0', '2', '8')
				} else {
					dst = append(dst, '\\', 'u', '2', '0', '2', '9')
				}
				i += width
				start = i
			case r == utf8.RuneError && width == 1:
				// An invalid byte becomes the replacement character, exactly
				// as a rune loop would decode it. A genuine U+FFFD is three
				// valid bytes and travels inside the clean run.
				if start < i {
					dst = append(dst, value[start:i]...)
				}
				dst = append(dst, "\uFFFD"...)
				i += width
				start = i
			default:
				i += width
			}
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

// JSONFloat encodes a float64 as JSON, the way encoding/json and jsonbind
// write one: the shortest round-trip decimal, in plain notation, switching
// to an exponent only below 1e-6 or from 1e21 up. It used to use the 'g'
// format, whose 1e+06 for a million was valid JSON but not the spelling the
// rest of the module writes.
func JSONFloat(value float64) string {
	abs := value
	if abs < 0 {
		abs = -abs
	}
	format := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	out := strconv.FormatFloat(value, format, -1, 64)
	if format == 'e' {
		// encoding/json trims the exponent's leading zero: 1e-07, not 1e-007
		// is what strconv gives, and 1e-7 is what the wire carries.
		if n := len(out); n >= 4 && out[n-4] == 'e' && out[n-3] == '-' && out[n-2] == '0' {
			out = out[:n-2] + out[n-1:]
		}
	}
	return out
}

// JSONOptional encodes a nil pointer as JSON null and otherwise delegates.
func JSONOptional[T any](value *T, encode func(T) string) string {
	if value == nil {
		return "null"
	}
	return encode(*value)
}

// JSONMember appends one member to a JSON object body under assembly, writing
// the separator only when something precedes it.
//
// It exists so a component's emitted parameters can omit an absent optional
// rather than writing null: the generated code guards the call, and a member
// that is never appended leaves no trace. One absence for JavaScript to test
// beats two, and it matches what the attribute context already does when it
// omits a whole attribute rather than emitting an empty one.
func JSONMember(body, name, encoded string) string {
	member := JSONString(name) + ":" + encoded
	if body == "" {
		return member
	}
	return body + "," + member
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
