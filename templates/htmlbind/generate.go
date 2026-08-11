package htmlbind

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// GenerateOptions controls the generated Go file and the static files
// extracted alongside it.
type GenerateOptions struct {
	// Package overrides the template package/module declaration.
	Package string
	// DataAttributePrefix names the generated update protocol data attributes.
	// Empty uses DefaultDataAttributePrefix.
	DataAttributePrefix string
	// Unit names the generation unit whose assets are extracted. Empty derives
	// it from the template file name.
	Unit string
	// PublicURLBase prefixes generated asset file names in head references.
	// Empty uses DefaultPublicURLBase. The value is used verbatim, so an
	// absolute URL path and a full CDN URL behave the same.
	PublicURLBase string
	// ContextExternals names the external functions whose Go implementation
	// takes a leading context.Context. Those calls receive the boundary's
	// context; every other external is called as an ordinary function.
	//
	// The caller discovers this by reading the package's Go sources, so the
	// template declaration stays the same either way and the choice belongs to
	// whoever writes the implementation.
	ContextExternals map[string]bool
	// PreserveWhitespace turns off requirement:static-whitespace-normalization,
	// so static output keeps the authoring indentation and newlines byte for
	// byte. It exists for a project comparing generated markup against
	// pre-existing golden files.
	PreserveWhitespace bool
	// ServerActions maps each handler name a template reaches through
	// ServerActionAttr to the endpoint URL the lowering writes. The caller
	// resolves it, because the URL depends on the route the template serves and
	// the compiler cannot see that; [ActionRefs] reports what needs resolving.
	//
	// A reference with no entry here is a compile error, so a template naming a
	// handler nobody resolved never silently emits a dead element.
	ServerActions map[string]string
	// ServerActionResolver answers a name ServerActions does not hold. It is what
	// lets a framework address a handler from its own route table, for a template
	// that sits outside the tree route discovery walks.
	//
	// The map wins, so configuring a resolver cannot retarget an action a
	// discovered package already declares.
	ServerActionResolver func(name string) (url string, ok bool)
	// ServerActionAttr is the attribute the lowering writes. Empty uses
	// [DefaultActionAttr]. A framework driving an existing client library points
	// it at that library's vocabulary, such as hx-post.
	ServerActionAttr string
	// ReferenceHooks rewrite the static values of the attributes they are
	// registered for, before analysis and before asset extraction, and declare
	// the conversions those rewrites depend on. They are how a build converts a
	// file the template points at, such as an image to a modern format or a
	// TypeScript entry point to JavaScript, without this package holding a
	// converter or a naming rule.
	//
	// A hook converts and returns the bytes, so it may decide the rewrite from
	// how the conversion turned out. A hook declaring a CacheKey lets the caller
	// reuse a stored result instead of converting again.
	//
	// Registering none leaves output byte-identical.
	ReferenceHooks []ReferenceHook
	// ContentHooks compile the component script blocks whose lang attribute
	// they claim, so a block written in TypeScript reaches the browser as
	// JavaScript without a compiler entering this module.
	//
	// Registering none is the ordinary case: a block with no lang marker is
	// written exactly as authored, and a marker naming no registered hook is a
	// generation error rather than a silent passthrough.
	ContentHooks []ContentHook
	// CSRFMode turns the automatic CSRF field off. Empty is [CSRFAuto], which
	// puts the hidden field in every unsafe form.
	CSRFMode CSRFMode
	// CSRFFieldName renames the hidden field. Empty uses
	// [DefaultCSRFFieldName]. It has to agree with whatever middleware reads the
	// token back out.
	CSRFFieldName string
	// BuiltinElements are the hyphenated elements a framework contributes, each
	// rewritten at generation time into plan steps. See [BuiltinElement].
	BuiltinElements []BuiltinElement
	// PassthroughElements are the hyphenated elements an application uses and
	// this package emits verbatim: its Web Components, named exactly or by a
	// prefix glob such as "sl-*".
	//
	// Registering neither leaves the hyphenated space closed and empty, so every
	// hyphenated element in a template is a generation error naming the file,
	// line, and column. That is the one behavior change for an existing project,
	// and it is the point: an unrecognized hyphenated element emitted unchanged
	// renders nothing and reports nothing.
	PassthroughElements []PassthroughElement
	// StrictReferenceHooks turns an expression-valued attribute at a registered
	// element and attribute pair into a compile error. It is off by default,
	// because a project may legitimately mix authored references with
	// user-supplied ones; [Result.DynamicReferences] reports them either way.
	StrictReferenceHooks bool
}

// Result is one compiled template module: the generated Go source and the
// static files requirement:static-asset-extraction pulled out of it.
type Result struct {
	GoSource []byte
	Assets   []Asset
	// Produced holds the files the hooks' conversions created, sorted by name.
	// They may outnumber the rewrites, because a conversion may write a file no
	// attribute names, and the caller writes them so they join the run's
	// declared outputs rather than appearing behind it.
	Produced []ProducedFile
	// Rewrites reports what the hooks did, including what they declined and
	// why. A build-time rewrite is invisible in the template, so the build is
	// the only place it can be seen.
	Rewrites []Rewrite
	// ReadSet lists the files the transforms reported reading beyond the sources
	// their cache keys named. What is named by neither is not hashed, and an
	// edit to it will not regenerate.
	ReadSet []string
	// DynamicReferences are the attributes a hook was registered for whose
	// value is an expression, and so could not be rewritten.
	DynamicReferences []DynamicReference
}

// Generate compiles an HTML template module to Go, discarding the extracted
// assets. Callers that write files use GenerateModule instead.
func Generate(filename string, source []byte, options GenerateOptions) ([]byte, error) {
	result, err := GenerateModule(filename, source, options)
	return result.GoSource, err
}

// GenerateModule parses, validates, and compiles an HTML template module to Go
// plus its extracted stylesheet and script files.
//
// Each component becomes an immutable render plan: an instruction list typed by
// its parameter struct, executed by the shared htmlbind coordinator. Generated
// code owns no response concerns, so it depends on neither net/http nor any
// content negotiation.
func GenerateModule(filename string, source []byte, options GenerateOptions) (Result, error) {
	module, err := Parse(filename, source)
	if err != nil {
		return Result{}, err
	}
	// Hooks run on the parsed module before analysis, so a rewritten value is
	// type checked, escaped, and folded exactly as an authored one, and before
	// extraction decides an external script or link is a passthrough, so
	// extraction sees the rewritten URL.
	hooks, err := applyReferenceHooks(filename, module, options.ReferenceHooks, options.StrictReferenceHooks)
	if err != nil {
		return Result{}, err
	}
	// A registration mistake is reported against whoever wrote the generate
	// command, before any template is examined for a marker.
	if err := ValidateContentHooks(options.ContentHooks); err != nil {
		return Result{}, err
	}
	result := Result{
		Produced:          hooks.produced,
		Rewrites:          hooks.rewrites,
		ReadSet:           hooks.read,
		DynamicReferences: hooks.dynamic,
	}
	compiler := newCompiler(filename, string(source), module, !options.PreserveWhitespace)
	// The whitelist is normalized before analysis, so a registration mistake is
	// reported against whoever wrote the generate command rather than against
	// the first template that happens to use the element.
	elements, err := normalizeElements(options.BuiltinElements, options.PassthroughElements)
	if err != nil {
		return Result{}, err
	}
	compiler.elements = elements
	compiler.csrfMode = options.CSRFMode
	compiler.csrfField = options.CSRFFieldName
	compiler.attrPrefix = options.DataAttributePrefix
	if compiler.attrPrefix == "" {
		compiler.attrPrefix = DefaultDataAttributePrefix
	}
	if err := compiler.analyze(); err != nil {
		return Result{}, err
	}
	// Extraction runs before emission so a plan's head carries the reference
	// tags rather than the style and script blocks themselves.
	assets, err := compiler.extractAssets(options)
	if err != nil {
		return Result{}, err
	}
	result.Assets = assets
	// A content transform's read set joins the reference transforms' own, so an
	// edit to a file either kind read regenerates through one path.
	result.ReadSet = append(result.ReadSet, compiler.contentReads...)
	generated, err := compiler.emit(options)
	if err != nil {
		return Result{}, err
	}
	formatted, err := format.Source(generated)
	if err != nil {
		result.GoSource = generated
		return result, fmt.Errorf("format generated HTML code: %w\n%s", err, generated)
	}
	result.GoSource = formatted
	return result, nil
}

type goEmitter struct {
	c *compiler
	b bytes.Buffer
	// declarations collects loop scope types and fill plans, which must be
	// declared outside the plan literal that references them.
	declarations bytes.Buffer
	// scopeCount makes generated scope and fill names unique per file.
	scopeCount int
	// scope is the style renaming of the component being emitted.
	scope *styleScope
	// shell marks that the component being emitted owns the document head.
	shell bool
	// jsonRecords names the record types needing a generated JSON encoder, in
	// emission order. Scalars, arrays, and optionals are encoded by the
	// generic htmlbind helpers instead, so nothing source-independent is
	// declared here and two templates of one package never collide.
	jsonRecords []valueType
	// canonRecords names the record types a boundary reads as input, which need
	// a generated canonical encoder for the input validator.
	canonRecords []valueType
	// prefix is the configured data attribute namespace.
	prefix string
	// boundaryRoot is the root element of the component being emitted, or nil
	// when it is not an update boundary.
	boundaryRoot *ElementNode
	// reloadable marks that the component being emitted is published as a
	// redraw endpoint, and kindConst names the constant holding its identity.
	reloadable bool
	kindConst  string
	// scopeRoot is the root element of a component declaring a script block,
	// and scopeID the declaration identity written onto it. They are separate
	// from boundaryRoot because the marker is not a boundary: an ordinary
	// component call opens none, and it is exactly those calls whose instances
	// a scoped script has to find.
	scopeRoot *ElementNode
	scopeID   string
	// foreignDepth mirrors the compiler's: inside SVG or MathML a hyphenated
	// name is a standard foreign-namespace element rather than a registered one.
	foreignDepth int
	// providerImports collects the packages the builtin elements actually used
	// need, so a project using none imports none.
	providerImports map[string]string
	// cacheRecords names the record types needing a generated cache key
	// encoder, on the same terms.
	cacheRecords []valueType
	// pkg is the resolved Go package name, used as the stable half of a cache
	// component's identity.
	pkg string
	// contextExternals mirrors GenerateOptions.ContextExternals.
	contextExternals map[string]bool
	// rootScope is the parameter scope of the component being emitted. A check
	// written against it can be hoisted to the plan, where it runs before the
	// component writes anything; a check on a loop item cannot, because the
	// item does not exist yet.
	rootScope *emitScope
	// checks collects the hoisted parameter checks of the component being
	// emitted.
	checks []string
	// actions and actionAttr mirror the ServerActions and ServerActionAttr
	// options, which together decide what a server-action attribute lowers to.
	// resolveAction mirrors ServerActionResolver and answers what actions does
	// not hold.
	actions       map[string]string
	resolveAction func(string) (string, bool)
	actionAttr    string
}

// packageName is the template module's package, which together with the file
// and the declaration names a component uniquely.
func (c *compiler) packageName() string {
	if c.module.Package == nil || c.module.Package.Name == "" {
		return "templates"
	}
	name := c.module.Package.Name
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	return goIdentifier(name)
}

// usesReloadable reports whether any component is published for redraw, which
// is what pulls the update runtime and its HTTP types into the imports.
func (c *compiler) usesReloadable() bool {
	for _, declaration := range c.module.Declarations {
		if component, ok := declaration.(*TemplateDecl); ok && c.components[component.Name].reloadable {
			return true
		}
	}
	return false
}

func (c *compiler) emit(options GenerateOptions) ([]byte, error) {
	e := &goEmitter{
		c:                c,
		contextExternals: options.ContextExternals,
		actions:          options.ServerActions,
		resolveAction:    options.ServerActionResolver,
		actionAttr:       options.ServerActionAttr,
	}
	if e.actionAttr == "" {
		e.actionAttr = DefaultActionAttr
	}
	e.prefix = options.DataAttributePrefix
	if e.prefix == "" {
		e.prefix = DefaultDataAttributePrefix
	}
	if err := validatePrefix(e.prefix); err != nil {
		return nil, err
	}
	pkg := options.Package
	if pkg == "" && c.module.Package != nil {
		pkg = c.module.Package.Name
		if index := strings.LastIndex(pkg, "."); index >= 0 {
			pkg = pkg[index+1:]
		}
	}
	if pkg == "" {
		pkg = "templates"
	}
	pkg = goIdentifier(pkg)
	e.pkg = pkg
	e.b.WriteString("// Code generated by tinybind HTML templates; DO NOT EDIT.\n\n")
	fmt.Fprintf(&e.b, "package %s\n\n", pkg)
	e.jsonRecords = e.collectJSONRecords()
	e.canonRecords = e.collectCanonRecords()
	e.cacheRecords = e.collectCacheRecords()
	e.providerImports = e.collectProviderImports()
	e.emitImports()
	e.emitDeclaredTypes()
	if err := e.emitComponentParams(); err != nil {
		return nil, err
	}
	e.emitJSONHelpers()
	e.emitCanonHelpers()
	if err := e.emitCacheKeyHelpers(); err != nil {
		return nil, err
	}
	// Plans are emitted into a side buffer first so the declarations they need
	// can be written ahead of them.
	var plans bytes.Buffer
	for _, declaration := range c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		start := e.b.Len()
		if err := e.emitComponentPlan(component); err != nil {
			return nil, err
		}
		plans.Write(e.b.Bytes()[start:])
		e.b.Truncate(start)
	}
	e.b.Write(e.declarations.Bytes())
	e.b.Write(plans.Bytes())
	return e.b.Bytes(), nil
}

func (e *goEmitter) emitImports() {
	e.b.WriteString("import (\n")
	// context reaches generated code through an await boundary's bindings,
	// through any synchronous external whose implementation declared one, and
	// through a redraw registration, whose Render takes one it does not read.
	if e.c.usesAwait() || e.usesSyncRenderContext() || e.c.usesReloadable() {
		e.b.WriteString("\t\"context\"\n")
	}
	// strings is used only by the generated record JSON encoders; every other
	// formatter now lives in htmlbind.
	if len(e.jsonRecords) > 0 {
		e.b.WriteString("\t\"strings\"\n")
	}
	if e.c.usesKind(kindDateTime) || e.c.usesKind(kindDate) || e.c.usesKind(kindTime) {
		e.b.WriteString("\t\"time\"\n")
	}
	if e.c.usesKind(kindURL) || e.c.usesReloadable() {
		e.b.WriteString("\t\"net/url\"\n")
	}
	e.b.WriteString("\n\t\"github.com/shibukawa/tinybind-go/htmlbind\"\n")
	if e.usesBoundary() {
		e.b.WriteString("\t\"github.com/shibukawa/tinybind-go/htmlbind/delta\"\n")
	}
	if e.c.usesReloadable() {
		e.b.WriteString("\t\"github.com/shibukawa/tinybind-go/htmlupdate\"\n")
	}
	// A provider's package is imported only where a builtin element backed by
	// one is actually written, so a project using none imports none.
	for _, path := range sortedKeys(e.providerImports) {
		alias := e.providerImports[path]
		if alias == pathBase(path) {
			fmt.Fprintf(&e.b, "\t%s\n", strconv.Quote(path))
			continue
		}
		fmt.Fprintf(&e.b, "\t%s %s\n", alias, strconv.Quote(path))
	}
	for _, imported := range e.c.module.Imports {
		alias := imported.Alias
		if alias == "" {
			alias = path.Base(imported.Path)
		}
		if e.c.usesQualified(alias) {
			fmt.Fprintf(&e.b, "\t%s %s\n", alias, strconv.Quote(imported.Path))
		}
	}
	e.b.WriteString(")\n\n")
}

func (e *goEmitter) emitDeclaredTypes() {
	for _, declaration := range e.c.module.Declarations {
		switch declaration := declaration.(type) {
		case *TypeDecl:
			fmt.Fprintf(&e.b, "type %s struct {\n", declaration.Name)
			for _, field := range declaration.Fields {
				t, _ := e.c.resolveType(field.Type)
				fmt.Fprintf(&e.b, "\t%s %s\n", goPublicName(field.Name), goType(t))
			}
			e.b.WriteString("}\n\n")
		case *EnumDecl:
			fmt.Fprintf(&e.b, "type %s string\n\nconst (\n", declaration.Name)
			for _, member := range declaration.Members {
				fmt.Fprintf(&e.b, "\t%s%s %s = %q\n", declaration.Name, member.Name, declaration.Name, member.Name)
			}
			e.b.WriteString(")\n\n")
		}
	}
}

// emitComponentParams declares one {Component}Params struct per component so
// every component keeps the same two-argument binding shape for zero, one, and
// many parameters.
func (e *goEmitter) emitComponentParams() error {
	for _, declaration := range e.c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		name := e.c.paramsGoName(component.Name)
		if e.c.nameExists(name) {
			return e.c.error(component.Pos, "generated parameter type conflicts with declaration "+name)
		}
		if len(component.Parameters) == 0 {
			fmt.Fprintf(&e.b, "type %s struct{}\n\n", name)
			continue
		}
		fmt.Fprintf(&e.b, "type %s struct {\n", name)
		for _, parameter := range component.Parameters {
			t, _ := e.c.resolveType(parameter.Type)
			fmt.Fprintf(&e.b, "\t%s %s\n", goPublicName(parameter.Name), goType(t))
		}
		e.b.WriteString("}\n\n")
	}
	return nil
}

// scopedClassList renames the classes the active component's style block
// declares. Classes it does not declare pass through untouched.
func (e *goEmitter) scopedClassList(value string) string {
	if e.scope == nil {
		return value
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return value
	}
	for i, field := range fields {
		fields[i] = e.scope.className(field)
	}
	return strings.Join(fields, " ")
}

// slotBodies maps each filled slot parameter of a component call to the nodes
// that fill it.
func (e *goEmitter) slotBodies(node *ComponentNode) (map[string][]Node, error) {
	fills, rest, err := splitFills(node.Children)
	if err != nil {
		return nil, err
	}
	bodies := map[string][]Node{}
	for _, fill := range fills {
		bodies[fill.Name] = fill.Body
	}
	if hasContent(rest) {
		bodies["children"] = rest
	}
	return bodies, nil
}

// renderStaticHTML serializes validated head contribution nodes to the literal
// bytes the merged head writes.
func renderStaticHTML(nodes []Node) string {
	var out strings.Builder
	for _, node := range nodes {
		switch node := node.(type) {
		case *TextNode:
			out.WriteString(node.Text)
		case *CommentNode:
			out.WriteString("<!--" + node.Text + "-->")
		case *ElementNode:
			out.WriteString("<" + node.Name)
			for _, attribute := range node.Attributes {
				if attribute.Boolean {
					out.WriteString(" " + attribute.Name)
					continue
				}
				value, _ := staticAttributeText(attribute)
				out.WriteString(" " + attribute.Name + "=\"" + value + "\"")
			}
			out.WriteString(">")
			out.WriteString(renderStaticHTML(node.Children))
			if !voidElements[node.Name] {
				out.WriteString("</" + node.Name + ">")
			}
		}
	}
	return out.String()
}

func (c *compiler) usesKind(kind valueKind) bool {
	for _, t := range c.exprTypes {
		if t.required().kind == kind {
			return true
		}
	}
	for _, info := range c.components {
		for _, t := range info.params {
			if kindUses(t, kind) {
				return true
			}
		}
	}
	for _, record := range c.records {
		for _, field := range record.Fields {
			t, err := c.resolveType(field.Type)
			if err == nil && kindUses(t, kind) {
				return true
			}
		}
	}
	return false
}

func kindUses(t valueType, kind valueKind) bool {
	if t.required().kind == kind {
		return true
	}
	return t.kind == kindArray && t.elem != nil && kindUses(*t.elem, kind)
}

// usesAwait reports whether any component in this file owns an await boundary.
func (c *compiler) usesAwait() bool {
	for _, info := range c.components {
		if info.await {
			return true
		}
	}
	return false
}

func (c *compiler) usesQualified(alias string) bool {
	return strings.Contains(c.source, alias+".")
}

// collectJSONRecords returns the record types reachable from a JsonForScript
// argument, sorted by name so emission is deterministic. Only records need a
// generated encoder: they are declared by this source, so their encoder is
// owned by this source too.
func (e *goEmitter) collectJSONRecords() []valueType {
	types := map[string]valueType{}
	for expr := range e.c.exprTypes {
		call, ok := expr.(*CallExpr)
		if !ok {
			continue
		}
		id, ok := call.Callee.(*IdentifierExpr)
		if !ok || id.Name != "JsonForScript" {
			continue
		}
		collectJSONTypes(types, e.c.exprTypes[call.Arguments[0]], e.c)
	}
	names := make([]string, 0, len(types))
	records := map[string]valueType{}
	for _, t := range types {
		base := t.required()
		if base.kind != kindRecord {
			continue
		}
		if _, seen := records[base.name]; seen {
			continue
		}
		records[base.name] = base
		names = append(names, base.name)
	}
	sort.Strings(names)
	out := make([]valueType, 0, len(names))
	for _, name := range names {
		out = append(out, records[name])
	}
	return out
}

func (e *goEmitter) emitJSONHelpers() {
	for _, record := range e.jsonRecords {
		e.emitJSONHelper(record)
	}
}

func collectJSONTypes(out map[string]valueType, t valueType, c *compiler) {
	key := jsonTypeKey(t)
	if _, ok := out[key]; ok {
		return
	}
	out[key] = t
	base := t.required()
	if base.kind == kindArray && base.elem != nil {
		collectJSONTypes(out, *base.elem, c)
	}
	if base.kind == kindRecord {
		for _, f := range c.records[base.name].Fields {
			ft, _ := c.resolveType(f.Type)
			collectJSONTypes(out, ft, c)
		}
	}
}

func jsonTypeKey(t valueType) string {
	prefix := ""
	if t.optional {
		prefix = "Optional"
		t.optional = false
	}
	switch t.kind {
	case kindArray:
		return prefix + "Array" + jsonTypeKey(*t.elem)
	case kindRecord, kindEnum:
		return prefix + t.name
	default:
		return prefix + goPublicName(string(t.kind))
	}
}

// emitJSONHelper writes the encoder for one declared record. Its fields are
// encoded through jsonEncodeCall, so nested scalars and slices reuse the
// generic htmlbind helpers.
func (e *goEmitter) emitJSONHelper(t valueType) {
	fmt.Fprintf(&e.b, "func %s(value %s) string {\n", jsonRecordEncoder(t.name), goType(t))
	e.b.WriteString("\tvar out strings.Builder\n\tout.WriteByte('{')\n")
	for i, f := range e.c.records[t.name].Fields {
		ft, _ := e.c.resolveType(f.Type)
		if i > 0 {
			e.b.WriteString("\tout.WriteByte(',')\n")
		}
		fmt.Fprintf(&e.b, "\tout.WriteString(%q)\n\tout.WriteString(%s)\n",
			strconv.Quote(f.Name)+":", jsonEncodeCall(ft, "value."+goPublicName(f.Name)))
	}
	e.b.WriteString("\tout.WriteByte('}')\n\treturn out.String()\n")
	e.b.WriteString("}\n\n")
}

// jsonRecordEncoder names the generated encoder for a declared record.
func jsonRecordEncoder(name string) string { return "_tinybindJSON" + name }

// jsonEncodeCall returns an expression encoding code as JSON.
func jsonEncodeCall(t valueType, code string) string {
	if t.optional {
		return "htmlbind.JSONOptional(" + code + ", " + jsonEncoder(t.required()) + ")"
	}
	if t.kind == kindArray && t.elem != nil {
		return "htmlbind.JSONArray(" + code + ", " + jsonEncoder(*t.elem) + ")"
	}
	return jsonEncoder(t) + "(" + code + ")"
}

// jsonEncoder returns a func value encoding one value of t, for the generic
// htmlbind helpers that take an element encoder.
func jsonEncoder(t valueType) string {
	if t.optional {
		return "func(value " + goType(t) + ") string { return " + jsonEncodeCall(t, "value") + " }"
	}
	switch t.kind {
	case kindBool:
		return "htmlbind.JSONBool"
	case kindInt:
		return "htmlbind.JSONInt"
	case kindFloat:
		return "htmlbind.JSONFloat"
	case kindRecord:
		return jsonRecordEncoder(t.name)
	case kindArray:
		return "func(value " + goType(t) + ") string { return " + jsonEncodeCall(t, "value") + " }"
	default:
		// string, decimal, enums, and the trusted string types are all ~string.
		return "htmlbind.JSONString[" + goType(t) + "]"
	}
}

func valueString(code string, t valueType) string {
	switch t.required().kind {
	case kindString, kindDecimal:
		return code
	case kindTrustedHTML, kindTrustedCSS, kindTrustedJS, kindScriptJSON, kindEnum:
		// These are generated named string types, so they need a conversion.
		return "string(" + code + ")"
	case kindBool:
		return "htmlbind.FormatBool(" + code + ")"
	case kindInt:
		return "htmlbind.FormatInt(" + code + ")"
	case kindFloat:
		return "htmlbind.FormatFloat(" + code + ")"
	case kindBytes:
		return "string(" + code + ")"
	case kindURL:
		return code + ".String()"
	case kindDateTime, kindDate, kindTime:
		return code + ".Format(time.RFC3339)"
	}
	return code
}

// escapeExempt reports whether valueString output for t can never contain a
// character HTML escaping rewrites — formatted bools, numbers, and RFC3339
// timestamps — so the emitter may write it unescaped.
func escapeExempt(t valueType) bool {
	switch t.required().kind {
	case kindBool, kindInt, kindFloat, kindDateTime, kindDate, kindTime:
		return true
	}
	return false
}

func goType(t valueType) string {
	// The handle wraps the whole settled type, so an optional async value is
	// one Pending of a pointer rather than a pointer to a Pending. A caller
	// then leaves it at its zero value to mean absent.
	if t.async {
		return "htmlbind.Pending[" + goType(t.awaited()) + "]"
	}
	var base string
	switch t.kind {
	case kindString, kindDecimal:
		base = "string"
	case kindBool:
		base = "bool"
	case kindInt:
		base = "int"
	case kindFloat:
		base = "float64"
	case kindBytes:
		base = "[]byte"
	case kindDateTime, kindDate, kindTime:
		base = "time.Time"
	case kindURL:
		base = "url.URL"
	case kindRecord, kindEnum:
		base = t.name
	case kindHTML:
		// A fragment carries its own absence, so an optional slot needs no
		// pointer indirection callers would have to satisfy.
		return "htmlbind.Fragment"
	case kindTrustedHTML:
		base = "htmlbind.TrustedHTML"
	case kindTrustedCSS:
		base = "htmlbind.TrustedCSS"
	case kindTrustedJS:
		base = "htmlbind.TrustedJavaScript"
	case kindScriptJSON:
		base = "htmlbind.ScriptJSON"
	case kindError:
		base = "htmlbind.AsyncError"
	case kindArray:
		if t.elem != nil {
			base = "[]" + goType(*t.elem)
		}
	}
	if t.optional {
		return "*" + base
	}
	return base
}

func (c *compiler) componentGoName(name string) string {
	if c.components[name].decl.Exported {
		return name
	}
	return "render" + name
}

// paramsGoName is the generated parameter struct name for a component. It
// follows the component's own exportedness so unexported components keep an
// unexported parameter type.
func (c *compiler) paramsGoName(name string) string { return c.componentGoName(name) + "Params" }

func goIdentifier(value string) string {
	var out strings.Builder
	for i, r := range value {
		if unicode.IsLetter(r) || r == '_' || (i > 0 && unicode.IsDigit(r)) {
			out.WriteRune(r)
		} else {
			out.WriteRune('_')
		}
	}
	if out.Len() == 0 {
		return "templates"
	}
	return goLocalName(out.String())
}

// goLocalName keeps a template name from colliding with a Go keyword.
func goLocalName(name string) string {
	if goKeywords[name] {
		return "_" + name
	}
	return name
}

var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}
