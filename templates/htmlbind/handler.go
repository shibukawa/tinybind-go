package htmlbind

import "strings"

// ClientHandlerPrefix marks an attribute naming a function the component's
// script block produced, as in on-click="increment".
//
// The hyphen is what makes the namespace free: rule:event-attribute-context
// excludes a hyphenated on- name from the event handler roster, so onclick keeps
// meaning inline JavaScript and this spelling means something else entirely.
// Taking the unhyphenated form instead would silently reinterpret a shipped
// feature, because a bare identifier is a valid expression statement.
const ClientHandlerPrefix = "on-"

// DefaultClientHandlerAttr is the attribute the lowering writes when
// GenerateOptions.ClientHandlerAttr is empty.
//
// The authored attributes are lowered into this one rather than left in place
// because CSS has no attribute-name prefix match: finding on-anything means
// walking every element on every mount and every swap, where one marker is a
// single indexed query. Leaving them would also claim the namespace
// rule:event-attribute-context assigns to custom elements.
const DefaultClientHandlerAttr = "data-tb-on"

// ClientHandlerSet is what one component's script block exposes, as the caller
// resolved it.
//
// The module reads no JavaScript. The caller parses the block reported by
// [ComponentScripts] and answers with this, which is the same arrangement
// GenerateOptions.ServerActions already uses for a URL the compiler cannot
// compute.
type ClientHandlerSet struct {
	// Resolved names the handlers the block exposes.
	Resolved []string
	// Unresolved maps a name the template referenced to why the caller refused
	// it. Reporting a refusal here rather than by omission is required: an
	// omission cannot be told from a map that was never populated, and the module
	// would report every name of a mis-parsed block as unknown.
	Unresolved map[string]string
}

func (s ClientHandlerSet) has(name string) bool {
	for _, resolved := range s.Resolved {
		if resolved == name {
			return true
		}
	}
	return false
}

// ClientHandlerRef is one on-prefixed attribute found in a template.
type ClientHandlerRef struct {
	// Component is the declaration the reference appears in.
	Component string
	// Event is the name after the prefix, such as click.
	Event string
	// Handler is the name the attribute's value gave.
	Handler string
	// Element is the element carrying the attribute.
	Element string
	// Pos is the attribute position, for a diagnostic that can quote the source.
	Pos Position
}

// isClientHandlerName reports the attribute shape this feature reserves: the
// literal prefix, then one or more ASCII lowercase letters to the end of the
// name.
//
// The tail is spelled the way rule:event-attribute-context spells its own, so
// the two rosters divide the on- space along one line rather than two. A name
// carrying a second hyphen, such as on-my-event, is not matched and stays the
// ordinary custom-element attribute that rule already calls it.
func isClientHandlerName(name string) bool {
	rest, ok := strings.CutPrefix(name, ClientHandlerPrefix)
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// isJSIdentifier reports whether a value can name a function in the block. The
// check is spelled here rather than borrowed from a parser because the module
// reads no JavaScript; it exists to keep a value that cannot possibly resolve,
// or that would break the emitted grammar, from reaching the caller's map.
func isJSIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r == '$':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// analyzeClientHandler validates one on-prefixed attribute and records the
// reference. It runs only inside a component declaring a script block; outside
// one the attribute keeps its ordinary meaning and never reaches here.
func (c *compiler) analyzeClientHandler(element string, attribute Attribute, siblings []Attribute) error {
	event := strings.TrimPrefix(attribute.Name, ClientHandlerPrefix)
	if attribute.Boolean {
		return c.error(attribute.Pos, attribute.Name+" must name a function the script block exports, as in "+
			attribute.Name+`="increment"`)
	}
	name, static := staticAttributeText(attribute)
	if !static {
		return c.error(attribute.Pos, attribute.Name+
			" must be a literal handler name; a computed value cannot be checked against the script block")
	}
	if !isJSIdentifier(name) {
		return c.error(attribute.Pos, attribute.Name+" value "+quoteName(name)+
			" must be a function name")
	}
	// The second would be lowered into the same entry as the first and lost, so
	// it is refused rather than silently dropped.
	for _, sibling := range siblings {
		if sibling.Name == attribute.Name && sibling.Pos != attribute.Pos {
			return c.error(attribute.Pos, "this element carries "+attribute.Name+
				" twice; one element binds one handler per event")
		}
	}

	component := ""
	if c.current != nil && c.current.decl != nil {
		component = c.current.decl.Name
	}
	if set, declared := c.clientHandlerSets[component]; declared {
		if reason, refused := set.Unresolved[name]; refused {
			return c.error(attribute.Pos, "the script block of component "+component+
				" does not provide "+quoteName(name)+": "+reason)
		}
		if !set.has(name) {
			return c.error(attribute.Pos, "the script block of component "+component+
				" does not export "+quoteName(name))
		}
	}
	c.clientHandlers = append(c.clientHandlers, ClientHandlerRef{
		Component: component,
		Event:     event,
		Handler:   name,
		Element:   element,
		Pos:       attribute.Pos,
	})
	return nil
}

// clientHandlerValue is the lowered attribute's value for one element: a comma
// between entries and a colon within one, as in "click:increment,blur:validate".
//
// The order is the order the attributes were authored in, so the output is
// deterministic and a reader of the markup finds what they wrote.
func clientHandlerValue(node *ElementNode) string {
	var out strings.Builder
	for _, attribute := range node.Attributes {
		if !isClientHandlerName(attribute.Name) {
			continue
		}
		name, _ := staticAttributeText(attribute)
		if out.Len() > 0 {
			out.WriteByte(',')
		}
		out.WriteString(strings.TrimPrefix(attribute.Name, ClientHandlerPrefix))
		out.WriteByte(':')
		out.WriteString(name)
	}
	return out.String()
}
