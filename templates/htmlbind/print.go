package htmlbind

import (
	"errors"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Printer returns the HTML body printer, the printing half of the format parser
// registered by Parse. It implements rule:html-template-layout.
func Printer() syntax.BodyPrinter { return bodyPrinter{} }

// RootPrinter is the registration the shared module printer needs.
func RootPrinter() syntax.RootPrinter {
	return syntax.RootPrinter{Kind: "html:component", Keyword: "component", Printer: Printer()}
}

type bodyPrinter struct{}

func (bodyPrinter) PrintBody(p *syntax.Printer, decl *syntax.TemplateDecl) error {
	body, ok := decl.Body.([]syntax.Node)
	if !ok {
		if body, ok = decl.Body.(Body); !ok {
			return errors.New("htmlbind: component body is not a node list")
		}
	}
	w := &htmlWriter{p: p, preserve: p.Options().PreserveWhitespace}
	// A document body may open lines freely: the parser discards whitespace
	// before the doctype and around the html element. A fragment body may not,
	// because its caller can place it between two inline boxes, so its edge
	// whitespace is only ever reshaped.
	kind := containerFlow
	if isDocumentBody(body) {
		kind = containerFree
	}
	p.Indent()
	if err := w.children(body, kind, modeBlock); err != nil {
		return err
	}
	p.Dedent()
	return nil
}

// layoutMode selects what a whitespace run becomes.
type layoutMode int

const (
	// modeBlock turns an available run into a line break.
	modeBlock layoutMode = iota
	// modeFlat keeps everything on one line, so a run becomes the single space
	// it collapses to.
	modeFlat
)

// container describes how much freedom the layout has between two children.
type container int

const (
	// containerFree is a position where the HTML parser discards a
	// whitespace-only run, so a break may be added where the source had none.
	containerFree container = iota
	// containerFlow is ordinary child position: an existing whitespace run may
	// become a break, and glued nodes stay glued.
	containerFlow
	// containerVerbatim is a whitespace-preserving subtree, copied byte for byte.
	// Its text is still template text, so a literal brace in it is escaped.
	containerVerbatim
	// containerRawText is a script or style body. Its whitespace is preserved
	// like containerVerbatim, and a brace in it is authored CSS or JavaScript
	// rather than template syntax, so it is written back as it stands.
	containerRawText
)

type htmlWriter struct {
	p        *syntax.Printer
	preserve bool
}

// containerFor reports the freedom inside an element, per
// rule:whitespace-preserving-contexts.
func (w *htmlWriter) containerFor(name string, attrs []Attribute) container {
	lower := strings.ToLower(name)
	// pre and textarea preserve whitespace but are ordinary template text; only
	// script and style hold another language, where a brace is its syntax.
	if lower == "script" || lower == "style" {
		return containerRawText
	}
	if whitespaceSignificantElements[lower] || hasPreserveWhitespace(attrs) {
		return containerVerbatim
	}
	if whitespaceDroppingElements[lower] {
		return containerFree
	}
	return containerFlow
}

func hasPreserveWhitespace(attrs []Attribute) bool {
	for _, a := range attrs {
		if a.Name == preserveWhitespaceAttribute {
			return true
		}
	}
	return false
}

// piece is one child with the separator that preceded it.
type piece struct {
	node syntax.Node
	// gap is the whitespace run the source had before this child. An empty gap
	// means the child was glued to the one before it and no break may be added.
	gap string
	// text is set for a text child, already stripped of the gaps that became
	// separators.
	text string
	// isText distinguishes a text child from a node child.
	isText bool
}

// split turns a child list into pieces, lifting whitespace runs out of text
// nodes so they can act as separators.
func split(nodes []syntax.Node) []piece {
	var out []piece
	gap := ""
	for _, node := range nodes {
		text, ok := node.(*TextNode)
		if !ok {
			out = append(out, piece{node: node, gap: gap})
			gap = ""
			continue
		}
		trimmed := strings.TrimLeft(text.Text, " \t\r\n")
		lead := text.Text[:len(text.Text)-len(trimmed)]
		if lead != "" {
			gap += lead
		}
		if trimmed == "" {
			continue
		}
		body := strings.TrimRight(trimmed, " \t\r\n")
		trail := trimmed[len(body):]
		out = append(out, piece{text: body, isText: true, gap: gap})
		gap = trail
	}
	if gap != "" && len(out) > 0 {
		// A trailing run belongs to the container's own closing position.
		out = append(out, piece{gap: gap})
	}
	return out
}

// children writes a child list, including the runs at both of its edges. The
// edges belong here rather than to the element around it, because whether a
// break is available there is the same question as anywhere else in the list.
func (w *htmlWriter) children(nodes []syntax.Node, kind container, mode layoutMode) error {
	if kind == containerVerbatim || kind == containerRawText {
		return w.verbatim(nodes, kind == containerRawText)
	}
	for _, item := range split(nodes) {
		w.gap(item.gap, kind, mode)
		switch {
		case item.isText:
			text, err := escapeText(item.text, false)
			if err != nil {
				return err
			}
			w.p.Write(text)
		case item.node != nil:
			if err := w.node(item.node, kind); err != nil {
				return err
			}
		}
	}
	return nil
}

// gap writes what stands before a child. A break may be added only where the
// HTML parser discards runs; elsewhere an existing run may be reshaped into one
// and a missing run may not be invented.
func (w *htmlWriter) gap(gap string, kind container, mode layoutMode) {
	if mode == modeFlat {
		if gap == "" {
			return
		}
		if w.preserve {
			w.p.WriteRaw(gap)
			return
		}
		w.p.Write(" ")
		return
	}
	switch {
	case kind == containerFree:
		// Adding a run here is invisible, so every child gets its own line.
		w.breakFor(gap)
	case gap == "":
		// Glued: creating a run would insert a rendered space.
	case w.preserve:
		// Collapse is off, so the run is output; it is copied as it was.
		w.p.WriteRaw(gap)
	default:
		// A run collapses to one space, and so does a break plus indentation.
		w.breakFor(gap)
	}
}

// breakFor opens the line before a child, keeping a blank line the author used
// to group sections. A blank line collapses to the same single space one
// newline does, so keeping it costs nothing and grouping is information.
func (w *htmlWriter) breakFor(gap string) {
	if strings.Count(gap, "\n") >= 2 {
		w.p.Blank()
		return
	}
	w.p.Line()
}

func (w *htmlWriter) node(node syntax.Node, kind container) error {
	switch n := node.(type) {
	case *CommentNode:
		w.p.Write("<!--" + n.Text + "-->")
	case *DoctypeNode:
		w.p.Write("<!" + n.Text + ">")
	case *ElementNode:
		return w.element(n)
	case *HeadNode:
		return w.block("head", nil, n.Children, false, containerFree)
	case *SlotNode:
		return w.slot(n)
	case *ComponentNode:
		return w.component(n)
	case *syntax.ExpressionNode:
		w.p.Write("{" + syntax.ExprString(n.Expression) + "}")
	case *syntax.MessageBlockNode:
		// The block is written back as the author wrote it: the reference, the
		// bound elements, and the closer. Each hole is one element, so the
		// children printer already knows how to lay them out.
		w.p.Write("{" + syntax.MessageString(n.Message) + "}")
		var bound []syntax.Node
		for _, hole := range n.Holes {
			bound = append(bound, hole.Nodes...)
		}
		if err := w.children(bound, kind, modeFlat); err != nil {
			return err
		}
		w.p.Write("{/t}")
	case *syntax.ValNode:
		// A leaf, not a control block. The nodes it scopes are its siblings and
		// stay at this level, which is the whole reason it has no closer.
		w.p.Write("{" + syntax.ValString(n) + "}")
	case *syntax.CheckNode:
		w.p.Write("{" + syntax.CheckString(n) + "}")
	case *syntax.IfNode, *syntax.ForNode, *syntax.AwaitNode:
		return w.control(node, kind)
	default:
		return errors.New("htmlbind: cannot print node type " + node.NodeType())
	}
	return nil
}

func (w *htmlWriter) element(n *ElementNode) error {
	inner := w.containerFor(n.Name, n.Attributes)
	return w.block(n.Name, n.Attributes, n.Children, n.SelfClosing, inner)
}

func (w *htmlWriter) component(n *ComponentNode) error {
	return w.block(n.Name, n.Arguments, n.Children, n.SelfClosing, containerFlow)
}

func (w *htmlWriter) slot(n *SlotNode) error {
	var attrs []Attribute
	if n.Name != "" {
		attrs = append(attrs, Attribute{Name: "name", Value: []AttributePart{{Kind: "html:text", Text: n.Name}}})
	}
	if n.Required {
		attrs = append(attrs, Attribute{Name: "required", Boolean: true})
	}
	return w.block("slot", attrs, n.Default, n.Default == nil, containerFlow)
}

// block writes one element-shaped node: its opening tag, its children, and its
// closing tag.
func (w *htmlWriter) block(name string, attrs []Attribute, children []syntax.Node, selfClosing bool, inner container) error {
	open, err := openingTag(name, attrs, selfClosing)
	if err != nil {
		return err
	}
	if selfClosing || (len(children) == 0 && isVoidElement(name)) {
		w.writeTag(open, attrs, name, selfClosing)
		return nil
	}
	w.writeTag(open, attrs, name, false)
	if len(children) == 0 {
		w.p.Write("</" + name + ">")
		return nil
	}
	// An element whose whole subtree fits stays on one line; the width test is
	// what keeps <li><a href="/x">y</a></li> from exploding into five lines.
	if inner != containerFree {
		if flat, ok := w.flat(children, inner); ok &&
			w.p.Column()+len(flat)+len(name)+3 <= w.p.Width() {
			w.p.Write(flat)
			w.p.Write("</" + name + ">")
			return nil
		}
	}
	w.p.Indent()
	if err := w.children(children, inner, modeBlock); err != nil {
		return err
	}
	w.p.Dedent()
	w.p.Write("</" + name + ">")
	return nil
}

// writeTag emits an opening tag, breaking its attributes out when the tag line
// does not fit.
func (w *htmlWriter) writeTag(flat string, attrs []Attribute, name string, selfClosing bool) {
	if w.p.Column()+len(flat) <= w.p.Width() || len(attrs) < 2 {
		w.p.Write(flat)
		return
	}
	w.p.Write("<" + name)
	w.p.Indent()
	for _, attr := range attrs {
		w.p.Line()
		text, err := attributeText(attr)
		if err != nil {
			// openingTag already reported this; reaching here is impossible.
			continue
		}
		w.p.Write(text)
	}
	w.p.Dedent()
	// The bracket rides the last attribute's line, so the tag does not read as
	// an element that closed empty.
	if selfClosing {
		w.p.Write("/>")
		return
	}
	w.p.Write(">")
}

// flat renders a subtree on one line, reporting false when something in it
// forbids the flat form.
func (w *htmlWriter) flat(nodes []syntax.Node, kind container) (string, bool) {
	if kind == containerVerbatim || kind == containerRawText || kind == containerFree {
		return "", false
	}
	sub := &htmlWriter{p: syntax.NewPrinter(syntax.PrintOptions{Width: 1 << 30, Indent: ""}), preserve: w.preserve}
	if err := sub.children(nodes, kind, modeFlat); err != nil {
		return "", false
	}
	out := sub.p.Raw()
	if strings.Contains(out, "\n") {
		return "", false
	}
	return out, true
}

// verbatim copies a whitespace-significant subtree byte for byte. rawText marks
// a script or style body, where a brace belongs to the authored language.
func (w *htmlWriter) verbatim(nodes []syntax.Node, rawText bool) error {
	kind := containerVerbatim
	if rawText {
		kind = containerRawText
	}
	for _, node := range nodes {
		if text, ok := node.(*TextNode); ok {
			escaped, err := escapeText(text.Text, rawText)
			if err != nil {
				return err
			}
			w.p.WriteRaw(escaped)
			continue
		}
		if err := w.node(node, kind); err != nil {
			return err
		}
	}
	return nil
}

func (w *htmlWriter) control(node syntax.Node, kind container) error {
	open, ok := syntax.ControlOpen(node)
	if !ok {
		return errors.New("htmlbind: cannot print node type " + node.NodeType())
	}
	branches := controlBranches(node)
	// A control block whose whole body fits stays inline, which is what keeps
	// {if flag}on{else}off{/if} readable inside a sentence.
	if flat, ok := w.flatControl(open, "{"+syntax.ControlClose(node)+"}", branches, kind); ok && w.p.Column()+len(flat) <= w.p.Width() {
		w.p.Write(flat)
		return nil
	}
	w.p.Write("{" + open + "}")
	for i, branch := range branches {
		if i > 0 {
			w.p.Line()
			w.p.Write(branch.label)
		}
		w.p.Indent()
		if err := w.children(branch.nodes, kind, modeBlock); err != nil {
			return err
		}
		w.p.Dedent()
	}
	w.p.Write("{" + syntax.ControlClose(node) + "}")
	return nil
}

func (w *htmlWriter) flatControl(open, closeMarker string, branches []controlBranch, kind container) (string, bool) {
	if kind == containerFree {
		return "", false
	}
	var b strings.Builder
	b.WriteString("{" + open + "}")
	for i, branch := range branches {
		if i > 0 {
			b.WriteString(branch.label)
		}
		flat, ok := w.flat(branch.nodes, containerFlow)
		if !ok {
			return "", false
		}
		b.WriteString(flat)
	}
	b.WriteString(closeMarker)
	return b.String(), true
}
