package htmlbind

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The hyphenated element space is a closed whitelist. A hyphen is HTML's own
// custom-element marker, so a template may legitimately write one — and an
// unrecognized one emitted unchanged renders nothing and reports nothing, which
// is the undiagnosable typo the whitelist removes.
//
// Two kinds go in it. A framework contributes builtin entries, rewritten at
// generation time into plan steps. An application contributes passthrough
// entries naming the Web Components it uses, emitted verbatim. Without the
// second kind a closed space would ban Web Components outright.
//
// Nothing is registered at runtime and nothing lives in a package init: the
// whole whitelist arrives with the generate command, exactly as call patterns
// and template file patterns do.

// BuiltinShape names how a builtin element produces its output.
type BuiltinShape string

// BuiltinMarkup lowers a fixed markup template with named holes. It is the only
// shape this milestone ships.
//
// The opaque shape — a provider returning a trusted value or a fragment, for
// output whose structure varies rather than only its values — is designed and
// not built. Its cost is that the trust assertion moves into framework code and
// the generator can no longer verify the emitted structure, which is why the
// verifiable shape went first.
const BuiltinMarkup BuiltinShape = "markup"

// ElementPlacement says which region of a document an element belongs in.
type ElementPlacement string

const (
	// PlaceEither is the zero value: the element may appear in the head or in
	// the body.
	PlaceEither ElementPlacement = ""
	// PlaceHead restricts an element to a head contribution.
	PlaceHead ElementPlacement = "head"
	// PlaceBody restricts an element to the document body.
	PlaceBody ElementPlacement = "body"
)

// ElementParam is one declared attribute of a builtin element. Its expression is
// type-checked at the call site exactly as an ordinary element's attribute is.
type ElementParam struct {
	// Name is the attribute an author writes, in kebab-case.
	Name string
	// Type is the template type name: string, int, bool, and the rest of the
	// core types.
	Type string
	// Required refuses a call site that leaves the attribute unset.
	Required bool
}

// ElementProvider names the Go function supplying a builtin element's
// per-request values.
//
// The signature is func(context.Context) (V, error). Nothing here checks it: as
// with a context-taking external, the caller reads its own Go sources and the Go
// compiler is the thing that rejects a mismatch. A generator that resolved Go
// symbols would have to load the target package, which is the dependency this
// package does not take.
type ElementProvider struct {
	// Package is the import path of the package holding the function. Empty
	// means the generated package's own.
	Package string
	// Alias overrides the import name. Empty uses the last path segment.
	Alias string
	// Name is the function.
	Name string
	// Result names its first result type, qualified the same way. It is needed
	// because a hole closure has to be written down, and Go infers a call's type
	// arguments but never a function literal's parameter types.
	//
	// For a single-hole element whose provider returns a bare value rather than
	// a struct, this is that value's type and the hole is written {{.}}.
	Result string
}

// BuiltinElement is one framework element the generator rewrites.
type BuiltinElement struct {
	// Name is the bare kebab-case element name an author writes.
	Name string
	// Params are the declared attributes, in the order a diagnostic lists them.
	Params []ElementParam
	// Context is the rule:template-context-safety insertion context this element
	// may appear in. Empty means html:child.
	Context string
	// Placement is the region that owns it. A head-only element written in the
	// body is a generation error rather than a page that half works.
	Placement ElementPlacement
	// Vary names the request properties this element's output depends on, such
	// as a cookie its provider reads.
	//
	// It is declared rather than derived, because only the implementation knows
	// what its provider reads. An undeclared axis is an invisible dependency: a
	// caller cannot build a Vary header for it and a shared cache cannot key on
	// it, and neither can find out by looking at the template.
	Vary []string
	// Assets are the static files this element requires. They join the required
	// set of every component that writes it, and their reference tags join its
	// head.
	Assets []Asset
	// Shape is how the output is produced. Empty means BuiltinMarkup.
	Shape BuiltinShape
	// Markup is the fixed output template. A hole is written {{.Name}} and names
	// either a declared parameter or a field of the provider's result; {{.}} is
	// the whole provider result, for a provider returning a bare value.
	//
	// Each hole is escaped for its position, and generation refuses a hole
	// anywhere but element text and an attribute value. That is what makes the
	// output unable to inject markup even if a provider returns hostile bytes.
	Markup string
	// Provider supplies the per-request holes. Nil means the element has none,
	// and then it costs nothing at render time: with no expression parameter
	// either, the whole thing folds into static bytes.
	Provider *ElementProvider
}

// PassthroughElement is a hyphenated element emitted verbatim.
//
// Name is either an exact element name or a prefix glob such as "sl-*", so a
// component library is declared once rather than per element.
type PassthroughElement struct {
	Name string
}

// perRequest reports whether rendering this element needs a context, which is
// derived from the shape rather than declared: a registration cannot then
// disagree with itself.
//
// A provider returning a process-constant value still counts. The safe
// direction is exclusion from a cache, so the conservative reading is the one
// taken.
func (b *BuiltinElement) perRequest() bool { return b.Provider != nil }

func (b *BuiltinElement) insertionContext() string {
	if b.Context == "" {
		return "html:child"
	}
	return b.Context
}

// elementSet is the resolved whitelist: an immutable snapshot taken before
// package analysis, so concurrent analysis shares it safely.
type elementSet struct {
	builtins map[string]*BuiltinElement
	// lowered holds each builtin's parsed markup, so the template is parsed once
	// at generation time and never at render time.
	lowered map[string][]markupSegment
	exact   map[string]bool
	globs   []string
	// names lists every declared name for the nearest-name suggestion, sorted.
	names []string
}

func (s *elementSet) empty() bool {
	return s == nil || (len(s.builtins) == 0 && len(s.exact) == 0 && len(s.globs) == 0)
}

// resolve reports how a hyphenated element name is declared.
func (s *elementSet) resolve(name string) (*BuiltinElement, bool) {
	if s == nil {
		return nil, false
	}
	if builtin, ok := s.builtins[name]; ok {
		return builtin, true
	}
	if s.exact[name] {
		return nil, true
	}
	// Patterns are matched after exact names, so a builtin never loses to a glob
	// that happens to cover it.
	for _, glob := range s.globs {
		if strings.HasPrefix(name, glob) {
			return nil, true
		}
	}
	return nil, false
}

// markupSegment is one piece of a lowered builtin element: fixed bytes, or a
// hole naming a parameter or a provider result field.
type markupSegment struct {
	static string
	// hole is the name inside {{ }}, empty for a static segment. "." is the
	// whole provider result.
	hole string
	// param is set when the hole names a declared parameter rather than a
	// provider field.
	param bool
}

// normalizeElements validates the registered entries and freezes them.
//
// Everything checkable without a template is checked here, because a
// registration mistake belongs to whoever wrote the generate command and should
// not wait for a template that happens to use the element.
func normalizeElements(builtins []BuiltinElement, passthrough []PassthroughElement) (*elementSet, error) {
	set := &elementSet{
		builtins: map[string]*BuiltinElement{},
		lowered:  map[string][]markupSegment{},
		exact:    map[string]bool{},
	}
	declared := map[string]string{}
	for i := range builtins {
		entry := builtins[i]
		if err := validCustomElementName(entry.Name); err != nil {
			return nil, fmt.Errorf("builtin element %q: %w", entry.Name, err)
		}
		if kind, taken := declared[entry.Name]; taken {
			return nil, fmt.Errorf("element %s is declared twice, once as %s and once as builtin; one contributor has to rename it", entry.Name, kind)
		}
		declared[entry.Name] = "builtin"
		segments, err := lowerMarkup(&entry)
		if err != nil {
			return nil, fmt.Errorf("builtin element %s: %w", entry.Name, err)
		}
		set.builtins[entry.Name] = &entry
		set.lowered[entry.Name] = segments
		set.names = append(set.names, entry.Name)
	}
	for _, entry := range passthrough {
		name := entry.Name
		if glob, ok := strings.CutSuffix(name, "*"); ok {
			if err := validGlobPrefix(glob); err != nil {
				return nil, fmt.Errorf("passthrough pattern %q: %w", name, err)
			}
			set.globs = append(set.globs, glob)
			set.names = append(set.names, name)
			continue
		}
		if err := validCustomElementName(name); err != nil {
			return nil, fmt.Errorf("passthrough element %q: %w", name, err)
		}
		if kind, taken := declared[name]; taken {
			return nil, fmt.Errorf("element %s is declared twice, once as %s and once as passthrough; one contributor has to rename it", name, kind)
		}
		declared[name] = "passthrough"
		set.exact[name] = true
		set.names = append(set.names, name)
	}
	return set, nil
}

// validCustomElementName rejects a name that is not a custom element name. The hyphen
// is what keeps the whitelist disjoint from real HTML: no standard element name
// carries one, so an entry can never shadow one.
func validCustomElementName(name string) error {
	if name == "" {
		return errors.New("name must not be empty")
	}
	if !strings.Contains(strings.Trim(name, "-"), "-") {
		return errors.New("a builtin or passthrough element name needs an interior hyphen, which is what keeps it out of the standard HTML element space")
	}
	return validNamePart(name)
}

func validGlobPrefix(prefix string) error {
	if !strings.HasSuffix(prefix, "-") {
		return errors.New("a glob has to end at a hyphen, as in \"sl-*\", so it cannot match a standard element name")
	}
	return validNamePart(strings.TrimSuffix(prefix, "-"))
}

func validNamePart(name string) error {
	if name[0] < 'a' || name[0] > 'z' {
		return errors.New("name must start with a lowercase letter")
	}
	if strings.HasSuffix(name, "-") {
		return errors.New("name must not end with a hyphen")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return errors.New("name must use lowercase letters, digits, and hyphens")
		}
	}
	return nil
}

// lowerMarkup parses a definition's markup once, at registration, and returns
// the segments a template using the element lowers to.
//
// Doing it here rather than per template file is what makes "no runtime template
// parsing" true of a second parse as well: the markup is read exactly once per
// generate command.
func lowerMarkup(entry *BuiltinElement) ([]markupSegment, error) {
	if entry.Shape != "" && entry.Shape != BuiltinMarkup {
		return nil, fmt.Errorf("shape %q is not implemented; only the markup shape ships today", entry.Shape)
	}
	if strings.TrimSpace(entry.Markup) == "" {
		return nil, errors.New("the markup shape needs markup")
	}
	params := map[string]string{}
	for _, param := range entry.Params {
		if err := validNamePart(param.Name); err != nil {
			return nil, fmt.Errorf("parameter %q: %w", param.Name, err)
		}
		key := holeKey(param.Name)
		if _, taken := params[key]; taken {
			return nil, fmt.Errorf("duplicate parameter %s", param.Name)
		}
		params[key] = param.Name
	}
	segments, holes, err := scanMarkup(entry.Markup)
	if err != nil {
		return nil, err
	}
	providerHoles := 0
	for i, segment := range segments {
		if segment.hole == "" {
			continue
		}
		if segment.hole != "." {
			if name, ok := params[holeKey(segment.hole)]; ok {
				segments[i].param = true
				segments[i].static = name
				continue
			}
		}
		providerHoles++
	}
	if providerHoles > 0 && entry.Provider == nil {
		return nil, fmt.Errorf("markup has a hole naming no declared parameter, so it needs a provider: %s", strings.Join(holes, ", "))
	}
	if providerHoles > 0 && entry.Provider.Result == "" {
		return nil, errors.New("a provider filling a hole needs its Result type named, because a hole closure has to be written down")
	}
	if entry.Provider != nil && entry.Provider.Result != "" {
		if err := validResultType(entry.Provider.Result); err != nil {
			return nil, fmt.Errorf("provider result %q: %w", entry.Provider.Result, err)
		}
	}
	if providerHoles > 0 && entry.Provider.Name == "" {
		return nil, errors.New("a provider needs a function name")
	}
	if entry.Provider != nil && providerHoles == 0 {
		return nil, errors.New("a provider is declared that no hole uses, so the element would be per-request for nothing")
	}
	return segments, nil
}

// holeKey folds a hole name and a parameter name to one spelling, so
// {{.ServiceName}} finds service-name.
//
// It drops separators and case rather than trying to split words, because
// splitting has to guess where an acronym ends: {{.ID}} against id, and
// {{.APIURL}} against api-url, are both wrong under every rule that inserts a
// separator before each capital. Getting that wrong is worse than it sounds —
// with a provider declared, a hole that fails to match a parameter is read as a
// provider field instead, so the value silently comes from the wrong place.
// validResultType closes the shape a provider result may take: a name, a
// qualified name, or a predeclared scalar.
//
// The point is not to be strict for its own sake. Emission has to decide whether
// to qualify the name with the provider's package, and any wider input makes
// that a guess: []Item and *CSRF carry no dot and are not scalars, so a rule that
// qualifies "everything else" produces fw.[]Item. Closing the input space is what
// makes the decision total instead of a heuristic that is wrong quietly.
//
// It also matches what a provider result is for. Holes read fields off it by
// name, so a slice or a map was never a usable shape, and a pointer would panic
// on a nil result rather than render an empty value.
func validResultType(name string) error {
	base, qualifier, found := name, "", false
	if at := strings.IndexByte(name, '.'); at >= 0 {
		qualifier, base, found = name[:at], name[at+1:], true
	}
	if found && !goIdentifierName(qualifier) {
		return errors.New("the package qualifier is not an identifier")
	}
	if !goIdentifierName(base) {
		return errors.New("a result must be a type name, a qualified type name, or a predeclared scalar;" +
			" a slice, a pointer, or a map cannot carry named holes")
	}
	return nil
}

// goIdentifierName reports whether a string is a bare Go identifier.
func goIdentifierName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func holeKey(name string) string {
	var out strings.Builder
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
			out.WriteRune(r - 'A' + 'a')
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		}
	}
	return out.String()
}

// scanMarkup splits a markup template into static runs and holes, and refuses a
// hole anywhere its value could not simply be escaped.
//
// The positions a hole may occupy are element text and a quoted attribute value,
// which take the same escaping. A hole in a tag name, an attribute name, an
// unquoted attribute value, or inside script or style would each need a
// different rule, and a seam that silently applied the wrong one is the failure
// this whole shape exists to prevent.
func scanMarkup(markup string) ([]markupSegment, []string, error) {
	var segments []markupSegment
	var holes []string
	var static strings.Builder
	// state tracks just enough HTML to place a hole: whether we are in text, in
	// a tag, or inside a quoted attribute value.
	const (
		inText = iota
		inTag
		inQuoted
	)
	state := inText
	var quote byte
	// raw names an element whose content is not markup, so a hole inside it
	// would need script or style escaping rather than HTML escaping.
	raw := ""
	for i := 0; i < len(markup); {
		if strings.HasPrefix(markup[i:], "{{") {
			end := strings.Index(markup[i:], "}}")
			if end < 0 {
				return nil, nil, errors.New("markup has an unterminated hole")
			}
			name := strings.TrimSpace(markup[i+2 : i+end])
			name = strings.TrimPrefix(name, ".")
			if name == "" {
				name = "."
			}
			switch {
			case raw != "":
				return nil, nil, fmt.Errorf("hole {{.%s}} sits inside <%s>, whose content is not markup; that needs the opaque shape", name, raw)
			case state == inTag:
				return nil, nil, fmt.Errorf("hole {{.%s}} sits in a tag name or an attribute name, which cannot be escaped as a value", name)
			case state == inQuoted, state == inText:
			}
			segments = append(segments, markupSegment{static: static.String()}, markupSegment{hole: name})
			static.Reset()
			holes = append(holes, name)
			i += end + 2
			continue
		}
		c := markup[i]
		switch state {
		case inText:
			if c == '<' {
				state = inTag
				// A closing tag ends whatever raw element we were in.
				if strings.HasPrefix(markup[i:], "</") {
					raw = ""
				}
			}
		case inTag:
			switch {
			case c == '"' || c == '\'':
				state, quote = inQuoted, c
			case c == '>':
				state = inText
				if raw == "" {
					raw = markupRawTextElement(markup[:i+1])
				}
			}
		case inQuoted:
			if c == quote {
				state = inTag
			}
		}
		static.WriteByte(c)
		i++
	}
	if state != inText {
		return nil, nil, errors.New("markup ends inside a tag")
	}
	if tail := static.String(); tail != "" {
		segments = append(segments, markupSegment{static: tail})
	}
	return segments, holes, nil
}

// rawTextElement reports whether the tag just closed opens an element whose
// content is not markup.
func markupRawTextElement(upToTagEnd string) string {
	start := strings.LastIndexByte(upToTagEnd, '<')
	if start < 0 {
		return ""
	}
	name := upToTagEnd[start+1:]
	for i := 0; i < len(name); i++ {
		if name[i] == ' ' || name[i] == '>' || name[i] == '/' || name[i] == '\t' || name[i] == '\n' {
			name = name[:i]
			break
		}
	}
	switch strings.ToLower(name) {
	case "script", "style", "textarea", "title":
		return strings.ToLower(name)
	}
	return ""
}

// nearest suggests a declared name for an undeclared one, so a typo reports the
// thing the author meant rather than only the thing they wrote.
//
// It is a prefix and edit-distance blend kept deliberately simple: the whitelist
// is small, and a suggestion that is merely plausible still beats listing every
// entry.
func (s *elementSet) nearest(name string) string {
	best, bestScore := "", 0
	for _, candidate := range s.names {
		score := commonPrefix(name, candidate)*2 - abs(len(candidate)-len(name))
		if score > bestScore {
			best, bestScore = candidate, score
		}
	}
	return best
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// undeclaredElement is the diagnostic for a hyphenated element the whitelist
// does not hold. It names the way out for each of the two reasons it happens.
func (s *elementSet) undeclaredElement(name string) string {
	message := "undeclared element <" + name + ">"
	if suggestion := s.nearest(name); suggestion != "" {
		message += "; did you mean <" + suggestion + ">?"
	}
	if s.empty() {
		return message + " no hyphenated element is declared, so every one is undeclared;" +
			" a framework registers a builtin entry and an application registers a passthrough entry for each Web Component it uses"
	}
	return message + " register it as a passthrough entry if it is a Web Component, or as a builtin if a framework rewrites it"
}

// foreignRoot reports whether an element opens a foreign-namespace subtree.
func foreignRoot(name string) bool { return name == "svg" || name == "math" }

// hyphenated reports whether a name sits in HTML's custom-element space.
func hyphenated(name string) bool {
	return strings.Contains(name, "-") && !strings.HasPrefix(name, "-")
}

// resolveElement classifies one element against the whitelist.
//
// It returns the definition for a builtin, and reports whether the name was
// declared at all. An undeclared hyphenated element is an error, which is the
// whole point of closing the space: the alternative is markup a browser ignores
// and a generator never mentions.
func (c *compiler) resolveElement(node *ElementNode) (*BuiltinElement, bool, error) {
	if !hyphenated(node.Name) || c.foreignDepth > 0 {
		return nil, false, nil
	}
	builtin, declared := c.elements.resolve(node.Name)
	if !declared {
		return nil, false, c.error(node.Pos, c.elements.undeclaredElement(node.Name))
	}
	return builtin, true, nil
}

// analyzeBuiltinElement checks one use of a registered builtin element and
// records what it costs the component writing it.
func (c *compiler) analyzeBuiltinElement(node *ElementNode, builtin *BuiltinElement, scope map[string]valueType) error {
	// A definition never constrains its allowed ancestor element, because the
	// enclosing form may live in a caller, a layout, or a slot fill in another
	// file. Placement is the one region question that is always answerable here,
	// since everything this walk reaches is the body.
	if builtin.Placement == PlaceHead {
		return c.error(node.Pos, "<"+node.Name+"> is declared head-only and is written in the body")
	}
	if hasContent(node.Children) {
		return c.error(node.Pos, "<"+node.Name+"> takes no children")
	}
	declared := map[string]ElementParam{}
	for _, param := range builtin.Params {
		declared[param.Name] = param
	}
	supplied := map[string]bool{}
	for _, attribute := range node.Attributes {
		param, ok := declared[attribute.Name]
		if !ok {
			return c.error(node.Pos, "<"+node.Name+"> has no attribute "+attribute.Name+
				"; it declares "+declaredList(builtin.Params))
		}
		if supplied[attribute.Name] {
			return c.error(node.Pos, "duplicate attribute "+attribute.Name+" on <"+node.Name+">")
		}
		supplied[attribute.Name] = true
		want, err := c.resolveType(TypeRef{Name: param.Type})
		if err != nil {
			return c.error(node.Pos, "<"+node.Name+"> declares "+attribute.Name+" as "+param.Type+", which is not a template type")
		}
		got, err := c.attributeValueType(attribute, scope)
		if err != nil {
			return err
		}
		if !assignable(want, got) {
			return c.error(node.Pos, "attribute "+attribute.Name+" of <"+node.Name+"> expects "+want.String()+", got "+got.String())
		}
	}
	for _, param := range builtin.Params {
		if param.Required && !supplied[param.Name] {
			return c.error(node.Pos, "<"+node.Name+"> requires the attribute "+param.Name)
		}
	}
	if c.current == nil {
		return nil
	}
	// A per-request element inside a cached region would store one visitor's
	// token and serve it to the next, which is a security failure rather than a
	// staleness bug. The same exclusion is why the flag propagates: a cached
	// component calling one that writes the element is the same mistake one
	// level up.
	if builtin.perRequest() {
		c.current.perRequest = true
		c.current.perRequestElement = node.Name
	}
	c.current.builtinAssets = append(c.current.builtinAssets, builtin.Assets...)
	c.current.vary = append(c.current.vary, builtin.Vary...)
	c.current.builtins = append(c.current.builtins, builtin.Name)
	return nil
}

func declaredList(params []ElementParam) string {
	if len(params) == 0 {
		return "none"
	}
	names := make([]string, 0, len(params))
	for _, param := range params {
		names = append(names, param.Name)
	}
	return strings.Join(names, ", ")
}

// builtinAt reports whether an element in the body is a registered builtin.
// Analysis already refused an undeclared one, so a miss here is a passthrough.
func (e *goEmitter) builtinAt(node *ElementNode) (*BuiltinElement, bool) {
	if !hyphenated(node.Name) || e.foreignDepth > 0 {
		return nil, false
	}
	builtin, _ := e.c.elements.resolve(node.Name)
	return builtin, builtin != nil
}

// emitBuiltinElement lowers one use of a registered element.
//
// The fixed part of the markup is folded into the surrounding static run, so it
// costs the same as if the author had typed it. What is left is the per-request
// part, and an element that has none — no provider and no expression attribute —
// leaves nothing at all: it reduces entirely to static bytes and adds no plan
// step.
func (e *goEmitter) emitBuiltinElement(p *planEmitter, node *ElementNode, builtin *BuiltinElement) error {
	segments := e.c.elements.lowered[builtin.Name]
	supplied := map[string]Attribute{}
	for _, attribute := range node.Attributes {
		supplied[attribute.Name] = attribute
	}
	if builtin.Provider == nil {
		return e.emitFoldedElement(p, node, builtin, segments, supplied)
	}
	return e.emitProvidedElement(p, node, builtin, segments, supplied)
}

// emitFoldedElement lowers an element with no provider: static bytes plus one
// ordinary text step per attribute-backed hole.
//
// Both positions a hole may occupy escape identically, and the quotes around an
// attribute value are static bytes this lowering writes itself, so one step kind
// covers both.
func (e *goEmitter) emitFoldedElement(p *planEmitter, node *ElementNode, builtin *BuiltinElement, segments []markupSegment, supplied map[string]Attribute) error {
	for _, segment := range segments {
		if segment.hole == "" {
			p.static(segment.static)
			continue
		}
		code, err := e.holeCode(p, node, builtin, segment, supplied)
		if err != nil {
			return err
		}
		p.flush()
		p.op(fmt.Sprintf("Text(func(%s %s) string { return %s })", receiverIdent, p.scope.goType, code))
	}
	return nil
}

// emitProvidedElement lowers an element whose holes come from a provider: one
// step, so the provider is called once for this occurrence rather than once per
// hole.
func (e *goEmitter) emitProvidedElement(p *planEmitter, node *ElementNode, builtin *BuiltinElement, segments []markupSegment, supplied map[string]Attribute) error {
	provider := builtin.Provider
	qualifier := e.providerQualifier(provider)
	result := provider.Result
	call := provider.Name
	if qualifier != "" {
		call = qualifier + "." + provider.Name
		// Registration closed the input space to a name, a qualified name, or a
		// predeclared scalar, so this covers every case rather than guessing at
		// the ones it recognizes.
		if !strings.Contains(result, ".") && !builtinScalarType(result) {
			result = qualifier + "." + result
		}
	}
	segmentType := fmt.Sprintf("htmlbind.Segment[%s, %s]", p.scope.goType, result)
	var parts []string
	for _, segment := range segments {
		if segment.hole == "" {
			if segment.static == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("\t{Static: %s},", strconv.Quote(segment.static)))
			continue
		}
		code, err := e.holeCode(p, node, builtin, segment, supplied)
		if err != nil {
			return err
		}
		if !segment.param {
			code = providerFieldCode(segment.hole)
		}
		parts = append(parts, fmt.Sprintf("\t{Hole: func(%s %s, v %s) string { return %s }},",
			receiverIdent, p.scope.goType, result, code))
	}
	p.flush()
	// The memo key is the provider rather than the element, so two elements
	// backed by one function — a hidden input and a meta tag carrying the same
	// token — cannot disagree with each other.
	p.raw(fmt.Sprintf("htmlbind.Provide(%s, %s, %s, []%s{\n%s\n})",
		strconv.Quote(builtin.Name), strconv.Quote(providerKey(provider)), call, segmentType, strings.Join(parts, "\n")))
	return nil
}

// providerKey identifies one provider function across the elements that use it.
// It is the import path and the name, which is what rule:go-types-symbol-identity
// already treats as a symbol's identity.
func providerKey(provider *ElementProvider) string {
	if provider.Package == "" {
		return provider.Name
	}
	return provider.Package + "." + provider.Name
}

// providerFieldCode reads one field of the provider's result. A bare {{.}} is
// the whole value, which is what a single-hole element backed by a provider
// returning a plain string uses.
func providerFieldCode(hole string) string {
	if hole == "." {
		return "v"
	}
	return "v." + hole
}

// builtinScalarType reports whether a provider's declared result is a predeclared
// Go type, which takes no package qualifier.
func builtinScalarType(name string) bool {
	switch name {
	case "string", "bool", "int", "int64", "float64", "byte", "rune":
		return true
	}
	return false
}

// holeCode produces the Go expression filling one hole.
func (e *goEmitter) holeCode(p *planEmitter, node *ElementNode, builtin *BuiltinElement, segment markupSegment, supplied map[string]Attribute) (string, error) {
	if !segment.param {
		return providerFieldCode(segment.hole), nil
	}
	// A parameter hole carries the parameter's own spelling, resolved once at
	// registration, so emission never re-derives it and the two cannot disagree.
	name := segment.static
	attribute, present := supplied[name]
	if !present {
		// Analysis already refused a missing required attribute, so this is an
		// optional one nobody set. An absent value writes nothing rather than
		// the word describing its absence.
		return `""`, nil
	}
	if e.attributeUsesRenderContext(attribute) {
		return "", e.c.error(node.Pos, "attribute "+name+" of <"+node.Name+
			"> calls an external that needs the render context, which a builtin element's hole cannot carry")
	}
	if len(attribute.Value) == 1 && attribute.Value[0].Expression != nil {
		part := attribute.Value[0]
		code, err := e.exprCode(part.Expression, p.scope)
		if err != nil {
			return "", err
		}
		t := e.c.exprTypes[part.Expression]
		if t.optional {
			return "func() string { if " + code + " == nil { return \"\" }; return " + valueString("*("+code+")", t.required()) + " }()", nil
		}
		return valueString(code, t), nil
	}
	// A literal or a mixed value: everything static resolves now, and each
	// expression contributes its own formatted piece.
	var pieces []string
	for _, part := range attribute.Value {
		if part.Expression == nil {
			pieces = append(pieces, strconv.Quote(part.Text))
			continue
		}
		code, err := e.exprCode(part.Expression, p.scope)
		if err != nil {
			return "", err
		}
		pieces = append(pieces, valueString(code, e.c.exprTypes[part.Expression]))
	}
	if len(pieces) == 0 {
		return `""`, nil
	}
	return strings.Join(pieces, " + "), nil
}

// providerQualifier is the name generated code calls a provider through. A
// provider in the generated package needs none.
func (e *goEmitter) providerQualifier(provider *ElementProvider) string {
	return providerAlias(provider)
}

func providerAlias(provider *ElementProvider) string {
	if provider.Package == "" {
		return ""
	}
	if provider.Alias != "" {
		return provider.Alias
	}
	return pathBase(provider.Package)
}

// collectProviderImports answers what the import block has to hold, before any
// plan is emitted.
//
// It cannot be a side effect of emitting the steps that need it: the import
// block is written first, so a package discovered while lowering an element
// would arrive after the only place it could be declared. The analysis pass
// already recorded which elements each component writes, so the answer is
// available without walking the templates again.
//
// Only an element a template actually writes counts, per
// rule:usage-directed-generation: registering a whitelist a module never uses
// must leave its imports alone.
func (e *goEmitter) collectProviderImports() map[string]string {
	imports := map[string]string{}
	for _, info := range e.c.components {
		for _, name := range info.builtins {
			builtin, ok := e.c.elements.builtins[name]
			if !ok || builtin.Provider == nil || builtin.Provider.Package == "" {
				continue
			}
			imports[builtin.Provider.Package] = providerAlias(builtin.Provider)
		}
	}
	if len(imports) == 0 {
		return nil
	}
	return imports
}

// sortedKeys returns a map's keys in a stable order, because `--check` in CI
// compares bytes and a map walk would produce a different file on every run.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func pathBase(importPath string) string {
	if at := strings.LastIndexByte(importPath, '/'); at >= 0 {
		return importPath[at+1:]
	}
	return importPath
}
