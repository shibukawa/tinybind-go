package dynamobind

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Formatting for requirement:template-source-formatting. The clause grammar is
// closed and small, so every declaration has exactly one spelling and the
// layout needs no width test: table first, then key, one clause per line.
//
// Comment placement is syntax.DeclFormatter, shared with the .tb.firestore
// grammar: both parse by tokenizing, so both drop comments and read them back
// from the source in a second pass.

// Format lays out one .tb.dynamo source. A source that does not parse is
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
	head += "): dynamo." + string(decl.Shape) + "<" + decl.ItemType + "> {"
	f.p.Write(head)
	f.decl.FlushTrailing(decl.Line)
	f.p.Line()
	f.decl.MarkWrote()
	f.p.Indent()

	f.decl.FlushBefore(decl.TableLine)
	f.p.Write("table " + decl.Table)
	f.decl.FlushTrailing(decl.TableLine)
	f.p.Line()

	if len(decl.Key) > 0 {
		f.decl.FlushBefore(decl.Key[0].Line)
		f.key(decl.Key)
	}
	f.p.Dedent()
	f.p.Write("}")
	f.p.Line()
	f.decl.MarkWrote()
}

// key writes the key clause, joining its predicates with and. It opens a line
// per predicate only when the joined form does not fit.
func (f *formatter) key(predicates []Predicate) {
	parts := make([]string, len(predicates))
	for i, predicate := range predicates {
		parts[i] = predicateText(predicate)
	}
	flat := "key " + strings.Join(parts, " and ")
	if f.p.Column()+len(flat) <= f.p.Width() {
		f.p.Write(flat)
		f.decl.FlushTrailing(predicates[len(predicates)-1].Line)
		f.p.Line()
		return
	}
	f.p.Write("key " + parts[0])
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

func predicateText(p Predicate) string {
	switch p.Op {
	case OpBeginsWith:
		return "begins_with(" + p.Attribute + ", {" + p.Params[0] + "})"
	case OpBetween:
		return p.Attribute + " between {" + p.Params[0] + "} and {" + p.Params[1] + "}"
	default:
		return p.Attribute + " " + string(p.Op) + " {" + p.Params[0] + "}"
	}
}
