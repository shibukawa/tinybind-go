package htmlbind

import (
	"errors"
	"fmt"
	"strings"
)

// A head contribution normally comes from a component, which declares it in its
// own head block and never sees a request. That covers everything a template can
// know at generation time, and nothing a caller learns per response: a title
// taken from the record the page just loaded, or a marker a framework emits only
// while some cookie is absent.
//
// The nodes below are that second channel. They travel on the render call, so
// they are in hand strictly before the head pass — the same ordering component
// contributions already satisfy — and they join the same merge, in the same
// deduplication, as the innermost contributor. Nothing is injected into the byte
// stream: an author's document shell stays an author's document.
//
// They are values rather than markup, so a caller cannot introduce an element by
// supplying a string. What a caller may contribute is exactly what a component
// may.

// HeadAttr is one attribute of a caller-supplied head node.
type HeadAttr struct {
	Name  string
	Value string
}

// HeadNode is one head contribution supplied at a render call.
//
// Build one with HeadTitle, HeadMeta, HeadLink, HeadScript, or HeadNoScript. The
// zero value contributes nothing.
type HeadNode struct {
	element  string
	attrs    []HeadAttr
	text     string
	children []HeadNode
	// err records a malformed node so the render entry can reject it before the
	// first byte, rather than a constructor returning an error into an option
	// list where a caller has nowhere to put it.
	err error
}

// ErrHeadNode reports a head contribution a render call could not write.
var ErrHeadNode = errors.New("htmlbind: invalid head contribution")

// HeadTitle contributes a document title. Its text is escaped, so a title taken
// from user data cannot close the element.
func HeadTitle(text string) HeadNode {
	return HeadNode{element: "title", text: text}
}

// HeadMeta contributes a meta element.
func HeadMeta(attrs ...HeadAttr) HeadNode {
	return headElement("meta", attrs, nil)
}

// HeadLink contributes a link element.
func HeadLink(attrs ...HeadAttr) HeadNode {
	return headElement("link", attrs, nil)
}

// HeadScript contributes a script element referencing an external file. It
// requires a src attribute: an asset is a reference to something served, so no
// path through this package ever writes inline script, and a policy may keep
// script-src to self with no nonce.
func HeadScript(attrs ...HeadAttr) HeadNode {
	node := headElement("script", attrs, nil)
	if node.err != nil {
		return node
	}
	for _, attr := range attrs {
		if attr.Name == "src" && attr.Value != "" {
			return node
		}
	}
	return HeadNode{err: fmt.Errorf("%w: script needs a src attribute; inline script is never contributed", ErrHeadNode)}
}

// HeadNoScript contributes a noscript element wrapping meta, link, or style
// children. It is what a page tells a browser with scripting disabled, and the
// only contributed element with element children.
func HeadNoScript(children ...HeadNode) HeadNode {
	for _, child := range children {
		if child.err != nil {
			return child
		}
		switch child.element {
		case "meta", "link", "style":
		default:
			return HeadNode{err: fmt.Errorf("%w: noscript cannot contain %s", ErrHeadNode, child.element)}
		}
	}
	return HeadNode{element: "noscript", children: children}
}

func headElement(name string, attrs []HeadAttr, children []HeadNode) HeadNode {
	for _, attr := range attrs {
		if !validAttrName(attr.Name) {
			return HeadNode{err: fmt.Errorf("%w: %s is not a usable attribute name", ErrHeadNode, quoteForError(attr.Name))}
		}
	}
	return HeadNode{element: name, attrs: attrs, children: children}
}

// validAttrName rejects a name that could not be written into a tag as itself.
// The value is escaped and therefore safe whatever it holds; a name is not, so
// it is checked instead.
func validAttrName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == ':', r == '.':
		default:
			return false
		}
	}
	return true
}

func quoteForError(value string) string {
	if value == "" {
		return `""`
	}
	return `"` + value + `"`
}

// voidHeadElements have no closing tag.
var voidHeadElements = map[string]bool{"meta": true, "link": true}

// RenderHeadNodes turns caller-supplied nodes into the ready-to-write tags the
// merge works in, or reports the first node it cannot write.
//
// A caller answering a fragment request — one with no document shell, and
// therefore no head to merge into — uses this to decide what to do with its own
// contributions rather than discovering later that they went nowhere.
func RenderHeadNodes(nodes []HeadNode) ([]string, error) {
	tags := make([]string, 0, len(nodes))
	for _, node := range nodes {
		tag, err := node.markup()
		if err != nil {
			return nil, err
		}
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (n HeadNode) markup() (string, error) {
	if n.err != nil {
		return "", n.err
	}
	if n.element == "" {
		return "", nil
	}
	var out strings.Builder
	out.WriteString("<" + n.element)
	for _, attr := range n.attrs {
		out.WriteString(" " + attr.Name + `="` + Escape(attr.Value) + `"`)
	}
	out.WriteString(">")
	if voidHeadElements[n.element] {
		return out.String(), nil
	}
	out.WriteString(Escape(n.text))
	for _, child := range n.children {
		markup, err := child.markup()
		if err != nil {
			return "", err
		}
		out.WriteString(markup)
	}
	out.WriteString("</" + n.element + ">")
	return out.String(), nil
}

// WithHead adds caller-supplied contributions to this render's merged head.
//
// They merge after every component contribution, as the innermost contributor,
// and through the same deduplication: a tag a component already declared is not
// written twice. Supplying none produces the head the render produced before
// this option existed.
//
// A malformed node fails the render before the first byte, so the response can
// still carry an error status.
//
//	htmlbind.RenderChain(w, chain, page,
//		htmlbind.WithHead(
//			htmlbind.HeadTitle(order.Customer),
//			htmlbind.HeadNoScript(htmlbind.HeadMeta(
//				htmlbind.HeadAttr{Name: "http-equiv", Value: "refresh"},
//				htmlbind.HeadAttr{Name: "content", Value: "0; url=/_handoff"},
//			)),
//		),
//	)
//
// This is a channel for the caller, not a way into the byte stream. Nothing here
// reaches template scope, and a component cannot read what a render contributed.
func WithHead(nodes ...HeadNode) Option {
	return func(o *renderOptions) { o.head = append(o.head, nodes...) }
}

// mergeCallerHead appends the caller's contributions to the merged component
// head, dropping any tag already present.
func mergeCallerHead(head []string, nodes []HeadNode) ([]string, error) {
	if len(nodes) == 0 {
		return head, nil
	}
	tags, err := RenderHeadNodes(nodes)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(head))
	for _, tag := range head {
		seen[tag] = true
	}
	for _, tag := range tags {
		if seen[tag] {
			continue
		}
		seen[tag] = true
		head = append(head, tag)
	}
	return head, nil
}
