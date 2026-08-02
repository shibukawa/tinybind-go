package htmlbind

import (
	"errors"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Tag and text rendering for rule:html-template-layout. Everything here is
// about putting bytes back exactly as the parser would read them again, which
// rule:template-format-fidelity requires before any layout question arises.

// controlBranch is one branch of a control node with the marker that opens it.
type controlBranch struct {
	label string
	nodes []syntax.Node
}

func controlBranches(node syntax.Node) []controlBranch {
	switch n := node.(type) {
	case *syntax.IfNode:
		branches := []controlBranch{{nodes: n.Then}}
		rest := n.Else
		for len(rest) == 1 {
			nested, ok := rest[0].(*syntax.IfNode)
			if !ok {
				break
			}
			open, _ := syntax.ControlOpen(nested)
			branches = append(branches, controlBranch{label: "{else " + open + "}", nodes: nested.Then})
			rest = nested.Else
		}
		if len(rest) > 0 {
			branches = append(branches, controlBranch{label: "{else}", nodes: rest})
		}
		return branches
	case *syntax.ForNode:
		return []controlBranch{{nodes: n.Body}}
	case *syntax.AwaitNode:
		branches := []controlBranch{{nodes: n.Primary}}
		if len(n.Fallback) > 0 {
			branches = append(branches, controlBranch{label: "{fallback}", nodes: n.Fallback})
		}
		if n.HasRecover {
			label := "{recover}"
			if n.ErrorName != "" {
				label = "{recover " + n.ErrorName + "}"
			}
			branches = append(branches, controlBranch{label: label, nodes: n.Recover})
		}
		return branches
	}
	return nil
}

// isVoidElement reports an element that never takes children or a closing tag.
func isVoidElement(name string) bool { return voidElements[strings.ToLower(name)] }

// openingTag renders a whole opening tag on one line, which is also the width
// measurement that decides whether its attributes have to break out.
func openingTag(name string, attrs []Attribute, selfClosing bool) (string, error) {
	var b strings.Builder
	b.WriteString("<")
	b.WriteString(name)
	for _, attr := range attrs {
		text, err := attributeText(attr)
		if err != nil {
			return "", err
		}
		b.WriteString(" ")
		b.WriteString(text)
	}
	if selfClosing {
		b.WriteString("/>")
	} else {
		b.WriteString(">")
	}
	return b.String(), nil
}

// attributeText renders one attribute. A boolean attribute stays bare, because
// adding ="" would change the token the HTML parser sees.
func attributeText(attr Attribute) (string, error) {
	if attr.Boolean {
		return attr.Name, nil
	}
	var value strings.Builder
	for _, part := range attr.Value {
		if part.Expression != nil {
			value.WriteString("{" + syntax.ExprString(part.Expression) + "}")
			continue
		}
		value.WriteString(part.Text)
	}
	text := value.String()
	quote, err := quoteFor(text)
	if err != nil {
		return "", errors.New("htmlbind: attribute " + attr.Name + ": " + err.Error())
	}
	return attr.Name + "=" + quote + text + quote, nil
}

// quoteFor picks the quote the value can carry unescaped. Escaping instead
// would rewrite the value, and the value is the author's bytes.
func quoteFor(value string) (string, error) {
	if !strings.Contains(value, `"`) {
		return `"`, nil
	}
	if !strings.Contains(value, "'") {
		return "'", nil
	}
	return "", errors.New("value contains both quote characters")
}

// escapeText re-encodes body text. The parser decodes {{x}} to {x}, so a
// literal brace that would otherwise be read as template syntax has to be
// wrapped again on the way out.
//
// raw marks a script or style body. There a brace is ordinary CSS or JavaScript
// and the parser already keeps it as text, so it is written back as it stands:
// escaping it would rewrite the authored language, and rule:template-format-
// fidelity forbids that whether or not the rewrite happens to round-trip. Only a
// brace the parser would read as an insertion is escaped, because that one is
// the only brace whose literal spelling the source cannot hold.
func escapeText(text string, raw bool) (string, error) {
	if !strings.ContainsAny(text, "{}") {
		return text, nil
	}
	var b strings.Builder
	for i := 0; i < len(text); {
		switch text[i] {
		case '{':
			if raw && !rawInsertionAt(text, i) {
				b.WriteByte('{')
				i++
				continue
			}
			end := strings.IndexByte(text[i:], '}')
			if end < 0 {
				if raw {
					b.WriteByte('{')
					i++
					continue
				}
				return "", errors.New("htmlbind: text contains an unpaired '{' that no source could spell")
			}
			b.WriteString("{{")
			b.WriteString(text[i+1 : i+end])
			b.WriteString("}}")
			i += end + 1
		case '}':
			if raw {
				b.WriteByte('}')
				i++
				continue
			}
			return "", errors.New("htmlbind: text contains an unpaired '}' that no source could spell")
		default:
			b.WriteByte(text[i])
			i++
		}
	}
	return b.String(), nil
}
