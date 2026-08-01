package dynamobind

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Formatting for requirement:template-source-formatting. The clause grammar is
// closed and small, so every declaration has exactly one spelling and the
// layout needs no width test: table first, then key, one clause per line.

// sourceComment is one comment kept for requirement:template-comment-retention.
// The declaration parser drops comments during tokenization, so the formatter
// reads them from the source in a second pass and places them by line.
type sourceComment struct {
	line        int
	text        string
	trailing    bool
	blankBefore bool
	used        bool
}

// scanComments collects every comment with the line it sits on.
func scanComments(source string) []sourceComment {
	var out []sourceComment
	line := 1
	blankRun := 0
	lineHasContent := false
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\n':
			if !lineHasContent {
				blankRun++
			} else {
				blankRun = 0
			}
			lineHasContent = false
			line++
			i++
		case source[i] == ' ' || source[i] == '\t' || source[i] == '\r':
			i++
		case source[i] == '/' && i+1 < len(source) && source[i+1] == '/':
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				end = len(source) - i
			}
			out = append(out, sourceComment{
				line:        line,
				text:        strings.TrimRight(source[i:i+end], " \t\r"),
				trailing:    lineHasContent,
				blankBefore: blankRun > 0 && !lineHasContent,
			})
			lineHasContent = true
			i += end
		default:
			lineHasContent = true
			i++
		}
	}
	return out
}

// Format lays out one .tb.dynamo source. A source that does not parse is
// reported rather than guessed at, so the caller can leave the file untouched.
func Format(filename string, source []byte, options syntax.PrintOptions) ([]byte, error) {
	decls, err := ParseQueries(filename, source)
	if err != nil {
		return nil, err
	}
	comments := scanComments(string(source))
	p := syntax.NewPrinter(options)
	f := &formatter{p: p, comments: comments}
	for _, decl := range decls {
		f.flushBefore(decl.Line)
		f.separateFor(decl.Line)
		f.declaration(decl)
	}
	f.flushRemaining()
	return []byte(p.String()), nil
}

type formatter struct {
	p        *syntax.Printer
	comments []sourceComment
	wrote    bool
	// afterComment and lastCommentLine keep a comment attached to what it
	// documents, exactly as the shared module printer does.
	afterComment    bool
	lastCommentLine int
}

// separateFor opens the blank line above a construct, unless a comment sits
// directly on the line above it.
func (f *formatter) separateFor(line int) {
	attached := f.afterComment && line <= f.lastCommentLine+1
	f.afterComment = false
	if !f.wrote || attached {
		return
	}
	f.p.Blank()
}

// flushBefore writes the standalone comments that stood above the given line.
func (f *formatter) flushBefore(line int) {
	for i := range f.comments {
		c := &f.comments[i]
		if c.used || c.trailing || c.line >= line {
			continue
		}
		if f.wrote && (c.blankBefore || !f.afterComment) {
			f.p.Blank()
		}
		f.p.Write(c.text)
		f.p.Line()
		c.used = true
		f.wrote = true
		f.afterComment = true
		f.lastCommentLine = c.line
	}
}

// flushTrailing writes the comment that ended the given line, if any.
func (f *formatter) flushTrailing(line int) {
	for i := range f.comments {
		c := &f.comments[i]
		if c.used || !c.trailing || c.line != line {
			continue
		}
		f.p.Write(" " + c.text)
		c.used = true
	}
}

func (f *formatter) flushRemaining() {
	for i := range f.comments {
		c := &f.comments[i]
		if c.used {
			continue
		}
		if f.wrote {
			f.p.Blank()
		}
		f.p.Write(c.text)
		f.p.Line()
		c.used = true
	}
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
	f.flushTrailing(decl.Line)
	f.p.Line()
	f.afterComment = false
	f.p.Indent()

	f.flushBefore(decl.TableLine)
	f.p.Write("table " + decl.Table)
	f.flushTrailing(decl.TableLine)
	f.p.Line()

	if len(decl.Key) > 0 {
		f.flushBefore(decl.Key[0].Line)
		f.key(decl.Key)
	}
	f.p.Dedent()
	f.p.Write("}")
	f.p.Line()
	f.wrote = true
	f.afterComment = false
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
		f.flushTrailing(predicates[len(predicates)-1].Line)
		f.p.Line()
		return
	}
	f.p.Write("key " + parts[0])
	f.flushTrailing(predicates[0].Line)
	f.p.Indent()
	for i, part := range parts[1:] {
		f.p.Line()
		f.p.Write("and " + part)
		f.flushTrailing(predicates[i+1].Line)
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
