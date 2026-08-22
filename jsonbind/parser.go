package jsonbind

import (
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// Parser reads a JSON document in a single forward pass. Generated codecs drive
// it directly: values are parsed out of the input buffer in place, so decoding
// allocates only the strings, slices and maps that end up in the result.
//
// It does not use reflect and does not import encoding/json.
type Parser struct {
	buf []byte
	pos int
	// Keys and values get separate scratch buffers so a decoded key stays
	// valid while the matching value is being decoded.
	keyScratch []byte
	scratch    []byte
}

// NewParser returns a Parser reading data. data is not copied, and values
// returned by RawValue alias it.
func NewParser(data []byte) *Parser { return &Parser{buf: data} }

// Reset points p at data, reusing its scratch buffers.
func (p *Parser) Reset(data []byte) { p.buf, p.pos = data, 0 }

func (p *Parser) fail(msg string) error { return newError("json_parse", msg, nil) }

func (p *Parser) ws() {
	for p.pos < len(p.buf) {
		switch p.buf[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

// ObjectStart consumes '{'. A JSON null reports isNull and consumes the literal.
func (p *Parser) ObjectStart() (isNull bool, err error) {
	p.ws()
	if p.pos >= len(p.buf) {
		return false, p.fail("unexpected end of JSON input")
	}
	if p.buf[p.pos] == 'n' {
		if err := p.readNull(); err != nil {
			return false, err
		}
		return true, nil
	}
	if p.buf[p.pos] != '{' {
		return false, p.fail("JSON value must be an object")
	}
	p.pos++
	return false, nil
}

// ObjectKey returns the next member name, or ok=false at '}'. n is the
// zero-based member index and tells the parser whether a comma is required, so
// the parser needs no nesting stack of its own.
//
// The returned key aliases parser scratch space and is only valid until the
// next ObjectKey call.
func (p *Parser) ObjectKey(n int) (key []byte, ok bool, err error) {
	span, escaped, ok, err := p.objectKeySpan(n)
	if err != nil || !ok {
		return nil, ok, err
	}
	if escaped {
		p.keyScratch = unescape(p.keyScratch[:0], span)
		span = p.keyScratch
	}
	return span, true, nil
}

// objectKeySpan is ObjectKey without the unescaping step. The span always
// aliases the input buffer, so a caller that needs the name to outlive the next
// member can keep or copy it on its own terms.
func (p *Parser) objectKeySpan(n int) (span []byte, escaped, ok bool, err error) {
	p.ws()
	if p.pos >= len(p.buf) {
		return nil, false, false, p.fail("unexpected end of JSON object")
	}
	if p.buf[p.pos] == '}' {
		p.pos++
		return nil, false, false, nil
	}
	if n > 0 {
		if p.buf[p.pos] != ',' {
			return nil, false, false, p.fail("missing ',' in JSON object")
		}
		p.pos++
		p.ws()
	}
	span, escaped, err = p.stringSpan()
	if err != nil {
		return nil, false, false, err
	}
	p.ws()
	if p.pos >= len(p.buf) || p.buf[p.pos] != ':' {
		return nil, false, false, p.fail("missing ':' after JSON object key")
	}
	p.pos++
	return span, escaped, true, nil
}

// ArrayStart consumes '['. A JSON null reports isNull and consumes the literal.
func (p *Parser) ArrayStart() (isNull bool, err error) {
	p.ws()
	if p.pos >= len(p.buf) {
		return false, p.fail("unexpected end of JSON input")
	}
	if p.buf[p.pos] == 'n' {
		if err := p.readNull(); err != nil {
			return false, err
		}
		return true, nil
	}
	if p.buf[p.pos] != '[' {
		return false, p.fail("JSON value must be an array")
	}
	p.pos++
	return false, nil
}

// ArrayNext reports whether another element follows. n is the element index.
func (p *Parser) ArrayNext(n int) (bool, error) {
	p.ws()
	if p.pos >= len(p.buf) {
		return false, p.fail("unexpected end of JSON array")
	}
	if p.buf[p.pos] == ']' {
		p.pos++
		return false, nil
	}
	if n > 0 {
		if p.buf[p.pos] != ',' {
			return false, p.fail("missing ',' in JSON array")
		}
		p.pos++
	}
	return true, nil
}

func (p *Parser) readNull() error {
	if p.pos+4 <= len(p.buf) && string(p.buf[p.pos:p.pos+4]) == "null" {
		p.pos += 4
		return nil
	}
	return p.fail("invalid JSON literal")
}

// IsNull consumes a null literal when the next value is null.
func (p *Parser) IsNull() bool {
	p.ws()
	if p.pos+4 <= len(p.buf) && string(p.buf[p.pos:p.pos+4]) == "null" {
		p.pos += 4
		return true
	}
	return false
}

// stringSpan returns the bytes between the quotes without copying. needsWork is
// set when the span holds an escape or any non-ASCII byte, so a pure-ASCII
// string skips both unescaping and UTF-8 validation.
func (p *Parser) stringSpan() (span []byte, needsWork bool, err error) {
	p.ws()
	if p.pos >= len(p.buf) || p.buf[p.pos] != '"' {
		return nil, false, p.fail("JSON value must be a string")
	}
	start := p.pos + 1
	i := start
	for i < len(p.buf) {
		c := p.buf[i]
		if c == '"' {
			p.pos = i + 1
			return p.buf[start:i], needsWork, nil
		}
		if c == '\\' {
			needsWork = true
			i += 2
			continue
		}
		if c < 0x20 {
			return nil, false, p.fail("invalid control character in JSON string")
		}
		if c >= utf8.RuneSelf {
			needsWork = true
		}
		i++
	}
	return nil, false, p.fail("unterminated JSON string")
}

// String decodes a JSON string. null decodes as "".
func (p *Parser) String() (string, error) {
	if p.IsNull() {
		return "", nil
	}
	span, work, err := p.stringSpan()
	if err != nil {
		return "", err
	}
	if !work {
		return string(span), nil
	}
	p.scratch = unescape(p.scratch[:0], span)
	if !utf8.Valid(p.scratch) {
		p.scratch = coerceUTF8(p.scratch)
	}
	return string(p.scratch), nil
}

// numberSpan returns the number token without copying.
func (p *Parser) numberSpan() ([]byte, error) {
	p.ws()
	start := p.pos
	for p.pos < len(p.buf) {
		c := p.buf[p.pos]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			p.pos++
			continue
		}
		break
	}
	if p.pos == start {
		return nil, p.fail("JSON value must be a number")
	}
	return p.buf[start:p.pos], nil
}

// Int decodes a JSON number as int. null decodes as 0.
func (p *Parser) Int() (int, error) {
	v, err := p.Int64()
	if err != nil {
		return 0, err
	}
	// int is 32-bit on TinyGo's wasm targets; do not truncate silently.
	if strconv.IntSize == 32 && (v > 1<<31-1 || v < -1<<31) {
		return 0, p.fail("JSON number out of range")
	}
	return int(v), nil
}

// Int64 decodes a JSON number as int64. null decodes as 0.
func (p *Parser) Int64() (int64, error) {
	if p.IsNull() {
		return 0, nil
	}
	span, err := p.numberSpan()
	if err != nil {
		return 0, err
	}
	return parseInt64(span)
}

// ErrIntegerRange is returned by generated code when a JSON integer is
// outside the range of the field's declared width. The generated codec makes
// the comparison, because the bound is a constant it knows at generation; this
// is the error it has to name, and exporting one value keeps the check from
// needing a fmt or errors import inside a generated file.
var ErrIntegerRange = newError("json_parse", "JSON number out of range", nil)

// Uint64 decodes a JSON number as uint64. null decodes as 0, and a negative
// number is an error rather than a wrapped value.
//
// This is the one unsigned reader. The narrower unsigned widths are read
// through it and range-checked by the generated codec against bounds it knows
// at generation, which keeps eight more methods out of the runtime a TinyGo
// target links.
func (p *Parser) Uint64() (uint64, error) {
	if p.IsNull() {
		return 0, nil
	}
	span, err := p.numberSpan()
	if err != nil {
		return 0, err
	}
	return parseUint64(span)
}

// Float64 decodes a JSON number. null decodes as 0.
func (p *Parser) Float64() (float64, error) {
	if p.IsNull() {
		return 0, nil
	}
	span, err := p.numberSpan()
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(string(span), 64)
	if err != nil {
		return 0, p.fail("invalid JSON number")
	}
	return v, nil
}

// Bool decodes a JSON boolean. null decodes as false.
func (p *Parser) Bool() (bool, error) {
	p.ws()
	switch {
	case p.pos+4 <= len(p.buf) && string(p.buf[p.pos:p.pos+4]) == "true":
		p.pos += 4
		return true, nil
	case p.pos+5 <= len(p.buf) && string(p.buf[p.pos:p.pos+5]) == "false":
		p.pos += 5
		return false, nil
	case p.pos+4 <= len(p.buf) && string(p.buf[p.pos:p.pos+4]) == "null":
		p.pos += 4
		return false, nil
	}
	return false, p.fail("JSON value must be a boolean")
}

// RawValue returns the next value's bytes as a subslice of the input. The
// result aliases the parser's buffer and must be copied to outlive it.
func (p *Parser) RawValue() ([]byte, error) {
	p.ws()
	start := p.pos
	if err := p.SkipValue(); err != nil {
		return nil, err
	}
	return p.buf[start:p.pos], nil
}

// Any decodes an arbitrary JSON value into the same Go shapes encoding/json
// produces for an `any` destination.
func (p *Parser) Any() (any, error) {
	p.ws()
	if p.pos >= len(p.buf) {
		return nil, p.fail("unexpected end of JSON input")
	}
	switch c := p.buf[p.pos]; {
	case c == '{':
		if _, err := p.ObjectStart(); err != nil {
			return nil, err
		}
		out := map[string]any{}
		for n := 0; ; n++ {
			key, ok, err := p.ObjectKey(n)
			if err != nil {
				return nil, err
			}
			if !ok {
				return out, nil
			}
			name := string(key)
			v, err := p.Any()
			if err != nil {
				return nil, err
			}
			out[name] = v
		}
	case c == '[':
		if _, err := p.ArrayStart(); err != nil {
			return nil, err
		}
		out := []any{}
		for i := 0; ; i++ {
			more, err := p.ArrayNext(i)
			if err != nil {
				return nil, err
			}
			if !more {
				return out, nil
			}
			v, err := p.Any()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	case c == '"':
		return p.String()
	case c == 't' || c == 'f':
		return p.Bool()
	case c == 'n':
		if err := p.readNull(); err != nil {
			return nil, err
		}
		return nil, nil
	default:
		return p.Float64()
	}
}

// SkipValue advances past the next value, whatever its shape. Structure is
// validated; the value's contents are not interpreted.
func (p *Parser) SkipValue() error {
	p.ws()
	if p.pos >= len(p.buf) {
		return p.fail("unexpected end of JSON input")
	}
	switch c := p.buf[p.pos]; {
	case c == '{':
		if _, err := p.ObjectStart(); err != nil {
			return err
		}
		for n := 0; ; n++ {
			_, ok, err := p.ObjectKey(n)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			if err := p.SkipValue(); err != nil {
				return err
			}
		}
	case c == '[':
		if _, err := p.ArrayStart(); err != nil {
			return err
		}
		for i := 0; ; i++ {
			more, err := p.ArrayNext(i)
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
			if err := p.SkipValue(); err != nil {
				return err
			}
		}
	case c == '"':
		_, _, err := p.stringSpan()
		return err
	case c == 't':
		if p.pos+4 <= len(p.buf) && string(p.buf[p.pos:p.pos+4]) == "true" {
			p.pos += 4
			return nil
		}
		return p.fail("invalid JSON literal")
	case c == 'f':
		if p.pos+5 <= len(p.buf) && string(p.buf[p.pos:p.pos+5]) == "false" {
			p.pos += 5
			return nil
		}
		return p.fail("invalid JSON literal")
	case c == 'n':
		return p.readNull()
	default:
		_, err := p.numberSpan()
		return err
	}
}

// End reports an error when anything but whitespace follows the document.
func (p *Parser) End() error {
	p.ws()
	if p.pos != len(p.buf) {
		return p.fail("unexpected trailing data after JSON value")
	}
	return nil
}

func unescape(dst, src []byte) []byte {
	for i := 0; i < len(src); {
		c := src[i]
		if c != '\\' {
			dst = append(dst, c)
			i++
			continue
		}
		i++
		if i >= len(src) {
			break
		}
		switch src[i] {
		case '"', '\\', '/':
			dst = append(dst, src[i])
			i++
		case 'b':
			dst = append(dst, '\b')
			i++
		case 'f':
			dst = append(dst, '\f')
			i++
		case 'n':
			dst = append(dst, '\n')
			i++
		case 'r':
			dst = append(dst, '\r')
			i++
		case 't':
			dst = append(dst, '\t')
			i++
		case 'u':
			r := hex4(src[i+1:])
			i += 5
			if utf16.IsSurrogate(rune(r)) && i+5 < len(src) && src[i] == '\\' && src[i+1] == 'u' {
				r2 := hex4(src[i+2:])
				if dec := utf16.DecodeRune(rune(r), rune(r2)); dec != utf8.RuneError {
					dst = utf8.AppendRune(dst, dec)
					i += 6
					continue
				}
			}
			dst = utf8.AppendRune(dst, rune(r))
		default:
			dst = append(dst, src[i])
			i++
		}
	}
	return dst
}

func hex4(s []byte) uint32 {
	if len(s) < 4 {
		return utf8.RuneError
	}
	var v uint32
	for i := range 4 {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			v = v<<4 | uint32(c-'0')
		case c >= 'a' && c <= 'f':
			v = v<<4 | uint32(c-'a'+10)
		case c >= 'A' && c <= 'F':
			v = v<<4 | uint32(c-'A'+10)
		default:
			return utf8.RuneError
		}
	}
	return v
}

// coerceUTF8 rewrites invalid UTF-8 as U+FFFD, matching encoding/json. U+FFFD
// is wider than the byte it replaces, so this cannot run in place.
func coerceUTF8(b []byte) []byte {
	out := make([]byte, 0, len(b)+8)
	for i := 0; i < len(b); {
		if c := b[i]; c < utf8.RuneSelf {
			out = append(out, c)
			i++
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			out = utf8.AppendRune(out, utf8.RuneError)
			i++
			continue
		}
		out = append(out, b[i:i+size]...)
		i += size
	}
	return out
}

// parseInt64 accepts exactly what encoding/json accepts for an integer field:
// a plain decimal literal. Fractional and exponent forms such as 1.0 or 1e3
// are rejected there too, so they are rejected here.
// parseUint64 is parseInt64 without the sign, refusing a negative number
// rather than wrapping it, and allocating nothing.
func parseUint64(span []byte) (uint64, error) {
	if len(span) == 0 {
		return 0, newError("json_parse", "invalid JSON integer", nil)
	}
	if span[0] == '-' {
		return 0, newError("json_parse", "JSON number out of range", nil)
	}
	var acc uint64
	const cutoff = ^uint64(0)
	for i := 0; i < len(span); i++ {
		c := span[i]
		if c < '0' || c > '9' {
			return 0, newError("json_parse", "invalid JSON integer", nil)
		}
		if acc > (cutoff-uint64(c-'0'))/10 {
			return 0, newError("json_parse", "JSON number out of range", nil)
		}
		acc = acc*10 + uint64(c-'0')
	}
	return acc, nil
}

func parseInt64(span []byte) (int64, error) {
	i, neg := 0, false
	if len(span) > 0 && span[0] == '-' {
		neg, i = true, 1
	}
	if i == len(span) {
		return 0, newError("json_parse", "invalid JSON integer", nil)
	}
	var acc uint64
	const cutoff = uint64(1) << 63
	for ; i < len(span); i++ {
		c := span[i]
		if c < '0' || c > '9' {
			return 0, newError("json_parse", "invalid JSON integer", nil)
		}
		if acc > (cutoff-uint64(c-'0'))/10 {
			return 0, newError("json_parse", "JSON number out of range", nil)
		}
		acc = acc*10 + uint64(c-'0')
	}
	if neg {
		if acc > cutoff {
			return 0, newError("json_parse", "JSON number out of range", nil)
		}
		return -int64(acc), nil
	}
	if acc >= cutoff {
		return 0, newError("json_parse", "JSON number out of range", nil)
	}
	return int64(acc), nil
}
