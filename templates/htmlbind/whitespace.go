package htmlbind

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// preserveWhitespaceAttribute marks an element whose whitespace is authored
// content rather than formatting. It covers the element and everything below
// it, and is never emitted.
//
// The escape is an attribute rather than a declaration annotation because the
// element that needs it is often a single div a stylesheet made
// whitespace-significant, not a whole component.
const preserveWhitespaceAttribute = "preserve-whitespace"

// whitespaceSignificantElements keep their text byte for byte. pre and textarea
// are whitespace-significant under the user agent stylesheet, and a textarea's
// content is its value. script and style bodies are raw text where a newline
// carries meaning, from JavaScript automatic semicolon insertion to where a
// line comment ends.
var whitespaceSignificantElements = map[string]bool{
	"pre": true, "textarea": true, "script": true, "style": true,
}

// whitespaceDroppingElements never keep character data the HTML parser would
// render: text scoped to a table is foster-parented out of it, and html and
// head hold no rendered text. A whitespace-only run directly below one of them
// is removed rather than collapsed.
//
// Elements whose children merely happen to be block boxes by default - ul, ol,
// select and the rest - are deliberately absent. display is a CSS property, so
// the compiler cannot prove their whitespace is invisible.
var whitespaceDroppingElements = map[string]bool{
	"html": true, "head": true, "table": true, "thead": true,
	"tbody": true, "tfoot": true, "tr": true, "colgroup": true,
}

// normalizeWhitespace rewrites the authoring whitespace of a component body so
// generated static byte runs carry markup instead of indentation. A run of
// ASCII whitespace becomes one space, which is what a user agent renders it as,
// so the page is unchanged. Outright deletion is limited to the positions where
// the HTML parser itself discards the run, because between two inline boxes a
// deleted run is a visibly missing space and no pass can tell which elements
// are inline.
//
// The walk runs even with collapse disabled, because the reserved
// preserve-whitespace attribute must be stripped either way.
func normalizeWhitespace(filename string, nodes []Node, collapse bool) ([]Node, error) {
	w := whitespaceWalk{filename: filename, collapse: collapse}
	return w.walk(nodes, isDocumentBody(nodes), false)
}

// isDocumentBody reports that a component body is a whole document rather than
// a fragment. The parser discards whitespace before the doctype and around the
// html element, so the newlines the declaration braces force onto the author
// are droppable there. A fragment body keeps its edge whitespace collapsed to a
// space instead, because its caller may place it between two inline boxes.
func isDocumentBody(nodes []Node) bool {
	for _, node := range nodes {
		switch node := node.(type) {
		case *DoctypeNode:
			return true
		case *ElementNode:
			if node.Name == "html" {
				return true
			}
		}
	}
	return false
}

type whitespaceWalk struct {
	filename string
	// collapse is false inside a preserved subtree and for a run that disabled
	// normalization; the walk then only strips the escape attribute.
	collapse bool
}

// walk normalizes nodes in place and returns the surviving ones. drop reports
// that this position discards whitespace-only text entirely; fills reports that
// this is a component call's children, where a named template writes no bytes.
func (w whitespaceWalk) walk(nodes []Node, drop, fills bool) ([]Node, error) {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		switch node := node.(type) {
		case *TextNode:
			if w.collapse {
				if drop && isASCIISpace(node.Text) {
					continue
				}
				node.Text = collapseASCIISpace(node.Text)
			}
		case *ElementNode:
			preserve, err := w.takePreserveWhitespace(node)
			if err != nil {
				return nil, err
			}
			inner := w
			if preserve || whitespaceSignificantElements[node.Name] {
				inner.collapse = false
			}
			children, err := inner.walk(node.Children, whitespaceDroppingElements[node.Name], false)
			if err != nil {
				return nil, err
			}
			node.Children = children
		case *HeadNode:
			children, err := w.walk(node.Children, w.collapse, false)
			if err != nil {
				return nil, err
			}
			node.Children = children
		case *SlotNode:
			defaults, err := w.walk(node.Default, drop, false)
			if err != nil {
				return nil, err
			}
			node.Default = defaults
		case *ComponentNode:
			// A fill is rendered wherever the callee places its slot, which
			// this component cannot see, so it never inherits drop.
			children, err := w.walk(node.Children, false, true)
			if err != nil {
				return nil, err
			}
			node.Children = children
		case *syntax.IfNode:
			then, err := w.walk(node.Then, drop, fills)
			if err != nil {
				return nil, err
			}
			otherwise, err := w.walk(node.Else, drop, fills)
			if err != nil {
				return nil, err
			}
			node.Then, node.Else = then, otherwise
		case *syntax.ForNode:
			body, err := w.walk(node.Body, drop, fills)
			if err != nil {
				return nil, err
			}
			node.Body = body
		case *syntax.AwaitNode:
			primary, err := w.walk(node.Primary, drop, fills)
			if err != nil {
				return nil, err
			}
			fallback, err := w.walk(node.Fallback, drop, fills)
			if err != nil {
				return nil, err
			}
			recovered, err := w.walk(node.Recover, drop, fills)
			if err != nil {
				return nil, err
			}
			node.Primary, node.Fallback, node.Recover = primary, fallback, recovered
		}
		out = append(out, node)
	}
	if w.collapse {
		out = elideAroundSilent(out, fills)
	}
	return out, nil
}

// elideAroundSilent drops a whitespace-only run touching a sibling that writes
// no bytes. A head contribution and a named slot fill are lifted out before
// emission, so the line breaks the author put around them are formatting for a
// construct with no output, and collapsing each to a space would leave two
// spaces where the source had one break.
func elideAroundSilent(nodes []Node, fills bool) []Node {
	silent := make([]bool, len(nodes))
	any := false
	for i, node := range nodes {
		silent[i] = isSilentNode(node, fills)
		any = any || silent[i]
	}
	if !any {
		return nodes
	}
	out := make([]Node, 0, len(nodes))
	for i, node := range nodes {
		text, ok := node.(*TextNode)
		if ok && isASCIISpace(text.Text) {
			if (i > 0 && silent[i-1]) || (i+1 < len(nodes) && silent[i+1]) {
				continue
			}
		}
		out = append(out, node)
	}
	return out
}

// isSilentNode reports that a node contributes no bytes at its own position.
func isSilentNode(node Node, fills bool) bool {
	switch node := node.(type) {
	case *HeadNode:
		return true
	case *ElementNode:
		if !fills || node.Name != "template" {
			return false
		}
		// splitFills lifts out a template carrying a static name; one without
		// a name attribute stays ordinary markup.
		for _, attribute := range node.Attributes {
			if attribute.Name != "name" {
				continue
			}
			_, static := staticAttributeText(attribute)
			return static
		}
	}
	return false
}

// takePreserveWhitespace removes the reserved escape attribute and reports
// whether the element carried it. A valued form is rejected rather than
// ignored, because "preserve-whitespace=false" reads as a disable yet would
// enable preservation.
func (w whitespaceWalk) takePreserveWhitespace(node *ElementNode) (bool, error) {
	for i, attribute := range node.Attributes {
		if attribute.Name != preserveWhitespaceAttribute {
			continue
		}
		if !attribute.Boolean {
			return false, &CompileError{Filename: w.filename, Pos: attribute.Pos, Message: preserveWhitespaceAttribute + " must be a bare attribute"}
		}
		node.Attributes = append(node.Attributes[:i:i], node.Attributes[i+1:]...)
		return true, nil
	}
	return false, nil
}

// collapseASCIISpace replaces every run of ASCII whitespace with one space. It
// scans bytes, which is safe because a UTF-8 continuation byte is never ASCII.
func collapseASCIISpace(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	pending := false
	for i := 0; i < len(text); i++ {
		if isASCIISpaceByte(text[i]) {
			pending = true
			continue
		}
		if pending {
			out.WriteByte(' ')
			pending = false
		}
		out.WriteByte(text[i])
	}
	if pending {
		out.WriteByte(' ')
	}
	return out.String()
}

func isASCIISpace(text string) bool {
	for i := 0; i < len(text); i++ {
		if !isASCIISpaceByte(text[i]) {
			return false
		}
	}
	return true
}

func isASCIISpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}
