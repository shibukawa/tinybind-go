package sqlbind

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// controlBranch is one branch of an embedded control node. label is empty for
// the first branch, whose opener already stands above it.
type controlBranch struct {
	label string
	body  sqlDoc
}

// controlDoc is an embedded if or for. It keeps its own layout so a branch
// holding whole clauses can open lines, while a branch holding a fragment of
// one clause stays inline.
type controlDoc struct {
	open     string
	close    string
	spaced   bool
	branches []controlBranch
}

func (d *controlDoc) flat() string {
	var b strings.Builder
	if d.spaced {
		b.WriteString(" ")
	}
	b.WriteString(d.open)
	for _, branch := range d.branches {
		b.WriteString(branch.label)
		if branch.body != nil {
			b.WriteString(branch.body.flat())
		}
	}
	b.WriteString(d.close)
	return b.String()
}

func (d *controlDoc) hasLineComment() bool {
	for _, branch := range d.branches {
		if branch.body != nil && branch.body.hasLineComment() {
			return true
		}
	}
	return false
}

// sqlRenderer writes a layout tree, opening a construct only when its flat form
// does not fit the remaining width.
type sqlRenderer struct{ p *syntax.Printer }

func (r *sqlRenderer) render(d sqlDoc) {
	if d == nil {
		return
	}
	if r.fits(d) {
		r.write(d.flat())
		return
	}
	switch v := d.(type) {
	case *stmtDoc:
		r.statement(v)
	case *clauseDoc:
		r.clause(v)
	case *parenDoc:
		r.paren(v)
	case *seqDoc:
		for _, part := range v.parts {
			r.render(part)
		}
	case *controlDoc:
		r.control(v)
	default:
		r.write(d.flat())
	}
}

// fits reports that a construct may stay on one line. A line comment owns the
// rest of its line, and a literal carrying its own newlines has already decided
// the question, so neither can be flat.
func (r *sqlRenderer) fits(d sqlDoc) bool {
	if d.hasLineComment() {
		return false
	}
	flat := d.flat()
	if strings.ContainsAny(flat, "\n") {
		return false
	}
	return r.p.Column()+len(strings.TrimLeft(flat, " ")) <= r.p.Width()
}

// write emits text, dropping the leading space a construct carries when it
// begins a line.
func (r *sqlRenderer) write(text string) {
	if r.p.AtLineStart() {
		text = strings.TrimLeft(text, " ")
	}
	if text == "" {
		return
	}
	if strings.Contains(text, "\n") {
		// A dollar-quoted literal keeps its own newlines; writing it raw keeps
		// the indentation out of the literal's own bytes.
		lines := strings.Split(text, "\n")
		r.p.Write(lines[0])
		for _, line := range lines[1:] {
			r.p.WriteRaw("\n" + line)
		}
		return
	}
	r.p.Write(text)
}

func (r *sqlRenderer) statement(d *stmtDoc) {
	for i, clause := range d.clauses {
		if i > 0 {
			r.p.Line()
		}
		if clause.indented {
			r.p.Indent()
			r.clause(clause)
			r.p.Dedent()
			continue
		}
		r.clause(clause)
	}
}

func (r *sqlRenderer) clause(d *clauseDoc) {
	if r.fits(d) {
		r.write(d.flat())
		return
	}
	if len(d.head) > 0 {
		lead := ""
		if d.head[0].spaced {
			lead = " "
		}
		r.write(lead + d.headText())
	}
	// The items may still fit beside the keyword; only when they do not does
	// the clause open a level of its own.
	if tail := d.itemsFlat(); tail != "" && !strings.Contains(tail, "\n") && !d.itemsHaveLineComment() &&
		r.p.Column()+len(tail) <= r.p.Width() {
		r.write(tail)
		return
	}
	r.p.Indent()
	for i, item := range d.items {
		r.p.Line()
		if !d.commaSeparated && i < len(d.seps) && d.seps[i].text != "" {
			r.write(d.seps[i].text + " ")
		}
		r.render(item)
		if d.commaSeparated && i+1 < len(d.seps) && d.seps[i+1].text != "" {
			r.write(d.seps[i+1].text)
		}
	}
	r.p.Dedent()
}

func (r *sqlRenderer) paren(d *parenDoc) {
	lead := ""
	if d.open.spaced {
		lead = " "
	}
	r.write(lead + "(")
	r.p.Indent()
	r.p.Line()
	r.render(d.inner)
	r.p.Dedent()
	r.p.Line()
	r.write(")")
}

func (r *sqlRenderer) control(d *controlDoc) {
	lead := ""
	if d.spaced {
		lead = " "
	}
	r.write(lead + d.open)
	for i, branch := range d.branches {
		if i > 0 {
			r.p.Line()
			r.write(branch.label)
		}
		if branch.body == nil {
			continue
		}
		r.p.Indent()
		r.p.Line()
		r.render(branch.body)
		r.p.Dedent()
		r.p.Line()
	}
	r.write(d.close)
}

// itemsFlat renders everything after the clause keyword on one line.
func (d *clauseDoc) itemsFlat() string {
	var b strings.Builder
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

func (d *clauseDoc) itemsHaveLineComment() bool {
	for _, item := range d.items {
		if item.hasLineComment() {
			return true
		}
	}
	return false
}
