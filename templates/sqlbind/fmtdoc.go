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
	// hasLineComment reports content that ends its own line, which forbids the
	// flat form no matter how short it is.
	hasLineComment() bool
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

func (d *atomsDoc) hasLineComment() bool {
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

func (d *seqDoc) hasLineComment() bool {
	for _, part := range d.parts {
		if part.hasLineComment() {
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

func (d *parenDoc) hasLineComment() bool {
	return d.inner != nil && d.inner.hasLineComment()
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

func (d *clauseDoc) hasLineComment() bool {
	for _, a := range d.head {
		if a.lineComment {
			return true
		}
	}
	for _, item := range d.items {
		if item.hasLineComment() {
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

func (d *stmtDoc) hasLineComment() bool {
	for _, clause := range d.clauses {
		if clause.hasLineComment() {
			return true
		}
	}
	return false
}
