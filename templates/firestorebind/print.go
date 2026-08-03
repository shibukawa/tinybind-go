package firestorebind

import (
	"strconv"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Formatting for requirement:template-source-formatting. The clause grammar is
// closed and small, so every declaration has exactly one spelling and the layout
// needs no width test beyond the where clause, which is the only one that can
// run long.
//
// Clauses come out in a fixed order regardless of how they were written: where,
// ancestor, select, distinct, order, start, end, limit, offset, index. Reading
// order follows what the query does rather than what the author happened to type
// first.
//
// Comment placement is syntax.DeclFormatter, shared with the .tb.dynamo grammar.

// Format lays out one .tb.firestore source. A source that does not parse is
// reported rather than guessed at, so the caller can leave the file untouched.
func Format(filename string, source []byte, options syntax.PrintOptions) ([]byte, error) {
	decls, err := ParseQueries(filename, source)
	if err != nil {
		return nil, err
	}
	p := syntax.NewPrinter(options)
	f := &formatter{p: p, decl: syntax.NewDeclFormatter(p, syntax.ScanDeclComments(string(source)))}
	for _, decl := range decls {
		f.decl.FlushBefore(decl.Line)
		f.decl.SeparateFor(decl.Line)
		f.declaration(decl)
	}
	f.decl.FlushRemaining()
	return []byte(p.String()), nil
}

type formatter struct {
	p    *syntax.Printer
	decl *syntax.DeclFormatter
}

func (f *formatter) declaration(decl QueryDecl) {
	head := ""
	if decl.Exported {
		head = "export "
	}
	head += "statement " + decl.Name + "("
	for i, param := range decl.Params {
		if i > 0 {
			head += ", "
		}
		head += param.Name + ": " + param.Type
	}
	head += "): firestore." + string(decl.Shape) + "<" + decl.EntityType + "> {"
	f.p.Write(head)
	f.decl.FlushTrailing(decl.Line)
	f.p.Line()
	f.decl.MarkWrote()
	f.p.Indent()

	if len(decl.Where) > 0 {
		f.decl.FlushBefore(decl.Where[0].Line)
		f.where(decl.Where)
	}
	if decl.Ancestor != "" {
		f.decl.FlushBefore(decl.AncestorLine)
		f.p.Write("ancestor {" + decl.Ancestor + "}")
		f.decl.FlushTrailing(decl.AncestorLine)
		f.p.Line()
	}
	if len(decl.Select) > 0 {
		f.decl.FlushBefore(decl.SelectLine)
		f.p.Write("select " + propertyList(decl.Select))
		f.decl.FlushTrailing(decl.SelectLine)
		f.p.Line()
	}
	if len(decl.Distinct) > 0 {
		f.decl.FlushBefore(decl.DistinctLine)
		f.p.Write("distinct " + propertyList(decl.Distinct))
		f.decl.FlushTrailing(decl.DistinctLine)
		f.p.Line()
	}
	if len(decl.Order) > 0 {
		f.decl.FlushBefore(decl.Order[0].Line)
		parts := make([]string, len(decl.Order))
		for i, order := range decl.Order {
			parts[i] = directionText(order.Property, order.Direction)
		}
		f.p.Write("order " + strings.Join(parts, ", "))
		f.decl.FlushTrailing(decl.Order[len(decl.Order)-1].Line)
		f.p.Line()
	}
	f.cursor("start", decl.Start, decl.StartLine)
	f.cursor("end", decl.End, decl.EndLine)
	f.bound("limit", decl.Limit)
	f.bound("offset", decl.Offset)
	if decl.HasIndex {
		f.decl.FlushBefore(decl.IndexLine)
		parts := make([]string, len(decl.Index))
		for i, property := range decl.Index {
			parts[i] = directionText(property.Name, property.Direction)
		}
		f.p.Write("index " + strings.Join(parts, ", "))
		f.decl.FlushTrailing(decl.IndexLine)
		f.p.Line()
	}

	f.p.Dedent()
	f.p.Write("}")
	f.p.Line()
	f.decl.MarkWrote()
}

// where writes the filters, joining them with and. It opens a line per predicate
// only when the joined form does not fit, which is the one place this grammar
// needs the width at all.
func (f *formatter) where(predicates []Predicate) {
	parts := make([]string, len(predicates))
	for i, predicate := range predicates {
		parts[i] = predicateText(predicate)
	}
	flat := "where " + strings.Join(parts, " and ")
	if f.p.Column()+len(flat) <= f.p.Width() {
		f.p.Write(flat)
		f.decl.FlushTrailing(predicates[len(predicates)-1].Line)
		f.p.Line()
		return
	}
	f.p.Write("where " + parts[0])
	f.decl.FlushTrailing(predicates[0].Line)
	f.p.Indent()
	for i, part := range parts[1:] {
		f.p.Line()
		f.p.Write("and " + part)
		f.decl.FlushTrailing(predicates[i+1].Line)
	}
	f.p.Dedent()
	f.p.Line()
}

func (f *formatter) bound(keyword string, bound Bound) {
	if !bound.Present {
		return
	}
	f.decl.FlushBefore(bound.Line)
	if bound.Param != "" {
		f.p.Write(keyword + " {" + bound.Param + "}")
	} else {
		f.p.Write(keyword + " " + strconv.Itoa(bound.Literal))
	}
	f.decl.FlushTrailing(bound.Line)
	f.p.Line()
}

func (f *formatter) cursor(keyword, param string, line int) {
	if param == "" {
		return
	}
	f.decl.FlushBefore(line)
	f.p.Write(keyword + " {" + param + "}")
	f.decl.FlushTrailing(line)
	f.p.Line()
}

func propertyList(properties []Projection) string {
	names := make([]string, len(properties))
	for i, property := range properties {
		names[i] = property.Name
	}
	return strings.Join(names, ", ")
}

func predicateText(p Predicate) string {
	return p.Property + " " + string(p.Op) + " {" + p.Param + "}"
}

// directionText writes a property with its direction, leaving ascending
// unwritten because it is the default and saying it adds nothing.
func directionText(name string, direction Direction) string {
	if direction == Descending {
		return name + " desc"
	}
	return name
}
