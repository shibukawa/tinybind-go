package htmlbind

import (
	"sort"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// MessageSymbol names the Go function one resolved message id calls.
//
// The mapping arrives as data rather than being computed from the id, because
// an id is not a Go identifier: it may carry hyphens, and how a catalog turns a
// slug into a symbol is the catalog owner's policy. See .knowledge
// requirement:message-symbol-resolution id_to_symbol_is_a_supplied_table.
type MessageSymbol struct {
	// Package is the import path holding the function. Empty means the
	// generated package's own, like ElementProvider.Package.
	Package string
	// Alias overrides the import name. Empty uses the last path segment.
	Alias string
	// Name is the function.
	Name string
	// Params names the function's parameters in declaration order, excluding
	// any leading argument GenerateOptions.MessageContext supplies. A reference
	// must name exactly these, which is also what fixes the call's argument
	// order: a reference writes its arguments by name and the emitter puts them
	// back in the order the function declares.
	Params []string
}

// MessageRef is one reference a template makes, reported so a caller can
// resolve what it needs before generation and reconcile a template against a
// catalog. It is the reference half of
// .knowledge requirement:template-parse-introspection.
type MessageRef struct {
	// Scope is the file's declared `messages` name, empty when none was
	// declared.
	Scope string
	// Written is the id as the author spelled it, and ID is what it resolves
	// to. They differ only for an unqualified reference.
	Written string
	ID      string
	// Args names the arguments the reference supplies, in source order.
	Args []string
	Pos  Position
}

// MessageRefs reports every message reference in a source file, with the file's
// declared scope. It runs the parser and the resolution rule and nothing else,
// so it answers before any symbol table exists — which is the order a caller
// needs, since the table is what this report is used to build.
//
// A reference that cannot be resolved, because the file declares no scope, is
// reported with an empty ID rather than failing, so a caller reconciling a tree
// of templates sees the whole picture instead of the first mistake.
func MessageRefs(filename string, source []byte) ([]MessageRef, error) {
	module, err := Parse(filename, source)
	if err != nil {
		return nil, err
	}
	scope := ""
	if module.Messages != nil {
		scope = module.Messages.Name
	}
	var refs []MessageRef
	for _, message := range moduleMessages(module) {
		ref := MessageRef{Scope: scope, Written: message.Written, Pos: message.Pos}
		if id, ok := resolveMessageID(message.Written, scope); ok {
			ref.ID = id
		}
		for _, arg := range message.Args {
			ref.Args = append(ref.Args, arg.Name)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// resolveMessageID applies the one resolution rule: a dotted id is absolute and
// a bare one takes the file's scope. It reports false when a bare id has no
// scope to take.
func resolveMessageID(written, scope string) (string, bool) {
	if strings.Contains(written, ".") {
		return written, true
	}
	if scope == "" {
		return "", false
	}
	return scope + "." + written, true
}

// moduleMessages walks every declaration body and returns the references in
// source order.
func moduleMessages(module *Module) []*MessageExpr {
	var out []*MessageExpr
	visit := func(expr Expr) {
		if message, ok := expr.(*MessageExpr); ok {
			out = append(out, message)
		}
	}
	for _, declaration := range module.Declarations {
		decl, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		body, ok := decl.Body.([]Node)
		if !ok {
			continue
		}
		walkNodeExprs(body, visit)
	}
	sort.SliceStable(out, func(i, j int) bool { return beforePosition(out[i].Pos, out[j].Pos) })
	return out
}

// walkNodeExprs visits every expression reachable from a node list, including
// the ones inside attribute values and control headers.
func walkNodeExprs(nodes []Node, visit func(Expr)) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *ExpressionNode:
			walkExpr(n.Expression, visit)
		case *syntax.MessageBlockNode:
			walkExpr(n.Message, visit)
			for _, hole := range n.Holes {
				walkNodeExprs(hole.Nodes, visit)
			}
		case *ElementNode:
			for _, attribute := range n.Attributes {
				for _, part := range attribute.Value {
					if part.Expression != nil {
						walkExpr(part.Expression, visit)
					}
				}
			}
			walkNodeExprs(n.Children, visit)
		case *HeadNode:
			walkNodeExprs(n.Children, visit)
		case *SlotNode:
			walkNodeExprs(n.Default, visit)
		case *ComponentNode:
			for _, argument := range n.Arguments {
				for _, part := range argument.Value {
					if part.Expression != nil {
						walkExpr(part.Expression, visit)
					}
				}
			}
			walkNodeExprs(n.Children, visit)
		case *IfNode:
			walkExpr(n.Condition, visit)
			walkNodeExprs(n.Then, visit)
			walkNodeExprs(n.Else, visit)
		case *ForNode:
			walkExpr(n.Iterable, visit)
			walkNodeExprs(n.Body, visit)
		case *ValNode:
			for _, binding := range n.Bindings {
				walkExpr(binding.Value, visit)
			}
			walkNodeExprs(n.Body, visit)
		case *AwaitNode:
			for _, binding := range n.Bindings {
				walkExpr(binding.Call, visit)
			}
			walkNodeExprs(n.Primary, visit)
			walkNodeExprs(n.Fallback, visit)
			walkNodeExprs(n.Recover, visit)
		}
	}
}

// messageAlias is the qualifier generated code calls a message symbol through.
// A symbol in the generated package needs none.
func messageAlias(symbol MessageSymbol) string {
	if symbol.Package == "" {
		return ""
	}
	if symbol.Alias != "" {
		return symbol.Alias
	}
	return pathBase(symbol.Package)
}

// collectMessageImports answers what the import block has to hold. Only the
// packages of symbols a template actually references are imported, so a project
// using no message generates the same bytes it does today.
func (e *goEmitter) collectMessageImports() map[string]string {
	imports := map[string]string{}
	for _, message := range moduleMessages(e.c.module) {
		symbol, ok := e.c.messages[message.ID]
		if !ok || symbol.Package == "" {
			continue
		}
		imports[symbol.Package] = messageAlias(symbol)
	}
	if len(imports) == 0 {
		return nil
	}
	return imports
}
