// Package firestorebind parses and prints typed Firestore Datastore
// access-pattern sources.
//
// The grammar is the .tb.dynamo one with the parts Datastore does not have
// removed and the parts it does have added. It declares no kind: a kind belongs
// to the type, not to the deployment, so the result type already says it. That
// is the one structural difference from the DynamoDB grammar, and it is why the
// two stay separate packages rather than one with a mode flag.
package firestorebind

import (
	"fmt"
	"strings"
	"unicode"
)

// DefaultTemplatePattern is the base-name glob for query declarations, beside
// the HTML, SQL and DynamoDB template patterns.
const DefaultTemplatePattern = "*.tb.firestore"

// ResultShape is what a declaration asks the generated function to return. It
// selects the request shape rather than a row count: a query always returns
// many, and the choice is whether the caller sees the batch boundaries.
type ResultShape string

const (
	// Batch issues one request and returns a page.
	Batch ResultShape = "batch"
	// Many iterates every batch.
	Many ResultShape = "many"
	// Count runs an aggregation query and decodes no entity.
	Count ResultShape = "count"
	// Keys runs a keys-only query, which is the cheap way to test existence in
	// bulk.
	Keys ResultShape = "keys"
)

// Op is a property filter comparison.
type Op string

const (
	OpEqual          Op = "=="
	OpNotEqual       Op = "!="
	OpLess           Op = "<"
	OpLessOrEqual    Op = "<="
	OpGreater        Op = ">"
	OpGreaterOrEqual Op = ">="
	OpIn             Op = "in"
	OpNotIn          Op = "not in"
)

// Multi reports whether the operator takes a slice of candidates rather than
// one value.
func (o Op) Multi() bool { return o == OpIn || o == OpNotIn }

// Direction is a sort direction.
type Direction string

const (
	Ascending  Direction = "asc"
	Descending Direction = "desc"
)

// QueryParam is one declared parameter of a query function.
type QueryParam struct {
	Name string
	// Type is the Go type as the declaration spells it, checked later against
	// the property's own Go type.
	Type string
	Line int
}

// Predicate is one comparison in a where clause.
type Predicate struct {
	Property string
	Op       Op
	Param    string
	Line     int
}

// Junction is how a Condition joins its operands.
type Junction string

const (
	// JunctionAnd requires every operand.
	JunctionAnd Junction = "and"
	// JunctionOr requires at least one.
	JunctionOr Junction = "or"
)

// Condition is one node of a where clause: either a comparison or a junction of
// other conditions.
//
// A where clause is a tree rather than a list because Datastore composes with
// both AND and OR. It was AND-only when this grammar was written, and the parser
// said so while rejecting or; the driver gained OR in tinygodriver v1.1.6 after
// this side asked whether the claim still held.
//
// Exactly one of Predicate and Operands is set. A leaf carries the comparison; a
// junction carries its operands and the word that joins them.
type Condition struct {
	Predicate *Predicate
	Junction  Junction
	Operands  []Condition
	Line      int
}

// IsLeaf reports whether the condition is a single comparison.
func (c Condition) IsLeaf() bool { return c.Predicate != nil }

// Walk calls visit on every leaf, in the order the source wrote them, which is
// the order a generated function binds its parameters in.
func (c Condition) Walk(visit func(*Predicate) error) error {
	if c.Predicate != nil {
		return visit(c.Predicate)
	}
	for i := range c.Operands {
		if err := c.Operands[i].Walk(visit); err != nil {
			return err
		}
	}
	return nil
}

// HasOr reports whether any junction below this one is an or, which is what
// decides whether generated code needs the condition tree at all.
func (c Condition) HasOr() bool {
	if c.Predicate != nil {
		return false
	}
	if c.Junction == JunctionOr {
		return true
	}
	for _, operand := range c.Operands {
		if operand.HasOr() {
			return true
		}
	}
	return false
}

// Flatten returns the leaves of an all-and tree, and reports whether the tree is
// one. The emitter uses it to keep the common declaration generating the same
// per-predicate Filter calls it always has.
func (c Condition) Flatten() ([]Predicate, bool) {
	if c.HasOr() {
		return nil, false
	}
	var out []Predicate
	_ = c.Walk(func(p *Predicate) error {
		out = append(out, *p)
		return nil
	})
	return out, true
}

// Order is one sort key of an order clause.
type Order struct {
	Property  string
	Direction Direction
	Line      int
}

// Projection is one property a select clause returns.
type Projection struct {
	Name string
	Line int
}

// IndexProperty is one property of a declared composite index.
type IndexProperty struct {
	Name      string
	Direction Direction
}

// Bound is a limit or an offset, which is either a literal or a parameter.
type Bound struct {
	// Literal is the constant form; Param names a parameter instead. Exactly one
	// is set when Present is true.
	Literal int
	Param   string
	Present bool
	Line    int
}

// QueryDecl is one declared access pattern.
//
// It names no kind. The result type names the bound Go type, and that type's
// generated Kind method is the kind, so a declaration cannot disagree with the
// codec about what it is querying.
type QueryDecl struct {
	Name       string
	Exported   bool
	Params     []QueryParam
	Shape      ResultShape
	EntityType string
	// Where is the filter tree, or nil when the declaration has no where clause.
	Where *Condition
	// Ancestor names the parameter holding the ancestor key, when the
	// declaration has an ancestor clause.
	Ancestor     string
	AncestorLine int
	Order        []Order
	Limit        Bound
	Offset       Bound
	// Select is the projection: the properties the query returns instead of
	// whole entities. The result type is unchanged; what is not projected
	// arrives as the zero value.
	Select     []Projection
	SelectLine int
	// Distinct collapses results sharing the named properties.
	Distinct     []Projection
	DistinctLine int
	// Start and End name the parameters holding the cursors this query resumes
	// from and stops at.
	Start     string
	StartLine int
	End       string
	EndLine   int
	// Index is the composite index this access pattern needs, when the author
	// declared one. Nothing derives it: the rule for when one is required is
	// subtle, and a derivation that is quietly wrong names an index that does
	// not fix the query.
	Index     []IndexProperty
	IndexLine int
	// HasIndex separates a declared empty index from no index clause.
	HasIndex   bool
	SourcePath string
	Line       int
}

// ParseQueries reads every declaration in one .tb.firestore source.
func ParseQueries(path string, source []byte) ([]QueryDecl, error) {
	p := &parser{file: path, tokens: lex(string(source))}
	return p.parseAll()
}

// token is one lexical item. The grammar is small enough that a token is its own
// text plus the line it came from.
type token struct {
	text string
	line int
}

func (t token) is(text string) bool { return t.text == text }

// lex splits a declaration source into tokens. Punctuation is single character
// except for the two-character comparisons, and a comment runs to the end of its
// line.
func lex(source string) []token {
	var out []token
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
		case c == '<' || c == '>' || c == '=' || c == '!':
			if i+1 < len(source) && source[i+1] == '=' {
				out = append(out, token{source[i : i+2], line})
				i += 2
				continue
			}
			out = append(out, token{string(c), line})
			i++
		case strings.ContainsRune("(){},;:[]", rune(c)):
			out = append(out, token{string(c), line})
			i++
		default:
			start := i
			for i < len(source) && isWordByte(source[i]) {
				i++
			}
			if i == start {
				// An unknown byte becomes its own token so the parser can name
				// it rather than looping.
				out = append(out, token{string(c), line})
				i++
				continue
			}
			out = append(out, token{source[start:i], line})
		}
	}
	return out
}

func isWordByte(c byte) bool {
	// A hyphen is a word byte because a Datastore property name may carry one.
	// Nothing in this grammar subtracts, so no expression wants it as an
	// operator.
	return c == '_' || c == '.' || c == '-' ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		c >= 0x80 // a multi-byte identifier stays one token, since Go permits one
}

type parser struct {
	file   string
	tokens []token
	at     int
}

func (p *parser) peek() token {
	if p.at >= len(p.tokens) {
		return token{"", p.lastLine()}
	}
	return p.tokens[p.at]
}

func (p *parser) next() token {
	t := p.peek()
	if p.at < len(p.tokens) {
		p.at++
	}
	return t
}

func (p *parser) lastLine() int {
	if len(p.tokens) == 0 {
		return 1
	}
	return p.tokens[len(p.tokens)-1].line
}

func (p *parser) done() bool { return p.at >= len(p.tokens) }

func (p *parser) errorf(line int, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", p.file, line, fmt.Sprintf(format, args...))
}

// expect consumes one token and reports what was found when it is not the one
// wanted, since a declaration is hand-written and the message is the diagnosis.
func (p *parser) expect(text string) (token, error) {
	t := p.next()
	if !t.is(text) {
		return t, p.errorf(t.line, "expected %q, found %s", text, describe(t))
	}
	return t, nil
}

func describe(t token) string {
	if t.text == "" {
		return "end of file"
	}
	return fmt.Sprintf("%q", t.text)
}

func (p *parser) parseAll() ([]QueryDecl, error) {
	var out []QueryDecl
	for !p.done() {
		decl, err := p.statement()
		if err != nil {
			return nil, err
		}
		out = append(out, decl)
	}
	return out, nil
}

func (p *parser) statement() (QueryDecl, error) {
	var decl QueryDecl
	t := p.next()
	if t.is("export") {
		decl.Exported = true
		t = p.next()
	}
	if !t.is("statement") {
		return decl, p.errorf(t.line, "expected \"statement\" or \"export statement\", found %s", describe(t))
	}
	decl.Line = t.line

	name := p.next()
	if !isIdentifier(name.text) {
		return decl, p.errorf(name.line, "expected a statement name, found %s", describe(name))
	}
	decl.Name = name.text
	// Go decides visibility by the name, so export has to agree with it rather
	// than rename anything. A keyword that parsed and did nothing would be the
	// silent no-op this generator rejects everywhere else.
	if nameIsExported(decl.Name) != decl.Exported {
		if decl.Exported {
			return decl, p.errorf(name.line, "statement %s is declared export but its name is unexported; capitalize it or drop export", decl.Name)
		}
		return decl, p.errorf(name.line, "statement %s has an exported name; write \"export statement %s\" or lowercase it", decl.Name, decl.Name)
	}

	params, err := p.params()
	if err != nil {
		return decl, err
	}
	decl.Params = params

	if _, err := p.expect(":"); err != nil {
		return decl, err
	}
	shape, entity, err := p.resultType()
	if err != nil {
		return decl, err
	}
	decl.Shape, decl.EntityType = shape, entity

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
		case ";":
			// A clause separator, so a one-line body reads as one.
			p.next()
		case "where":
			p.next()
			if decl.Where != nil {
				return decl, p.errorf(t.line, "statement %s declares more than one where clause", decl.Name)
			}
			if decl.Where, err = p.whereClause(); err != nil {
				return decl, err
			}
		case "ancestor":
			p.next()
			if decl.Ancestor != "" {
				return decl, p.errorf(t.line, "statement %s declares more than one ancestor clause", decl.Name)
			}
			if decl.Ancestor, err = p.placeholder(); err != nil {
				return decl, err
			}
			decl.AncestorLine = t.line
		case "order":
			p.next()
			if decl.Order != nil {
				return decl, p.errorf(t.line, "statement %s declares more than one order clause", decl.Name)
			}
			if decl.Order, err = p.orderClause(); err != nil {
				return decl, err
			}
		case "limit":
			p.next()
			if decl.Limit.Present {
				return decl, p.errorf(t.line, "statement %s declares more than one limit clause", decl.Name)
			}
			if decl.Limit, err = p.bound(t.line); err != nil {
				return decl, err
			}
		case "offset":
			p.next()
			if decl.Offset.Present {
				return decl, p.errorf(t.line, "statement %s declares more than one offset clause", decl.Name)
			}
			if decl.Offset, err = p.bound(t.line); err != nil {
				return decl, err
			}
		case "index":
			p.next()
			if decl.HasIndex {
				return decl, p.errorf(t.line, "statement %s declares more than one index clause", decl.Name)
			}
			if decl.Index, err = p.indexClause(); err != nil {
				return decl, err
			}
			decl.HasIndex, decl.IndexLine = true, t.line
		case "select":
			p.next()
			if decl.Select != nil {
				return decl, p.errorf(t.line, "statement %s declares more than one select clause", decl.Name)
			}
			if decl.Select, err = p.properties("select"); err != nil {
				return decl, err
			}
			decl.SelectLine = t.line
		case "distinct":
			p.next()
			if decl.Distinct != nil {
				return decl, p.errorf(t.line, "statement %s declares more than one distinct clause", decl.Name)
			}
			if decl.Distinct, err = p.properties("distinct"); err != nil {
				return decl, err
			}
			decl.DistinctLine = t.line
		case "start":
			p.next()
			if decl.Start != "" {
				return decl, p.errorf(t.line, "statement %s declares more than one start clause", decl.Name)
			}
			if decl.Start, err = p.placeholder(); err != nil {
				return decl, err
			}
			decl.StartLine = t.line
		case "end":
			p.next()
			if decl.End != "" {
				return decl, p.errorf(t.line, "statement %s declares more than one end clause", decl.Name)
			}
			if decl.End, err = p.placeholder(); err != nil {
				return decl, err
			}
			decl.EndLine = t.line
		default:
			return decl, p.errorf(t.line, "expected a clause, found %s", describe(t))
		}
	}
	if decl.Shape == Count {
		switch {
		case len(decl.Order) > 0:
			return decl, p.errorf(decl.Line, "statement %s counts, so an order clause changes nothing and is probably a mistake", decl.Name)
		case len(decl.Select) > 0:
			return decl, p.errorf(decl.Line, "statement %s counts, so a select clause has nothing to return", decl.Name)
		case decl.Start != "" || decl.End != "":
			return decl, p.errorf(decl.Line, "statement %s counts, so there is no batch to resume", decl.Name)
		}
	}
	if decl.Shape == Keys && len(decl.Select) > 0 {
		return decl, p.errorf(decl.SelectLine,
			"statement %s returns keys, which is already a projection on the key; a select clause on top of it says two different things", decl.Name)
	}
	return decl, nil
}

func (p *parser) params() ([]QueryParam, error) {
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	var out []QueryParam
	if p.peek().is(")") {
		p.next()
		return out, nil
	}
	for {
		name := p.next()
		if !isIdentifier(name.text) {
			return nil, p.errorf(name.line, "expected a parameter name, found %s", describe(name))
		}
		if _, err := p.expect(":"); err != nil {
			return nil, err
		}
		typeName, err := p.goType()
		if err != nil {
			return nil, err
		}
		out = append(out, QueryParam{Name: name.text, Type: typeName, Line: name.line})

		t := p.next()
		if t.is(")") {
			return out, nil
		}
		if !t.is(",") {
			return nil, p.errorf(t.line, "expected \",\" or \")\" in the parameter list, found %s", describe(t))
		}
	}
}

// goType reads a Go type as the declaration spells it: a name, a qualified name,
// or a slice of either.
func (p *parser) goType() (string, error) {
	t := p.next()
	if t.is("[") {
		if _, err := p.expect("]"); err != nil {
			return "", err
		}
		elem := p.next()
		if !isIdentifier(elem.text) {
			return "", p.errorf(elem.line, "expected a slice element type, found %s", describe(elem))
		}
		return "[]" + elem.text, nil
	}
	if !isIdentifier(t.text) {
		return "", p.errorf(t.line, "expected a type, found %s", describe(t))
	}
	return t.text, nil
}

func (p *parser) resultType() (ResultShape, string, error) {
	t := p.next()
	var shape ResultShape
	switch t.text {
	case "firestore.many":
		shape = Many
	case "firestore.batch":
		shape = Batch
	case "firestore.count":
		shape = Count
	case "firestore.keys":
		shape = Keys
	default:
		return "", "", p.errorf(t.line,
			"expected \"firestore.many<T>\", \"firestore.batch<T>\", \"firestore.count<T>\" or \"firestore.keys<T>\", found %s", describe(t))
	}
	if _, err := p.expect("<"); err != nil {
		return "", "", err
	}
	entity := p.next()
	if !isIdentifier(entity.text) {
		return "", "", p.errorf(entity.line, "expected an entity type, found %s", describe(entity))
	}
	if _, err := p.expect(">"); err != nil {
		return "", "", err
	}
	return shape, entity.text, nil
}

// whereClause parses the filter tree.
//
// Precedence is Go's: and binds tighter than or, so "a or b and c" is
// "a or (b and c)". Parentheses group, because a grammar with two operators and
// no way to override precedence forces an author to restructure a query to say
// what they meant.
func (p *parser) whereClause() (*Condition, error) {
	c, err := p.orExpr()
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (p *parser) orExpr() (Condition, error) {
	first, err := p.andExpr()
	if err != nil {
		return Condition{}, err
	}
	if !p.peek().is("or") {
		return first, nil
	}
	out := Condition{Junction: JunctionOr, Operands: []Condition{first}, Line: first.Line}
	for p.peek().is("or") {
		p.next()
		operand, err := p.andExpr()
		if err != nil {
			return Condition{}, err
		}
		out.Operands = append(out.Operands, operand)
	}
	return out, nil
}

func (p *parser) andExpr() (Condition, error) {
	first, err := p.operand()
	if err != nil {
		return Condition{}, err
	}
	if !p.peek().is("and") {
		return first, nil
	}
	out := Condition{Junction: JunctionAnd, Operands: []Condition{first}, Line: first.Line}
	for p.peek().is("and") {
		p.next()
		operand, err := p.operand()
		if err != nil {
			return Condition{}, err
		}
		out.Operands = append(out.Operands, operand)
	}
	return out, nil
}

// operand is a parenthesised group or a single comparison.
func (p *parser) operand() (Condition, error) {
	if p.peek().is("(") {
		open := p.next()
		inner, err := p.orExpr()
		if err != nil {
			return Condition{}, err
		}
		if _, err := p.expect(")"); err != nil {
			return Condition{}, err
		}
		// The group is kept as written rather than collapsed, so the formatter
		// can print back what the author grouped and a reader is not asked to
		// re-derive the precedence.
		inner.Line = open.line
		return inner, nil
	}
	predicate, err := p.predicate()
	if err != nil {
		return Condition{}, err
	}
	return Condition{Predicate: &predicate, Line: predicate.Line}, nil
}

func (p *parser) predicate() (Predicate, error) {
	var out Predicate
	t := p.next()
	out.Line = t.line
	if !isPropertyName(t.text) {
		return out, p.errorf(t.line, "expected a property name, found %s", describe(t))
	}
	out.Property = t.text

	op := p.next()
	switch op.text {
	case "==", "!=", "<", "<=", ">", ">=":
		out.Op = Op(op.text)
	case "in":
		out.Op = OpIn
	case "not":
		if _, err := p.expect("in"); err != nil {
			return out, err
		}
		out.Op = OpNotIn
	case "=":
		return out, p.errorf(op.line, "equality is spelled \"==\" here, as in Go, not \"=\"")
	default:
		return out, p.errorf(op.line, "expected a comparison after %s, found %s", out.Property, describe(op))
	}

	param, err := p.placeholder()
	if err != nil {
		return out, err
	}
	out.Param = param
	return out, nil
}

func (p *parser) orderClause() ([]Order, error) {
	var out []Order
	for {
		t := p.next()
		if !isPropertyName(t.text) {
			return nil, p.errorf(t.line, "expected a property name to order by, found %s", describe(t))
		}
		order := Order{Property: t.text, Direction: Ascending, Line: t.line}
		switch p.peek().text {
		case "asc":
			p.next()
		case "desc":
			p.next()
			order.Direction = Descending
		}
		out = append(out, order)
		if !p.peek().is(",") {
			return out, nil
		}
		p.next()
	}
}

// bound reads a limit or an offset, either as a literal or as a parameter, so a
// page size chosen by the caller needs no second declaration.
func (p *parser) bound(line int) (Bound, error) {
	out := Bound{Present: true, Line: line}
	if p.peek().is("{") {
		param, err := p.placeholder()
		if err != nil {
			return out, err
		}
		out.Param = param
		return out, nil
	}
	t := p.next()
	n, err := atoi(t.text)
	if err != nil {
		return out, p.errorf(t.line, "expected a number or a parameter, found %s", describe(t))
	}
	if n < 0 {
		return out, p.errorf(t.line, "a negative bound has no meaning")
	}
	out.Literal = n
	return out, nil
}

// properties reads a comma-separated property list, which select and distinct
// both take.
func (p *parser) properties(keyword string) ([]Projection, error) {
	var out []Projection
	for {
		t := p.next()
		if !isPropertyName(t.text) {
			return nil, p.errorf(t.line, "expected a property name in the %s clause, found %s", keyword, describe(t))
		}
		out = append(out, Projection{Name: t.text, Line: t.line})
		if !p.peek().is(",") {
			return out, nil
		}
		p.next()
	}
}

// indexClause reads the composite index this access pattern needs.
func (p *parser) indexClause() ([]IndexProperty, error) {
	var out []IndexProperty
	for {
		t := p.next()
		if !isPropertyName(t.text) {
			return nil, p.errorf(t.line, "expected a property name in the index, found %s", describe(t))
		}
		property := IndexProperty{Name: t.text, Direction: Ascending}
		switch p.peek().text {
		case "asc":
			p.next()
		case "desc":
			p.next()
			property.Direction = Descending
		}
		out = append(out, property)
		if !p.peek().is(",") {
			return out, nil
		}
		p.next()
	}
}

// placeholder reads "{name}", which is how a declaration names a parameter,
// matching the SQL and DynamoDB template value syntax.
func (p *parser) placeholder() (string, error) {
	if _, err := p.expect("{"); err != nil {
		return "", err
	}
	name := p.next()
	if !isIdentifier(name.text) {
		return "", p.errorf(name.line, "expected a parameter name in braces, found %s", describe(name))
	}
	if _, err := p.expect("}"); err != nil {
		return "", err
	}
	return name.text, nil
}

func atoi(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// nameIsExported reports Go's own visibility rule for a declaration name.
func nameIsExported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

func isIdentifier(s string) bool {
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

// isPropertyName accepts what Datastore accepts as a property name, which is
// wider than a Go identifier.
func isPropertyName(s string) bool {
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
