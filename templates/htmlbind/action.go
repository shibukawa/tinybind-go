package htmlbind

import (
	"fmt"
	"strings"
)

// ServerActionAttr is the reserved attribute naming a Go handler instead of a
// URL. It never reaches the output; the compiler replaces it with the attribute
// that carries the handler's endpoint.
//
// See decision:server-action-lowering. The value is a static handler name
// because the symbol has to resolve at generation, and only the caller can
// resolve it: the URL depends on the route the template serves, which the
// compiler cannot see. That is why lowering takes two passes, with [ActionRefs]
// reporting what a module references and GenerateOptions.ServerActions carrying
// the answers back.
const ServerActionAttr = "server-action"

// DefaultActionAttr is the attribute the lowering writes when
// GenerateOptions.ServerActionAttr is empty. A framework driving its own client
// library points it at that library's vocabulary instead.
const DefaultActionAttr = "data-tb-action"

// DefaultActionSelectorField is the hidden field a form carries to say which
// handler a native submit is for. The form posts to its own page, so the URL no
// longer identifies the handler and this does.
const DefaultActionSelectorField = "_action"

// serverActionName returns the handler an element names, if it names one. It is
// what lets a form be recognized as unsafe before the attribute loop runs: the
// generator writes method="post" onto such a form, so the absence of an authored
// method says nothing about whether a token is needed.
func serverActionName(node *ElementNode) (string, bool) {
	for _, attribute := range node.Attributes {
		if attribute.Name != ServerActionAttr {
			continue
		}
		name, static := staticAttributeText(attribute)
		return name, static
	}
	return "", false
}

// ActionRef is one server-action reference found in a template.
type ActionRef struct {
	// Component is the declaration the reference appears in.
	Component string
	// Handler is the Go function name the attribute named.
	Handler string
	// Element is the element carrying the attribute, such as form or button.
	Element string
	// Pos is the attribute position, for a diagnostic that can quote the source.
	Pos Position
}

// ActionRefs parses and analyzes a template module and returns every
// server-action reference it makes, in source order.
//
// It is the first of the two passes lowering needs: a caller resolves these
// names against the Go package beside the template, derives an endpoint URL for
// each, and passes the result to [Generate] as GenerateOptions.ServerActions.
//
// Like [Signatures] it runs the same analysis [Generate] does, so a module that
// fails to compile fails here with the same diagnostic rather than yielding a
// partial answer.
func ActionRefs(filename string, source []byte, options ...AnalysisOption) ([]ActionRef, error) {
	module, err := Parse(filename, source)
	if err != nil {
		return nil, err
	}
	bindings, err := applyAnalysisOptions(options)
	if err != nil {
		return nil, err
	}
	compiler := newCompiler(filename, string(source), module, true)
	compiler.bindings = bindings
	if err := compiler.analyze(); err != nil {
		return nil, err
	}
	return compiler.actions, nil
}

// analyzeServerAction validates one server-action attribute and records the
// reference. element is the element carrying it, which decides what else on
// that element is contradictory.
func (c *compiler) analyzeServerAction(element string, attribute Attribute, siblings []Attribute) error {
	if attribute.Boolean {
		return c.error(attribute.Pos, ServerActionAttr+" must name a Go function, as in "+
			ServerActionAttr+`="Update"`)
	}
	name, static := staticAttributeText(attribute)
	if !static {
		return c.error(attribute.Pos, ServerActionAttr+
			" must be a literal function name; a computed value cannot be resolved at generation")
	}
	if !isExportedGoIdent(name) {
		return c.error(attribute.Pos, ServerActionAttr+" value "+quoteName(name)+
			" must be an exported Go function name")
	}
	if name == ReservedPageFunc {
		return c.error(attribute.Pos, ServerActionAttr+" cannot name "+ReservedPageFunc+
			", which is the page's own entry point rather than a server action")
	}

	// A form carrying an author-written action would submit somewhere other than
	// the handler it names, and a method other than post cannot reach one at all.
	if element == "form" {
		for _, sibling := range siblings {
			switch sibling.Name {
			case "action":
				return c.error(sibling.Pos, "a form carrying "+ServerActionAttr+
					" cannot also carry action; the generator supplies the target")
			case "method":
				if value, ok := staticAttributeText(sibling); !ok || !strings.EqualFold(value, "post") {
					return c.error(sibling.Pos, "a form carrying "+ServerActionAttr+
						` must use method="post"`)
				}
			}
		}
	}

	component := ""
	if c.current != nil && c.current.decl != nil {
		component = c.current.decl.Name
	}
	c.actions = append(c.actions, ActionRef{
		Component: component,
		Handler:   name,
		Element:   element,
		Pos:       attribute.Pos,
	})
	return nil
}

// ReservedPageFunc is the Go entry point name a route package gives its own
// page, which is therefore never a server action.
const ReservedPageFunc = "Load"

// isExportedGoIdent reports the shape a server action name must have. The check
// is spelled here rather than borrowed from go/token because the value is
// template source at this point, and the diagnostic should name the template
// position rather than a Go parse failure.
func isExportedGoIdent(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			if i == 0 {
				return false
			}
		case r >= 'A' && r <= 'Z':
		case r == '_':
			if i == 0 {
				return false
			}
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

func quoteName(name string) string { return fmt.Sprintf("%q", name) }
