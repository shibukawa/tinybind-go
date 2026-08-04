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

	if decl.Where != nil {
		f.decl.FlushBefore(decl.Where.Line)
		f.where(*decl.Where)
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

// where writes the filter tree. It keeps to one line while that fits, and
// otherwise breaks at the outermost junction, which is where a reader looks
// first.
func (f *formatter) where(c Condition) {
	flat := "where " + conditionText(c, false)
	if f.p.Column()+len(flat) <= f.p.Width() || c.IsLeaf() {
		f.p.Write(flat)
		f.decl.FlushTrailing(lastLineOf(c))
		f.p.Line()
		return
	}
	// Breaking puts the junction at the start of each continuation line, so the
	// word that joins two operands is the first thing on the line rather than
	// something trailing off the end of the one above.
	f.p.Write("where " + conditionText(c.Operands[0], needsParens(c.Operands[0], c.Junction)))
	f.decl.FlushTrailing(lastLineOf(c.Operands[0]))
	f.p.Indent()
	for _, operand := range c.Operands[1:] {
		f.p.Line()
		f.p.Write(string(c.Junction) + " " + conditionText(operand, needsParens(operand, c.Junction)))
		f.decl.FlushTrailing(lastLineOf(operand))
	}
	f.p.Dedent()
	f.p.Line()
}

// conditionText renders one node, parenthesising it when the surrounding
// junction would otherwise read as a different tree.
func conditionText(c Condition, parens bool) string {
	if c.Predicate != nil {
		return predicateText(*c.Predicate)
	}
	parts := make([]string, len(c.Operands))
	for i, operand := range c.Operands {
		parts[i] = conditionText(operand, needsParens(operand, c.Junction))
	}
	text := strings.Join(parts, " "+string(c.Junction)+" ")
	if parens {
		return "(" + text + ")"
	}
	return text
}

// needsParens reports whether an operand has to be grouped inside the given
// junction. Only an or inside an and does: and binds tighter, so an and inside
// an or already reads the way it parses, and printing those brackets would add
// noise the parser does not need.
func needsParens(operand Condition, within Junction) bool {
	return within == JunctionAnd && !operand.IsLeaf() && operand.Junction == JunctionOr
}

// lastLineOf is the source line the node ended on, which is where a trailing
// comment would have been written.
func lastLineOf(c Condition) int {
	if c.Predicate != nil {
		return c.Predicate.Line
	}
	if len(c.Operands) == 0 {
		return c.Line
	}
	return lastLineOf(c.Operands[len(c.Operands)-1])
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
