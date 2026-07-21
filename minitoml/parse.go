package minitoml

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse parses restricted TOML bytes into an intermediate Document.
//
// Allowed: standard tables, nested tables, dotted bare keys, bare keys only,
// scalar values, and arrays of primitive scalars.
// Forbidden: quoted keys, inline tables, arrays of tables.
func Parse(data []byte) (Document, error) {
	p := &parser{
		src:  data,
		line: 1,
		col:  1,
		doc:  NewDocument(),
	}
	if err := p.parse(); err != nil {
		return Document{}, err
	}
	return p.doc, nil
}

// ParseString parses restricted TOML from a string.
func ParseString(s string) (Document, error) {
	return Parse([]byte(s))
}

type parser struct {
	src         []byte
	pos         int
	line        int
	col         int
	tablePrefix string
	doc         Document
}

func (p *parser) parse() error {
	for {
		p.skipSpaceAndComments()
		if p.eof() {
			return nil
		}
		if p.peek() == '\n' {
			p.advance()
			continue
		}
		if p.peek() == '[' {
			if err := p.parseTableHeader(); err != nil {
				return err
			}
			continue
		}
		if err := p.parseKeyValue(); err != nil {
			return err
		}
	}
}

func (p *parser) parseTableHeader() error {
	startLine, startCol := p.line, p.col
	p.advance() // [
	if p.peek() == '[' {
		return p.errorf(startLine, startCol, "arrays of tables are not allowed")
	}
	p.skipSpace()
	path, err := p.parseKeyPath()
	if err != nil {
		return err
	}
	p.skipSpace()
	if p.peek() != ']' {
		return p.errorf(p.line, p.col, "expected ']' after table name")
	}
	p.advance()
	p.skipSpace()
	if err := p.expectEOLOrComment(); err != nil {
		return err
	}
	p.tablePrefix = path
	return nil
}

func (p *parser) parseKeyValue() error {
	path, err := p.parseKeyPath()
	if err != nil {
		return err
	}
	p.skipSpace()
	if p.peek() != '=' {
		return p.errorf(p.line, p.col, "expected '=' after key")
	}
	p.advance()
	p.skipSpace()
	val, err := p.parseValue()
	if err != nil {
		return err
	}
	p.skipSpace()
	if err := p.expectEOLOrComment(); err != nil {
		return err
	}
	full := joinKey(p.tablePrefix, path)
	p.doc.Set(full, val)
	return nil
}

func (p *parser) parseKeyPath() (string, error) {
	var parts []string
	for {
		p.skipSpace()
		ch := p.peek()
		if ch == '"' || ch == '\'' {
			return "", p.errorf(p.line, p.col, "quoted keys are not allowed")
		}
		part, err := p.parseBareKey()
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
		p.skipSpace()
		if p.peek() != '.' {
			break
		}
		p.advance()
	}
	return strings.Join(parts, "."), nil
}

func (p *parser) parseBareKey() (string, error) {
	if p.eof() {
		return "", p.errorf(p.line, p.col, "expected bare key")
	}
	ch := p.peek()
	if !isBareKeyChar(ch) {
		return "", p.errorf(p.line, p.col, "expected bare key, got %q", ch)
	}
	start := p.pos
	for !p.eof() && isBareKeyChar(p.peek()) {
		p.advance()
	}
	return string(p.src[start:p.pos]), nil
}

func (p *parser) parseValue() (Value, error) {
	if p.eof() {
		return Value{}, p.errorf(p.line, p.col, "expected value")
	}
	ch := p.peek()
	switch ch {
	case '{':
		return Value{}, p.errorf(p.line, p.col, "inline tables are not allowed")
	case '[':
		return p.parseArray()
	case '"':
		s, err := p.parseBasicString()
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindString, Str: s}, nil
	case '\'':
		s, err := p.parseLiteralString()
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindString, Str: s}, nil
	case 't', 'f':
		return p.parseBool()
	case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.parseNumber()
	default:
		return Value{}, p.errorf(p.line, p.col, "unexpected value start %q", ch)
	}
}

func (p *parser) parseArray() (Value, error) {
	startLine, startCol := p.line, p.col
	p.advance() // [
	p.skipSpaceAndComments()
	if p.peek() == ']' {
		p.advance()
		return Value{Kind: KindArray, Array: nil}, nil
	}
	var elems []Value
	for {
		p.skipSpaceAndComments()
		if p.peek() == '[' {
			return Value{}, p.errorf(p.line, p.col, "arrays of tables and nested arrays are not allowed")
		}
		v, err := p.parseValue()
		if err != nil {
			return Value{}, err
		}
		if v.Kind == KindArray {
			return Value{}, p.errorf(startLine, startCol, "arrays of primitive scalars only; nested arrays are not allowed")
		}
		elems = append(elems, v)
		p.skipSpaceAndComments()
		if p.peek() == ',' {
			p.advance()
			p.skipSpaceAndComments()
			if p.peek() == ']' {
				p.advance()
				break
			}
			continue
		}
		if p.peek() == ']' {
			p.advance()
			break
		}
		return Value{}, p.errorf(p.line, p.col, "expected ',' or ']' in array")
	}
	return Value{Kind: KindArray, Array: elems}, nil
}

func (p *parser) parseBool() (Value, error) {
	if p.hasPrefix("true") {
		p.advanceN(4)
		return Value{Kind: KindBool, Bool: true}, nil
	}
	if p.hasPrefix("false") {
		p.advanceN(5)
		return Value{Kind: KindBool, Bool: false}, nil
	}
	return Value{}, p.errorf(p.line, p.col, "invalid boolean")
}

func (p *parser) parseNumber() (Value, error) {
	start := p.pos
	line, col := p.line, p.col
	if p.peek() == '+' || p.peek() == '-' {
		p.advance()
	}
	if p.eof() || !isDigit(p.peek()) {
		return Value{}, p.errorf(line, col, "invalid number")
	}
	for !p.eof() && (isDigit(p.peek()) || p.peek() == '_') {
		p.advance()
	}
	isFloat := false
	if p.peek() == '.' {
		isFloat = true
		p.advance()
		if p.eof() || !isDigit(p.peek()) {
			return Value{}, p.errorf(line, col, "invalid float")
		}
		for !p.eof() && (isDigit(p.peek()) || p.peek() == '_') {
			p.advance()
		}
	}
	if p.peek() == 'e' || p.peek() == 'E' {
		isFloat = true
		p.advance()
		if p.peek() == '+' || p.peek() == '-' {
			p.advance()
		}
		if p.eof() || !isDigit(p.peek()) {
			return Value{}, p.errorf(line, col, "invalid float exponent")
		}
		for !p.eof() && isDigit(p.peek()) {
			p.advance()
		}
	}
	raw := strings.ReplaceAll(string(p.src[start:p.pos]), "_", "")
	if isFloat {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return Value{}, p.errorf(line, col, "invalid float %q", raw)
		}
		return Value{Kind: KindFloat, Float: f}, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return Value{}, p.errorf(line, col, "invalid integer %q", raw)
	}
	return Value{Kind: KindInt, Int: n}, nil
}

func (p *parser) parseBasicString() (string, error) {
	line, col := p.line, p.col
	p.advance() // "
	var b strings.Builder
	for !p.eof() {
		ch := p.peek()
		if ch == '"' {
			p.advance()
			return b.String(), nil
		}
		if ch == '\n' || ch == '\r' {
			return "", p.errorf(line, col, "unterminated string")
		}
		if ch == '\\' {
			p.advance()
			if p.eof() {
				return "", p.errorf(line, col, "unterminated string escape")
			}
			esc := p.peek()
			p.advance()
			switch esc {
			case '"', '\\', '/':
				b.WriteByte(esc)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				return "", p.errorf(p.line, p.col, "unsupported string escape \\%c", esc)
			}
			continue
		}
		p.advance()
		b.WriteByte(ch)
	}
	return "", p.errorf(line, col, "unterminated string")
}

func (p *parser) parseLiteralString() (string, error) {
	line, col := p.line, p.col
	p.advance() // '
	start := p.pos
	for !p.eof() {
		ch := p.peek()
		if ch == '\'' {
			s := string(p.src[start:p.pos])
			p.advance()
			return s, nil
		}
		if ch == '\n' || ch == '\r' {
			return "", p.errorf(line, col, "unterminated literal string")
		}
		p.advance()
	}
	return "", p.errorf(line, col, "unterminated literal string")
}

func (p *parser) expectEOLOrComment() error {
	p.skipSpace()
	if p.eof() || p.peek() == '\n' || p.peek() == '\r' || p.peek() == '#' {
		if p.peek() == '#' {
			p.skipComment()
		}
		return nil
	}
	return p.errorf(p.line, p.col, "unexpected trailing content %q", p.peek())
}

func (p *parser) skipSpaceAndComments() {
	for !p.eof() {
		p.skipSpace()
		if p.peek() == '#' {
			p.skipComment()
			continue
		}
		if p.peek() == '\n' || p.peek() == '\r' {
			p.advance()
			continue
		}
		return
	}
}

func (p *parser) skipSpace() {
	for !p.eof() {
		ch := p.peek()
		if ch == ' ' || ch == '\t' {
			p.advance()
			continue
		}
		return
	}
}

func (p *parser) skipComment() {
	for !p.eof() && p.peek() != '\n' && p.peek() != '\r' {
		p.advance()
	}
}

func (p *parser) eof() bool {
	return p.pos >= len(p.src)
}

func (p *parser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) advance() {
	if p.eof() {
		return
	}
	ch := p.src[p.pos]
	p.pos++
	if ch == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
}

func (p *parser) advanceN(n int) {
	for i := 0; i < n; i++ {
		p.advance()
	}
}

func (p *parser) hasPrefix(s string) bool {
	if p.pos+len(s) > len(p.src) {
		return false
	}
	return string(p.src[p.pos:p.pos+len(s)]) == s
}

func (p *parser) errorf(line, col int, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("minitoml:%d:%d: %s", line, col, msg)
}

func joinKey(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "" {
		return prefix
	}
	return prefix + "." + path
}

func isBareKeyChar(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_' || ch == '-'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
