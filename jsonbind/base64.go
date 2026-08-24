package jsonbind

import (
	"encoding/base64"
	"slices"
	"strconv"
)

// A byte slice is a string on the wire, not a list of numbers.
//
// encoding/json and encoding/json/v2 both write one as base64, and a client
// reading JSON expects that; a list of numbers would also be roughly three
// times the bytes for the same payload. The Go type carries the same statement
// either way, so the wire form follows the convention rather than the mapping
// that happens to fall out of uint8.
//
// Padded standard encoding is what both of those write, so it is what a round
// trip through another decoder has to survive.

// AppendBase64 appends v as a base64 JSON string.
//
// A nil slice and an empty one both write "", which is the rule this codec
// already applies to a nil slice and a nil map: nothing on the Go side
// separates "no bytes" from "an empty blob", so nothing on the wire does
// either. encoding/json writes null for the nil case.
func AppendBase64(dst []byte, v []byte) []byte {
	dst = append(dst, '"')
	if n := base64.StdEncoding.EncodedLen(len(v)); n > 0 {
		start := len(dst)
		dst = slices.Grow(dst, n)[:start+n]
		base64.StdEncoding.Encode(dst[start:], v)
	}
	return append(dst, '"')
}

// ParseBase64 decodes a base64 JSON string member into a new slice. A null
// member decodes as a nil slice, which leaves an already-bound field alone the
// way a null array does.
func ParseBase64(p *Parser, field string) ([]byte, error) {
	src, isNull, err := base64Span(p, field)
	if err != nil || isNull {
		return nil, err
	}
	out := make([]byte, base64.StdEncoding.DecodedLen(len(src)))
	n, err := base64.StdEncoding.Decode(out, src)
	if err != nil {
		return nil, FieldError(field, "invalid base64", err)
	}
	return out[:n], nil
}

// ParseBase64Into decodes a base64 JSON string member into a fixed-length
// destination, under the same two-ended contract [ParseArray] has: a short
// payload fills what arrived and zeroes the rest, and one too long to fit is
// [ErrArrayTooLong] rather than a blob quietly cut to length.
func ParseBase64Into(p *Parser, field string, dst []byte) error {
	src, isNull, err := base64Span(p, field)
	if err != nil || isNull {
		return err
	}
	// DecodedLen overshoots by up to two bytes for a padded payload, so a value
	// it calls too long may still fit. Only that case pays for a scratch
	// buffer; the exact count is what decides either way.
	buf, direct := dst, true
	if base64.StdEncoding.DecodedLen(len(src)) > len(dst) {
		buf, direct = make([]byte, base64.StdEncoding.DecodedLen(len(src))), false
	}
	n, err := base64.StdEncoding.Decode(buf, src)
	if err != nil {
		return FieldError(field, "invalid base64", err)
	}
	if n > len(dst) {
		return FieldError(field, "expected at most "+strconv.Itoa(len(dst))+" bytes", ErrArrayTooLong)
	}
	if !direct {
		copy(dst, buf[:n])
	}
	for i := n; i < len(dst); i++ {
		dst[i] = 0
	}
	return nil
}

// base64Span returns the encoded payload without copying it, which is what
// keeps a large blob to the one allocation its decoded form needs.
//
// An escape inside a base64 payload is legal JSON and vanishingly rare, so it
// goes through the parser's unescape scratch rather than growing a decoder that
// handles escapes as it reads.
func base64Span(p *Parser, field string) (src []byte, isNull bool, err error) {
	if p.IsNull() {
		return nil, true, nil
	}
	span, work, err := p.stringSpan()
	if err != nil {
		return nil, false, FieldError(field, "invalid base64", err)
	}
	if !work {
		return span, false, nil
	}
	p.scratch = unescape(p.scratch[:0], span)
	return p.scratch, false, nil
}
