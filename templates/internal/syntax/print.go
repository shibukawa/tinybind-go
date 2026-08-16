package syntax

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultWidth is the soft line width every layout pass aims at. It is soft
// because a construct that cannot break without changing meaning stays long.
const DefaultWidth = 100

// DefaultIndent is two spaces rather than a tab, because inside an HTML body
// the indentation is character data and a tab renders differently there.
const DefaultIndent = "  "

// PrintOptions configures the shared printer and every format printer.
type PrintOptions struct {
	// Width is the soft line width; zero uses DefaultWidth.
	Width int
	// Indent is one indentation level; empty uses DefaultIndent.
	Indent string
	// PreserveWhitespace mirrors the generator option of the same name. With it
	// set, static whitespace is no longer collapsed at generation time, so an
	// HTML layout pass may only touch positions the HTML parser discards.
	PreserveWhitespace bool
}

func (o PrintOptions) width() int {
	if o.Width <= 0 {
		return DefaultWidth
	}
	return o.Width
}

func (o PrintOptions) indent() string {
	if o.Indent == "" {
		return DefaultIndent
	}
	return o.Indent
}

// BodyPrinter lays out one declaration body, from just after the opening brace
// to just before the closing one. It is the printing half of FormatParser: a
// format that can be parsed can be printed.
//
// The body printer owns its own indentation and the line breaks at both braces,
// because whether a body may open on its own line is a property of the format:
// SQL whitespace is insignificant and HTML whitespace is content.
type BodyPrinter interface {
	PrintBody(p *Printer, decl *TemplateDecl) error
}

// RootPrinter registers one format's body printer against the declaration kind
// its parser produced.
type RootPrinter struct {
	// Kind is the TemplateDecl.Kind the format parser assigned.
	Kind string
	// Keyword is the root declaration keyword to print.
	Keyword string
	Printer BodyPrinter
}

// Printer is the shared output buffer with indentation state. Format printers
// write through it so one file has one notion of a line and a level.
type Printer struct {
	buf   strings.Builder
	opts  PrintOptions
	depth int
	// pending counts the newlines owed before the next write. Owing them rather
	// than writing them is what keeps a line that turns out to be empty from
	// leaving trailing indentation behind.
	pending int
}

// NewPrinter creates a standalone printer, for a format printer under test.
func NewPrinter(options PrintOptions) *Printer { return &Printer{opts: options} }

// Options returns the active options.
func (p *Printer) Options() PrintOptions { return p.opts }

// Width returns the soft line width.
func (p *Printer) Width() int { return p.opts.width() }

// Depth returns the current indentation level.
func (p *Printer) Depth() int { return p.depth }

// Indent opens one level.
func (p *Printer) Indent() { p.depth++ }

// Dedent closes one level.
func (p *Printer) Dedent() {
	if p.depth > 0 {
		p.depth--
	}
}

// Write appends text on the current line, opening the line first when one is
// owed. Text must not contain a newline; use Line for that.
func (p *Printer) Write(text string) {
	if text == "" {
		return
	}
	if p.pending > 0 {
		p.buf.WriteString(strings.Repeat("\n", p.pending))
		p.buf.WriteString(strings.Repeat(p.opts.indent(), p.depth))
		p.pending = 0
	}
	p.buf.WriteString(text)
}

// WriteRaw appends text exactly, without indentation and without owing a line.
// It exists for content copied byte for byte, such as a script body.
func (p *Printer) WriteRaw(text string) {
	if p.pending > 0 {
		p.buf.WriteString(strings.Repeat("\n", p.pending))
		p.pending = 0
	}
	p.buf.WriteString(text)
}

// Line ends the current line. The newline and the next line's indentation are
// written lazily, so a line that turns out to be empty leaves no trailing
// spaces behind.
func (p *Printer) Line() {
	if p.pending < 1 {
		p.pending = 1
	}
}

// Blank ends the current line and leaves one empty line after it.
func (p *Printer) Blank() {
	if p.buf.Len() == 0 {
		return
	}
	p.pending = 2
}

// AtLineStart reports that nothing has been written on the current line, which
// is when a layout drops the leading space a construct carries.
func (p *Printer) AtLineStart() bool {
	if p.pending > 0 {
		return true
	}
	text := p.buf.String()
	return text == "" || strings.HasSuffix(text, "\n")
}

// Column reports how many bytes are already on the current line, for a layout
// deciding whether the next construct still fits the width.
func (p *Printer) Column() int {
	if p.pending > 0 {
		return len(p.opts.indent()) * p.depth
	}
	text := p.buf.String()
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		return len(text) - i - 1
	}
	return len(text)
}

// Raw returns the buffer as written, without the trailing-whitespace cleanup
// String applies. An inline measurement needs it, because a trailing space it
// produced is a space the layout has to keep.
func (p *Printer) Raw() string { return p.buf.String() }

// String returns the printed text with exactly one trailing newline.
func (p *Printer) String() string {
	return strings.TrimRight(p.buf.String(), " \t\n") + "\n"
}

// PrintModule prints a whole module: the shared declaration part here, and each
// declaration body through the format printer registered for its kind.
func PrintModule(module *Module, roots []RootPrinter, options PrintOptions) (string, error) {
	byKind := make(map[string]RootPrinter, len(roots))
	for _, root := range roots {
		if root.Kind == "" || root.Keyword == "" || root.Printer == nil {
			return "", fmt.Errorf("invalid root printer registration")
		}
		byKind[root.Kind] = root
	}
	p := &Printer{opts: options}
	m := &modulePrinter{p: p, roots: byKind, comments: module.Comments}
	if err := m.print(module); err != nil {
		return "", err
	}
	return p.String(), nil
}

type modulePrinter struct {
	p        *Printer
	roots    map[string]RootPrinter
	comments []Comment
	// wrote reports that something has already been printed, so the first
	// declaration does not open with a blank line.
	wrote bool
	// afterComment and lastCommentLine keep a comment attached to the
	// declaration it documents: a blank line is opened above a declaration only
	// when the source did not put the comment directly above it.
	afterComment    bool
	lastCommentLine int
}

func (m *modulePrinter) print(module *Module) error {
	if module.Package != nil {
		m.flushBefore(module.Package.Pos)
		m.separateFor(module.Package.Pos)
		m.p.Write(strings.TrimPrefix(module.Package.Kind, "template:") + " " + module.Package.Name)
		m.p.Line()
		m.wrote = true
	}
	if module.Messages != nil {
		m.flushBefore(module.Messages.Pos)
		m.separateFor(module.Messages.Pos)
		m.p.Write("messages " + module.Messages.Name)
		m.p.Line()
		m.wrote = true
	}
	for i, imp := range module.Imports {
		m.flushBefore(imp.Pos)
		// Imports form one block, so only the first is preceded by a blank line.
		if i == 0 {
			m.separateFor(imp.Pos)
		}
		m.p.Write("import " + strconv.Quote(imp.Path))
		if imp.Alias != "" {
			m.p.Write(" as " + imp.Alias)
		}
		m.p.Line()
		m.wrote = true
	}
	for _, decl := range module.Declarations {
		if err := m.declaration(decl); err != nil {
			return err
		}
	}
	// Whatever trails the last declaration is still the author's text.
	m.flushRemaining()
	return nil
}

func (m *modulePrinter) declaration(decl Declaration) error {
	switch d := decl.(type) {
	case *TypeDecl:
		m.flushBefore(d.Pos)
		m.separateFor(d.Pos)
		m.typeDecl(d)
	case *EnumDecl:
		m.flushBefore(d.Pos)
		m.separateFor(d.Pos)
		m.enumDecl(d)
	case *ExternalDecl:
		m.flushBefore(d.Pos)
		m.separateFor(d.Pos)
		m.externalDecl(d)
	case *TemplateDecl:
		root, ok := m.roots[d.Kind]
		if !ok {
			return fmt.Errorf("no printer registered for declaration kind %q", d.Kind)
		}
		// An annotation belongs above its declaration, so comments flush above
		// the first annotation rather than above the keyword line.
		anchor := d.Pos
		if len(d.Annotations) > 0 {
			anchor = d.Annotations[0].Pos
		}
		m.flushBefore(anchor)
		m.separateFor(anchor)
		for _, annotation := range d.Annotations {
			m.p.Write(annotationText(annotation))
			m.p.Line()
		}
		m.p.Write(declarationHeader(root.Keyword, d))
		// The body printer owns the break after the brace and the one before
		// it. In HTML a line break is content, so whether the body opens on its
		// own line is a question only the format can answer.
		if err := root.Printer.PrintBody(m.p, d); err != nil {
			return err
		}
		m.p.Write("}")
		m.p.Line()
	default:
		return fmt.Errorf("unknown declaration type %T", decl)
	}
	m.wrote = true
	// A comment inside the declaration documented something in it, not whatever
	// declaration comes next.
	m.afterComment = false
	return nil
}

// separateFor writes the blank line above a top-level construct. A comment the
// source put directly above it is its documentation, so nothing is inserted
// between the two.
func (m *modulePrinter) separateFor(pos Position) {
	if !m.wrote {
		return
	}
	attached := m.afterComment && pos.Line <= m.lastCommentLine+1
	m.afterComment = false
	if attached {
		return
	}
	m.p.Blank()
}

// flushBefore prints the comments that stood above pos. A comment that was
// written at the end of a previous line is dropped onto its own line here,
// because the shared printer has already ended that line; only a body printer
// can keep a trailing comment inline, and only within its own body.
func (m *modulePrinter) flushBefore(pos Position) {
	before, rest := CommentsBefore(m.comments, pos)
	m.comments = rest
	for _, c := range before {
		if m.wrote && (c.BlankBefore || !m.afterComment) {
			m.p.Blank()
		}
		m.writeComment(c)
		m.wrote = true
		m.afterComment = true
		m.lastCommentLine = c.Pos.Line + strings.Count(c.Text, "\n")
	}
}

func (m *modulePrinter) flushRemaining() {
	for _, c := range m.comments {
		if m.wrote && (c.BlankBefore || !m.afterComment) {
			m.p.Blank()
		}
		m.writeComment(c)
		m.wrote = true
		m.afterComment = true
	}
	m.comments = nil
}

func (m *modulePrinter) writeComment(c Comment) {
	for i, line := range strings.Split(c.Text, "\n") {
		if i > 0 {
			m.p.Line()
			// A block comment's continuation lines keep their own relative
			// shape; only the leading run is re-indented.
			m.p.Write(strings.TrimLeft(line, " \t"))
			continue
		}
		m.p.Write(line)
	}
	m.p.Line()
}

func (m *modulePrinter) typeDecl(d *TypeDecl) {
	head := "type " + d.Name + " {"
	inline := head
	for i, f := range d.Fields {
		if i > 0 {
			inline += ","
		}
		inline += " " + f.Name + ": " + TypeRefString(f.Type)
	}
	inline += " }"
	if len(d.Fields) > 0 && len(inline)+m.p.Column() <= m.p.Width() {
		m.p.Write(inline)
		m.p.Line()
		return
	}
	m.p.Write(head)
	m.p.Line()
	m.p.Indent()
	for _, f := range d.Fields {
		m.flushBefore(f.Pos)
		m.p.Write(f.Name + ": " + TypeRefString(f.Type))
		m.p.Line()
	}
	m.p.Dedent()
	m.p.Write("}")
	m.p.Line()
}

func (m *modulePrinter) enumDecl(d *EnumDecl) {
	inline := "enum " + d.Name + " {"
	for i, member := range d.Members {
		if i > 0 {
			inline += ","
		}
		inline += " " + member.Name
	}
	inline += " }"
	if len(d.Members) > 0 && len(inline)+m.p.Column() <= m.p.Width() {
		m.p.Write(inline)
		m.p.Line()
		return
	}
	m.p.Write("enum " + d.Name + " {")
	m.p.Line()
	m.p.Indent()
	for _, member := range d.Members {
		m.flushBefore(member.Pos)
		m.p.Write(member.Name)
		m.p.Line()
	}
	m.p.Dedent()
	m.p.Write("}")
	m.p.Line()
}

func (m *modulePrinter) externalDecl(d *ExternalDecl) {
	head := "external "
	switch {
	case d.Async:
		head += "async "
	case d.Live:
		head += "live "
	}
	m.p.Write(head + d.Name + parameterList(d.Parameters) + ": " + TypeRefString(d.Result))
	m.p.Line()
}

// declarationHeader renders the line a declaration body opens on.
func declarationHeader(keyword string, d *TemplateDecl) string {
	head := ""
	if d.Exported {
		head = "export "
	}
	return head + keyword + " " + d.Name + parameterList(d.Parameters) + ": " + TypeRefString(d.Output) + " {"
}

func parameterList(params []Parameter) string {
	var b strings.Builder
	b.WriteString("(")
	for i, param := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(param.Name)
		b.WriteString(": ")
		b.WriteString(TypeRefString(param.Type))
	}
	b.WriteString(")")
	return b.String()
}

func annotationText(a Annotation) string {
	var b strings.Builder
	b.WriteString("@")
	b.WriteString(a.Name)
	if len(a.Args) == 0 {
		return b.String()
	}
	b.WriteString("(")
	for i, arg := range a.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(arg.Name)
		b.WriteString(": ")
		b.WriteString(strconv.Quote(arg.Value))
	}
	b.WriteString(")")
	return b.String()
}

// TypeRefString renders a type expression. The AST does not distinguish the
// [T] and T[] spellings of an array, so both print as T[].
func TypeRefString(t TypeRef) string {
	var b strings.Builder
	if t.Async {
		b.WriteString("async ")
	}
	b.WriteString(t.Name)
	if len(t.Arguments) > 0 {
		b.WriteString("<")
		for i, arg := range t.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(TypeRefString(arg))
		}
		b.WriteString(">")
	}
	if t.Array {
		b.WriteString("[]")
	}
	if t.Optional {
		b.WriteString("?")
	}
	return b.String()
}
