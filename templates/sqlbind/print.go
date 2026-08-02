package sqlbind

import (
	"errors"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Printer returns the SQL body printer, the printing half of the format parser
// registered by Parse. It implements rule:sql-template-layout.
func Printer() syntax.BodyPrinter { return bodyPrinter{} }

// RootPrinter is the registration the shared module printer needs.
func RootPrinter() syntax.RootPrinter {
	return syntax.RootPrinter{Kind: "sql:statement", Keyword: "statement", Printer: Printer()}
}

type bodyPrinter struct{}

func (bodyPrinter) PrintBody(p *syntax.Printer, decl *syntax.TemplateDecl) error {
	body, ok := decl.Body.([]syntax.Node)
	if !ok {
		if body, ok = decl.Body.(Body); !ok {
			return errors.New("sqlbind: statement body is not a node list")
		}
	}
	builder := &docBuilder{}
	doc, err := builder.body(body)
	if err != nil {
		return err
	}
	if doc == nil {
		p.Line()
		return nil
	}
	p.Indent()
	p.Line()
	r := &sqlRenderer{p: p}
	// A statement's own clauses always open lines, however short they are: one
	// clause per line is the layout, not a fallback for a long line. A nested
	// statement keeps the width test, so a small subquery stays inline.
	if stmt, ok := doc.(*stmtDoc); ok && len(stmt.clauses) > 1 {
		r.statement(stmt)
	} else {
		r.render(doc)
	}
	p.Dedent()
	p.Line()
	return nil
}

// element is one unit of a body: either a scanned SQL atom or a control node
// that carries its own sub-layout.
type element struct {
	atom atom
	doc  sqlDoc
}

func (e element) isAtom() bool { return e.doc == nil }

func (e element) is(text string) bool {
	return e.isAtom() && e.atom.kind == atomPunct && e.atom.text == text
}

func (e element) word() string {
	if e.isAtom() && e.atom.kind == atomWord {
		return e.atom.upper
	}
	return ""
}

func (e element) depth() int {
	if e.isAtom() {
		return e.atom.depth
	}
	return -1
}

// docBuilder turns a statement body into the layout tree. One builder carries
// the SQL lexer across text nodes and control branches, because parenthesis
// depth spans them even though quotes and comments never do.
type docBuilder struct {
	lexer sqlLexer
	// pendingSpace carries "the previous node ended with whitespace" across a
	// node boundary, so `id={id}` and `id = {id}` stay as the author wrote them.
	pendingSpace bool
}

func (b *docBuilder) takeSpace() bool {
	spaced := b.pendingSpace
	b.pendingSpace = false
	return spaced
}

func endsWithSpace(text string) bool {
	if text == "" {
		return false
	}
	switch text[len(text)-1] {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

func (b *docBuilder) body(nodes []syntax.Node) (sqlDoc, error) {
	elements, err := b.elements(nodes)
	if err != nil {
		return nil, err
	}
	if len(elements) == 0 {
		return nil, nil
	}
	depth := elements[0].depth()
	if depth < 0 {
		depth = 0
	}
	doc, _ := b.parseStatement(elements, 0, depth)
	return doc, nil
}

func (b *docBuilder) elements(nodes []syntax.Node) ([]element, error) {
	var out []element
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			atoms, ok := b.lexer.scanAtoms(n.Text)
			if !ok {
				return nil, errors.New("sqlbind: unterminated SQL literal or comment")
			}
			if len(atoms) > 0 {
				atoms[0].spaced = atoms[0].spaced || b.takeSpace()
				b.pendingSpace = endsWithSpace(n.Text)
			} else if endsWithSpace(n.Text) {
				b.pendingSpace = true
			}
			for _, a := range atoms {
				out = append(out, element{atom: a})
			}
		case *ExpressionNode:
			out = append(out, element{atom: atom{
				kind: atomNode, text: "{" + syntax.ExprString(n.Expression) + "}",
				depth: b.lexer.depth, spaced: b.takeSpace(), node: n,
			}})
		case *RelationNode:
			out = append(out, element{atom: atom{
				kind: atomNode, text: relationText(n), depth: b.lexer.depth, spaced: b.takeSpace(), node: n,
			}})
		default:
			doc, err := b.control(node)
			if err != nil {
				return nil, err
			}
			out = append(out, element{doc: doc})
		}
	}
	return out, nil
}

// control builds the layout for one shared control node. Its branches are laid
// out by the same builder, so a clause inside a branch is still a clause.
func (b *docBuilder) control(node syntax.Node) (sqlDoc, error) {
	open, ok := syntax.ControlOpen(node)
	if !ok {
		return nil, errors.New("sqlbind: cannot print node type " + node.NodeType())
	}
	doc := &controlDoc{open: "{" + open + "}", close: "{" + syntax.ControlClose(node) + "}", spaced: b.takeSpace()}
	switch n := node.(type) {
	case *syntax.IfNode:
		body, err := b.body(n.Then)
		if err != nil {
			return nil, err
		}
		doc.branches = append(doc.branches, controlBranch{body: body})
		// An else holding nothing but one if is the else-if chain the author
		// most likely wrote; the AST cannot tell the two spellings apart, so the
		// chain is the one printed.
		rest := n.Else
		for len(rest) == 1 {
			nested, isIf := rest[0].(*syntax.IfNode)
			if !isIf {
				break
			}
			nestedOpen, _ := syntax.ControlOpen(nested)
			body, err := b.body(nested.Then)
			if err != nil {
				return nil, err
			}
			doc.branches = append(doc.branches, controlBranch{label: "{else " + nestedOpen + "}", body: body})
			rest = nested.Else
		}
		if len(rest) > 0 {
			body, err := b.body(rest)
			if err != nil {
				return nil, err
			}
			doc.branches = append(doc.branches, controlBranch{label: "{else}", body: body})
		}
	case *syntax.ForNode:
		body, err := b.body(n.Body)
		if err != nil {
			return nil, err
		}
		doc.branches = append(doc.branches, controlBranch{body: body})
	default:
		return nil, errors.New("sqlbind: cannot print node type " + node.NodeType())
	}
	return doc, nil
}

func relationText(n *RelationNode) string {
	var b strings.Builder
	b.WriteString("subquery ")
	b.WriteString(n.Name)
	b.WriteString("(")
	for i, arg := range n.Arguments {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(syntax.ExprString(arg))
	}
	b.WriteString(") AS ")
	b.WriteString(n.Alias)
	return b.String()
}

// parseStatement collects the clauses at one nesting level, stopping at the
// closing parenthesis of the group it was called for.
func (b *docBuilder) parseStatement(els []element, i, depth int) (*stmtDoc, int) {
	stmt := &stmtDoc{}
	for i < len(els) {
		if els[i].is(")") && els[i].depth() < depth {
			break
		}
		clause, next := b.parseClause(els, i, depth)
		if next == i {
			next = i + 1
		}
		stmt.clauses = append(stmt.clauses, clause)
		i = next
	}
	return stmt, i
}

func (b *docBuilder) parseClause(els []element, i, depth int) (*clauseDoc, int) {
	clause := &clauseDoc{}
	boolean := false
	if head, kind, ok := matchClauseHead(els, i, depth); ok {
		clause.head = make([]atom, 0, head)
		for n := 0; n < head; n++ {
			clause.head = append(clause.head, els[i+n].atom)
		}
		clause.commaSeparated = kind.comma
		clause.indented = kind.indented
		clause.absorbing = kind.absorbing
		boolean = kind.boolean
		i += head
	}
	var current []element
	flush := func(sep atom) {
		clause.items = append(clause.items, b.run(current))
		clause.seps = append(clause.seps, sep)
		current = nil
	}
	sep := atom{}
	for i < len(els) {
		e := els[i]
		if e.is(")") && e.depth() < depth {
			break
		}
		if e.isAtom() && e.atom.depth == depth {
			if _, _, ok := matchClauseHead(els, i, depth); ok && len(clause.head) > 0 {
				// An absorbing clause ends only where the statement itself moves
				// on, not at every keyword inside its own action.
				if !clause.absorbing || endsAbsorbingClause(e.word()) {
					break
				}
			}
			if clause.commaSeparated && e.is(",") {
				// A comma trails the item it ends, so it is carried into the
				// separator of the item that follows.
				flush(sep)
				sep = e.atom
				i++
				continue
			}
			if boolean && (e.word() == "AND" || e.word() == "OR") {
				flush(sep)
				sep = e.atom
				i++
				continue
			}
		}
		if e.is("(") {
			paren, next := b.parseParen(els, i)
			current = append(current, element{doc: paren})
			i = next
			continue
		}
		current = append(current, e)
		i++
	}
	if len(current) > 0 || len(clause.items) > 0 {
		flush(sep)
	}
	return clause, i
}

// parseParen groups one parenthesized run. A parenthesis whose first word opens
// a statement holds a subquery or a CTE body and is the only kind that opens a
// level of its own.
func (b *docBuilder) parseParen(els []element, i int) (sqlDoc, int) {
	open := els[i].atom
	inner := i + 1
	depth := open.depth + 1
	doc := &parenDoc{open: open, statement: opensStatement(els, inner)}
	var innerDoc sqlDoc
	var next int
	if doc.statement {
		innerDoc, next = b.parseStatement(els, inner, depth)
	} else {
		innerDoc, next = b.parseGroup(els, inner, depth)
	}
	doc.inner = innerDoc
	if next < len(els) && els[next].is(")") {
		doc.close = els[next].atom
		next++
	}
	return doc, next
}

// parseGroup collects a parenthesized run that is not a statement, such as a
// column list or an argument list.
func (b *docBuilder) parseGroup(els []element, i, depth int) (sqlDoc, int) {
	var current []element
	for i < len(els) {
		if els[i].is(")") && els[i].depth() < depth {
			break
		}
		if els[i].is("(") {
			paren, next := b.parseParen(els, i)
			current = append(current, element{doc: paren})
			i = next
			continue
		}
		current = append(current, els[i])
		i++
	}
	return b.run(current), i
}

// run packs a sequence of elements into one document, keeping adjacent atoms
// together and letting an embedded control node keep its own layout.
func (b *docBuilder) run(els []element) sqlDoc {
	seq := &seqDoc{}
	var atoms []atom
	flush := func() {
		if len(atoms) > 0 {
			seq.parts = append(seq.parts, &atomsDoc{atoms: atoms})
			atoms = nil
		}
	}
	for _, e := range els {
		if e.isAtom() {
			atoms = append(atoms, e.atom)
			continue
		}
		flush()
		seq.parts = append(seq.parts, e.doc)
	}
	flush()
	if len(seq.parts) == 1 {
		return seq.parts[0]
	}
	return seq
}

// opensStatement reports that the tokens after an opening parenthesis start a
// statement rather than a value list.
func opensStatement(els []element, i int) bool {
	for ; i < len(els); i++ {
		if !els[i].isAtom() {
			return false
		}
		if els[i].atom.kind == atomComment {
			continue
		}
		switch els[i].word() {
		case "SELECT", "WITH", "VALUES", "INSERT", "UPDATE", "DELETE", "TABLE":
			return true
		}
		return false
	}
	return false
}
