package htmlbind

import (
	"fmt"
	"sort"
	"strconv"
)

// DefaultComponentParameterAttr is the attribute a component's emitted
// parameters are written to when GenerateOptions.ComponentParameterAttr is
// empty.
//
// It rides the same root element as the declaration marker of
// requirement:scoped-script-declaration, which is why a component declaring a
// script block already has to render exactly one root: the invariant this needs
// exists for that marker and is reused rather than added.
const DefaultComponentParameterAttr = "data-tb-props"

// validateComponentParameters checks the caller's chosen sets against the
// declarations they name.
//
// The caller derives these sets by reading the script block, which this module
// never parses, so every failure here is one the module can still see: a
// component that does not exist, one with no block to consume the values, a
// parameter that was never declared, or a type that has no JSON form.
func (c *compiler) validateComponentParameters() error {
	if len(c.componentParameters) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.componentParameters))
	for name := range c.componentParameters {
		names = append(names, name)
	}
	// The map has no order, and a diagnostic that changes between runs is worse
	// than a slow one.
	sort.Strings(names)

	for _, component := range names {
		parameters := c.componentParameters[component]
		if len(parameters) == 0 {
			continue
		}
		info := c.components[component]
		if info == nil || info.decl == nil {
			return fmt.Errorf("htmlbind: %s: parameters were named for component %q, which this module does not declare",
				c.filename, component)
		}
		if info.script == "" {
			return c.error(info.decl.Pos, "parameters were named for component "+component+
				", which declares no script block; the object is emitted for a block to read and nothing else would consume it")
		}
		for _, parameter := range parameters {
			t, declared := info.params[parameter]
			if !declared {
				return c.error(info.decl.Pos, "component "+component+" has no parameter "+quoteName(parameter)+
					", so it cannot be emitted")
			}
			// The rule is the one JsonForScript already applies, rather than the
			// query-string rule a reloadable component carries: that one refuses a
			// record and a slice because a query string must carry them
			// deterministically, and an attribute holding JSON is not a query
			// string.
			if !c.jsonSerializable(t, map[string]bool{}) {
				return c.error(info.decl.Pos, "parameter "+quoteName(parameter)+" of component "+component+
					" has no JSON form, so it cannot be emitted; the author asked for it by naming it in code that uses it, so this is an error rather than a silent omission")
			}
		}
	}
	return nil
}

// emitComponentParameters writes the object of named parameters onto the
// component's root element.
//
// It is a render-time instruction rather than static text, unlike the
// declaration marker beside it: the marker is a compile-time constant and this
// is the instance's own arguments. That cost is why the set is chosen by the
// caller rather than being every parameter.
func (e *goEmitter) emitComponentParameters(p *planEmitter, node *ElementNode) error {
	if node != e.scopeRoot || e.scopeComponent == "" {
		return nil
	}
	parameters := e.componentParams[e.scopeComponent]
	if len(parameters) == 0 {
		return nil
	}
	info := e.c.components[e.scopeComponent]

	body := "body := \"\"\n"
	for _, parameter := range parameters {
		t := info.params[parameter]
		field := "p." + goPublicName(parameter)
		if t.optional {
			// An absent optional omits its key rather than writing null, so
			// JavaScript has one absence to test instead of two.
			body += "if " + field + " != nil {\n"
			body += "body = htmlbind.JSONMember(body, " + strconv.Quote(parameter) + ", " +
				jsonEncodeCall(t.required(), "*"+field) + ")\n"
			body += "}\n"
			continue
		}
		body += "body = htmlbind.JSONMember(body, " + strconv.Quote(parameter) + ", " +
			jsonEncodeCall(t, field) + ")\n"
	}
	// The attribute op writes its value verbatim, so the escaping is this
	// closure's to do.
	body += `return htmlbind.Escape("{" + body + "}"), true`

	p.flush()
	p.op(fmt.Sprintf("Attr(%s, func(p %s) (string, bool) {\n%s\n})",
		strconv.Quote(e.componentParamAttr), p.scope.goType, body))
	return nil
}
