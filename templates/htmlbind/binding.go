package htmlbind

import (
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// normalizeBindings rewrites every body list so a value binding owns the
// siblings that follow it.
//
// The source form has no closer, which is what keeps a bound subtree from being
// indented for a reason the markup does not have. This lowering cannot read it
// that way: emitScope is per instruction list, so a binding changes the Go
// receiver type for everything after it and the whole remainder has to become
// one list against one type. Rewriting here rather than at emission is what
// keeps containment a property of the tree, so every later traversal sees the
// shape it already understands instead of maintaining an invariant.
//
// A node binding several names becomes one nested node per name, so the
// analyzer and the emitter only ever see one binding at a time. The nesting is
// lowering, not meaning: the bindings of one directive are independent, and
// parseVal refuses one that reads another.
//
// The flat form survives in whatever else reads the parser's output, so
// printing round-trips the source the author wrote.
func (c *compiler) normalizeBindings(nodes []Node) ([]Node, error) {
	var hoisted []hoistedBinding
	body, err := c.stripBindings(nodes, &hoisted)
	if err != nil {
		return nil, err
	}
	for i := len(hoisted) - 1; i >= 0; i-- {
		body = []Node{&syntax.ValNode{
			Kind:     "template:val",
			Pos:      hoisted[i].binding.Pos,
			Context:  hoisted[i].context,
			Bindings: []syntax.ValBinding{hoisted[i].binding},
			Body:     body,
		}}
	}
	return body, nil
}

type hoistedBinding struct {
	binding syntax.ValBinding
	context string
}

// stripBindings removes every binding directive of one block, in written order,
// and normalizes each nested control block on the way.
//
// Markup nesting is walked with the same collector, because an element opens no
// block: a binding written inside a div belongs to the block the div sits in,
// and reaches past the div's closing tag. A control construct is walked as its
// own block instead, so its bindings stay inside it.
func (c *compiler) stripBindings(nodes []Node, into *[]hoistedBinding) ([]Node, error) {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		if value, ok := node.(*syntax.ValNode); ok {
			for _, binding := range value.Bindings {
				*into = append(*into, hoistedBinding{binding: binding, context: value.Context})
			}
			continue
		}
		for _, list := range controlChildLists(node) {
			rewritten, err := c.normalizeBindings(*list.nodes)
			if err != nil {
				return nil, err
			}
			*list.nodes = rewritten
		}
		for _, list := range markupChildLists(node) {
			rewritten, err := c.stripBindings(*list.nodes, into)
			if err != nil {
				return nil, err
			}
			*list.nodes = rewritten
		}
		out = append(out, node)
	}
	return out, nil
}

// valueRead reports whether nodes read name.
//
// A binding nothing reads still calls its external on every render, and the
// only thing an external may do is answer a query, so the call is paid for and
// thrown away. That is a mistake rather than a style, which is why the caller
// refuses it instead of dropping the instruction.
//
// A for or await clause may still shadow, so the scan stops at one of those;
// a value binding may not, so nothing stops it there.
func valueRead(nodes []Node, name string) bool {
	for _, node := range nodes {
		switch node := node.(type) {
		case *syntax.ExpressionNode:
			if syntax.ExprReads(node.Expression, name) {
				return true
			}
		case *syntax.IfNode:
			if syntax.ExprReads(node.Condition, name) {
				return true
			}
		case *syntax.ForNode:
			if syntax.ExprReads(node.Iterable, name) {
				return true
			}
			// The loop variables are the body's own names, so a body reading
			// one of them is not reading this binding.
			if node.Variable == name || node.Index == name {
				continue
			}
		case *syntax.ValNode:
			// No stop on a rebind: a value binding may not shadow, so a nested
			// one carrying this name is already a generation error and the scan
			// must not pre-empt that diagnostic with an unread one.
			for _, binding := range node.Bindings {
				if syntax.ExprReads(binding.Value, name) {
					return true
				}
			}
		case *syntax.AwaitNode:
			for _, binding := range node.Bindings {
				if syntax.ExprReads(binding.Call, name) {
					return true
				}
			}
			// A clause binding this name shadows it in the primary subtree
			// alone; the fallback and recover subtrees never see the bindings,
			// so they still read the outer one.
			shadowed := false
			for _, binding := range node.Bindings {
				if binding.Name == name {
					shadowed = true
				}
			}
			if !shadowed && valueRead(node.Primary, name) {
				return true
			}
			if valueRead(node.Fallback, name) || (node.ErrorName != name && valueRead(node.Recover, name)) {
				return true
			}
			continue
		case *ElementNode:
			if attributesRead(node.Attributes, name) {
				return true
			}
		case *ComponentNode:
			if attributesRead(node.Arguments, name) {
				return true
			}
		}
		for _, list := range bindingChildLists(node) {
			if valueRead(*list.nodes, name) {
				return true
			}
		}
	}
	return false
}

func attributesRead(attributes []Attribute, name string) bool {
	for _, attribute := range attributes {
		for _, part := range attribute.Value {
			if part.Expression != nil && syntax.ExprReads(part.Expression, name) {
				return true
			}
		}
	}
	return false
}

type bindingChildList struct{ nodes *[]Node }

// controlChildLists names the body lists that are their own block. They are the
// constructs that decide whether their contents render at all, or how often, so
// a binding inside one belongs to it.
func controlChildLists(node Node) []bindingChildList {
	switch node := node.(type) {
	case *syntax.IfNode:
		return []bindingChildList{{&node.Then}, {&node.Else}}
	case *syntax.ForNode:
		return []bindingChildList{{&node.Body}}
	case *syntax.AwaitNode:
		return []bindingChildList{{&node.Primary}, {&node.Fallback}, {&node.Recover}}
	}
	return nil
}

// markupChildLists names the body lists that are part of the enclosing block.
// Markup structure carries no scope, so a binding written inside one of these
// is hoisted out of it.
func markupChildLists(node Node) []bindingChildList {
	switch node := node.(type) {
	case *ElementNode:
		return []bindingChildList{{&node.Children}}
	case *HeadNode:
		return []bindingChildList{{&node.Children}}
	case *ComponentNode:
		return []bindingChildList{{&node.Children}}
	case *SlotNode:
		return []bindingChildList{{&node.Default}}
	}
	return nil
}

// bindingChildLists names every body list a node owns, for the usage scan, which
// walks the tree after normalization and does not care which lists were blocks.
func bindingChildLists(node Node) []bindingChildList {
	if lists := controlChildLists(node); lists != nil {
		return lists
	}
	if lists := markupChildLists(node); lists != nil {
		return lists
	}
	if value, ok := node.(*syntax.ValNode); ok {
		return []bindingChildList{{&value.Body}}
	}
	return nil
}
