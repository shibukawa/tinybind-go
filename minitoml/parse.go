package minitoml

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse parses restricted TOML bytes into an intermediate Document.
//
// Allowed: standard tables, nested tables, dotted bare keys, bare keys only,
// scalar values, arrays of primitive scalars, and arrays of tables ([[key]]).
// Forbidden: quoted keys, inline tables, arrays of inline tables.
func Parse(data []byte) (Document, error) {
	p := &parser{
		src:  data,
		line: 1,
		col:  1,
		doc:  NewDocument(),
	}
	p.cur = p.doc
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
	src  []byte
	pos  int
	line int
	col  int
	doc  Document
	// cur is the document key/value pairs land in: the root document, or the
	// element document of the innermost open [[array]] header.
	cur Document
	// tablePrefix is the current [table] path relative to cur.
	tablePrefix string
	// scopes are the open [[array]] headers, outermost first. Every element
	// document is created by NewDocument, so writing through a Document copy
	// updates the shared map the parent value already holds.
	scopes []arrayScope
}

// arrayScope is one open array-of-tables element: the header path as written in
// the file, and the document its keys are written into.
type arrayScope struct {
	path string
	doc  Document
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
	isArray := false
	if p.peek() == '[' {
		isArray = true
		p.advance()
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
	if isArray {
		if p.peek() != ']' {
			return p.errorf(p.line, p.col, "expected ']]' after array of tables name")
		}
		p.advance()
	}
	p.skipSpace()
	if err := p.expectEOLOrComment(); err != nil {
		return err
	}

	target, rel := p.enterScope(path)
	if isArray {
		return p.beginTableArrayElement(target, path, rel, startLine, startCol)
	}
	if err := p.checkTableArrayConflict(target, rel, path, startLine, startCol); err != nil {
		return err
	}
	p.cur = target
	p.tablePrefix = rel
	return nil
}

// enterScope closes the open array-of-tables scopes that path leaves behind and
// returns the document the header writes into, plus path relative to it.
func (p *parser) enterScope(path string) (Document, string) {
	for len(p.scopes) > 0 {
		top := p.scopes[len(p.scopes)-1]
		if strings.HasPrefix(path, top.path+".") {
			return top.doc, path[len(top.path)+1:]
		}
		p.scopes = p.scopes[:len(p.scopes)-1]
	}
	return p.doc, path
}

// beginTableArrayElement appends one element to the array of tables at rel and
// makes it the destination for following keys.
func (p *parser) beginTableArrayElement(target Document, path, rel string, line, col int) error {
	if err := p.checkAncestorTableArray(target, rel, path, line, col); err != nil {
		return err
	}
	v, ok := target.Get(rel)
	if ok && v.Kind != KindTableArray {
		return p.errorf(line, col, "[[%s]] conflicts with key %q already defined as %s", path, path, v.KindName())
	}
	if !ok {
		v = Value{Kind: KindTableArray}
	}
	elem := NewDocument()
	v.Tables = append(v.Tables, elem)
	target.Set(rel, v)
	p.scopes = append(p.scopes, arrayScope{path: path, doc: elem})
	p.cur = elem
	p.tablePrefix = ""
	return nil
}

// checkTableArrayConflict rejects a key or [table] header that would reopen or
// reach through an array of tables in target.
func (p *parser) checkTableArrayConflict(target Document, rel, path string, line, col int) error {
	if v, ok := target.Get(rel); ok && v.Kind == KindTableArray {
		return p.errorf(line, col, "%q is an array of tables; use a [[%s]] header", path, path)
	}
	return p.checkAncestorTableArray(target, rel, path, line, col)
}

// checkAncestorTableArray rejects a path whose parent is an array of tables, so
// [servers.tls] never silently lands outside the [[servers]] element it reads as.
func (p *parser) checkAncestorTableArray(target Document, rel, path string, line, col int) error {
	base := path[:len(path)-len(rel)]
	for i := 0; i < len(rel); i++ {
		if rel[i] != '.' {
			continue
		}
		v, ok := target.Get(rel[:i])
		if !ok || v.Kind != KindTableArray {
			continue
		}
		ancestor := base + rel[:i]
		return p.errorf(line, col, "%q is an array of tables; %q must follow its [[%s]] header", ancestor, path, ancestor)
	}
	return nil
}

func (p *parser) parseKeyValue() error {
	keyLine, keyCol := p.line, p.col
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
	if err := p.checkTableArrayConflict(p.cur, full, joinKey(p.basePath(), full), keyLine, keyCol); err != nil {
		return err
	}
	p.cur.Set(full, val)
	return nil
}

// basePath is the header path of the document keys currently land in, so
// diagnostics can name a key by its full path in the file.
func (p *parser) basePath() string {
	if len(p.scopes) == 0 {
		return ""
	}
	return p.scopes[len(p.scopes)-1].path
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
			return Value{}, p.errorf(p.line, p.col, "nested arrays are not allowed")
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
