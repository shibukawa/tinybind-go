package sqlbind

import "strings"

// The layout tree for rule:sql-template-layout. Every node can render itself
// flat; the printer breaks a node only when its flat form does not fit, so a
// short statement stays on one line and a long one opens exactly as far as it
// has to.

type sqlDoc interface {
	// flat renders the node on one line, including its own leading space when
	// the source had one. Leading spaces are trimmed at each line start, which
	// is what lets concatenation stay this simple.
	flat() string
	// forbidsFlat reports content that cannot share a line: a line comment,
	// which owns the rest of its own, or a control block laid out as clauses.
	// Neither takes the flat form however short it is.
	forbidsFlat() bool
}

// atomsDoc is a run with no break point of its own.
type atomsDoc struct{ atoms []atom }

func (d *atomsDoc) flat() string {
	var b strings.Builder
	for _, a := range d.atoms {
		if a.spaced {
			b.WriteString(" ")
		}
		b.WriteString(a.text)
	}
	return b.String()
}

func (d *atomsDoc) forbidsFlat() bool {
	for _, a := range d.atoms {
		if a.lineComment {
			return true
		}
	}
	return false
}

// seqDoc concatenates parts that share one line budget.
type seqDoc struct{ parts []sqlDoc }

func (d *seqDoc) flat() string {
	var b strings.Builder
	for _, part := range d.parts {
		b.WriteString(part.flat())
	}
	return b.String()
}

func (d *seqDoc) forbidsFlat() bool {
	for _, part := range d.parts {
		if part.forbidsFlat() {
			return true
		}
	}
	return false
}

// parenDoc is a parenthesized group. statement marks the parenthesis that holds
// a subquery or a CTE body, which is the only kind that opens its own level.
type parenDoc struct {
	open      atom
	inner     sqlDoc
	close     atom
	statement bool
}

func (d *parenDoc) flat() string {
	lead := ""
	if d.open.spaced {
		lead = " "
	}
	inner := ""
	if d.inner != nil {
		inner = strings.TrimLeft(d.inner.flat(), " ")
	}
	return lead + "(" + inner + ")"
}

func (d *parenDoc) forbidsFlat() bool {
	return d.inner != nil && d.inner.forbidsFlat()
}

// clauseDoc is one clause: its keyword run, then the items that keyword governs,
// each introduced by the separator that preceded it.
type clauseDoc struct {
	head []atom
	// items are the clause's operands. seps[i] is the separator atom printed
	// with items[i]: a comma trails the previous item, a boolean operator leads
	// its own.
	items []sqlDoc
	seps  []atom
	// commaSeparated reports that separators trail rather than lead, which is
	// the difference between a select list and a WHERE chain.
	commaSeparated bool
	// indented marks a clause that sits one level below the clause it continues,
	// which is what an ON does under its JOIN.
	indented bool
	// absorbing marks a clause whose own keywords do not open lines.
	absorbing bool
}

func (d *clauseDoc) headText() string {
	var b strings.Builder
	for i, a := range d.head {
		if a.spaced && i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(a.text)
	}
	return b.String()
}

func (d *clauseDoc) flat() string {
	var b strings.Builder
	if len(d.head) > 0 && d.head[0].spaced {
		b.WriteString(" ")
	}
	b.WriteString(d.headText())
	for i, item := range d.items {
		if i < len(d.seps) && d.seps[i].text != "" {
			if !d.commaSeparated && d.seps[i].spaced {
				b.WriteString(" ")
			}
			b.WriteString(d.seps[i].text)
		}
		b.WriteString(item.flat())
	}
	return b.String()
}

func (d *clauseDoc) forbidsFlat() bool {
	for _, a := range d.head {
		if a.lineComment {
			return true
		}
	}
	for _, item := range d.items {
		if item.forbidsFlat() {
			return true
		}
	}
	return false
}

// stmtDoc is a statement: a sequence of clauses at one nesting level.
type stmtDoc struct{ clauses []*clauseDoc }

func (d *stmtDoc) flat() string {
	var b strings.Builder
	for _, clause := range d.clauses {
		b.WriteString(clause.flat())
	}
	return b.String()
}

func (d *stmtDoc) forbidsFlat() bool {
	for _, clause := range d.clauses {
		if clause.forbidsFlat() {
			return true
		}
	}
	return false
}

// trailingLineComment reports that the clause holds exactly one line comment
// and it is the clause's last token. Such a clause can still stay on one line:
// the comment ends the line the clause ends anyway, and nothing is swallowed.
func (d *clauseDoc) trailingLineComment() bool {
	count := 0
	for _, a := range d.head {
		if a.lineComment {
			count++
		}
	}
	for _, item := range d.items {
		count += lineComments(item)
	}
	if count != 1 || len(d.items) == 0 {
		return false
	}
	last := lastAtom(d.items[len(d.items)-1])
	return last != nil && last.lineComment
}

// lineComments counts the line comments anywhere in a document.
func lineComments(d sqlDoc) int {
	switch v := d.(type) {
	case *atomsDoc:
		n := 0
		for _, a := range v.atoms {
			if a.lineComment {
				n++
			}
		}
		return n
	case *seqDoc:
		n := 0
		for _, part := range v.parts {
			n += lineComments(part)
		}
		return n
	case *parenDoc:
		if v.inner == nil {
			return 0
		}
		return lineComments(v.inner)
	case *clauseDoc:
		n := 0
		for _, a := range v.head {
			if a.lineComment {
				n++
			}
		}
		for _, item := range v.items {
			n += lineComments(item)
		}
		return n
	case *stmtDoc:
		n := 0
		for _, clause := range v.clauses {
			n += lineComments(clause)
		}
		return n
	case *controlDoc:
		n := 0
		for _, branch := range v.branches {
			if branch.body != nil {
				n += lineComments(branch.body)
			}
		}
		return n
	}
	return 0
}

// lastAtom is the token a document ends on, or nil when it ends on a marker
// that is not an atom, such as a control closer.
func lastAtom(d sqlDoc) *atom {
	switch v := d.(type) {
	case *atomsDoc:
		if len(v.atoms) == 0 {
			return nil
		}
		return &v.atoms[len(v.atoms)-1]
	case *seqDoc:
		if len(v.parts) == 0 {
			return nil
		}
		return lastAtom(v.parts[len(v.parts)-1])
	case *parenDoc:
		if v.close.text != "" {
			return &v.close
		}
		if v.inner == nil {
			return &v.open
		}
		return lastAtom(v.inner)
	case *clauseDoc:
		if len(v.items) > 0 {
			return lastAtom(v.items[len(v.items)-1])
		}
		if len(v.head) > 0 {
			return &v.head[len(v.head)-1]
		}
	case *stmtDoc:
		if len(v.clauses) > 0 {
			return lastAtom(v.clauses[len(v.clauses)-1])
		}
	}
	return nil
}
