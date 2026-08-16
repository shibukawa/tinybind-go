package htmlbind

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// ImplicitBinding is a name the embedder puts in every template's scope, so an
// application does not thread a framework value through every component and
// every layout in a chain.
//
// The value is produced by a Go function taking the render context, which is
// the same shape requirement:render-value-provider already uses for a builtin
// element. That keeps decision:reflection-free intact: nothing is looked up by
// string at run time, and a binding a template never writes emits nothing.
//
// This module learns no meaning from a binding. The names, the values and what
// they stand for are the embedder's, per
// .knowledge requirement:embedder-implicit-bindings.
type ImplicitBinding struct {
	// Name is what an author writes, as a bare identifier. It must not collide
	// with a component parameter; one that does is a generation error naming
	// the collision, because a silently shadowed framework value renders
	// whatever the parameter holds.
	Name string
	// Provider names the Go function returning the value. It is called with the
	// render context and must return a string.
	Provider BindingProvider
	// PathSegment marks a binding that stands for one URL path segment.
	//
	// It is the only kind permitted into a URL attribute despite not being
	// url-typed, and the only one whose empty value collapses the separator
	// before it. Both are scoped to the kind rather than to emptiness: an
	// ordinary empty interpolation must keep rendering nothing, or a future
	// url-typed value would acquire behavior it never asked for.
	//
	// The value is percent-encoded at emission, because a binding is
	// embedder-supplied but usually request-derived — a language resolved from
	// Accept-Language or from a path prefix is attacker-influenced by
	// definition. See .knowledge requirement:embedder-implicit-bindings.
	PathSegment bool
	// VaryAxis is the response header name this binding's value depends on,
	// empty for one an application recovers from the URL.
	//
	// The axis is the embedder's to name because only it knows how the value
	// was resolved: an application carrying it in a path prefix declares none,
	// since two languages are already two URLs, while one negotiating from a
	// request header must declare that header or nothing downstream can cache
	// the page correctly. See .knowledge decision:implicit-binding-cache-identity.
	VaryAxis string
}

// BindingProvider names the Go function behind an implicit binding, on the same
// terms as ElementProvider.
type BindingProvider struct {
	// Package is the import path holding the function. Empty means the
	// generated package's own.
	Package string
	// Alias overrides the import name. Empty uses the last path segment.
	Alias string
	// Name is the function.
	Name string
	// Result names the Go type the function returns, qualified the same way.
	// Empty means string.
	//
	// A binding returning something other than a string cannot be interpolated
	// into markup — there is no escaping rule for a type this module has never
	// seen — so it is usable only as GenerateOptions.MessageContextBinding.
	// Reading one in a template is a generation error naming the binding.
	Result string
}

// AnalysisOption configures an analysis-only entry point such as [Signatures],
// [ComponentScripts] or [ActionRefs].
//
// Those read a template without generating one, and a template reading an
// implicit binding cannot be analyzed without knowing the binding's name: an
// undeclared name is an unknown identifier, which is what the check exists for.
// The symbol table needs no equivalent, because a message reference is checked
// against it at generation rather than during analysis.
type AnalysisOption func(*analysisOptions)

type analysisOptions struct {
	bindings []ImplicitBinding
}

// WithAnalysisBindings supplies the same list [GenerateOptions.ImplicitBindings]
// carries, so an analysis entry point reads what generation will compile.
func WithAnalysisBindings(bindings []ImplicitBinding) AnalysisOption {
	return func(o *analysisOptions) { o.bindings = bindings }
}

// applyAnalysisOptions resolves the binding table for an analysis-only entry.
func applyAnalysisOptions(options []AnalysisOption) (*bindingSet, error) {
	var resolved analysisOptions
	for _, option := range options {
		option(&resolved)
	}
	return normalizeImplicitBindings(resolved.bindings)
}

// bindingSet is the resolved binding table, frozen before analysis so a
// registration mistake is reported against whoever wrote the generate command
// rather than against the first template that happens to use a name.
type bindingSet struct {
	byName map[string]ImplicitBinding
	// order is the declaration order, which fixes the order binding values are
	// framed into a cache key. Read order would make the key depend on where an
	// author happened to write the first read.
	order []string
}

func normalizeImplicitBindings(bindings []ImplicitBinding) (*bindingSet, error) {
	if len(bindings) == 0 {
		return &bindingSet{}, nil
	}
	set := &bindingSet{byName: map[string]ImplicitBinding{}}
	for _, binding := range bindings {
		if binding.Name == "" {
			return nil, fmt.Errorf("implicit binding has no name")
		}
		if !isLowerCamelName(binding.Name) {
			return nil, fmt.Errorf("implicit binding %q must be a lowerCamelCase identifier", binding.Name)
		}
		if binding.PathSegment && !bindingIsString(binding) {
			return nil, fmt.Errorf("implicit binding %q is a path segment, so its provider must return a string", binding.Name)
		}
		if binding.Provider.Name == "" {
			return nil, fmt.Errorf("implicit binding %q has no provider function", binding.Name)
		}
		if _, duplicate := set.byName[binding.Name]; duplicate {
			return nil, fmt.Errorf("duplicate implicit binding %q", binding.Name)
		}
		set.byName[binding.Name] = binding
		set.order = append(set.order, binding.Name)
	}
	return set, nil
}

// isLowerCamelName is the same shape the template language requires of a
// parameter name, so a binding cannot be spelled in a way no author could have
// written as a parameter.
func isLowerCamelName(value string) bool {
	if value == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(value)
	if !unicode.IsLower(r) {
		return false
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func (s *bindingSet) lookup(name string) (ImplicitBinding, bool) {
	if s == nil || s.byName == nil {
		return ImplicitBinding{}, false
	}
	binding, ok := s.byName[name]
	return binding, ok
}

// bindingAlias is the qualifier generated code calls a provider through. A
// provider in the generated package needs none.
func bindingAlias(provider BindingProvider) string {
	if provider.Package == "" {
		return ""
	}
	if provider.Alias != "" {
		return provider.Alias
	}
	return pathBase(provider.Package)
}

// bindingIsString reports whether a binding may be interpolated into markup.
func bindingIsString(binding ImplicitBinding) bool {
	return binding.Provider.Result == "" || binding.Provider.Result == "string"
}

// bindingCall is the Go expression reading a binding at render time.
func bindingCall(binding ImplicitBinding) string {
	call := binding.Provider.Name
	if alias := bindingAlias(binding.Provider); alias != "" {
		call = alias + "." + binding.Provider.Name
	}
	return call + "(" + contextIdent + ")"
}

// collectBindingImports answers what the import block has to hold. Only the
// packages of bindings a template actually reads are imported, so a project
// declaring some and using none regenerates unchanged.
func (e *goEmitter) collectBindingImports() map[string]string {
	imports := map[string]string{}
	for _, info := range e.c.components {
		for _, name := range info.bindings {
			binding, ok := e.c.bindings.lookup(name)
			if !ok || binding.Provider.Package == "" {
				continue
			}
			imports[binding.Provider.Package] = bindingAlias(binding.Provider)
		}
	}
	if len(imports) == 0 {
		return nil
	}
	return imports
}

// recordBindingRead notes that the component being analyzed reads a binding.
// It is recorded per component rather than per module because every consumer —
// the cache refusal, the vary fold and the import block — asks about a
// component's own call graph.
func (c *compiler) recordBindingRead(name string) {
	if c.current == nil {
		return
	}
	for _, existing := range c.current.bindings {
		if existing == name {
			return
		}
	}
	c.current.bindings = append(c.current.bindings, name)
	// The axis folds up the call graph exactly as a builtin element's does, so
	// a caller writing a Vary header sees what a nested component depends on.
	// An empty axis contributes nothing, which is what an application carrying
	// the value in its URL declares.
	if binding, ok := c.bindings.lookup(name); ok && binding.VaryAxis != "" {
		c.current.vary = append(c.current.vary, binding.VaryAxis)
	}
}

// refuseBindingShadow reports a local name colliding with a declared binding.
//
// Every binder has to be covered, not only the parameter list the request
// names: a val binding or a loop variable taking the name shadows just as
// silently, and scope wins over the binding table by construction, so the
// binding would simply stop being what the author reads.
func (c *compiler) refuseBindingShadow(name, kind string, pos Position) error {
	if name == "" {
		return nil
	}
	if _, declared := c.bindings.lookup(name); !declared {
		return nil
	}
	return c.error(pos, kind+" "+name+" shadows the implicit binding "+name+
		", which every template already has in scope")
}

// isPathSegmentRead reports whether an expression is a bare read of a declared
// path-segment binding.
//
// It is the one exception to the url type gate in
// requirement:url-attribute-scheme-safety, and it is deliberately narrow: only a
// bare identifier qualifies, so no expression built out of one can smuggle a
// plain string into a URL attribute.
func (c *compiler) isPathSegmentRead(expr Expr) bool {
	identifier, ok := expr.(*IdentifierExpr)
	if !ok {
		return false
	}
	binding, declared := c.bindings.lookup(identifier.Name)
	return declared && binding.PathSegment
}

// transitiveBindings collects the implicit bindings a component's output
// depends on, over the same call graph as transitiveVary: a nested component
// reading one makes the whole output depend on it, and only the value a caller
// holds can say so.
//
// The result is in declaration order, so a cache key is stable across
// regenerations that add no binding. It is the reached set rather than the
// declared one, so a project declaring three bindings does not make every
// cached component miss on all three.
func (c *compiler) transitiveBindings(name string) []string {
	reached := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string)
	visit = func(current string) {
		if visited[current] {
			return
		}
		visited[current] = true
		info, ok := c.components[current]
		if !ok {
			return
		}
		for _, binding := range info.bindings {
			reached[binding] = true
		}
		for _, called := range c.calledComponents(info) {
			visit(called)
		}
	}
	visit(name)
	if len(reached) == 0 {
		return nil
	}
	var out []string
	for _, declared := range c.bindings.order {
		if reached[declared] {
			out = append(out, declared)
		}
	}
	return out
}
