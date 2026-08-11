package htmlbind

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

type valueKind string

const (
	kindInvalid     valueKind = "invalid"
	kindString      valueKind = "string"
	kindBool        valueKind = "bool"
	kindInt         valueKind = "int"
	kindFloat       valueKind = "float"
	kindDecimal     valueKind = "decimal"
	kindDateTime    valueKind = "datetime"
	kindDate        valueKind = "date"
	kindTime        valueKind = "time"
	kindURL         valueKind = "url"
	kindBytes       valueKind = "bytes"
	kindRecord      valueKind = "record"
	kindEnum        valueKind = "enum"
	kindArray       valueKind = "array"
	kindHTML        valueKind = "html"
	kindTrustedHTML valueKind = "trusted_html"
	kindTrustedCSS  valueKind = "trusted_css"
	kindTrustedJS   valueKind = "trusted_javascript"
	kindScriptJSON  valueKind = "script_json"
	kindError       valueKind = "error"
)

// errorFields are the presentation-safe fields an await recover clause may read.
// They mirror htmlbind.AsyncError; the raw Go error never reaches a template.
var errorFields = map[string]valueType{
	"code":      {kind: kindString},
	"message":   {kind: kindString},
	"retryable": {kind: kindBool},
	"timeout":   {kind: kindBool},
}

type valueType struct {
	kind     valueKind
	name     string
	elem     *valueType
	optional bool
	// async marks a value the caller started and the template must bind in an
	// await clause before reading. It wraps the whole type, so it survives
	// neither required() nor awaited().
	async bool
}

func (t valueType) String() string {
	base := t.name
	if base == "" {
		base = string(t.kind)
	}
	if t.kind == kindArray && t.elem != nil {
		base = "[]" + t.elem.String()
	}
	if t.optional {
		base += "?"
	}
	if t.async {
		base = "async " + base
	}
	return base
}

func (t valueType) required() valueType { t.optional = false; return t }

// awaited is the type an await binding sees once the value settles.
func (t valueType) awaited() valueType { t.async = false; return t }

type functionSig struct {
	params []valueType
	result valueType
	// async marks a function that takes a context and returns an error. It may
	// only be called in an await binding.
	async bool
	// live marks a function returning a sequence of deliveries. It may only be
	// called in a live binding.
	live bool
}

// cachePolicy is the validated form of a component's cache annotation.
//
// The annotation does two separable things, and which ones it does is decided by
// the ttl argument alone: writing one asks for storage, and omitting one leaves
// a declaration of scope and nothing else. That is why a layout can carry the
// annotation at all — it can never store, so a duration on one would describe an
// expiry that cannot happen.
type cachePolicy struct {
	// ttl is zero when the annotation only declares scope.
	ttl time.Duration
	// public is the declared scope. False is private, which is the default,
	// because a component that is actually shared costs a miss when it is
	// treated as per-reader and serves one reader's output to another when the
	// mistake runs the other way.
	public bool
	// pos is where the scope was declared, for the diagnostic a refused public
	// assertion prints.
	pos Position
}

// stores reports whether this annotation asks for output to be stored, which is
// what every decision:cache-component-declaration eligibility rule is about.
func (p *cachePolicy) stores() bool { return p != nil && p.ttl > 0 }

// stores is the componentInfo form, tolerating an absent component so a caller
// walking declarations does not repeat the nil check.
func (i *componentInfo) stores() bool { return i != nil && i.cache.stores() }

type componentInfo struct {
	decl   *TemplateDecl
	params map[string]valueType
	order  []Parameter
	// reloadable marks a component published as a redraw endpoint. It is an
	// opt-in because registering one publishes an HTTP endpoint whose
	// parameters the caller supplies.
	reloadable bool
	// head holds the nodes contributed by head elements declared outside the
	// document shell, already scoped and ready to merge.
	head []Node
	// headTags holds the same contributions as ready to write HTML, one entry
	// per tag, with extracted assets replaced by their reference tags. It is
	// filled by extractAssets.
	headTags []headTag
	// assets holds the extracted files this component's own contributions
	// reference, in the order they were declared. It is filled by extractAssets
	// beside headTags, because the two are the same declarations read as markup
	// and as identity.
	assets []Asset
	// style is the scoped CSS of this component's style block, which
	// requirement:static-asset-extraction moves into a generated stylesheet.
	style string
	// script is the authored body of this component's own script block, the
	// requirement:component-script-block declaration extracted into its own
	// file and bound to this component's instances. It is empty for a component
	// declaring none, which is every component that predates the feature.
	script string
	// scriptAttributes are the block's authored attributes, minus the marker
	// that identified it. They travel to the emitted reference tag the way a
	// head script's do.
	scriptAttributes []Attribute
	// scriptPos locates the block, for a diagnostic a transform returns.
	scriptPos Position
	// scope carries the requirement:scoped-component-style renaming applied to
	// this component's style block, or nil when it declares none.
	scope *styleScope
	// shell marks a component that owns the document head, which is where
	// merged contributions are injected.
	shell bool
	// cache holds the validated cache annotation, or nil when the component is
	// rendered on every invocation.
	cache *cachePolicy
	// await marks a component that owns at least one await boundary. A cached
	// component may not reach one, because a boundary is emitted in two pieces
	// and cannot be stored as one byte range.
	await bool
	// live marks a component that owns at least one live boundary. Every live
	// boundary is also an await boundary, so this is a subset of await: it
	// exists so a caller can tell a screen that keeps changing from one that is
	// merely progressive.
	live bool
	// perRequest marks a component writing a builtin element whose output comes
	// from a provider, and perRequestElement names the first one for the
	// diagnostic. A cached component may not reach one, because storing a
	// per-request value and serving it to the next visitor is a security failure
	// rather than a staleness bug.
	perRequest        bool
	perRequestElement string
	// builtinAssets and vary are what the builtin elements this component writes
	// require and depend on. They roll up over the call graph exactly as head
	// contributions do.
	builtinAssets []Asset
	vary          []string
	// builtins names the builtin elements this component writes, in source
	// order, so emission can resolve each occurrence without re-walking.
	builtins []string
}

type compiler struct {
	filename    string
	source      string
	module      *Module
	records     map[string]*TypeDecl
	enums       map[string]*EnumDecl
	enumMembers map[string]valueType
	externals   map[string]functionSig
	components  map[string]*componentInfo
	exprTypes   map[Expr]valueType
	// elements is the resolved hyphenated-element whitelist, frozen before
	// analysis.
	elements *elementSet
	// csrfMode, csrfField, and attrPrefix decide whether an unsafe form carries
	// a token, what the field is called, and what the opt-out attribute is named.
	csrfMode   CSRFMode
	csrfField  string
	attrPrefix string
	// contentReads collects the files the content transforms reported reading,
	// so an edit to one regenerates the block that depends on it.
	contentReads []string
	// foreignDepth is non-zero inside an SVG or MathML subtree, where a
	// hyphenated name is a standard foreign-namespace element rather than a
	// custom element and so sits outside the whitelist entirely.
	foreignDepth int
	// liveBoundaries marks the await boundaries that bind at least one live
	// source, and therefore re-render per delivery instead of settling once.
	// It is derived from the bindings rather than written at the wait site,
	// because how often a value arrives is what its declaration says.
	liveBoundaries map[*syntax.AwaitNode]bool

	// collapseWhitespace enables requirement:static-whitespace-normalization.
	// It is on unless the run asked for byte-identical output.
	collapseWhitespace bool

	// current tracks the component being analyzed so slot elements can bind to
	// its parameters.
	current *componentInfo
	// slotUsed records which slot parameters are already rendered on the
	// current execution path. Mutually exclusive if branches get independent
	// copies, so the same slot may appear in both.
	slotUsed map[string]bool
	// loopDepth is non-zero inside a for body, where a slot cannot guarantee
	// at most one rendering.
	loopDepth int
	// awaitDepth is non-zero inside any await clause, where a slot would be
	// rendered by both the fallback and the replacement.
	awaitDepth int
	// awaitCall is the one call expression currently allowed to name an async
	// external, so a nested async call inside an await header is still rejected.
	awaitCall Expr
	// awaitSource is the one expression currently allowed to evaluate to an
	// async type. It is the binding source of the await clause being analyzed,
	// so an async value read anywhere else is a local error with a position.
	awaitSource Expr
	// actions collects the server-action references the module makes, in source
	// order, for the resolution pass described on ServerActionAttr.
	actions []ActionRef
}

type CompileError struct {
	Filename string
	Pos      Position
	Message  string
}

func (e *CompileError) Error() string {
	name := e.Filename
	if name == "" {
		name = "<template>"
	}
	return fmt.Sprintf("%s:%d:%d: %s", name, e.Pos.Line, e.Pos.Col, e.Message)
}

func newCompiler(filename, source string, module *Module, collapseWhitespace bool) *compiler {
	return &compiler{
		filename: filename, source: source, module: module,
		records: map[string]*TypeDecl{}, enums: map[string]*EnumDecl{},
		enumMembers: map[string]valueType{}, externals: map[string]functionSig{},
		components: map[string]*componentInfo{}, exprTypes: map[Expr]valueType{},
		liveBoundaries:     map[*syntax.AwaitNode]bool{},
		collapseWhitespace: collapseWhitespace,
	}
}

func (c *compiler) analyze() error {
	for _, declaration := range c.module.Declarations {
		switch declaration := declaration.(type) {
		case *TypeDecl:
			if c.nameExists(declaration.Name) {
				return c.error(declaration.Pos, "duplicate declaration "+declaration.Name)
			}
			c.records[declaration.Name] = declaration
		case *EnumDecl:
			if c.nameExists(declaration.Name) {
				return c.error(declaration.Pos, "duplicate declaration "+declaration.Name)
			}
			c.enums[declaration.Name] = declaration
			for _, member := range declaration.Members {
				if _, exists := c.enumMembers[member.Name]; exists {
					return c.error(member.Pos, "duplicate enum member "+member.Name)
				}
				c.enumMembers[member.Name] = valueType{kind: kindEnum, name: declaration.Name}
			}
		}
	}
	for _, declaration := range c.module.Declarations {
		switch declaration := declaration.(type) {
		case *TypeDecl:
			seen := map[string]bool{}
			for _, field := range declaration.Fields {
				if seen[field.Name] {
					return c.error(field.Pos, "duplicate field "+field.Name)
				}
				seen[field.Name] = true
				if _, err := c.resolveType(field.Type); err != nil {
					return err
				}
			}
		case *ExternalDecl:
			if c.nameExists(declaration.Name) {
				return c.error(declaration.Pos, "duplicate declaration "+declaration.Name)
			}
			var sig functionSig
			for _, parameter := range declaration.Parameters {
				t, err := c.resolveType(parameter.Type)
				if err != nil {
					return err
				}
				// An external is an ordinary blocking Go function, so a pending
				// handle has no meaning in its signature; asynchrony there is
				// the `external async` modifier instead.
				if t.async {
					return c.error(parameter.Pos, "external parameter "+parameter.Name+" cannot be async; declare the function external async instead")
				}
				sig.params = append(sig.params, t)
			}
			result, err := c.resolveType(declaration.Result)
			if err != nil {
				return err
			}
			if result.async {
				return c.error(declaration.Pos, "external "+declaration.Name+" cannot return an async type; declare the function external async instead")
			}
			sig.result = result
			sig.async = declaration.Async
			sig.live = declaration.Live
			c.externals[declaration.Name] = sig
		case *TemplateDecl:
			if declaration.Kind != "html:component" || declaration.Output.Name != "html" {
				return c.error(declaration.Pos, "HTML generator only accepts html:component declarations")
			}
			if declaration.Output.Async {
				return c.error(declaration.Pos, "component output cannot be async; a component waits inside an await clause instead")
			}
			if c.nameExists(declaration.Name) {
				return c.error(declaration.Pos, "duplicate declaration "+declaration.Name)
			}
			info := &componentInfo{decl: declaration, params: map[string]valueType{}, order: declaration.Parameters}
			for _, parameter := range declaration.Parameters {
				if _, exists := info.params[parameter.Name]; exists {
					return c.error(parameter.Pos, "duplicate parameter "+parameter.Name)
				}
				t, err := c.resolveType(parameter.Type)
				if err != nil {
					return err
				}
				// A slot parameter is a bound continuation rather than a value,
				// so there is nothing for a caller to have started.
				if t.async && t.kind == kindHTML {
					return c.error(parameter.Pos, "html parameter "+parameter.Name+" cannot be async")
				}
				info.params[parameter.Name] = t
			}
			if err := c.applyAnnotations(info, declaration); err != nil {
				return err
			}
			c.components[declaration.Name] = info
		}
	}
	for _, declaration := range c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		info := c.components[component.Name]
		scope := copyScope(info.params)
		body, ok := component.Body.([]syntax.Node)
		if !ok {
			return c.error(component.Pos, "invalid HTML component body")
		}
		// Whitespace is normalized before the head is collected, so the
		// contributions collectHead captures are the rewritten nodes.
		body, err := normalizeWhitespace(c.filename, body, c.collapseWhitespace)
		if err != nil {
			return err
		}
		// The component's own script block is taken out before anything else
		// looks at the body, because it is a declaration rather than markup and
		// every later pass would otherwise treat it as content to render.
		body, err = c.collectScriptBlock(info, body)
		if err != nil {
			return err
		}
		component.Body = body
		c.current = info
		c.slotUsed = map[string]bool{}
		c.loopDepth = 0
		// Head contributions are collected before the body is analyzed so the
		// style scope is known when class attributes are checked.
		if err := c.collectHead(info, body); err != nil {
			return err
		}
		if err := c.analyzeNodes(body, scope); err != nil {
			return err
		}
		c.current = nil
	}
	return c.validateCachedComponents()
}

// applyAnnotations validates the declaration's annotations. An unknown name is
// an error rather than a no-op, because an ignored annotation reads as enabled
// behavior at the call site.
func (c *compiler) applyAnnotations(info *componentInfo, declaration *TemplateDecl) error {
	for _, annotation := range declaration.Annotations {
		switch annotation.Name {
		case "cache":
			policy, err := c.cacheAnnotation(declaration, annotation)
			if err != nil {
				return err
			}
			info.cache = policy
		case "reloadable":
			if len(annotation.Args) > 0 {
				return c.error(annotation.Args[0].Pos, "@reloadable takes no arguments")
			}
			info.reloadable = true
		default:
			return c.error(annotation.Pos, "unknown annotation @"+annotation.Name)
		}
	}
	return nil
}

// cacheAnnotation reads @cache(ttl: "5m", scope: "public"). The ttl is parsed
// here so a malformed duration is reported with its own position instead of
// failing at run time, and its presence is what decides whether the annotation
// stores anything at all.
func (c *compiler) cacheAnnotation(declaration *TemplateDecl, annotation Annotation) (*cachePolicy, error) {
	for _, argument := range annotation.Args {
		if argument.Name != "ttl" && argument.Name != "scope" {
			return nil, c.error(argument.Pos, "unknown @cache argument "+argument.Name)
		}
	}
	policy := &cachePolicy{pos: annotation.Pos}
	scope, hasScope := annotation.Argument("scope")
	if hasScope {
		switch scope.Value {
		case "private":
		case "public":
			policy.public = true
		default:
			return nil, c.error(scope.Pos, "@cache scope is not private or public: "+scope.Value)
		}
		policy.pos = scope.Pos
	}
	argument, hasTTL := annotation.Argument("ttl")
	if !hasTTL {
		// Without a ttl the annotation stores nothing, so it has to be declaring
		// a scope or it says nothing at all.
		if !hasScope {
			return nil, c.error(annotation.Pos, "@cache needs a ttl to store output or a scope to declare one, for example @cache(ttl: \"5m\") or @cache(scope: \"private\")")
		}
		// A declaration is not a cache, so none of the eligibility rules below
		// apply to it: each exists because bytes are stored.
		return policy, nil
	}
	// A component that cannot store cannot be given a duration, because the
	// duration would describe an expiry that never happens. A slot owner is the
	// case that matters: a layout carries the annotation to declare scope over
	// the chain beneath it, and nothing about that stores.
	if slot := c.htmlParameter(declaration); slot != nil {
		return nil, c.error(argument.Pos, "component "+declaration.Name+" cannot declare a @cache ttl, because the html parameter "+
			slot.Name+" makes it a slot owner and a slot owner stores nothing; drop the ttl to declare scope alone")
	}
	ttl, err := time.ParseDuration(argument.Value)
	if err != nil {
		return nil, c.error(argument.Pos, "@cache ttl is not a duration: "+argument.Value)
	}
	if ttl <= 0 {
		return nil, c.error(argument.Pos, "@cache ttl must be positive")
	}
	policy.ttl = ttl
	for _, parameter := range declaration.Parameters {
		t, err := c.resolveType(parameter.Type)
		if err != nil {
			return nil, err
		}
		// Stored bytes stand in for a fresh render, and a pending value belongs
		// to the one request that started it, so it can be neither part of the
		// key nor part of what the key stands for.
		if c.containsAsync(t, map[string]bool{}) {
			return nil, c.error(parameter.Pos, "cached component "+declaration.Name+" cannot declare the async parameter "+parameter.Name+"; a pending value belongs to one request")
		}
	}
	return policy, nil
}

// htmlParameter returns the component's first html parameter, which is what
// makes it a slot owner, or nil when it declares none.
func (c *compiler) htmlParameter(declaration *TemplateDecl) *Parameter {
	for i, parameter := range declaration.Parameters {
		t, err := c.resolveType(parameter.Type)
		if err != nil {
			continue
		}
		if t.kind == kindHTML {
			return &declaration.Parameters[i]
		}
	}
	return nil
}

// validateCachedComponents rejects the cases where stored bytes could not stand
// in for a fresh render.
func (c *compiler) validateCachedComponents() error {
	for _, declaration := range c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		info := c.components[component.Name]
		if info == nil || info.cache == nil {
			continue
		}
		// A public assertion is about the subtree beneath it, so a declared
		// private component inside that subtree contradicts it. An undeclared one
		// does not: it inherits the assertion, and refusing it would leave nothing
		// publishable. The refusal is a generation error rather than a silent
		// downgrade because the source says public and a reviewer reads the source.
		if info.cache.public {
			if owner := c.reachesDeclaredPrivate(component.Name, map[string]bool{}); owner != "" {
				where := "it"
				if owner != component.Name {
					where = owner
				}
				return c.error(info.cache.pos, "component "+component.Name+" cannot declare @cache scope public; "+
					where+" declares private, and a public assertion covers everything it renders")
			}
		}
		// Everything below is about output that is stored. An annotation with no
		// ttl stores nothing, so none of it has a premise.
		if !info.cache.stores() {
			continue
		}
		if info.shell {
			return c.error(component.Pos, "cached component "+component.Name+" cannot own the document head, because the merged head depends on the chain rather than on its parameters")
		}
		if owner := c.reachesAwait(component.Name, map[string]bool{}); owner != "" {
			where := "it"
			if owner != component.Name {
				where = owner
			}
			return c.error(component.Pos, "cached component "+component.Name+" cannot reach an await boundary; "+where+" declares one")
		}
		// A stored body outlives the request that produced it, so a per-request
		// value inside one is served to whoever asks next. For a CSRF token that
		// is not a staleness bug but a security failure, which is why the
		// exclusion is a generation error rather than a note.
		if owner, element := c.reachesPerRequest(component.Name, map[string]bool{}); owner != "" {
			where := "it"
			if owner != component.Name {
				where = owner
			}
			return c.error(component.Pos, "cached component "+component.Name+" cannot reach the per-request <"+element+
				">; "+where+" writes one, and a stored body would serve one request's value to the next")
		}
	}
	return nil
}

// reachesDeclaredPrivate returns the first component in the call graph that
// declared its scope private, and empty when none does.
//
// It walks the same graph as reachesAwait and reads a different bit, which is
// what makes the public assertion affordable to check. Only an explicit
// declaration counts: an undeclared component inherits whatever is asserted
// around it, so treating it as private would make nothing publishable.
func (c *compiler) reachesDeclaredPrivate(name string, seen map[string]bool) string {
	if seen[name] {
		return ""
	}
	seen[name] = true
	info, ok := c.components[name]
	if !ok {
		return ""
	}
	if info.cache != nil && !info.cache.public {
		return name
	}
	for _, called := range c.calledComponents(info) {
		if owner := c.reachesDeclaredPrivate(called, seen); owner != "" {
			return owner
		}
	}
	return ""
}

// declaresPrivate reports whether this component or anything it calls declared
// private, and names the component that did. It is the emitted form of
// reachesDeclaredPrivate, and it folds upward for the reason head contributions
// do: a private component's bytes end up inside whatever renders it.
func (c *compiler) declaresPrivate(name string) string {
	return c.reachesDeclaredPrivate(name, map[string]bool{})
}

// reachesPerRequest returns the first component in the call graph writing a
// builtin element backed by a provider, and the element it writes.
func (c *compiler) reachesPerRequest(name string, seen map[string]bool) (string, string) {
	if seen[name] {
		return "", ""
	}
	seen[name] = true
	info, ok := c.components[name]
	if !ok {
		return "", ""
	}
	if info.perRequest {
		return name, info.perRequestElement
	}
	for _, called := range c.calledComponents(info) {
		if owner, element := c.reachesPerRequest(called, seen); owner != "" {
			return owner, element
		}
	}
	return "", ""
}

// transitiveVary collects the request properties a component's output depends
// on, over the same call graph as transitiveHead: a nested call reading a cookie
// makes the whole response depend on it, and only the value a caller holds can
// say so.
func (c *compiler) transitiveVary(name string) []string {
	var out []string
	visited, emitted := map[string]bool{}, map[string]bool{}
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
		for _, axis := range info.vary {
			if axis == "" || emitted[axis] {
				continue
			}
			emitted[axis] = true
			out = append(out, axis)
		}
		for _, called := range c.calledComponents(info) {
			visit(called)
		}
	}
	visit(name)
	return out
}

// reachesLive returns the name of the first component in the call graph that
// owns a live boundary. It walks the same graph as reachesAwait and answers the
// other half of the question a caller asks before rendering: not whether this
// response needs the boundary runtime, but whether the screen will keep
// changing once the document has finished.
func (c *compiler) reachesLive(name string, seen map[string]bool) string {
	if seen[name] {
		return ""
	}
	seen[name] = true
	info, ok := c.components[name]
	if !ok {
		return ""
	}
	if info.live {
		return name
	}
	for _, called := range c.calledComponents(info) {
		if owner := c.reachesLive(called, seen); owner != "" {
			return owner
		}
	}
	return ""
}

// reachesAwait returns the name of the first component in the call graph that
// owns an await boundary.
func (c *compiler) reachesAwait(name string, seen map[string]bool) string {
	if seen[name] {
		return ""
	}
	seen[name] = true
	info, ok := c.components[name]
	if !ok {
		return ""
	}
	if info.await {
		return name
	}
	for _, called := range c.calledComponents(info) {
		if owner := c.reachesAwait(called, seen); owner != "" {
			return owner
		}
	}
	return ""
}

func (c *compiler) nameExists(name string) bool {
	_, record := c.records[name]
	_, enum := c.enums[name]
	_, external := c.externals[name]
	_, component := c.components[name]
	return record || enum || external || component
}

func (c *compiler) resolveType(ref TypeRef) (valueType, error) {
	var result valueType
	switch ref.Name {
	case "string":
		result.kind = kindString
	case "bool":
		result.kind = kindBool
	case "int":
		result.kind = kindInt
	case "float":
		result.kind = kindFloat
	case "decimal":
		result.kind = kindDecimal
	case "datetime":
		result.kind = kindDateTime
	case "date":
		result.kind = kindDate
	case "time":
		result.kind = kindTime
	case "url":
		result.kind = kindURL
	case "bytes":
		result.kind = kindBytes
	case "html":
		result.kind = kindHTML
	case "trusted_html":
		result.kind = kindTrustedHTML
	case "trusted_css":
		result.kind = kindTrustedCSS
	case "trusted_javascript":
		result.kind = kindTrustedJS
	case "script_json":
		result.kind = kindScriptJSON
	case "error":
		result.kind = kindError
	default:
		if _, ok := c.records[ref.Name]; ok {
			result = valueType{kind: kindRecord, name: ref.Name}
		} else if _, ok := c.enums[ref.Name]; ok {
			result = valueType{kind: kindEnum, name: ref.Name}
		} else {
			return valueType{}, c.error(ref.Pos, "unknown type "+ref.Name)
		}
	}
	if ref.Array {
		elem := result
		result = valueType{kind: kindArray, elem: &elem}
	}
	result.optional = ref.Optional
	// The modifier is applied last because it wraps the whole type expression:
	// `async User?` is a pending optional, not an optional pending.
	result.async = ref.Async
	return result, nil
}

// containsAsync reports whether t is async or reaches an async field. It is
// what keeps a pending value out of the places that must be able to stand in
// for a value they already have, such as a cache key.
func (c *compiler) containsAsync(t valueType, visiting map[string]bool) bool {
	if t.async {
		return true
	}
	switch t.kind {
	case kindArray:
		return t.elem != nil && c.containsAsync(*t.elem, visiting)
	case kindRecord:
		record, ok := c.records[t.name]
		if !ok || visiting[t.name] {
			return false
		}
		visiting[t.name] = true
		defer delete(visiting, t.name)
		for _, field := range record.Fields {
			fieldType, err := c.resolveType(field.Type)
			if err == nil && c.containsAsync(fieldType, visiting) {
				return true
			}
		}
	}
	return false
}

func (c *compiler) analyzeNodes(nodes []syntax.Node, scope map[string]valueType) error {
	for _, node := range nodes {
		switch node := node.(type) {
		case *TextNode, *CommentNode, *DoctypeNode:
		case *syntax.ExpressionNode:
			t, err := c.infer(node.Expression, scope)
			if err != nil {
				return annotateRawTextInsertion(node.Context, err)
			}
			if err := c.validateInsertion(node.Context, t, exprPos(node.Expression)); err != nil {
				return annotateRawTextInsertion(node.Context, err)
			}
			if err := c.markHTMLParameterUse(node.Expression, t); err != nil {
				return err
			}
		case *syntax.IfNode:
			t, err := c.infer(node.Condition, scope)
			if err != nil {
				return err
			}
			if t.kind != kindBool || t.optional {
				return c.error(exprPos(node.Condition), "if condition must be bool")
			}
			// Branches are mutually exclusive, so each starts from the state
			// before the if and the result is their union.
			before := copyUsage(c.slotUsed)
			if err := c.analyzeNodes(node.Then, copyScope(scope)); err != nil {
				return err
			}
			thenUsed := c.slotUsed
			c.slotUsed = copyUsage(before)
			if err := c.analyzeNodes(node.Else, copyScope(scope)); err != nil {
				return err
			}
			for name := range thenUsed {
				c.slotUsed[name] = true
			}
		case *syntax.ForNode:
			t, err := c.infer(node.Iterable, scope)
			if err != nil {
				return err
			}
			if t.kind != kindArray || t.optional || t.elem == nil {
				return c.error(exprPos(node.Iterable), "for expression must be an array")
			}
			inner := copyScope(scope)
			inner[node.Variable] = *t.elem
			if node.Index != "" {
				inner[node.Index] = valueType{kind: kindInt}
			}
			c.loopDepth++
			if err := c.analyzeNodes(node.Body, inner); err != nil {
				return err
			}
			c.loopDepth--
		case *ElementNode:
			unsafe, err := c.unsafeForm(node)
			if err != nil {
				return err
			}
			if unsafe && c.current != nil {
				// The token is a per-request value, so a component reaching one
				// cannot be a cached body: a stored form would serve one
				// session's token to whoever asked next.
				c.current.perRequest = true
				c.current.perRequestElement = "form"
			}
			builtin, handled, err := c.resolveElement(node)
			if err != nil {
				return err
			}
			if builtin != nil {
				if err := c.analyzeBuiltinElement(node, builtin, scope); err != nil {
					return err
				}
				continue
			}
			_ = handled
			for _, attribute := range node.Attributes {
				if attribute.Name == ServerActionAttr {
					if err := c.analyzeServerAction(node.Name, attribute, node.Attributes); err != nil {
						return err
					}
					continue
				}
				if err := c.analyzeAttribute(node.Name, attribute, scope); err != nil {
					return err
				}
			}
			// A hyphenated name inside SVG or MathML is a standard
			// foreign-namespace element, not a custom element, so the whitelist
			// does not reach into one.
			if foreign := foreignRoot(node.Name); foreign {
				c.foreignDepth++
			}
			if err := c.analyzeNodes(node.Children, scope); err != nil {
				return err
			}
			if foreignRoot(node.Name) {
				c.foreignDepth--
			}
		case *HeadNode:
			// Contributions are validated and scoped by collectHead before the
			// body is analyzed, and they carry no expressions to type-check.
		case *SlotNode:
			if err := c.analyzeSlot(node, scope); err != nil {
				return err
			}
		case *syntax.AwaitNode:
			if err := c.analyzeAwait(node, scope); err != nil {
				return err
			}
		case *ComponentNode:
			if err := c.analyzeComponentCall(node, scope); err != nil {
				return err
			}
		default:
			return c.error(Position{Line: 1, Col: 1}, fmt.Sprintf("unsupported HTML node %T", node))
		}
	}
	return nil
}

// collectScriptBlock takes the component's own script block out of the body and
// records it, returning the body the component actually renders.
//
// The block is removed rather than skipped at emission because it is not markup
// at all: requirement:static-asset-extraction writes it to a file and the merged
// head references it, exactly as it already does for a head script.
//
// It must be a direct child of the component. A block inside a control flow
// block would be a component-wide declaration written as though it were
// conditional, and nothing about the file it produces could honor that.
func (c *compiler) collectScriptBlock(info *componentInfo, body []Node) ([]Node, error) {
	var block *ElementNode
	kept := make([]Node, 0, len(body))
	for _, node := range body {
		element, ok := node.(*ElementNode)
		if !ok || !isComponentScriptBlock(element.Name, element.Attributes) {
			kept = append(kept, node)
			continue
		}
		if block != nil {
			return nil, c.error(element.Pos, "component "+info.decl.Name+" declares more than one script block")
		}
		block = element
	}
	if err := c.rejectNestedScriptBlocks(info, kept); err != nil {
		return nil, err
	}
	if block == nil {
		return body, nil
	}
	info.script = strings.TrimSpace(elementText(block))
	info.scriptPos = block.Pos
	if specifier := relativeImportSpecifier(info.script); specifier != "" {
		return nil, c.error(block.Pos, "component "+info.decl.Name+" imports "+specifier+
			" from its script block; a relative specifier resolves against the generated file's URL under the public asset directory rather than against this template, so write the served path instead")
	}
	for _, attr := range block.Attributes {
		if attr.Name == componentScriptMarker && attr.Boolean {
			continue
		}
		// A lifecycle method is an export, and a classic script has no export
		// surface to reach one through. Its only per-visit behavior is
		// re-execution, which re-runs customElements.define and re-adds every
		// listener, so this is the bug the block exists to remove rather than a
		// weaker version of it.
		if attr.Name == "global" && attr.Boolean {
			return nil, c.error(attr.Pos, "component "+info.decl.Name+" declares a global script block; a component script block is a module, because the lifecycle it exports cannot be reached from a classic script")
		}
		if attr.Name == "module" && attr.Boolean {
			continue
		}
		info.scriptAttributes = append(info.scriptAttributes, attr)
	}
	return kept, nil
}

// relativeImportSpecifier returns the first relative module specifier a script
// block imports, or empty when it imports none.
//
// An extracted block is served from the public asset directory under a
// content-hashed name, so `./util.js` resolves next to that generated file
// rather than next to the template the author is looking at. The result is a
// 404 for a file that exists, which is worth a diagnostic even though this
// module otherwise reads none of the JavaScript it extracts.
//
// Only a specifier on a statement beginning its own line is examined. A real
// import is a top-level statement, and the restriction keeps the same words
// inside a comment or a string from being mistaken for one.
func relativeImportSpecifier(script string) string {
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "import") && !strings.HasPrefix(line, "export") {
			continue
		}
		for _, quote := range []string{`"`, "'"} {
			start := strings.Index(line, quote+".")
			if start < 0 {
				continue
			}
			rest := line[start+1:]
			end := strings.Index(rest, quote)
			if end < 0 {
				continue
			}
			specifier := rest[:end]
			if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") {
				return specifier
			}
		}
	}
	return ""
}

// rejectNestedScriptBlocks reports a marked script anywhere but the component's
// own top level, where collectScriptBlock has already taken the legal one.
func (c *compiler) rejectNestedScriptBlocks(info *componentInfo, nodes []Node) error {
	for _, node := range nodes {
		var children [][]Node
		switch node := node.(type) {
		case *ElementNode:
			if isComponentScriptBlock(node.Name, node.Attributes) {
				return c.error(node.Pos, "component "+info.decl.Name+" declares a script block inside markup; declare it at the top of the component, beside its head block")
			}
			children = [][]Node{node.Children}
		case *HeadNode:
			children = [][]Node{node.Children}
		case *SlotNode:
			children = [][]Node{node.Default}
		case *ComponentNode:
			children = [][]Node{node.Children}
		case *syntax.IfNode:
			children = [][]Node{node.Then, node.Else}
		case *syntax.ForNode:
			children = [][]Node{node.Body}
		case *syntax.AwaitNode:
			children = [][]Node{node.Primary, node.Fallback, node.Recover}
		default:
			continue
		}
		for _, branch := range children {
			if err := c.rejectNestedScriptBlocks(info, branch); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectHead gathers the component's head contributions, scopes its style
// block, and records whether the component owns the document shell.
func (c *compiler) collectHead(info *componentInfo, body []Node) error {
	var heads []*HeadNode
	var walk func(nodes []Node)
	walk = func(nodes []Node) {
		for _, node := range nodes {
			switch node := node.(type) {
			case *HeadNode:
				heads = append(heads, node)
			case *ElementNode:
				if node.Name == "head" {
					info.shell = true
				}
				walk(node.Children)
			case *SlotNode:
				walk(node.Default)
			case *syntax.IfNode:
				walk(node.Then)
				walk(node.Else)
			case *syntax.ForNode:
				walk(node.Body)
			case *syntax.AwaitNode:
				walk(node.Primary)
				walk(node.Fallback)
				walk(node.Recover)
			case *ComponentNode:
				walk(node.Children)
			}
		}
	}
	walk(body)
	if len(heads) == 0 {
		return nil
	}
	if len(heads) > 1 {
		return c.error(heads[1].Pos, "component "+info.decl.Name+" declares more than one head element")
	}
	head := heads[0]
	for _, node := range head.Children {
		if err := c.validateHeadChild(node); err != nil {
			return err
		}
	}
	if err := c.scopeHeadStyles(info, head); err != nil {
		return err
	}
	info.head = head.Children
	return nil
}

// noscriptHeadChildren are the elements HTML allows inside a head noscript.
// Anything else there is body content, which a head contribution never carries.
var noscriptHeadChildren = map[string]bool{"link": true, "meta": true, "style": true}

// validateHeadChild keeps head contributions static, because the merged head
// is written before any body byte and cannot wait for request data.
func (c *compiler) validateHeadChild(node Node) error {
	switch node := node.(type) {
	case *TextNode, *CommentNode:
		return nil
	case *ElementNode:
		switch node.Name {
		case "link", "meta", "style", "script", "title":
		case "noscript":
			// The one contributed element with element children. It is what a
			// page tells a browser with scripting disabled, and HTML permits it
			// in the head around a link, a style, or a meta.
			if err := c.validateHeadAttributes(node); err != nil {
				return err
			}
			for _, child := range node.Children {
				switch child := child.(type) {
				case *TextNode, *CommentNode:
					continue
				case *ElementNode:
					if !noscriptHeadChildren[child.Name] {
						return c.error(child.Pos, "head noscript cannot contain "+child.Name)
					}
					if err := c.validateHeadChild(child); err != nil {
						return err
					}
				default:
					return c.error(node.Pos, "head noscript accepts static markup only")
				}
			}
			return nil
		default:
			return c.error(node.Pos, "head contribution cannot contain "+node.Name)
		}
		if err := c.validateHeadAttributes(node); err != nil {
			return err
		}
		for _, child := range node.Children {
			if _, ok := child.(*TextNode); !ok {
				return c.error(node.Pos, "head contribution "+node.Name+" accepts static text only")
			}
		}
		return nil
	default:
		return c.error(Position{Line: 1, Col: 1}, "head contribution must be static markup")
	}
}

func (c *compiler) validateHeadAttributes(node *ElementNode) error {
	for _, attribute := range node.Attributes {
		if _, static := staticAttributeText(attribute); !static && !attribute.Boolean {
			return c.error(attribute.Pos, "head contribution attributes must be static")
		}
	}
	return nil
}

// scopeHeadStyles rewrites the component's style block so its class and
// keyframes names cannot collide with another component's.
func (c *compiler) scopeHeadStyles(info *componentInfo, head *HeadNode) error {
	for _, node := range head.Children {
		element, ok := node.(*ElementNode)
		if !ok || element.Name != "style" {
			continue
		}
		if info.scope != nil {
			return c.error(element.Pos, "component "+info.decl.Name+" declares more than one style block")
		}
		var source strings.Builder
		for _, child := range element.Children {
			source.WriteString(child.(*TextNode).Text)
		}
		rewritten, scope, err := rewriteCSS(source.String(), scopeSuffix(c.filename, info.decl.Name))
		if err != nil {
			return c.error(element.Pos, err.Error())
		}
		element.Children = []Node{&TextNode{Kind: "html:text", Pos: element.Pos, Text: rewritten}}
		info.scope = scope
		info.style = rewritten
	}
	return nil
}

// scopeSuffix derives a stable per-component identifier. It depends only on the
// template path and component name, so unrelated edits keep generated class
// names unchanged.
func scopeSuffix(filename, component string) string {
	const offset64 = 14695981039346656037
	const prime64 = 1099511628211
	hash := uint64(offset64)
	for _, b := range []byte(filename + "\x00" + component) {
		hash ^= uint64(b)
		hash *= prime64
	}
	const digits = "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, 6)
	for i := range out {
		out[i] = digits[hash%uint64(len(digits))]
		hash /= uint64(len(digits))
	}
	return string(out)
}

// awaitedType infers a binding source that is not a call, with the async read
// rule lifted for that one expression. Everything inside it is still checked
// normally, so awaiting a field of a pending record stays an error naming the
// value that has to be awaited first.
func (c *compiler) awaitedType(expr Expr, scope map[string]valueType) (valueType, error) {
	outer := c.awaitSource
	c.awaitSource = expr
	defer func() { c.awaitSource = outer }()
	return c.infer(expr, scope)
}

// analyzeAwait types one boundary. The bindings are visible only in the primary
// subtree, and the error name only in the recover subtree, so no clause can
// read a value that does not exist when it renders.
func (c *compiler) analyzeAwait(node *syntax.AwaitNode, scope map[string]valueType) error {
	if c.current != nil {
		c.current.await = true
	}
	primaryScope := copyScope(scope)
	for _, binding := range node.Bindings {
		if binding.Name == "outer" {
			return c.error(binding.Pos, "await binding cannot be named outer; the generated scope reserves that name")
		}
		call, isCall := binding.Call.(*CallExpr)
		if !isCall {
			// The other source is a value the caller already started, read
			// through a parameter or a record field. It settles beside any
			// call in the same clause.
			t, err := c.awaitedType(binding.Call, scope)
			if err != nil {
				return err
			}
			if !t.async {
				return c.error(binding.Pos, "await binding "+binding.Name+" reads "+t.String()+"; only an async value or an async external call can be awaited")
			}
			primaryScope[binding.Name] = t.awaited()
			continue
		}
		identifier, ok := call.Callee.(*IdentifierExpr)
		if !ok {
			return c.error(binding.Pos, "await binding "+binding.Name+" must call an async external function")
		}
		sig, ok := c.externals[identifier.Name]
		if !ok {
			return c.error(binding.Pos, "unknown function "+identifier.Name)
		}
		if !sig.async && !sig.live {
			return c.error(binding.Pos, identifier.Name+" is not async; declare it as external async or external live to await it")
		}
		if sig.live {
			// The boundary re-renders per delivery rather than settling once.
			// Nothing about the clause changes: whether a value arrives once or
			// many times is what its declaration says, not what the wait site
			// asks for.
			c.liveBoundaries[node] = true
			if c.current != nil {
				c.current.live = true
			}
		}
		outerCall := c.awaitCall
		c.awaitCall = call
		t, err := c.infer(call, scope)
		c.awaitCall = outerCall
		if err != nil {
			return err
		}
		primaryScope[binding.Name] = t
	}
	if c.liveBoundaries[node] {
		if err := c.rejectStatefulControls(node.Primary); err != nil {
			return err
		}
	}
	c.awaitDepth++
	defer func() { c.awaitDepth-- }()
	// Every clause starts from the slot usage before the boundary, because at
	// most one of them ends up in the document.
	before := copyUsage(c.slotUsed)
	if err := c.analyzeNodes(node.Primary, primaryScope); err != nil {
		return err
	}
	c.slotUsed = copyUsage(before)
	if err := c.analyzeNodes(node.Fallback, copyScope(scope)); err != nil {
		return err
	}
	c.slotUsed = copyUsage(before)
	if node.HasRecover {
		recoverScope := copyScope(scope)
		if node.ErrorName != "" {
			recoverScope[node.ErrorName] = valueType{kind: kindError}
		}
		if err := c.analyzeNodes(node.Recover, recoverScope); err != nil {
			return err
		}
		c.slotUsed = copyUsage(before)
	}
	return nil
}

// analyzeSlot binds a slot element to the html parameter it names and enforces
// the rules that keep slot rendering statically predictable.
func (c *compiler) analyzeSlot(node *SlotNode, scope map[string]valueType) error {
	if c.current == nil {
		return c.error(node.Pos, "slot is only allowed inside a component body")
	}
	name := node.Parameter()
	t, ok := c.current.params[name]
	if !ok {
		return c.error(node.Pos, "slot "+name+" has no matching parameter; declare "+name+": html")
	}
	if t.kind != kindHTML {
		return c.error(node.Pos, "slot "+name+" must bind an html parameter, got "+t.String())
	}
	if node.Required && t.optional {
		return c.error(node.Pos, "required slot "+name+" must be declared html, not html?")
	}
	if !node.Required && !t.optional {
		return c.error(node.Pos, "slot "+name+" binds a required parameter; add the required attribute")
	}
	if node.Required && len(node.Default) > 0 {
		return c.error(node.Pos, "required slot "+name+" cannot declare default content")
	}
	if c.loopDepth > 0 {
		return c.error(node.Pos, "slot "+name+" cannot appear inside a for body")
	}
	if c.awaitDepth > 0 {
		return c.error(node.Pos, "slot "+name+" cannot appear inside an await block; the fallback and the replacement would both render it")
	}
	if c.slotUsed[name] {
		return c.error(node.Pos, "slot "+name+" is rendered more than once on the same path")
	}
	c.slotUsed[name] = true
	return c.analyzeNodes(node.Default, copyScope(scope))
}

// markHTMLParameterUse records a bare html parameter reference so mixing
// {children} and a slot element for the same parameter is still caught.
func (c *compiler) markHTMLParameterUse(expr Expr, t valueType) error {
	if c.current == nil || t.kind != kindHTML {
		return nil
	}
	identifier, ok := expr.(*IdentifierExpr)
	if !ok {
		return nil
	}
	if _, ok := c.current.params[identifier.Name]; !ok {
		return nil
	}
	if c.loopDepth > 0 {
		return c.error(identifier.Pos, "slot "+identifier.Name+" cannot appear inside a for body")
	}
	if c.awaitDepth > 0 {
		return c.error(identifier.Pos, "slot "+identifier.Name+" cannot appear inside an await block; the fallback and the replacement would both render it")
	}
	if c.slotUsed[identifier.Name] {
		return c.error(identifier.Pos, "slot "+identifier.Name+" is rendered more than once on the same path")
	}
	c.slotUsed[identifier.Name] = true
	return nil
}

func (c *compiler) analyzeAttribute(element string, attribute Attribute, scope map[string]valueType) error {
	mixed := len(attribute.Value) > 1 || (len(attribute.Value) == 1 && attribute.Value[0].Expression == nil)
	for _, part := range attribute.Value {
		if part.Expression == nil {
			continue
		}
		t, err := c.infer(part.Expression, scope)
		if err != nil {
			return err
		}
		if isURLAttribute(attribute.Name) && t.required().kind != kindURL {
			return c.error(part.Pos, "attribute "+attribute.Name+" requires url, got "+t.String())
		}
		if mixed && t.optional {
			return c.error(part.Pos, "optional expression must be the entire attribute value")
		}
		context := "html:attribute"
		if isEventAttribute(attribute.Name) {
			context = "html:event"
		}
		if err := c.validateInsertion(context, t, part.Pos); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) analyzeComponentCall(node *ComponentNode, scope map[string]valueType) error {
	component, ok := c.components[node.Name]
	if !ok {
		return c.error(node.Pos, "unknown component "+node.Name)
	}
	provided := map[string]bool{}
	for _, argument := range node.Arguments {
		want, ok := component.params[argument.Name]
		if !ok {
			return c.error(argument.Pos, "unknown argument "+argument.Name+" for "+node.Name)
		}
		if provided[argument.Name] {
			return c.error(argument.Pos, "duplicate argument "+argument.Name)
		}
		provided[argument.Name] = true
		got, err := c.attributeValueType(argument, scope)
		if err != nil {
			return err
		}
		if !assignable(want, got) {
			return c.error(argument.Pos, "argument "+argument.Name+" expects "+want.String()+", got "+got.String())
		}
	}
	fills, rest, err := splitFills(node.Children)
	if err != nil {
		return c.error(node.Pos, err.Error())
	}
	filled := map[string]bool{}
	for _, fill := range fills {
		want, ok := component.params[fill.Name]
		if !ok || want.kind != kindHTML {
			return c.error(fill.Pos, "component "+node.Name+" has no slot named "+fill.Name)
		}
		if provided[fill.Name] || filled[fill.Name] {
			return c.error(fill.Pos, "component "+node.Name+" received slot "+fill.Name+" twice")
		}
		filled[fill.Name] = true
	}
	if hasContent(rest) {
		childrenType, acceptsChildren := component.params["children"]
		if !acceptsChildren || childrenType.kind != kindHTML {
			return c.error(node.Pos, "component "+node.Name+" does not accept children")
		}
		if provided["children"] {
			return c.error(node.Pos, "component "+node.Name+" received children twice")
		}
		filled["children"] = true
	}
	for name, want := range component.params {
		if provided[name] || filled[name] {
			continue
		}
		if want.kind == kindHTML {
			if want.optional {
				continue
			}
			if name == "children" {
				return c.error(node.Pos, "component "+node.Name+" requires children")
			}
			return c.error(node.Pos, "component "+node.Name+" requires slot "+name)
		}
		return c.error(node.Pos, "missing argument "+name+" for "+node.Name)
	}
	// Fill content belongs to the caller, so it keeps the caller slot usage
	// state rather than the callee's.
	for _, fill := range fills {
		if err := c.analyzeNodes(fill.Body, copyScope(scope)); err != nil {
			return err
		}
	}
	return c.analyzeNodes(rest, scope)
}

// slotFill is one template element carrying a static name attribute directly
// under a component call.
type slotFill struct {
	Name string
	Pos  Position
	Body []Node
}

// splitFills separates named fill blocks from the content that fills the
// unnamed slot. A template element without a name attribute is ordinary markup.
func splitFills(children []Node) ([]slotFill, []Node, error) {
	var fills []slotFill
	var rest []Node
	for _, child := range children {
		element, ok := child.(*ElementNode)
		if !ok || element.Name != "template" {
			rest = append(rest, child)
			continue
		}
		name := ""
		named := false
		for _, attribute := range element.Attributes {
			if attribute.Name != "name" {
				continue
			}
			value, static := staticAttributeText(attribute)
			if !static {
				return nil, nil, fmt.Errorf("slot fill name must be a static value")
			}
			name, named = value, true
		}
		if !named {
			rest = append(rest, child)
			continue
		}
		if len(element.Attributes) != 1 {
			return nil, nil, fmt.Errorf("slot fill template accepts only the name attribute")
		}
		fills = append(fills, slotFill{Name: name, Pos: element.Pos, Body: element.Children})
	}
	return fills, rest, nil
}

// hasContent reports whether nodes carry anything but insignificant whitespace,
// so formatting between fill blocks does not count as unnamed slot content.
func hasContent(nodes []Node) bool {
	for _, node := range nodes {
		text, ok := node.(*TextNode)
		if !ok {
			return true
		}
		if strings.TrimSpace(text.Text) != "" {
			return true
		}
	}
	return false
}

func (c *compiler) attributeValueType(attribute Attribute, scope map[string]valueType) (valueType, error) {
	if attribute.Boolean {
		return valueType{kind: kindBool}, nil
	}
	if len(attribute.Value) == 1 && attribute.Value[0].Expression != nil {
		return c.infer(attribute.Value[0].Expression, scope)
	}
	for _, part := range attribute.Value {
		if part.Expression == nil {
			continue
		}
		t, err := c.infer(part.Expression, scope)
		if err != nil {
			return valueType{}, err
		}
		if t.required().kind != kindString {
			return valueType{}, c.error(part.Pos, "mixed attribute value expressions must be string")
		}
		if t.optional {
			return valueType{}, c.error(part.Pos, "optional expression must be the entire attribute value")
		}
	}
	return valueType{kind: kindString}, nil
}

// annotateRawTextInsertion adds the raw-text hint to an analysis diagnostic. A
// brace the parser accepted as an insertion can still be authored JavaScript
// that happened to match an insertion shape, such as `{name}` written as an
// object shorthand, and those reach analysis rather than the parser.
func annotateRawTextInsertion(context string, err error) error {
	if !isRawTextContext(context) {
		return err
	}
	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		return err
	}
	compileErr.Message += rawTextHint(context, false)
	return err
}

func (c *compiler) validateInsertion(context string, t valueType, pos Position) error {
	base := t.required().kind
	switch context {
	case "html:child":
		if base == kindTrustedCSS || base == kindTrustedJS || base == kindScriptJSON || base == kindRecord || base == kindArray || base == kindError {
			return c.error(pos, "cannot insert "+t.String()+" into html:child")
		}
	case "html:attribute":
		if base == kindTrustedHTML || base == kindTrustedCSS || base == kindTrustedJS || base == kindScriptJSON || base == kindRecord || base == kindArray || base == kindHTML || base == kindError {
			return c.error(pos, "cannot insert "+t.String()+" into html:attribute")
		}
	case "html:event":
		// An event handler's value is compiled as JavaScript, so the same rule
		// html:script carries applies here. script_json does not carry over: a
		// handler body is code, and embedded data has no meaning in it.
		if base != kindTrustedJS {
			return c.error(pos, "html:event requires trusted_javascript; wrap the value in RawJavaScript to state that it is code, or attach the behavior with "+
				ServerActionAttr+" instead")
		}
	case "html:script":
		if base != kindTrustedJS && base != kindScriptJSON {
			return c.error(pos, "html:script requires trusted_javascript or script_json")
		}
	case "html:style":
		if base != kindTrustedCSS {
			return c.error(pos, "html:style requires trusted_css")
		}
	default:
		return c.error(pos, "unknown HTML insertion context "+context)
	}
	return nil
}

func (c *compiler) infer(expr Expr, scope map[string]valueType) (valueType, error) {
	if known, ok := c.exprTypes[expr]; ok {
		return known, nil
	}
	var result valueType
	var err error
	switch expr := expr.(type) {
	case *IdentifierExpr:
		if t, ok := scope[expr.Name]; ok {
			result = t
		} else if t, ok := c.enumMembers[expr.Name]; ok {
			result = t
		} else {
			err = c.error(expr.Pos, "unknown identifier "+expr.Name)
		}
	case *LiteralExpr:
		switch expr.ValueKind {
		case "string":
			result.kind = kindString
		case "bool":
			result.kind = kindBool
		case "number":
			if strings.Contains(expr.Value.(string), ".") {
				result.kind = kindFloat
			} else {
				result.kind = kindInt
			}
		case "null":
			result = valueType{kind: kindInvalid, optional: true}
		default:
			err = c.error(expr.Pos, "unknown literal type")
		}
	case *MemberExpr:
		var object valueType
		object, err = c.infer(expr.Object, scope)
		if err == nil {
			if object.optional {
				err = c.error(expr.Pos, "member access on optional "+object.String())
			} else if object.kind == kindError {
				field, ok := errorFields[expr.Member]
				if !ok {
					err = c.error(expr.Pos, "unknown field "+expr.Member+" on error")
				} else {
					result = field
				}
			} else if object.kind != kindRecord {
				err = c.error(expr.Pos, "member access requires a record")
			} else {
				field, ok := findField(c.records[object.name], expr.Member)
				if !ok {
					err = c.error(expr.Pos, "unknown field "+expr.Member+" on "+object.name)
				} else {
					result, err = c.resolveType(field.Type)
				}
			}
		}
	case *IndexExpr:
		var object, index valueType
		object, err = c.infer(expr.Object, scope)
		if err == nil {
			index, err = c.infer(expr.Index, scope)
		}
		if err == nil && (object.kind != kindArray || object.optional) {
			err = c.error(expr.Pos, "indexing requires an array")
		}
		if err == nil && index.kind != kindInt {
			err = c.error(expr.Pos, "array index must be int")
		}
		if err == nil {
			result = *object.elem
		}
	case *CallExpr:
		result, err = c.inferCall(expr, scope)
	case *UnaryExpr:
		var operand valueType
		operand, err = c.infer(expr.Operand, scope)
		if err == nil {
			switch expr.Operator {
			case "!", "not":
				if operand.kind != kindBool || operand.optional {
					err = c.error(expr.Pos, "not requires bool")
				} else {
					result = operand
				}
			case "+", "-":
				if !numeric(operand) {
					err = c.error(expr.Pos, "numeric unary operator requires number")
				} else {
					result = operand
				}
			default:
				err = c.error(expr.Pos, "unsupported unary operator "+expr.Operator)
			}
		}
	case *BinaryExpr:
		var left, right valueType
		left, err = c.infer(expr.Left, scope)
		if err == nil {
			right, err = c.infer(expr.Right, scope)
		}
		if err == nil {
			result, err = c.binaryType(expr, left, right)
		}
	case *ConditionalExpr:
		var condition, thenType, elseType valueType
		condition, err = c.infer(expr.Condition, scope)
		if err == nil && (condition.kind != kindBool || condition.optional) {
			err = c.error(expr.Pos, "conditional condition must be bool")
		}
		if err == nil {
			thenType, err = c.infer(expr.Then, scope)
		}
		if err == nil {
			elseType, err = c.infer(expr.Else, scope)
		}
		if err == nil {
			if !assignable(thenType, elseType) || !assignable(elseType, thenType) {
				err = c.error(expr.Pos, "conditional branches must have the same type")
			} else {
				result = thenType
			}
		}
	default:
		err = c.error(Position{Line: 1, Col: 1}, fmt.Sprintf("unsupported expression %T", expr))
	}
	if err != nil {
		return valueType{}, err
	}
	// A pending value has no rendering, no fields, and no operators until it
	// settles, so the one place it may be read is the await binding that waits
	// for it. Checking here covers every expression form at once, including the
	// object of a member access and the operand of a comparison.
	if result.async && c.awaitSource != expr {
		return valueType{}, c.error(exprPos(expr), "async value "+result.String()+" must be bound by an await clause before it is read")
	}
	c.exprTypes[expr] = result
	return result, nil
}

func (c *compiler) inferCall(call *CallExpr, scope map[string]valueType) (valueType, error) {
	identifier, ok := call.Callee.(*IdentifierExpr)
	if !ok {
		return valueType{}, c.error(call.Pos, "only named functions can be called")
	}
	if intrinsic, ok := intrinsicResult(identifier.Name); ok {
		if len(call.Arguments) != 1 {
			return valueType{}, c.error(call.Pos, identifier.Name+" expects one argument")
		}
		argument, err := c.infer(call.Arguments[0], scope)
		if err != nil {
			return valueType{}, err
		}
		if identifier.Name != "JsonForScript" && (argument.kind != kindString || argument.optional) {
			return valueType{}, c.error(call.Pos, identifier.Name+" expects string")
		}
		if identifier.Name == "JsonForScript" && !c.jsonSerializable(argument, map[string]bool{}) {
			return valueType{}, c.error(call.Pos, "JsonForScript argument is not statically serializable")
		}
		return intrinsic, nil
	}
	sig, ok := c.externals[identifier.Name]
	if !ok {
		return valueType{}, c.error(call.Pos, "unknown function "+identifier.Name)
	}
	// An async result exists only inside the boundary that waits for it, so the
	// only legal call site is that boundary's own binding.
	if sig.live && c.awaitCall != call {
		return valueType{}, c.error(call.Pos, "live function "+identifier.Name+" can only be called in an await binding")
	}
	if sig.async && c.awaitCall != call {
		return valueType{}, c.error(call.Pos, "async function "+identifier.Name+" can only be called in an await binding")
	}
	if len(call.Arguments) != len(sig.params) {
		return valueType{}, c.error(call.Pos, fmt.Sprintf("%s expects %d arguments", identifier.Name, len(sig.params)))
	}
	for i, argument := range call.Arguments {
		got, err := c.infer(argument, scope)
		if err != nil {
			return valueType{}, err
		}
		if !assignable(sig.params[i], got) {
			return valueType{}, c.error(exprPos(argument), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.params[i], got))
		}
	}
	return sig.result, nil
}

func (c *compiler) binaryType(expr *BinaryExpr, left, right valueType) (valueType, error) {
	switch expr.Operator {
	case "and", "&&", "or", "||":
		if left.kind != kindBool || right.kind != kindBool || left.optional || right.optional {
			return valueType{}, c.error(expr.Pos, "boolean operator requires bool")
		}
		return valueType{kind: kindBool}, nil
	case "==", "!=":
		if left.kind == kindInvalid && left.optional {
			if !right.optional {
				return valueType{}, c.error(expr.Pos, "null can only compare with optional")
			}
			return valueType{kind: kindBool}, nil
		}
		if right.kind == kindInvalid && right.optional {
			if !left.optional {
				return valueType{}, c.error(expr.Pos, "null can only compare with optional")
			}
			return valueType{kind: kindBool}, nil
		}
		if !assignable(left, right) && !assignable(right, left) {
			return valueType{}, c.error(expr.Pos, "incompatible comparison")
		}
		if !c.comparable(left, map[string]bool{}) || !c.comparable(right, map[string]bool{}) {
			return valueType{}, c.error(expr.Pos, "values are not comparable")
		}
		return valueType{kind: kindBool}, nil
	case "<", "<=", ">", ">=":
		if !numeric(left) || !numeric(right) {
			return valueType{}, c.error(expr.Pos, "ordered comparison requires numbers")
		}
		return valueType{kind: kindBool}, nil
	case "+":
		if left.kind == kindString && right.kind == kindString && !left.optional && !right.optional {
			return valueType{kind: kindString}, nil
		}
		fallthrough
	case "-", "*", "/", "%":
		if !numeric(left) || !numeric(right) || left.kind != right.kind {
			return valueType{}, c.error(expr.Pos, "arithmetic operands must have the same numeric type")
		}
		return left, nil
	default:
		return valueType{}, c.error(expr.Pos, "unsupported binary operator "+expr.Operator)
	}
}

func intrinsicResult(name string) (valueType, bool) {
	switch name {
	case "RawHTML":
		return valueType{kind: kindTrustedHTML}, true
	case "RawCSS":
		return valueType{kind: kindTrustedCSS}, true
	case "RawJavaScript":
		return valueType{kind: kindTrustedJS}, true
	case "JsonForScript":
		return valueType{kind: kindScriptJSON}, true
	}
	return valueType{}, false
}

func assignable(want, got valueType) bool {
	if got.kind == kindInvalid && got.optional {
		return want.optional
	}
	if want.kind != got.kind || want.name != got.name || want.optional != got.optional {
		return false
	}
	if want.kind == kindArray {
		return want.elem != nil && got.elem != nil && assignable(*want.elem, *got.elem)
	}
	return true
}

func numeric(t valueType) bool {
	return !t.optional && (t.kind == kindInt || t.kind == kindFloat)
}
func (c *compiler) jsonSerializable(t valueType, visiting map[string]bool) bool {
	// A pending value has no serialization until it settles, and a record
	// carrying one cannot be written into a script as a whole.
	if t.async {
		return false
	}
	if t.optional {
		t.optional = false
	}
	switch t.kind {
	case kindString, kindBool, kindInt, kindFloat, kindDecimal, kindEnum:
		return true
	case kindArray:
		return t.elem != nil && c.jsonSerializable(*t.elem, visiting)
	case kindRecord:
		record, ok := c.records[t.name]
		if !ok {
			return false
		}
		if visiting[t.name] {
			return true
		}
		visiting[t.name] = true
		defer delete(visiting, t.name)
		for _, field := range record.Fields {
			fieldType, err := c.resolveType(field.Type)
			if err != nil || !c.jsonSerializable(fieldType, visiting) {
				return false
			}
		}
		return true
	}
	return false
}

func (c *compiler) comparable(t valueType, visiting map[string]bool) bool {
	if t.async {
		return false
	}
	if t.optional {
		return true
	}
	switch t.kind {
	case kindString, kindBool, kindInt, kindFloat, kindDecimal, kindDateTime, kindDate, kindTime, kindURL,
		kindEnum, kindTrustedHTML, kindTrustedCSS, kindTrustedJS, kindScriptJSON:
		return true
	case kindRecord:
		record := c.records[t.name]
		if record == nil {
			return false
		}
		if visiting[t.name] {
			return false
		}
		visiting[t.name] = true
		defer delete(visiting, t.name)
		for _, field := range record.Fields {
			fieldType, err := c.resolveType(field.Type)
			if err != nil || !c.comparable(fieldType, visiting) {
				return false
			}
		}
		return true
	}
	return false
}

// isURLAttribute reports whether a browser resolves this attribute's value as a
// URL. Membership decides two things at once: the analysis below requires a url
// expression here, and the emitter routes the value through the runtime's
// scheme policy instead of through plain text escaping.
//
// The roster is what it is because a name missing from it is not merely
// unchecked — it falls back to the ordinary text path, which accepts any string
// and escapes it for the wrong context. See rule:url-bearing-attributes.
func isURLAttribute(name string) bool {
	switch name {
	// Navigation and loading, the positions where a hostile scheme executes.
	case "href", "src", "action", "formaction", "poster", "data", "xlink:href":
		return true
	// Resolved but not navigated. They earn their place by taking the url type
	// rather than by the scheme test, which a legitimate destination passes.
	case "cite", "background", "longdesc", "manifest":
		return true
	// Obsolete plugin loading. The mechanism is gone from browsers; membership
	// costs nothing and removes the question.
	case "classid", "codebase", "archive", "profile":
		return true
	}
	return false
}

// isURLListAttribute reports whether the attribute holds several URLs, and in
// which grammar. srcset separates candidates with commas and lets each carry a
// descriptor; ping separates plain URLs with whitespace.
//
// These are analyzed as ordinary text rather than as a url expression, because
// neither grammar is expressible as one url.URL. The emitter still routes them
// through the scheme policy, per entry.
func isURLListAttribute(name string) (shape string, ok bool) {
	switch name {
	case "srcset", "imagesrcset":
		return "srcset", true
	case "ping":
		return "space", true
	}
	return "", false
}

// isEventAttribute reports whether the attribute is an event handler, whose
// value a browser compiles as JavaScript.
//
// The match is the on prefix followed by ASCII lowercase letters rather than a
// roster of known handler names, because a roster goes stale as browsers add
// handlers and the safe reading of an unrecognized on-name is that it is one. A
// hyphenated name such as on-click is not a handler content attribute and
// belongs to a custom element, so it does not match. See
// rule:event-attribute-context.
func isEventAttribute(name string) bool {
	if len(name) < 3 || name[0] != 'o' || name[1] != 'n' {
		return false
	}
	for i := 2; i < len(name); i++ {
		if name[i] < 'a' || name[i] > 'z' {
			return false
		}
	}
	return true
}
func findField(record *TypeDecl, name string) (Field, bool) {
	if record == nil {
		return Field{}, false
	}
	for _, field := range record.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return Field{}, false
}
func copyUsage(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
func copyScope(in map[string]valueType) map[string]valueType {
	out := make(map[string]valueType, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
func (c *compiler) error(pos Position, message string) error {
	return &CompileError{Filename: c.filename, Pos: pos, Message: message}
}
func goPublicName(name string) string {
	if name == "" {
		return name
	}
	r, size := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(r)) + name[size:]
}
func exprPos(expr Expr) Position {
	switch expr := expr.(type) {
	case *IdentifierExpr:
		return expr.Pos
	case *LiteralExpr:
		return expr.Pos
	case *MemberExpr:
		return expr.Pos
	case *IndexExpr:
		return expr.Pos
	case *CallExpr:
		return expr.Pos
	case *UnaryExpr:
		return expr.Pos
	case *BinaryExpr:
		return expr.Pos
	case *ConditionalExpr:
		return expr.Pos
	}
	return Position{Line: 1, Col: 1}
}

// statefulControls are the elements whose state the browser owns rather than the
// markup: what the user typed, where the caret is, what is selected. Replacing
// one discards all of it.
var statefulControls = map[string]bool{
	"input":    true,
	"textarea": true,
	"select":   true,
	"form":     true,
}

// rejectStatefulControls reports a form control in a live boundary's primary
// subtree, which is the one subtree a delivery replaces.
//
// A navigation already had this exposure once per page transition. A live
// boundary turns it into once every few seconds, arriving on the server's clock
// while the user is typing, so a warning would be read as noise and the loss is
// silent. The rule is therefore an error, and a control that genuinely should
// reset says so with an annotation.
//
// It does not walk the fallback or recover subtrees: neither is re-rendered by a
// delivery, so a control there is as safe as one outside the boundary. It also
// does not follow component calls, because a component's own body is checked
// where it is declared only if that body holds the boundary; a control reached
// through a call stays authoring guidance rather than a diagnostic.
func (c *compiler) rejectStatefulControls(nodes []Node) error {
	for _, node := range nodes {
		switch node := node.(type) {
		case *ElementNode:
			if statefulControls[node.Name] {
				return c.error(node.Pos, "<"+node.Name+"> cannot appear in a live boundary; a delivery replaces this subtree and discards what the user typed. Put the control outside the boundary and update it through its parameters instead")
			}
			if err := c.rejectStatefulControls(node.Children); err != nil {
				return err
			}
		case *syntax.IfNode:
			if err := c.rejectStatefulControls(node.Then); err != nil {
				return err
			}
			if err := c.rejectStatefulControls(node.Else); err != nil {
				return err
			}
		case *syntax.ForNode:
			if err := c.rejectStatefulControls(node.Body); err != nil {
				return err
			}
		case *syntax.AwaitNode:
			// A nested boundary's own subtrees are replaced by that boundary,
			// which this delivery also re-runs, so they are in scope too.
			if err := c.rejectStatefulControls(node.Primary); err != nil {
				return err
			}
			if err := c.rejectStatefulControls(node.Fallback); err != nil {
				return err
			}
			if err := c.rejectStatefulControls(node.Recover); err != nil {
				return err
			}
		case *SlotNode:
			if err := c.rejectStatefulControls(node.Default); err != nil {
				return err
			}
		}
	}
	return nil
}
