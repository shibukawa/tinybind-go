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
	if binding, ok := syntax.DuplicateValBinding(nodes); ok {
		return nil, c.error(binding.Pos, "duplicate value binding "+binding.Name+
			"; a second binding of one name in the same block is a redeclaration, so rename it or move it inside a nested element to shadow deliberately")
	}
	out := make([]Node, 0, len(nodes))
	for i, node := range nodes {
		value, ok := node.(*syntax.ValNode)
		if !ok {
			rewritten, err := c.normalizeBindingChildren(node)
			if err != nil {
				return nil, err
			}
			out = append(out, rewritten)
			continue
		}
		// Everything after this binding is what it scopes. The remainder is
		// normalized as its own list, which is what gives a nested binding its
		// own scope without this having to track depth.
		rest, err := c.normalizeBindings(nodes[i+1:])
		if err != nil {
			return nil, err
		}
		return append(out, nestValBindings(value, rest)), nil
	}
	return out, nil
}

// nestValBindings turns one node binding several names into a chain of nodes
// binding one name each, innermost holding the body. Evaluation order is the
// written order, so a later binding reads an earlier one as an ordinary name.
func nestValBindings(node *syntax.ValNode, body []Node) *syntax.ValNode {
	for i := len(node.Bindings) - 1; i >= 0; i-- {
		nested := &syntax.ValNode{
			Kind:     node.Kind,
			Pos:      node.Bindings[i].Pos,
			Context:  node.Context,
			Bindings: []syntax.ValBinding{node.Bindings[i]},
			Body:     body,
		}
		body = []Node{nested}
	}
	return body[0].(*syntax.ValNode)
}

// normalizeBindingChildren rewrites the child lists a node owns. A node type
// with no children is returned unchanged, which is why the default is to do
// nothing rather than to fail: this walk is over markup, and a node that holds
// no body holds no binding either.
func (c *compiler) normalizeBindingChildren(node Node) (Node, error) {
	lists := bindingChildLists(node)
	for _, list := range lists {
		rewritten, err := c.normalizeBindings(*list.nodes)
		if err != nil {
			return nil, err
		}
		*list.nodes = rewritten
	}
	return node, nil
}

// valueRead reports whether nodes read name.
//
// A binding nothing reads still calls its external on every render, and the
// only thing an external may do is answer a query, so the call is paid for and
// thrown away. That is a mistake rather than a style, which is why the caller
// refuses it instead of dropping the instruction.
//
// A subtree that rebinds the name is not scanned past that point: a reference
// there resolves to the inner binding and leaves the outer one still unread.
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
			for _, binding := range node.Bindings {
				if syntax.ExprReads(binding.Value, name) {
					return true
				}
				if binding.Name == name {
					return false
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

// bindingChildLists names every body list a node owns. It is the one place the
// walk knows about node types, so a new container type is added here rather
// than in each pass that recurses.
func bindingChildLists(node Node) []bindingChildList {
	switch node := node.(type) {
	case *ElementNode:
		return []bindingChildList{{&node.Children}}
	case *HeadNode:
		return []bindingChildList{{&node.Children}}
	case *ComponentNode:
		return []bindingChildList{{&node.Children}}
	case *SlotNode:
		return []bindingChildList{{&node.Default}}
	case *syntax.IfNode:
		return []bindingChildList{{&node.Then}, {&node.Else}}
	case *syntax.ForNode:
		return []bindingChildList{{&node.Body}}
	case *syntax.ValNode:
		// Only the usage scan reaches this, over a tree normalization has
		// already rewritten. normalizeBindings handles a binding itself, so it
		// never asks for its children here.
		return []bindingChildList{{&node.Body}}
	case *syntax.AwaitNode:
		return []bindingChildList{{&node.Primary}, {&node.Fallback}, {&node.Recover}}
	}
	return nil
}
