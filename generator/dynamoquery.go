package generator

import (
	"fmt"
	"strings"
	"unicode"
)

// DefaultDynamoTemplatePattern is the base-name glob for query declarations,
// beside the HTML and SQL template patterns.
const DefaultDynamoTemplatePattern = "*.tb.dynamo"

// DynamoResultShape is what a declaration asks the generated function to
// return. It selects the request shape rather than a row count: a Query always
// returns many, and the choice is whether the caller sees the page boundaries.
type DynamoResultShape string

const (
	// DynamoPage issues one request and returns a page.
	DynamoPage DynamoResultShape = "page"
	// DynamoMany iterates every page.
	DynamoMany DynamoResultShape = "many"
)

// DynamoOp is a key condition operator.
type DynamoOp string

const (
	DynamoEqual          DynamoOp = "="
	DynamoLess           DynamoOp = "<"
	DynamoLessOrEqual    DynamoOp = "<="
	DynamoGreater        DynamoOp = ">"
	DynamoGreaterOrEqual DynamoOp = ">="
	DynamoBetween        DynamoOp = "between"
	DynamoBeginsWith     DynamoOp = "begins_with"
)

// DynamoQueryParam is one declared parameter of a query function.
type DynamoQueryParam struct {
	Name string
	// Type is the Go type as the declaration spells it, checked later against
	// the attribute's own Go type.
	Type string
	Line int
}

// DynamoPredicate is one comparison in a key clause. Params holds one name, or
// two for BETWEEN.
type DynamoPredicate struct {
	Attribute string
	Op        DynamoOp
	Params    []string
	Line      int
}

// DynamoQueryDecl is one declared access pattern.
type DynamoQueryDecl struct {
	Name       string
	Exported   bool
	Params     []DynamoQueryParam
	Shape      DynamoResultShape
	ItemType   string
	Key        []DynamoPredicate
	SourcePath string
	Line       int
}

// parseDynamoQueries reads every declaration in one .tb.dynamo source.
func parseDynamoQueries(path string, source []byte) ([]DynamoQueryDecl, error) {
	p := &dynamoParser{file: path, tokens: lexDynamo(string(source))}
	return p.parseAll()
}

// dynamoToken is one lexical item. The grammar is small enough that a token is
// its own text plus the line it came from.
type dynamoToken struct {
	text string
	line int
}

func (t dynamoToken) is(text string) bool { return t.text == text }

// lexDynamo splits a declaration source into tokens. Punctuation is single
// character except for the two-character comparisons, and a comment runs to the
// end of its line.
func lexDynamo(source string) []dynamoToken {
	var out []dynamoToken
	line := 1
	for i := 0; i < len(source); {
		c := source[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case c == '<' || c == '>':
			if i+1 < len(source) && source[i+1] == '=' {
				out = append(out, dynamoToken{source[i : i+2], line})
				i += 2
				continue
			}
			out = append(out, dynamoToken{string(c), line})
			i++
		case strings.ContainsRune("(){},:=[]", rune(c)):
			out = append(out, dynamoToken{string(c), line})
			i++
		default:
			start := i
			for i < len(source) && isDynamoWordByte(source[i]) {
				i++
			}
			if i == start {
				// An unknown byte becomes its own token so the parser can name
				// it rather than looping.
				out = append(out, dynamoToken{string(c), line})
				i++
				continue
			}
			out = append(out, dynamoToken{source[start:i], line})
		}
	}
	return out
}

func isDynamoWordByte(c byte) bool {
	return c == '_' || c == '.' || c == '*' ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		c >= 0x80 // a multi-byte identifier stays one token, since Go permits one
}

type dynamoParser struct {
	file   string
	tokens []dynamoToken
	at     int
}

func (p *dynamoParser) peek() dynamoToken {
	if p.at >= len(p.tokens) {
		return dynamoToken{"", p.lastLine()}
	}
	return p.tokens[p.at]
}

func (p *dynamoParser) next() dynamoToken {
	t := p.peek()
	if p.at < len(p.tokens) {
		p.at++
	}
	return t
}

func (p *dynamoParser) lastLine() int {
	if len(p.tokens) == 0 {
		return 1
	}
	return p.tokens[len(p.tokens)-1].line
}

func (p *dynamoParser) done() bool { return p.at >= len(p.tokens) }

func (p *dynamoParser) errorf(line int, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", p.file, line, fmt.Sprintf(format, args...))
}

// expect consumes one token and reports what was found when it is not the one
// wanted, since a declaration is hand-written and the message is the diagnosis.
func (p *dynamoParser) expect(text string) (dynamoToken, error) {
	t := p.next()
	if !t.is(text) {
		return t, p.errorf(t.line, "expected %q, found %s", text, describeDynamoToken(t))
	}
	return t, nil
}

func describeDynamoToken(t dynamoToken) string {
	if t.text == "" {
		return "end of file"
	}
	return fmt.Sprintf("%q", t.text)
}

// parseAll reads every declaration in one source.
func (p *dynamoParser) parseAll() ([]DynamoQueryDecl, error) {
	var out []DynamoQueryDecl
	for !p.done() {
		decl, err := p.statement()
		if err != nil {
			return nil, err
		}
		out = append(out, decl)
	}
	return out, nil
}

func (p *dynamoParser) statement() (DynamoQueryDecl, error) {
	var decl DynamoQueryDecl
	t := p.next()
	if t.is("export") {
		decl.Exported = true
		t = p.next()
	}
	if !t.is("statement") {
		return decl, p.errorf(t.line, "expected \"statement\" or \"export statement\", found %s", describeDynamoToken(t))
	}
	decl.Line = t.line

	name := p.next()
	if !isDynamoIdentifier(name.text) {
		return decl, p.errorf(name.line, "expected a statement name, found %s", describeDynamoToken(name))
	}
	decl.Name = name.text

	params, err := p.params()
	if err != nil {
		return decl, err
	}
	decl.Params = params

	if _, err := p.expect(":"); err != nil {
		return decl, err
	}
	shape, item, err := p.resultType()
	if err != nil {
		return decl, err
	}
	decl.Shape, decl.ItemType = shape, item

	if _, err := p.expect("{"); err != nil {
		return decl, err
	}
	for {
		t := p.peek()
		if t.is("}") {
			p.next()
			break
		}
		if t.text == "" {
			return decl, p.errorf(t.line, "unterminated statement %s", decl.Name)
		}
		switch t.text {
		case "key":
			p.next()
			if decl.Key != nil {
				return decl, p.errorf(t.line, "statement %s declares more than one key clause", decl.Name)
			}
			decl.Key, err = p.keyClause()
			if err != nil {
				return decl, err
			}
		case "filter":
			return decl, p.errorf(t.line, "a filter clause is not supported yet; this round declares key conditions only")
		default:
			return decl, p.errorf(t.line, "expected a clause, found %s", describeDynamoToken(t))
		}
	}
	if len(decl.Key) == 0 {
		return decl, p.errorf(decl.Line, "statement %s declares no key clause", decl.Name)
	}
	return decl, nil
}

func (p *dynamoParser) params() ([]DynamoQueryParam, error) {
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	var out []DynamoQueryParam
	if p.peek().is(")") {
		p.next()
		return out, nil
	}
	for {
		name := p.next()
		if !isDynamoIdentifier(name.text) {
			return nil, p.errorf(name.line, "expected a parameter name, found %s", describeDynamoToken(name))
		}
		if _, err := p.expect(":"); err != nil {
			return nil, err
		}
		typeName, err := p.goType()
		if err != nil {
			return nil, err
		}
		out = append(out, DynamoQueryParam{Name: name.text, Type: typeName, Line: name.line})

		t := p.next()
		if t.is(")") {
			return out, nil
		}
		if !t.is(",") {
			return nil, p.errorf(t.line, "expected \",\" or \")\" in the parameter list, found %s", describeDynamoToken(t))
		}
	}
}

// goType reads a Go type as the declaration spells it. Only the forms a key
// attribute can have are reachable: a name, a qualified name, or a byte slice.
func (p *dynamoParser) goType() (string, error) {
	t := p.next()
	if t.is("[") {
		if _, err := p.expect("]"); err != nil {
			return "", err
		}
		elem := p.next()
		if !isDynamoIdentifier(elem.text) {
			return "", p.errorf(elem.line, "expected a slice element type, found %s", describeDynamoToken(elem))
		}
		return "[]" + elem.text, nil
	}
	if !isDynamoIdentifier(t.text) {
		return "", p.errorf(t.line, "expected a type, found %s", describeDynamoToken(t))
	}
	return t.text, nil
}

func (p *dynamoParser) resultType() (DynamoResultShape, string, error) {
	t := p.next()
	shape := DynamoResultShape("")
	switch t.text {
	case "dynamo.many":
		shape = DynamoMany
	case "dynamo.page":
		shape = DynamoPage
	default:
		return "", "", p.errorf(t.line, "expected \"dynamo.many<T>\" or \"dynamo.page<T>\", found %s", describeDynamoToken(t))
	}
	if _, err := p.expect("<"); err != nil {
		return "", "", err
	}
	item := p.next()
	if !isDynamoIdentifier(item.text) {
		return "", "", p.errorf(item.line, "expected an item type, found %s", describeDynamoToken(item))
	}
	if _, err := p.expect(">"); err != nil {
		return "", "", err
	}
	return shape, item.text, nil
}

// keyClause parses the key condition: one predicate, optionally joined to a
// second by "and".
func (p *dynamoParser) keyClause() ([]DynamoPredicate, error) {
	var out []DynamoPredicate
	for {
		predicate, err := p.predicate()
		if err != nil {
			return nil, err
		}
		out = append(out, predicate)
		if !p.peek().is("and") {
			return out, nil
		}
		p.next()
	}
}

func (p *dynamoParser) predicate() (DynamoPredicate, error) {
	var out DynamoPredicate
	t := p.next()
	out.Line = t.line

	if t.is("begins_with") {
		if _, err := p.expect("("); err != nil {
			return out, err
		}
		attribute := p.next()
		if !isDynamoAttributeName(attribute.text) {
			return out, p.errorf(attribute.line, "expected an attribute name, found %s", describeDynamoToken(attribute))
		}
		if _, err := p.expect(","); err != nil {
			return out, err
		}
		param, err := p.placeholder()
		if err != nil {
			return out, err
		}
		if _, err := p.expect(")"); err != nil {
			return out, err
		}
		out.Attribute, out.Op, out.Params = attribute.text, DynamoBeginsWith, []string{param}
		return out, nil
	}

	if !isDynamoAttributeName(t.text) {
		return out, p.errorf(t.line, "expected an attribute name or begins_with, found %s", describeDynamoToken(t))
	}
	out.Attribute = t.text

	op := p.next()
	switch op.text {
	case "=", "<", "<=", ">", ">=":
		out.Op = DynamoOp(op.text)
		param, err := p.placeholder()
		if err != nil {
			return out, err
		}
		out.Params = []string{param}
	case "between":
		out.Op = DynamoBetween
		low, err := p.placeholder()
		if err != nil {
			return out, err
		}
		if _, err := p.expect("and"); err != nil {
			return out, err
		}
		high, err := p.placeholder()
		if err != nil {
			return out, err
		}
		out.Params = []string{low, high}
	default:
		return out, p.errorf(op.line, "expected a comparison after %s, found %s", out.Attribute, describeDynamoToken(op))
	}
	return out, nil
}

// placeholder reads "{name}", which is how a declaration names a parameter,
// matching the SQL template's own value syntax.
func (p *dynamoParser) placeholder() (string, error) {
	if _, err := p.expect("{"); err != nil {
		return "", err
	}
	name := p.next()
	if !isDynamoIdentifier(name.text) {
		return "", p.errorf(name.line, "expected a parameter name in braces, found %s", describeDynamoToken(name))
	}
	if _, err := p.expect("}"); err != nil {
		return "", err
	}
	return name.text, nil
}

func isDynamoIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || unicode.IsLetter(r):
		case i > 0 && unicode.IsDigit(r):
		case i > 0 && r == '.': // a qualified type such as time.Time
		default:
			return false
		}
	}
	return true
}

// isDynamoAttributeName accepts what DynamoDB accepts as an attribute name in
// an expression, which is wider than a Go identifier.
func isDynamoAttributeName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == '_' || r == '-' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}
