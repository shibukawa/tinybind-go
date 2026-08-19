package htmlbind

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/shibukawa/tinybind-go/internal/linedirective"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// emitScope describes the Go value an instruction list reads from. The top
// level of a component reads its parameter struct; a for body reads a generated
// scope struct holding the enclosing value plus the loop variables.
type emitScope struct {
	goType  string
	builder string
	// paths maps a template identifier to its field path from the receiver.
	paths map[string]string
	types map[string]valueType
}

func (s *emitScope) child(goType, builder string) *emitScope {
	paths := make(map[string]string, len(s.paths)+2)
	for name, path := range s.paths {
		paths[name] = "Outer." + path
	}
	types := make(map[string]valueType, len(s.types)+2)
	for name, t := range s.types {
		types[name] = t
	}
	return &emitScope{goType: goType, builder: builder, paths: paths, types: types}
}

const receiverIdent = "p"

// contextIdent is the name a plan closure gives the render context. It matches
// the name an await clause already uses, so a template author reading generated
// code sees one spelling.
const contextIdent = "ctx"

// takesRenderContext reports whether a synchronous external's Go implementation
// declared a leading context.Context. An async or live external is excluded:
// those already receive the boundary context through their own call shape, and
// they can only be called in an await binding, where ctx is in scope anyway.
func (e *goEmitter) takesRenderContext(name string) bool {
	if !e.contextExternals[name] {
		return false
	}
	signature, ok := e.c.externals[name]
	return ok && !signature.async && !signature.live
}

// usesSyncRenderContext reports whether the module calls such an external
// anywhere. Imports are written before any plan is emitted, so the answer has to
// come from the typed expressions rather than from what emission produced.
func (e *goEmitter) usesSyncRenderContext() bool {
	// A rich-text message calls its catalog function with the context binding
	// and is a node, so it is not reachable through the expression map below.
	if e.c.usesMessageBlock {
		return true
	}
	for expr := range e.c.exprTypes {
		// A binding lowers to a provider call taking the context, so reading
		// one is what pulls the context import in, exactly as a context-taking
		// external does.
		if identifier, ok := expr.(*IdentifierExpr); ok {
			if _, declared := e.c.bindings.lookup(identifier.Name); declared {
				return true
			}
		}
		if _, isMessage := expr.(*MessageExpr); isMessage && e.c.messageContextBinding != "" {
			return true
		}
		call, ok := expr.(*CallExpr)
		if !ok {
			continue
		}
		identifier, ok := call.Callee.(*IdentifierExpr)
		if ok && e.takesRenderContext(identifier.Name) {
			return true
		}
	}
	return false
}

// usesRenderContext reports whether evaluating expr calls such an external, so
// the closure holding it has to be given the context.
func (e *goEmitter) usesRenderContext(expr Expr) bool {
	found := false
	walkExpr(expr, func(node Expr) {
		// A binding lowers to a provider call taking the render context, so an
		// expression reading one needs the context-carrying instruction for the
		// same reason a context-taking external does.
		if identifier, ok := node.(*IdentifierExpr); ok {
			// Nothing else can be spelled this way: a parameter, a val binding
			// and a loop variable taking a declared name are all refused at
			// analysis, so the name here is the binding.
			if _, declared := e.c.bindings.lookup(identifier.Name); declared {
				found = true
			}
		}
		if _, isMessage := node.(*MessageExpr); isMessage && e.c.messageContextBinding != "" {
			// A reference reads the context binding, whose provider takes the
			// render context, so the instruction holding it needs one.
			found = true
		}
		call, ok := node.(*CallExpr)
		if !ok {
			return
		}
		identifier, ok := call.Callee.(*IdentifierExpr)
		if ok && e.takesRenderContext(identifier.Name) {
			found = true
		}
	})
	return found
}

// attributeUsesRenderContext is usesRenderContext over every expression part of
// one attribute value, which may mix literal text with several expressions.
func (e *goEmitter) attributeUsesRenderContext(attribute Attribute) bool {
	for _, part := range attribute.Value {
		if part.Expression != nil && e.usesRenderContext(part.Expression) {
			return true
		}
	}
	return false
}

// walkExpr visits expr and every expression inside it.
func walkExpr(expr Expr, visit func(Expr)) {
	if expr == nil {
		return
	}
	visit(expr)
	switch expr := expr.(type) {
	case *MemberExpr:
		walkExpr(expr.Object, visit)
	case *IndexExpr:
		walkExpr(expr.Object, visit)
		walkExpr(expr.Index, visit)
	case *CallExpr:
		for _, argument := range expr.Arguments {
			walkExpr(argument, visit)
		}
	case *MessageExpr:
		// A message argument is an ordinary expression, so it may itself reach
		// a context-taking external. Skipping it here would emit a plain
		// instruction whose closure then names a context it was never given.
		for _, argument := range expr.Args {
			walkExpr(argument.Value, visit)
		}
	case *UnaryExpr:
		walkExpr(expr.Operand, visit)
	case *BinaryExpr:
		walkExpr(expr.Left, visit)
		walkExpr(expr.Right, visit)
	case *ConditionalExpr:
		walkExpr(expr.Condition, visit)
		walkExpr(expr.Then, visit)
		walkExpr(expr.Else, visit)
	}
}

// closureParams writes the parameter list of a plan closure. A closure whose
// body reaches a context-taking external is given the render context as a
// leading parameter, which is what the matching Ctx instruction supplies.
func closureParams(goType string, withContext bool) string {
	if withContext {
		return contextIdent + " context.Context, " + receiverIdent + " " + goType
	}
	return receiverIdent + " " + goType
}

// ctxOp names the instruction matching closureParams. Without the context the
// name is unchanged, so a template calling no such external emits exactly the
// instructions it emitted before this existed.
func ctxOp(name string, withContext bool) string {
	if withContext {
		return name + "Ctx"
	}
	return name
}

// plannedOp is one instruction plus the template line it came from. The line is
// zero for an instruction the emitter wrote for itself, which keeps its own
// position rather than borrowing a template's.
type plannedOp struct {
	call string
	line int
}

// planEmitter accumulates the instruction list of one plan.
type planEmitter struct {
	e       *goEmitter
	scope   *emitScope
	ops     []plannedOp
	pending strings.Builder
	// line is the template line of the node being walked, which every
	// instruction that node produces is attributed to.
	line int
	// staticLine is where the pending run started. A static run coalesces across
	// template constructs, so it owns a span rather than a point and its start is
	// the only position that is true of all of it.
	staticLine int
}

func (p *planEmitter) static(text string) {
	if p.pending.Len() == 0 {
		p.staticLine = p.line
	}
	p.pending.WriteString(text)
}

func (p *planEmitter) flush() {
	if p.pending.Len() == 0 {
		return
	}
	text := p.pending.String()
	p.pending.Reset()
	p.append(p.scope.builder+".Static("+strconv.Quote(text)+")", p.staticLine)
}

// op appends a builder call. Static output is flushed first so instruction
// order matches document order.
func (p *planEmitter) op(call string) {
	p.append(p.scope.builder+"."+call, p.line)
}

func (p *planEmitter) raw(call string) { p.append(call, p.line) }

func (p *planEmitter) append(call string, line int) {
	p.ops = append(p.ops, plannedOp{call: call, line: line})
}

// literal renders the instruction list as a Go composite literal.
func (p *planEmitter) literal() string {
	p.flush()
	if len(p.ops) == 0 {
		return "nil"
	}
	directives := p.e.lineDirectives
	var out strings.Builder
	fmt.Fprintf(&out, "[]htmlbind.Op[%s]{\n", p.scope.goType)
	mapped := 0
	for _, op := range p.ops {
		nested := strings.Contains(op.call, "\n")
		if directives && op.line > 0 && op.line != mapped {
			out.WriteString(linedirective.Directive(p.e.sourcePath, op.line) + "\n")
			mapped = op.line
		}
		out.WriteString("\t" + indentBlock(op.call, "\t") + ",\n")
		// A nested list closed its own span before its literal ended, so the
		// element after it is unmapped whatever line it claims.
		if nested {
			mapped = 0
		}
	}
	if directives && mapped != 0 {
		out.WriteString(linedirective.Restore() + "\n")
	}
	out.WriteString("}")
	return out.String()
}

// emitComponentPlan writes the plan, builder, and entry points of one component.
func (e *goEmitter) emitComponentPlan(component *TemplateDecl) error {
	info := e.c.components[component.Name]
	e.scope = info.scope
	e.shell = info.shell
	e.boundaryRoot = e.boundaryCandidate(component)
	e.reloadable = e.c.components[component.Name].reloadable
	e.kindConst = e.c.componentGoName(component.Name) + "Kind"
	e.scopeRoot, e.scopeID, e.scopeComponent = nil, "", ""
	defer func() {
		e.scope, e.shell, e.boundaryRoot, e.reloadable = nil, false, nil, false
		e.scopeRoot, e.scopeID, e.scopeComponent = nil, "", ""
	}()
	if e.c.components[component.Name].reloadable {
		if err := e.checkReloadable(component); err != nil {
			return err
		}
	}
	if info.script != "" {
		root := boundaryRoot(component.Body.([]Node))
		if root == nil {
			return e.c.error(component.Pos, "component "+component.Name+" declares a script block and must render exactly one root element, because the marker naming its declaration lives on that element")
		}
		e.scopeRoot = root
		e.scopeID = componentKind(e.c.packageName(), e.c.filename, component.Name)
		e.scopeComponent = component.Name
	}

	params := e.c.paramsGoName(component.Name)
	prefix := e.planPrefix(component.Name)
	builder := prefix + "Ops"
	fmt.Fprintf(&e.b, "var %s = htmlbind.Builder[%s]{}\n\n", builder, params)

	scope := &emitScope{goType: params, builder: builder, paths: map[string]string{}, types: map[string]valueType{}}
	for name, t := range info.params {
		scope.paths[name] = goPublicName(name)
		scope.types[name] = t
	}
	plan := &planEmitter{e: e, scope: scope}
	e.rootScope, e.checks = scope, nil
	defer func() { e.rootScope, e.checks = nil, nil }()
	if err := e.emitOps(plan, component.Body.([]Node)); err != nil {
		return err
	}
	// transitiveHead already starts at this component, so it covers the component's
	// own contribution as well as every one it reaches.
	head, headSources := "nil", ""
	if tags := e.c.transitiveHead(component.Name); len(tags) > 0 {
		htmlParts := make([]string, 0, len(tags))
		sourceParts := make([]string, 0, len(tags))
		for _, tag := range tags {
			htmlParts = append(htmlParts, strconv.Quote(tag.html))
			sourceParts = append(sourceParts, strconv.Quote(tag.source))
		}
		head = "[]string{" + strings.Join(htmlParts, ", ") + "}"
		// Written only beside a non-empty head, so a project with no contribution
		// anywhere keeps its previous generated output byte for byte.
		headSources = "\tHeadSources: []string{" + strings.Join(sourceParts, ", ") + "},\n"
	}
	// The required set is what Head cannot be: a head entry is markup, so a
	// caller reading it gets a tag rather than something it can compare, refuse,
	// or put in a document shell ahead of a swap that will need it. It is written
	// only for a component that requires a file, so a project extracting none
	// regenerates byte for byte.
	assets := ""
	// Vary is written only for a component whose builtin elements declare an
	// axis, so a project registering none regenerates byte for byte. It exists
	// because nothing else says so: an element reading a cookie makes the whole
	// response depend on it, and the template shows a caller nothing.
	vary := ""
	if axes := e.c.transitiveVary(component.Name); len(axes) > 0 {
		parts := make([]string, 0, len(axes))
		for _, axis := range axes {
			parts = append(parts, strconv.Quote(axis))
		}
		vary = "\tVary: []string{" + strings.Join(parts, ", ") + "},\n"
	}
	if required := e.c.transitiveAssets(component.Name); len(required) > 0 {
		parts := make([]string, 0, len(required))
		for _, asset := range required {
			// Scope is written only for a component script block, so a project
			// declaring none regenerates byte for byte.
			scope := ""
			if asset.Owner != "" {
				scope = ", Scope: " + strconv.Quote(asset.Owner)
			}
			parts = append(parts, fmt.Sprintf("{ID: %s, Type: %s, URL: %s%s}",
				strconv.Quote(asset.Base), strconv.Quote(asset.MediaType()), strconv.Quote(asset.URL), scope))
		}
		assets = "\tAssets: []htmlbind.Asset{" + strings.Join(parts, ", ") + "},\n"
	}
	ops := plan.literal()
	// The flag is written only when it is true, so a project with no await
	// boundary anywhere keeps its previous generated output byte for byte. The
	// walk is over the call graph, so a component that merely calls an async one
	// carries the flag too.
	await := ""
	if e.c.reachesAwait(component.Name, map[string]bool{}) != "" {
		await = "\tHasAwaitBlock: true,\n"
	}
	// Same rule for the live flag, over the same call graph. It is a subset of
	// the await flag, so a component reporting this one always reports that one
	// too, and a project with no live boundary regenerates unchanged.
	if e.c.reachesLive(component.Name, map[string]bool{}) != "" {
		await += "\tHasLiveBlock: true,\n"
	}
	// The scope declarations, written only when something declared one, so a
	// project declaring none regenerates byte for byte. Private folds over the
	// call graph because a private component's bytes end up inside whatever
	// renders it; public stays where it was written, because asserting a subtree
	// is shared says nothing about the markup wrapped around it.
	cacheScope := ""
	if source := e.c.declaresPrivate(component.Name); source != "" {
		cacheScope = "\tDeclaresPrivate: true,\n\tPrivateSource: " + strconv.Quote(source) + ",\n"
	}
	if info.cache != nil && info.cache.public {
		cacheScope += "\tDeclaresPublic: true,\n"
	}
	// The Slots field is written only for a component that declares an html
	// parameter, so every other component keeps its previous output byte for
	// byte. It reaches the components a caller handed in, whose head the binder
	// could not otherwise see: a slot argument is a whole component sitting
	// inside a parameter struct, and reading it back out is exactly what a
	// reflection-free runtime cannot do for itself.
	slots := ""
	if accessors := e.slotAccessors(component, info); len(accessors) > 0 {
		slots = fmt.Sprintf("\tSlots: func(%s %s) []htmlbind.Fragment {\n\t\treturn []htmlbind.Fragment{%s}\n\t},\n",
			receiverIdent, params, strings.Join(accessors, ", "))
	}
	// The Check field is written only for a component with a required async
	// parameter, so every other component keeps its previous output.
	check := ""
	if len(e.checks) > 0 {
		check = fmt.Sprintf("\tCheck: func(%s %s) error {\n%s\n\t\treturn nil\n\t},\n",
			receiverIdent, params, indentBlock(strings.Join(e.checks, "\n"), "\t"))
	}
	// The Cache field is written only for a component that stores. An annotation
	// with no ttl declares scope and nothing else, so it emits the bits above and
	// no policy at all.
	cache := ""
	if info.cache.stores() {
		cache = fmt.Sprintf("\tCache: &%sCache,\n", prefix)
		if err := e.emitCachePolicy(component, prefix, params, head, ops); err != nil {
			return err
		}
	}
	// An update boundary and a redraw endpoint are two separate opt-ins, and a
	// component can be either, both, or neither.
	boundaryField := ""
	kind := componentKind(e.c.packageName(), e.c.filename, component.Name)
	if e.boundaryRoot != nil {
		e.emitBoundary(component, prefix, params, kind)
		boundaryField = fmt.Sprintf("\tBoundary: %sBoundary,\n", prefix)
	}
	if e.c.components[component.Name].reloadable {
		if err := e.emitReloadable(component, params, kind); err != nil {
			return err
		}
	}
	fmt.Fprintf(&e.b, "var %sPlan = &htmlbind.Plan[%s]{\n\tHead: %s,\n%s%s%s%s%s%s%s%s%s\tOps: %s,\n}\n\n",
		prefix, params, head, headSources, assets, vary, boundaryField, await, cacheScope, slots, check, cache, indentBlock(ops, "\t"))

	name := e.c.componentGoName(component.Name)
	fmt.Fprintf(&e.b, "// %s binds %s to its parameters, producing a renderable fragment.\n", name, component.Name)
	fmt.Fprintf(&e.b, "func %s(params %s) htmlbind.Fragment { return htmlbind.Bind(%sPlan, params) }\n\n", name, params, prefix)
	// Only an exported component is part of the composition surface, so only it
	// needs a chain binder.
	if children, ok := info.params["children"]; ok && children.kind == kindHTML && component.Exported {
		binder := "Bind" + goPublicName(name)
		fmt.Fprintf(&e.b, "// %s binds %s as a chain wrapper filling its unnamed slot.\n", binder, component.Name)
		fmt.Fprintf(&e.b, "func %s(params %s) htmlbind.Wrapper {\n", binder, params)
		fmt.Fprintf(&e.b, "\treturn htmlbind.BindWrapper(%sPlan, params, func(target *%s, children htmlbind.Fragment) { target.Children = children })\n}\n\n",
			prefix, params)
	}
	return nil
}

// slotAccessors names each html parameter of a component, in declaration order.
//
// The order is the declaration's rather than the map's, because `--check` in CI
// compares bytes and a map walk would produce a different file on every run.
func (e *goEmitter) slotAccessors(component *TemplateDecl, info *componentInfo) []string {
	var accessors []string
	for _, parameter := range info.order {
		if t, ok := info.params[parameter.Name]; !ok || t.kind != kindHTML {
			continue
		}
		accessors = append(accessors, fmt.Sprintf("%s.%s", receiverIdent, goPublicName(parameter.Name)))
	}
	return accessors
}

// emitCachePolicy writes the cache configuration of one component: its identity
// plus a fingerprint of the plan this run produced, and the generated function
// encoding its parameters into a key.
//
// Fingerprinting the emitted instruction list is what makes regenerated code
// unable to read entries written by the previous code, including changes that
// came from a nested component rather than from this template.
func (e *goEmitter) emitCachePolicy(component *TemplateDecl, prefix, params, head, ops string) error {
	info := e.c.components[component.Name]
	// Identity uses the package and the template's base name rather than the
	// path the generator was invoked with, so the same source produces the same
	// key on every machine. Template files of one package share a directory, so
	// base names are unique within it.
	identity := e.pkg + "/" + path.Base(e.c.filename) + ":" + component.Name
	id := identity + ":" + fingerprint(head+"\x00"+ops)
	var parts []string
	for _, parameter := range component.Parameters {
		t, err := e.c.resolveType(parameter.Type)
		if err != nil {
			return err
		}
		parts = append(parts, cacheKeyCall(t, receiverIdent+"."+goPublicName(parameter.Name)))
	}
	key := `""`
	if len(parts) > 0 {
		key = strings.Join(parts, " + ")
	}
	// Scoped is written only for a private component, so a public one keeps the
	// key it had before scoping existed. Private is the default, so this is the
	// line most cached components now carry.
	scoped := ""
	if !info.cache.public {
		scoped = "\tScoped: true,\n"
	}
	// A binding is not a declared parameter, so nothing above would tell two of
	// its values apart. The line is written only for a component that reaches
	// one, so a project declaring none emits exactly what it did before.
	bindings := ""
	if reached := e.c.transitiveBindings(component.Name); len(reached) > 0 {
		var framed []string
		for _, name := range reached {
			binding, _ := e.c.bindings.lookup(name)
			framed = append(framed, "htmlbind.KeyString("+bindingCall(binding)+")")
		}
		bindings = fmt.Sprintf("\tBindings: func(%s context.Context) string { return %s },\n",
			contextIdent, strings.Join(framed, " + "))
	}
	fmt.Fprintf(&e.b, "var %sCache = htmlbind.CachePolicy[%s]{\n\tID: %s,\n\tTTL: %d, // %s\n%s%s\tKey: func(%s %s) string { return %s },\n}\n\n",
		prefix, params, strconv.Quote(id), int64(info.cache.ttl), info.cache.ttl, scoped, bindings, receiverIdent, params, key)
	return nil
}

// fingerprint is a stable 64-bit FNV-1a digest rendered as hex. It only has to
// change when the generated code changes, so a non-cryptographic hash is enough.
func fingerprint(value string) string {
	const offset64 = 14695981039346656037
	const prime64 = 1099511628211
	hash := uint64(offset64)
	for _, b := range []byte(value) {
		hash ^= uint64(b)
		hash *= prime64
	}
	return strconv.FormatUint(hash, 16)
}

// cacheKeyCall encodes one value into its framed cache key contribution.
func cacheKeyCall(t valueType, code string) string {
	if t.optional {
		return "htmlbind.KeyOptional(" + code + ", " + cacheKeyEncoder(t.required()) + ")"
	}
	if t.kind == kindArray && t.elem != nil {
		return "htmlbind.KeyArray(" + code + ", " + cacheKeyEncoder(*t.elem) + ")"
	}
	return cacheKeyEncoder(t) + "(" + code + ")"
}

// cacheKeyEncoder returns a func value encoding one value of t, for the generic
// runtime helpers that take an element encoder.
func cacheKeyEncoder(t valueType) string {
	if t.optional {
		return "func(value " + goType(t) + ") string { return " + cacheKeyCall(t, "value") + " }"
	}
	switch t.kind {
	case kindBool:
		return "htmlbind.KeyBool"
	case kindInt:
		return "htmlbind.KeyInt"
	case kindFloat:
		return "htmlbind.KeyFloat"
	case kindBytes:
		return "htmlbind.KeyBytes"
	case kindDateTime, kindDate, kindTime:
		return "htmlbind.KeyTime"
	case kindURL:
		return "func(value url.URL) string { return htmlbind.KeyString(value.String()) }"
	case kindRecord:
		return cacheKeyRecordEncoder(t.name)
	case kindArray:
		return "func(value " + goType(t) + ") string { return " + cacheKeyCall(t, "value") + " }"
	default:
		// string, decimal, enums, and the trusted string types are all ~string.
		return "htmlbind.KeyString[" + goType(t) + "]"
	}
}

// cacheKeyRecordEncoder names the generated key encoder for a declared record.
func cacheKeyRecordEncoder(name string) string { return "_tinybindKey" + name }

// emitCacheKeyHelpers writes one encoder per record reachable from a cached
// component's parameters.
func (e *goEmitter) emitCacheKeyHelpers() error {
	for _, record := range e.cacheRecords {
		var parts []string
		for _, field := range e.c.records[record.name].Fields {
			t, err := e.c.resolveType(field.Type)
			if err != nil {
				return err
			}
			parts = append(parts, cacheKeyCall(t, "value."+goPublicName(field.Name)))
		}
		body := `""`
		if len(parts) > 0 {
			body = strings.Join(parts, " + ")
		}
		fmt.Fprintf(&e.b, "func %s(value %s) string { return %s }\n\n", cacheKeyRecordEncoder(record.name), goType(record), body)
	}
	return nil
}

// collectCacheRecords returns the records reachable from a cached component's
// parameters, sorted by name so emission is deterministic.
func (e *goEmitter) collectCacheRecords() []valueType {
	found := map[string]valueType{}
	var visit func(valueType)
	visit = func(t valueType) {
		base := t.required()
		if base.kind == kindArray && base.elem != nil {
			visit(*base.elem)
			return
		}
		if base.kind != kindRecord {
			return
		}
		if _, seen := found[base.name]; seen {
			return
		}
		found[base.name] = base
		for _, field := range e.c.records[base.name].Fields {
			if fieldType, err := e.c.resolveType(field.Type); err == nil {
				visit(fieldType)
			}
		}
	}
	for _, declaration := range e.c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		// Only a component that stores has a key, so only its parameters need
		// encoders. A declaring component has none — and a declaring layout's
		// html parameter has no encoding at all, which is the case that would
		// otherwise reach a walk that cannot describe it.
		if info := e.c.components[component.Name]; !info.stores() {
			continue
		}
		for _, parameter := range component.Parameters {
			if t, err := e.c.resolveType(parameter.Type); err == nil {
				visit(t)
			}
		}
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]valueType, 0, len(names))
	for _, name := range names {
		out = append(out, found[name])
	}
	return out
}

// planPrefix derives the unexported variable prefix for a component's plan.
func (e *goEmitter) planPrefix(name string) string {
	return "plan" + goPublicName(name)
}

// nodeLine reads the template line of one body node. The Node interface carries
// only NodeType, so the field every node struct already has is not reachable
// from the walk without this switch.
func nodeLine(node Node) (int, bool) {
	switch n := node.(type) {
	case *TextNode:
		return n.Pos.Line, true
	case *CommentNode:
		return n.Pos.Line, true
	case *DoctypeNode:
		return n.Pos.Line, true
	case *HeadNode:
		return n.Pos.Line, true
	case *ElementNode:
		return n.Pos.Line, true
	case *ComponentNode:
		return n.Pos.Line, true
	case *SlotNode:
		return n.Pos.Line, true
	case *messageInnerNode:
		return n.Pos.Line, true
	case *syntax.ExpressionNode:
		return n.Pos.Line, true
	case *syntax.IfNode:
		return n.Pos.Line, true
	case *syntax.ForNode:
		return n.Pos.Line, true
	case *syntax.ValNode:
		return n.Pos.Line, true
	case *syntax.CheckNode:
		return n.Pos.Line, true
	case *syntax.MessageBlockNode:
		return n.Pos.Line, true
	case *syntax.AwaitNode:
		return n.Pos.Line, true
	}
	return 0, false
}

func (e *goEmitter) emitOps(p *planEmitter, nodes []Node) error {
	// The attribution in force outside this list is restored per node rather
	// than once at the end, because a nested walk returns here between nodes.
	outer := p.line
	defer func() { p.line = outer }()
	for _, node := range nodes {
		if line, ok := nodeLine(node); ok {
			p.line = line
		}
		switch node := node.(type) {
		case *TextNode:
			p.static(node.Text)
		case *CommentNode:
			p.static("<!--" + node.Text + "-->")
		case *DoctypeNode:
			p.static("<!" + node.Text + ">")
		case *HeadNode:
			// Contributions are hoisted into the merged head.
		case *syntax.ExpressionNode:
			if err := e.emitValueOp(p, node.Expression, node.Context); err != nil {
				return err
			}
		case *syntax.IfNode:
			if err := e.emitIfOp(p, node); err != nil {
				return err
			}
		case *syntax.ForNode:
			if err := e.emitForOp(p, node); err != nil {
				return err
			}
		case *syntax.ValNode:
			if err := e.emitValOp(p, node); err != nil {
				return err
			}
		case *syntax.CheckNode:
			if err := e.emitCheckOp(p, node); err != nil {
				return err
			}
		case *syntax.MessageBlockNode:
			if err := e.emitMessageBlockOp(p, node); err != nil {
				return err
			}
		case *messageInnerNode:
			p.flush()
			p.op("MessageInner()")
		case *syntax.AwaitNode:
			if err := e.emitAwaitOp(p, node); err != nil {
				return err
			}
		case *SlotNode:
			if err := e.emitSlotOp(p, node); err != nil {
				return err
			}
		case *ElementNode:
			if err := e.emitElementOps(p, node); err != nil {
				return err
			}
		case *ComponentNode:
			if err := e.emitComponentOp(p, node); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported HTML node %T", node)
		}
	}
	return nil
}

func (e *goEmitter) emitElementOps(p *planEmitter, node *ElementNode) error {
	// A builtin element is not an element in the output at all: it was checked
	// under its own name and it lowers to whatever its definition's markup says.
	if builtin, ok := e.builtinAt(node); ok {
		return e.emitBuiltinElement(p, node, builtin)
	}
	if foreignRoot(node.Name) {
		e.foreignDepth++
		defer func() { e.foreignDepth-- }()
	}
	p.static("<" + node.Name)
	// The instance attribute goes first, so it stays in a predictable position
	// and cannot be displaced by an author attribute rendered conditionally.
	if node == e.boundaryRoot {
		p.flush()
		p.op("BoundaryAttr()")
		// A reloadable component carries its id and its kind on every render,
		// initial and redrawn alike. Without the kind on the replacement the
		// region could be redrawn exactly once.
		if e.reloadable {
			p.op(fmt.Sprintf("Attr(%q, func(p %s) (string, bool) { return htmlbind.Escape(p.%s), true })",
				"id", p.scope.goType, goPublicName(reloadableIDParameter)))
			// The kind is a hash of this very instruction list, so it cannot be
			// a literal here. A package-level constant closes the loop, since
			// Go declarations are order independent.
			p.op(fmt.Sprintf("Attr(%q, func(%s) (string, bool) { return %s, true })",
				"data-"+e.prefix+"-kind", p.scope.goType, e.kindConst))
		}
	}
	// The declaration marker of a scoped script. It is static markup rather than
	// an instruction because the identity is a compile-time constant: it costs
	// nothing per render, and unlike the instance attribute it lands on an
	// ordinary component call, which opens no boundary and would otherwise carry
	// nothing at all.
	//
	// It also lands on a render that collects nothing, so a first load — which
	// has instances and no manifest, the manifest being a header the client
	// sends back — still tells a client which elements belong to which script.
	if node == e.scopeRoot {
		p.static(` data-` + e.prefix + `-component="` + e.scopeID + `"`)
		// The instance's own arguments, for the block that marker just named.
		// Unlike the marker this is per render, which is why the caller names the
		// set rather than every parameter crossing.
		if err := e.emitComponentParameters(p, node); err != nil {
			return err
		}
	}
	// Every on-prefixed attribute on this element becomes one marker, so a client
	// runtime finds them with a single indexed query rather than by reading the
	// attributes of every element on every mount and every swap.
	if e.reservesClientHandlers() {
		if value := clientHandlerValue(node); value != "" {
			p.static(" " + e.clientHandlerAttr + `="` + escapeAttributeValue(value) + `"`)
		}
	}
	optOut := "data-" + e.c.attrPrefix + "-no-csrf"
	for _, attribute := range node.Attributes {
		// The opt-out is read at generation time and means nothing to a browser,
		// so it does not travel.
		if attribute.Name == optOut {
			continue
		}
		// The authored attribute is never emitted, exactly as server-action is
		// never emitted; the marker written above replaced it.
		if e.reservesClientHandlers() && isClientHandlerName(attribute.Name) {
			continue
		}
		if err := e.emitAttributeOp(p, node, attribute); err != nil {
			return err
		}
	}
	if node.SelfClosing {
		p.static(" />")
		return nil
	}
	p.static(">")
	// The selector says which handler a native submit is for, since the form
	// posts to the page rather than to the handler's own address.
	e.emitActionFormFields(p, node)
	// The field goes first, so a later field of the same name cannot displace it
	// and an author reading the output finds it in one place.
	if unsafe, err := e.c.unsafeForm(node); err != nil {
		return err
	} else if unsafe {
		p.flush()
		p.op("CSRFField(" + strconv.Quote(e.c.csrfFieldName()) + ")")
	}
	if err := e.emitOps(p, node.Children); err != nil {
		return err
	}
	// The shell's head element is where every chain member's contributions land.
	if node.Name == "head" && e.shell {
		p.flush()
		p.op("MergedHead()")
	}
	if !voidElements[node.Name] {
		p.static("</" + node.Name + ">")
	}
	return nil
}

func (e *goEmitter) emitAttributeOp(p *planEmitter, node *ElementNode, attribute Attribute) error {
	if attribute.Name == ServerActionAttr {
		return e.emitServerAction(p, node, attribute)
	}
	if attribute.Boolean {
		p.static(" " + attribute.Name)
		return nil
	}
	if !attributeHasExpression(attribute) {
		value := staticAttributeValue(attribute)
		if attribute.Name == "class" {
			value = e.scopedClassList(value)
		}
		p.static(" " + attribute.Name + `="` + value + `"`)
		return nil
	}
	// A single boolean expression toggles the bare attribute name.
	if len(attribute.Value) == 1 && attribute.Value[0].Expression != nil {
		t := e.c.exprTypes[attribute.Value[0].Expression]
		if t.required().kind == kindBool {
			code, err := e.exprCode(attribute.Value[0].Expression, p.scope)
			if err != nil {
				return err
			}
			condition := code
			if t.optional {
				condition = "(" + code + " != nil && *(" + code + "))"
			}
			p.flush()
			withContext := e.usesRenderContext(attribute.Value[0].Expression)
			p.op(fmt.Sprintf("%s(%s, func(%s) bool { return %s })",
				ctxOp("BoolAttr", withContext), strconv.Quote(attribute.Name),
				closureParams(p.scope.goType, withContext), condition))
			return nil
		}
	}
	// A URL-bearing attribute is assembled unescaped and handed to the op, which
	// applies the render's scheme policy and escapes the result. The escaping
	// cannot stay in this closure, because the policy is a render option and the
	// closure receives only the parameters.
	listShape, isList := isURLListAttribute(attribute.Name)
	urlBearing := isURLAttribute(attribute.Name) || isList
	value, optional, err := e.attributeValueCode(attribute, p.scope, urlBearing)
	if err != nil {
		return err
	}
	p.flush()
	withContext := e.attributeUsesRenderContext(attribute)
	switch {
	case isList:
		p.op(fmt.Sprintf("%s(%s, htmlbind.URLList%s, func(%s) (string, bool) { %s })",
			ctxOp("URLListAttr", withContext), strconv.Quote(attribute.Name),
			listShapeConst(listShape),
			closureParams(p.scope.goType, withContext), optional(value)))
	case urlBearing:
		p.op(fmt.Sprintf("%s(%s, func(%s) (string, bool) { %s })",
			ctxOp("URLAttr", withContext), strconv.Quote(attribute.Name),
			closureParams(p.scope.goType, withContext), optional(value)))
	default:
		p.op(fmt.Sprintf("%s(%s, func(%s) (string, bool) { %s })",
			ctxOp("Attr", withContext), strconv.Quote(attribute.Name),
			closureParams(p.scope.goType, withContext), optional(value)))
	}
	return nil
}

func listShapeConst(shape string) string {
	if shape == "srcset" {
		return "Srcset"
	}
	return "Space"
}

// emitServerAction replaces the reserved attribute with the one carrying the
// handler's endpoint. The URL is a compile-time constant, because the direct
// entry point holds no path parameter, so the whole lowering is static text.
//
// On a form it also writes the method, so a native submit is a POST to the page
// rather than a GET carrying the fields in the query string. The action
// attribute is deliberately not written: a form declaring none submits to the
// document URL, which is already the page pattern, and a POST keeps that URL's
// query rather than replacing it. The hidden selector that names the handler is
// written by [goEmitter.emitActionFormFields], once the start tag is closed.
func (e *goEmitter) emitServerAction(p *planEmitter, node *ElementNode, attribute Attribute) error {
	name, _ := staticAttributeText(attribute)
	// A refused name is one the caller resolved and then declined, so it is
	// reported as what it is rather than as unknown. Leaving it merely absent
	// would name a missing registration, which is the wrong cause and points an
	// author at the wrong fix.
	if reason, refused := e.refusedActions[name]; refused {
		return e.c.error(attribute.Pos, "server action "+quoteName(name)+" cannot be reached from a template: "+reason)
	}
	url, ok := e.actions[name]
	if !ok && e.resolveAction != nil {
		url, ok = e.resolveAction(name)
	}
	if !ok {
		where := "it must be an exported handler in the Go package beside this template"
		if e.resolveAction != nil {
			// With a resolver configured the handler may legitimately live
			// anywhere, so the message names both sources rather than asserting
			// the one that happens to be built in.
			where = "no exported handler in the Go package beside this template declares it, and the configured resolver did not answer for it"
		}
		return e.c.error(attribute.Pos, "no server action was resolved for "+quoteName(name)+"; "+where)
	}
	p.static(" " + e.actionAttr + `="` + escapeAttributeValue(url) + `"`)
	// An author-written method is already post, because analyzeServerAction
	// refuses every other value, so writing a second one would only duplicate it.
	if node.Name == "form" && e.actionSelector(name) != "" && !hasAttribute(node, "method") {
		p.static(` method="post"`)
	}
	return nil
}

// reservesClientHandlers reports whether the component being emitted is one the
// on- namespace is reserved in, which is a component declaring a script block.
// Outside one the attribute was never analyzed and keeps its ordinary meaning,
// so emission has to make the same choice analysis did.
func (e *goEmitter) reservesClientHandlers() bool { return e.scopeID != "" }

// actionSelector returns the opaque value a native submit carries to name the
// handler. An empty result means the caller resolved an address but no selector,
// which is the framework case of requirement:external-action-resolution: that
// framework owns the route a form would post to, so this module writes no form
// markup for it.
func (e *goEmitter) actionSelector(name string) string {
	return e.actionSelectors[name]
}

// emitActionFormFields writes the hidden fields a native submit needs, directly
// after the form's start tag so an author reading the output finds them in one
// place and a later field of the same name cannot displace them.
func (e *goEmitter) emitActionFormFields(p *planEmitter, node *ElementNode) {
	if node.Name != "form" {
		return
	}
	name, ok := serverActionName(node)
	if !ok {
		return
	}
	selector := e.actionSelector(name)
	if selector == "" {
		return
	}
	p.static(`<input type="hidden" name="` + escapeAttributeValue(e.actionSelectorField) +
		`" value="` + escapeAttributeValue(selector) + `" />`)
}

// attributeValueEscaper makes a caller-supplied URL safe in a double-quoted
// attribute. A generated endpoint contains none of these characters, but the
// prefix is configurable and therefore not ours to trust.
var attributeValueEscaper = strings.NewReplacer(
	"&", "&amp;",
	"\"", "&#34;",
	"<", "&lt;",
	">", "&gt;",
)

func escapeAttributeValue(value string) string { return attributeValueEscaper.Replace(value) }

// attributeValueCode builds the attribute value and the body that reports
// whether it is present.
//
// raw leaves the value unescaped for the caller to escape after inspecting it,
// which is what a URL-bearing attribute needs: the scheme has to be read before
// the ampersands and quotes are encoded, and a static prefix concatenated ahead
// of an expression is part of the URL the browser resolves rather than text
// around it.
func (e *goEmitter) attributeValueCode(attribute Attribute, scope *emitScope, raw bool) (string, func(string) string, error) {
	escaped := func(code string, t valueType) string {
		if raw || escapeExempt(t) {
			return code
		}
		return "htmlbind.Escape(" + code + ")"
	}
	if len(attribute.Value) == 1 && attribute.Value[0].Expression != nil {
		expr := attribute.Value[0].Expression
		t := e.c.exprTypes[expr]
		code, err := e.exprCode(expr, scope)
		if err != nil {
			return "", nil, err
		}
		if t.optional {
			value := escaped(valueString("*("+code+")", t.required()), t)
			return value, func(v string) string {
				return "if " + code + " == nil { return \"\", false }; return " + v + ", true"
			}, nil
		}
		return escaped(valueString(code, t), t), func(v string) string {
			return "return " + v + ", true"
		}, nil
	}
	var parts []string
	for index, part := range attribute.Value {
		if part.Expression == nil {
			text := part.Text
			if attribute.Name == "class" {
				text = e.scopedClassList(text)
			}
			// A separator immediately before a path segment is written by the
			// segment helper instead, because collapsing it is what an empty
			// segment has to do. Emitting it here too would double it.
			if e.segmentConsumesTrailingSlash(attribute, index) {
				text = strings.TrimSuffix(text, "/")
			}
			parts = append(parts, strconv.Quote(text))
			continue
		}
		code, err := e.exprCode(part.Expression, scope)
		if err != nil {
			return "", nil, err
		}
		if prefix, ok := e.pathSegmentPrefix(attribute, index); ok {
			// collapse only where something follows: "/{seg}" with an empty seg
			// is the root, not the empty string.
			collapse := index < len(attribute.Value)-1
			parts = append(parts, fmt.Sprintf("htmlbind.URLPathSegment(%s, %s, %t)",
				strconv.Quote(prefix), code, collapse))
			continue
		}
		parts = append(parts, escaped(valueString(code, e.c.exprTypes[part.Expression]), e.c.exprTypes[part.Expression]))
	}
	if len(parts) == 0 {
		parts = append(parts, `""`)
	}
	return strings.Join(parts, " + "), func(v string) string { return "return " + v + ", true" }, nil
}

// pathSegmentPrefix reports whether the part at index is a path-segment binding
// in a URL attribute, and the separator the helper takes over writing.
func (e *goEmitter) pathSegmentPrefix(attribute Attribute, index int) (string, bool) {
	part := attribute.Value[index]
	if part.Expression == nil || !isURLAttribute(attribute.Name) || !e.c.isPathSegmentRead(part.Expression) {
		return "", false
	}
	if index == 0 {
		return "", true
	}
	previous := attribute.Value[index-1]
	if previous.Expression != nil || !strings.HasSuffix(previous.Text, "/") {
		return "", true
	}
	return "/", true
}

// segmentConsumesTrailingSlash reports whether the static part at index ends in
// a separator the next part's segment helper writes instead.
func (e *goEmitter) segmentConsumesTrailingSlash(attribute Attribute, index int) bool {
	if index+1 >= len(attribute.Value) {
		return false
	}
	prefix, ok := e.pathSegmentPrefix(attribute, index+1)
	return ok && prefix == "/"
}

func (e *goEmitter) emitValueOp(p *planEmitter, expr Expr, context string) error {
	t := e.c.exprTypes[expr]
	code, err := e.exprCode(expr, p.scope)
	if err != nil {
		return err
	}
	withContext := e.usesRenderContext(expr)
	if t.kind == kindHTML {
		p.flush()
		p.op(fmt.Sprintf("%s(func(%s) htmlbind.Fragment { return %s }, nil)",
			ctxOp("Slot", withContext), closureParams(p.scope.goType, withContext), code))
		return nil
	}
	// JsonForScript is lowered rather than called: the encoding is generated per
	// argument type, so the argument has to be reached through the call. A
	// script_json value arriving any other way — a parameter, a record field, an
	// external result — is already encoded, so it falls through to the raw path
	// below rather than being unwrapped.
	if context == "html:script" && t.kind == kindScriptJSON {
		if call, ok := jsonForScriptCall(expr); ok {
			argument := call.Arguments[0]
			argCode, err := e.exprCode(argument, p.scope)
			if err != nil {
				return err
			}
			p.flush()
			p.op(fmt.Sprintf("%s(func(%s) string { return %s })",
				ctxOp("Raw", withContext), closureParams(p.scope.goType, withContext),
				jsonEncodeCall(e.c.exprTypes[argument], argCode)))
			return nil
		}
	}
	raw := t.required().kind == kindTrustedHTML || t.required().kind == kindTrustedCSS || t.required().kind == kindTrustedJS || t.required().kind == kindScriptJSON
	kind := "Text"
	if raw || escapeExempt(t) {
		// escapeExempt values render byte-identically through Raw, minus the
		// escaping scan.
		kind = "Raw"
	}
	body := "return " + valueString(code, t)
	if t.optional {
		body = "if " + code + " == nil { return \"\" }; return " + valueString("*("+code+")", t.required())
	}
	p.flush()
	p.op(fmt.Sprintf("%s(func(%s) string { %s })",
		ctxOp(kind, withContext), closureParams(p.scope.goType, withContext), body))
	return nil
}

// jsonForScriptCall reports whether an expression is a direct JsonForScript
// call, which is the only form whose argument the emitter can encode.
func jsonForScriptCall(expr Expr) (*CallExpr, bool) {
	call, ok := expr.(*CallExpr)
	if !ok || len(call.Arguments) != 1 {
		return nil, false
	}
	identifier, ok := call.Callee.(*IdentifierExpr)
	if !ok || identifier.Name != "JsonForScript" {
		return nil, false
	}
	return call, true
}

func (e *goEmitter) emitIfOp(p *planEmitter, node *syntax.IfNode) error {
	condition, err := e.exprCode(node.Condition, p.scope)
	if err != nil {
		return err
	}
	then := &planEmitter{e: e, scope: p.scope}
	if err := e.emitOps(then, node.Then); err != nil {
		return err
	}
	otherwise := &planEmitter{e: e, scope: p.scope}
	if err := e.emitOps(otherwise, node.Else); err != nil {
		return err
	}
	p.flush()
	withContext := e.usesRenderContext(node.Condition)
	p.op(fmt.Sprintf("%s(func(%s) bool { return %s },\n%s,\n%s)",
		ctxOp("If", withContext), closureParams(p.scope.goType, withContext), condition,
		indentBlock(then.literal(), "\t"), indentBlock(otherwise.literal(), "\t")))
	return nil
}

// emitMessageBlockOp lowers a rich-text message. The catalog decides the order
// of the text and the holes at render time, so the template contributes one ops
// list per hole and this module interleaves them; the text runs go through the
// ordinary escaper on the way out.
//
// See .knowledge decision:message-hole-lowering.
func (e *goEmitter) emitMessageBlockOp(p *planEmitter, node *syntax.MessageBlockNode) error {
	symbol, ok := e.c.messages[node.Message.ID]
	if !ok {
		return fmt.Errorf("unknown message %s", node.Message.ID)
	}
	var holes []string
	for _, hole := range node.Holes {
		// The bound element is written empty and its content comes from the
		// translation, so the children position becomes the inner-text op.
		bound := make([]Node, 0, len(hole.Nodes))
		for _, boundNode := range hole.Nodes {
			element, ok := boundNode.(*ElementNode)
			if !ok {
				bound = append(bound, boundNode)
				continue
			}
			filled := *element
			filled.Children = []Node{&messageInnerNode{Pos: element.Pos}}
			filled.SelfClosing = false
			bound = append(bound, &filled)
		}
		holeEmitter := &planEmitter{e: e, scope: p.scope}
		if err := e.emitOps(holeEmitter, bound); err != nil {
			return err
		}
		holes = append(holes, fmt.Sprintf("{Name: %s, Ops: %s}",
			strconv.Quote(hole.Name), indentBlock(holeEmitter.literal(), "\t")))
	}
	call := symbol.Name
	if alias := messageAlias(symbol); alias != "" {
		call = alias + "." + symbol.Name
	}
	var args []string
	if binding, ok := e.c.bindings.lookup(e.c.messageContextBinding); ok {
		args = append(args, bindingCall(binding))
	}
	byName := map[string]MessageArg{}
	for _, arg := range node.Message.Args {
		byName[arg.Name] = arg
	}
	for _, name := range symbol.Params {
		code, err := e.exprCode(byName[name].Value, p.scope)
		if err != nil {
			return err
		}
		args = append(args, code)
	}
	p.flush()
	p.op(fmt.Sprintf("Message(func(%s context.Context, %s %s) []htmlbind.MessageSegment { return %s(%s) },\n[]htmlbind.MessageHoleOps[%s]{\n%s,\n})",
		contextIdent, receiverIdent, p.scope.goType, call, strings.Join(args, ", "),
		p.scope.goType, strings.Join(holes, ",\n")))
	return nil
}

func (e *goEmitter) emitForOp(p *planEmitter, node *syntax.ForNode) error {
	iterable, err := e.exprCode(node.Iterable, p.scope)
	if err != nil {
		return err
	}
	elem := *e.c.exprTypes[node.Iterable].elem
	e.scopeCount++
	scopeType := fmt.Sprintf("%sScope%d", p.scope.builder, e.scopeCount)
	builder := scopeType + "Ops"
	inner := p.scope.child(scopeType, builder)
	inner.paths[node.Variable] = "Item"
	inner.types[node.Variable] = elem
	if node.Index != "" {
		inner.paths[node.Index] = "Index"
		inner.types[node.Index] = valueType{kind: kindInt}
	}
	fmt.Fprintf(&e.declarations, "type %s struct {\n\tOuter %s\n\tItem %s\n\tIndex int\n}\n\n", scopeType, p.scope.goType, goType(elem))
	fmt.Fprintf(&e.declarations, "var %s = htmlbind.Builder[%s]{}\n\n", builder, scopeType)

	body := &planEmitter{e: e, scope: inner}
	if err := e.emitOps(body, node.Body); err != nil {
		return err
	}
	p.flush()
	withContext := e.usesRenderContext(node.Iterable)
	p.raw(fmt.Sprintf("htmlbind.%s(\n\tfunc(%s) []%s { return %s },\n\tfunc(%s %s, item %s, index int) %s { return %s{Outer: %s, Item: item, Index: index} },\n%s)",
		ctxOp("For", withContext), closureParams(p.scope.goType, withContext), goType(elem), iterable,
		receiverIdent, p.scope.goType, goType(elem), scopeType, scopeType, receiverIdent,
		indentBlock(body.literal(), "\t")))
	return nil
}

// emitValOp writes one value binding. It is emitForOp without the iteration:
// the value is computed once, a generated scope struct carries it beside the
// enclosing parameters, and the body reads it as a statically typed field
// rather than as a lookup.
//
// Normalization split a node binding several names into one node per name, so
// the loop below runs once in practice; it is written to nest anyway, because a
// binding that reads an earlier one has to see it in an enclosing scope and
// nesting is what puts it there.
func (e *goEmitter) emitValOp(p *planEmitter, node *syntax.ValNode) error {
	return e.emitValBinding(p, node, 0)
}

func (e *goEmitter) emitValBinding(p *planEmitter, node *syntax.ValNode, index int) error {
	if index == len(node.Bindings) {
		return e.emitOps(p, node.Body)
	}
	binding := node.Bindings[index]
	value, err := e.exprCode(binding.Value, p.scope)
	if err != nil {
		return err
	}
	t := e.c.exprTypes[binding.Value]
	e.scopeCount++
	scopeType := fmt.Sprintf("%sVal%d", p.scope.builder, e.scopeCount)
	builder := scopeType + "Ops"
	inner := p.scope.child(scopeType, builder)
	field := goPublicName(binding.Name)
	inner.paths[binding.Name] = field
	inner.types[binding.Name] = t
	fmt.Fprintf(&e.declarations, "type %s struct {\n\tOuter %s\n\t%s %s\n}\n\n", scopeType, p.scope.goType, field, goType(t))
	fmt.Fprintf(&e.declarations, "var %s = htmlbind.Builder[%s]{}\n\n", builder, scopeType)

	body := &planEmitter{e: e, scope: inner}
	if err := e.emitValBinding(body, node, index+1); err != nil {
		return err
	}
	p.flush()
	withContext := e.usesRenderContext(binding.Value)
	// A failing external returns its error alongside the value, so the closure
	// hands both back and the instruction decides. Analysis has already confined
	// such a call to this position, so the shape of the closure is decided by
	// the outermost call alone.
	op, results, returns := ctxOp("Val", withContext), goType(t), "return "+value
	if e.failingCall(binding.Value) {
		op, results, returns = ctxOp("ValErr", withContext), "("+goType(t)+", error)", "return "+value
	}
	p.raw(fmt.Sprintf("htmlbind.%s(\n\tfunc(%s) %s { %s },\n\tfunc(%s %s, value %s) %s { return %s{Outer: %s, %s: value} },\n%s)",
		op, closureParams(p.scope.goType, withContext), results, returns,
		receiverIdent, p.scope.goType, goType(t), scopeType, scopeType, receiverIdent, field,
		indentBlock(body.literal(), "\t")))
	return nil
}

// emitCheckOp lowers one check directive to a Require instruction: it runs
// where it stands, writes nothing, and ends the render when the call fails.
//
// The instruction needs no scope struct and no body, because the directive binds
// no name. Normalization has already put it at the top of its block, which is
// what leaves the response status free for the error to choose.
func (e *goEmitter) emitCheckOp(p *planEmitter, node *syntax.CheckNode) error {
	call, err := e.exprCode(node.Call, p.scope)
	if err != nil {
		return err
	}
	// A declared result is dropped on the floor. The template asked whether the
	// call failed, so the error is taken and the value it came with is not.
	body := "return " + call
	if e.c.exprTypes[node.Call].kind != kindNone {
		body = "_, err := " + call + "; return err"
	}
	p.flush()
	withContext := e.usesRenderContext(node.Call)
	p.op(fmt.Sprintf("%s(func(%s) error { %s })",
		ctxOp("Require", withContext), closureParams(p.scope.goType, withContext), body))
	return nil
}

// failingCall reports whether expr is a call to a synchronous external that
// returns an error. Only the outermost call can be one, because analysis refuses
// a failing call anywhere else, so this looks no deeper.
func (e *goEmitter) failingCall(expr Expr) bool {
	call, ok := expr.(*CallExpr)
	if !ok {
		return false
	}
	identifier, ok := call.Callee.(*IdentifierExpr)
	if !ok || !e.errorExternals[identifier.Name] {
		return false
	}
	signature, known := e.c.externals[identifier.Name]
	return known && !signature.async && !signature.live
}

// awaitSourceName renders a binding's source the way the template wrote it, so
// an unset value is reported by the name the caller knows it by rather than by
// its generated field path. It falls back to the bound name for a source that
// is not a plain path.
func awaitSourceName(source Expr, bound string) string {
	var path func(Expr) string
	path = func(expr Expr) string {
		switch expr := expr.(type) {
		case *IdentifierExpr:
			return expr.Name
		case *MemberExpr:
			if object := path(expr.Object); object != "" {
				return object + "." + expr.Member
			}
		}
		return ""
	}
	if name := path(source); name != "" {
		return name
	}
	return bound
}

// emitAwaitOp writes one boundary. The primary and recover subtrees each get a
// generated scope struct, so the bound results and the error stay statically
// typed instead of becoming lookups.
func (e *goEmitter) emitAwaitOp(p *planEmitter, node *syntax.AwaitNode) error {
	e.scopeCount++
	scopeType := fmt.Sprintf("%sAwait%d", p.scope.builder, e.scopeCount)
	recoverType := scopeType + "Recover"
	primary := p.scope.child(scopeType, scopeType+"Ops")
	recovery := p.scope.child(recoverType, recoverType+"Ops")

	// live boundaries bind at least one source that keeps delivering, so their
	// bindings become subscription pumps rather than tasks joined once. A
	// settle-once binding in such a clause is a pump that delivers once and
	// returns, which is what lets one clause mix the two.
	live := e.c.liveBoundaries[node]
	// The bindings run concurrently and each assigns its own scope field, so
	// they share no memory and need no lock.
	var fields, tasks, checks []string
	// liveBindings holds one subscription pump per binding. Each writes its own
	// scope field and the boundary re-renders whenever any of them moves, so
	// nothing has to select between them.
	var liveBindings []string
	// pump wraps one binding's body in the LiveBinding shape.
	pump := func(body string) string {
		return fmt.Sprintf("func(deliver func(func(*%s), error) bool) error {\n%s\n}", scopeType, body)
	}
	for _, binding := range node.Bindings {
		t := e.c.exprTypes[binding.Call].awaited()
		field := goPublicName(binding.Name)
		primary.paths[binding.Name] = field
		primary.types[binding.Name] = t
		fields = append(fields, "\t"+field+" "+goType(t))
		call, isCall := binding.Call.(*CallExpr)
		if !isCall {
			// A value the caller started. Waiting for it is the same join as a
			// call, so it settles beside one in the same clause and the first
			// failure in declaration order still decides the boundary.
			//
			// The unset check below runs as a plan-level Require, which is the
			// one instruction with no context-carrying form: it exists to fail
			// before anything is written, and a check that could call out is a
			// different thing. Naming a context-taking external here is
			// therefore a generation error rather than a broken call.
			if e.usesRenderContext(binding.Call) {
				return e.c.error(exprPos(binding.Call),
					"an await binding over a caller-supplied value cannot call an external that takes the render context")
			}
			code, err := e.exprCode(binding.Call, p.scope)
			if err != nil {
				return err
			}
			if live {
				liveBindings = append(liveBindings, pump(fmt.Sprintf(
					"\tvalue, err := %s.Wait(ctx)\n\tdeliver(func(scope *%s) { scope.%s = value }, err)\n\treturn nil",
					code, scopeType, field)))
			} else {
				tasks = append(tasks, fmt.Sprintf("\t\tfunc() error { value, err := %s.Wait(ctx); scope.%s = value; return err },", code, field))
			}
			if !t.optional {
				// Absence is legal only where the template declared the
				// settled type optional. Everywhere else an unset handle is a
				// caller bug, and it has to be reported while the response can
				// still carry an error status.
				line := fmt.Sprintf("\tif !%s.IsSet() {\n\t\treturn htmlbind.ErrUnsetPending(%s)\n\t}", code, strconv.Quote(awaitSourceName(binding.Call, binding.Name)))
				if p.scope == e.rootScope {
					// Reachable from the parameters alone, so it becomes the
					// plan's own check and runs before this component writes
					// its first byte.
					e.checks = append(e.checks, line)
				} else {
					// Rooted in a loop item or an enclosing boundary's scope,
					// which exist only once rendering reaches them.
					checks = append(checks, line)
				}
			}
			continue
		}
		var args []string
		for _, argument := range call.Arguments {
			code, err := e.exprCode(argument, p.scope)
			if err != nil {
				return err
			}
			args = append(args, code)
		}
		// The external stays an ordinary blocking call; htmlbind.Concurrent owns
		// running it in a goroutine and joining the results. An implementation
		// that declared a leading context.Context receives the boundary's, so a
		// call that can abort gets what it needs without a second declaration
		// form in the template.
		callee := call.Callee.(*IdentifierExpr).Name
		if e.c.externals[callee].live {
			// A live external's context is mandatory rather than discovered:
			// an endless source with no context has nothing to make it return
			// when the subscription ends.
			source := fmt.Sprintf("%s(%s)", callee, strings.Join(append([]string{"ctx"}, args...), ", "))
			// The pump mirrors the source's own loop: one range, one deliver
			// per value, and the error travels beside the value exactly as the
			// sequence yields it.
			liveBindings = append(liveBindings, pump(fmt.Sprintf(
				"\tfor value, err := range %s {\n\t\tif !deliver(func(scope *%s) { scope.%s = value }, err) {\n\t\t\treturn nil\n\t\t}\n\t}\n\treturn nil",
				source, scopeType, field)))
			continue
		}
		if e.contextExternals[callee] {
			args = append([]string{"ctx"}, args...)
		}
		if live {
			// A settle-once source beside a live one: it delivers its value,
			// which satisfies the boundary's wait for every binding, and then
			// returns without holding the subscription open.
			liveBindings = append(liveBindings, pump(fmt.Sprintf(
				"\tvalue, err := %s(%s)\n\tdeliver(func(scope *%s) { scope.%s = value }, err)\n\treturn nil",
				callee, strings.Join(args, ", "), scopeType, field)))
			continue
		}
		tasks = append(tasks, fmt.Sprintf("\t\tfunc() error { value, err := %s(%s); scope.%s = value; return err },",
			callee, strings.Join(args, ", "), field))
	}
	fmt.Fprintf(&e.declarations, "type %s struct {\n\tOuter %s\n%s\n}\n\n", scopeType, p.scope.goType, strings.Join(fields, "\n"))
	fmt.Fprintf(&e.declarations, "var %sOps = htmlbind.Builder[%s]{}\n\n", scopeType, scopeType)
	fmt.Fprintf(&e.declarations, "type %s struct {\n\tOuter %s\n\tErr htmlbind.AsyncError\n}\n\n", recoverType, p.scope.goType)
	fmt.Fprintf(&e.declarations, "var %sOps = htmlbind.Builder[%s]{}\n\n", recoverType, recoverType)

	primaryOps := &planEmitter{e: e, scope: primary}
	if err := e.emitOps(primaryOps, node.Primary); err != nil {
		return err
	}
	fallbackOps := &planEmitter{e: e, scope: p.scope}
	if err := e.emitOps(fallbackOps, node.Fallback); err != nil {
		return err
	}
	handler := "nil"
	if node.HasRecover {
		if node.ErrorName != "" {
			recovery.paths[node.ErrorName] = "Err"
			recovery.types[node.ErrorName] = valueType{kind: kindError}
		}
		recoverOps := &planEmitter{e: e, scope: recovery}
		if err := e.emitOps(recoverOps, node.Recover); err != nil {
			return err
		}
		handler = recoverOps.literal()
	}

	if live {
		return e.finishLiveOp(p, liveOpParts{
			scopeType:   scopeType,
			recoverType: recoverType,
			bindings:    liveBindings,
			primary:     primaryOps.literal(),
			fallback:    fallbackOps.literal(),
			handler:     handler,
		})
	}
	// On failure the scope is discarded rather than returned: a cancelled wait
	// leaves its abandoned tasks still writing those fields.
	resolve := fmt.Sprintf("func(ctx context.Context, %s %s) (%s, error) {\n\tscope := %s{Outer: %s}\n\tif err := htmlbind.Concurrent(ctx,\n%s\n\t); err != nil {\n\t\tvar zero %s\n\t\treturn zero, err\n\t}\n\treturn scope, nil\n}",
		receiverIdent, p.scope.goType, scopeType, scopeType, receiverIdent, strings.Join(tasks, "\n"), scopeType)
	build := fmt.Sprintf("func(%s %s, err htmlbind.AsyncError) %s { return %s{Outer: %s, Err: err} }",
		receiverIdent, p.scope.goType, recoverType, recoverType, receiverIdent)
	p.flush()
	if len(checks) > 0 {
		p.raw(fmt.Sprintf("htmlbind.Require(func(%s %s) error {\n%s\n\treturn nil\n})",
			receiverIdent, p.scope.goType, strings.Join(checks, "\n")))
	}
	p.raw(fmt.Sprintf("htmlbind.Await(\n\t%s,\n\t%s,\n%s,\n%s,\n%s)",
		indentBlock(resolve, "\t"), build,
		indentBlock(primaryOps.literal(), "\t"),
		indentBlock(fallbackOps.literal(), "\t"),
		indentBlock(handler, "\t")))
	return nil
}

func (e *goEmitter) emitSlotOp(p *planEmitter, node *SlotNode) error {
	path := receiverIdent + "." + p.scope.paths[node.Parameter()]
	fallback := &planEmitter{e: e, scope: p.scope}
	if err := e.emitOps(fallback, node.Default); err != nil {
		return err
	}
	p.flush()
	p.op(fmt.Sprintf("Slot(func(%s %s) htmlbind.Fragment { return %s },\n%s)",
		receiverIdent, p.scope.goType, path, indentBlock(fallback.literal(), "\t")))
	return nil
}

func (e *goEmitter) emitComponentOp(p *planEmitter, node *ComponentNode) error {
	component := e.c.components[node.Name]
	values := map[string]string{}
	for _, argument := range node.Arguments {
		code, err := e.argumentCode(argument, p.scope)
		if err != nil {
			return err
		}
		values[argument.Name] = code
	}
	bodies, err := e.slotBodies(node)
	if err != nil {
		return err
	}
	// Each fill becomes its own plan over the caller's scope, so the callee
	// never needs to know the caller's parameter type.
	//
	// Numbering follows the callee's parameter order rather than map iteration
	// order, because generated code must be byte-identical between runs.
	fills := map[string]string{}
	for _, parameter := range component.order {
		body, filled := bodies[parameter.Name]
		if !filled {
			continue
		}
		name := parameter.Name
		e.scopeCount++
		fillPlan := fmt.Sprintf("%sFill%d", p.scope.builder, e.scopeCount)
		fill := &planEmitter{e: e, scope: p.scope}
		if err := e.emitOps(fill, body); err != nil {
			return err
		}
		fmt.Fprintf(&e.declarations, "var %sPlan = &htmlbind.Plan[%s]{Ops: %s}\n\n",
			fillPlan, p.scope.goType, indentBlock(fill.literal(), "\t"))
		fills[name] = fmt.Sprintf("htmlbind.Bind(%sPlan, %s)", fillPlan, receiverIdent)
	}
	var fields []string
	for _, parameter := range component.order {
		if code, ok := fills[parameter.Name]; ok {
			fields = append(fields, goPublicName(parameter.Name)+": "+code)
			continue
		}
		if code, ok := values[parameter.Name]; ok {
			fields = append(fields, goPublicName(parameter.Name)+": "+code)
		}
	}
	p.flush()
	withContext := false
	for _, argument := range node.Arguments {
		if e.attributeUsesRenderContext(argument) {
			withContext = true
			break
		}
	}
	p.op(fmt.Sprintf("%s(func(%s) htmlbind.Fragment { return %s(%s{%s}) })",
		ctxOp("Component", withContext), closureParams(p.scope.goType, withContext),
		e.c.componentGoName(node.Name),
		e.c.paramsGoName(node.Name), strings.Join(fields, ", ")))
	return nil
}

func (e *goEmitter) argumentCode(attribute Attribute, scope *emitScope) (string, error) {
	if attribute.Boolean {
		return "true", nil
	}
	if len(attribute.Value) == 1 && attribute.Value[0].Expression != nil {
		return e.exprCode(attribute.Value[0].Expression, scope)
	}
	var parts []string
	for _, part := range attribute.Value {
		if part.Expression == nil {
			parts = append(parts, strconv.Quote(part.Text))
			continue
		}
		code, err := e.exprCode(part.Expression, scope)
		if err != nil {
			return "", err
		}
		parts = append(parts, code)
	}
	if len(parts) == 0 {
		return `""`, nil
	}
	return strings.Join(parts, " + "), nil
}

func attributeHasExpression(attribute Attribute) bool {
	for _, part := range attribute.Value {
		if part.Expression != nil {
			return true
		}
	}
	return false
}

func staticAttributeValue(attribute Attribute) string {
	var out strings.Builder
	for _, part := range attribute.Value {
		out.WriteString(part.Text)
	}
	return out.String()
}

// indentBlock indents every line of value except a line directive, which the
// compiler recognizes only at the left margin and silently reads as an ordinary
// comment anywhere else.
func indentBlock(value, indent string) string {
	if !strings.Contains(value, "//line ") {
		return strings.ReplaceAll(value, "\n", "\n"+indent)
	}
	lines := strings.Split(value, "\n")
	for i := 1; i < len(lines); i++ {
		if linedirective.IsDirective(lines[i]) {
			continue
		}
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

// exprCode renders a template expression as Go code reading from the scope
// receiver.
func (e *goEmitter) exprCode(expr Expr, scope *emitScope) (string, error) {
	switch expr := expr.(type) {
	case *IdentifierExpr:
		if t, ok := e.c.enumMembers[expr.Name]; ok {
			return t.name + expr.Name, nil
		}
		path, ok := scope.paths[expr.Name]
		if !ok {
			// A declared binding is not in the params struct; it is read from
			// the render context through the provider the embedder named.
			if binding, declared := e.c.bindings.lookup(expr.Name); declared {
				return bindingCall(binding), nil
			}
			return "", fmt.Errorf("unknown identifier %s", expr.Name)
		}
		return receiverIdent + "." + path, nil
	case *LiteralExpr:
		switch expr.ValueKind {
		case "string":
			return strconv.Quote(expr.Value.(string)), nil
		case "bool":
			return strconv.FormatBool(expr.Value.(bool)), nil
		case "number":
			return expr.Value.(string), nil
		case "null":
			return "nil", nil
		}
	case *MemberExpr:
		object, err := e.exprCode(expr.Object, scope)
		if err != nil {
			return "", err
		}
		return object + "." + goPublicName(expr.Member), nil
	case *IndexExpr:
		object, err := e.exprCode(expr.Object, scope)
		if err != nil {
			return "", err
		}
		index, err := e.exprCode(expr.Index, scope)
		if err != nil {
			return "", err
		}
		return object + "[" + index + "]", nil
	case *CallExpr:
		callee := expr.Callee.(*IdentifierExpr)
		var args []string
		for _, argument := range expr.Arguments {
			code, err := e.exprCode(argument, scope)
			if err != nil {
				return "", err
			}
			args = append(args, code)
		}
		// An implementation that declared a leading context.Context receives the
		// render context. The enclosing closure was given it by the instruction
		// this expression is emitted into, so the name is always in scope here.
		if e.takesRenderContext(callee.Name) {
			args = append([]string{contextIdent}, args...)
		}
		if _, intrinsic := intrinsicResult(callee.Name); intrinsic {
			return args[0], nil
		}
		return callee.Name + "(" + strings.Join(args, ", ") + ")", nil
	case *MessageExpr:
		symbol, ok := e.c.messages[expr.ID]
		if !ok {
			return "", fmt.Errorf("unknown message %s", expr.ID)
		}
		// Arguments are written by name at the reference and by position in Go,
		// so the parameter list the table carries is what fixes the order.
		byName := map[string]MessageArg{}
		for _, arg := range expr.Args {
			byName[arg.Name] = arg
		}
		var args []string
		if binding, ok := e.c.bindings.lookup(e.c.messageContextBinding); ok {
			args = append(args, bindingCall(binding))
		}
		for _, name := range symbol.Params {
			code, err := e.exprCode(byName[name].Value, scope)
			if err != nil {
				return "", err
			}
			args = append(args, code)
		}
		call := symbol.Name
		if alias := messageAlias(symbol); alias != "" {
			call = alias + "." + symbol.Name
		}
		return call + "(" + strings.Join(args, ", ") + ")", nil
	case *UnaryExpr:
		operand, err := e.exprCode(expr.Operand, scope)
		if err != nil {
			return "", err
		}
		operator := expr.Operator
		if operator == "not" {
			operator = "!"
		}
		return operator + "(" + operand + ")", nil
	case *BinaryExpr:
		left, err := e.exprCode(expr.Left, scope)
		if err != nil {
			return "", err
		}
		right, err := e.exprCode(expr.Right, scope)
		if err != nil {
			return "", err
		}
		operator := expr.Operator
		if operator == "and" {
			operator = "&&"
		} else if operator == "or" {
			operator = "||"
		}
		return "(" + left + " " + operator + " " + right + ")", nil
	case *ConditionalExpr:
		condition, err := e.exprCode(expr.Condition, scope)
		if err != nil {
			return "", err
		}
		thenCode, err := e.exprCode(expr.Then, scope)
		if err != nil {
			return "", err
		}
		elseCode, err := e.exprCode(expr.Else, scope)
		if err != nil {
			return "", err
		}
		t := e.c.exprTypes[expr]
		return "(func() " + goType(t) + " { if " + condition + " { return " + thenCode + " }; return " + elseCode + " })()", nil
	}
	return "", fmt.Errorf("unsupported expression %T", expr)
}

// headTag is one contributed head tag together with the component that declared
// it. The origin is carried so a caller rejecting a contribution can name it;
// see requirement:head-contribution-provenance.
type headTag struct {
	html   string
	source string
}

// transitiveHead collects the head contributions of a component and every
// component reachable from it, because a nested call renders after the shell
// head is already written.
//
// One entry per contributed tag rather than per contributing component, because
// requirement:head-merging deduplicates on the tag and identity is per tag: two
// components sharing a stylesheet link must collapse to one even when the rest
// of their contributions differ.
func (c *compiler) transitiveHead(name string) []headTag {
	var out []headTag
	visited := map[string]bool{}
	// Identity is the tag, so two components contributing the same stylesheet
	// link collapse here and the first declarer keeps the attribution. Merging
	// across chain members deduplicates again, because a member cannot see the
	// contributions of the members it is composed with.
	emitted := map[string]bool{}
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
		for _, tag := range c.headTags(info) {
			if emitted[tag.html] {
				continue
			}
			emitted[tag.html] = true
			out = append(out, tag)
		}
		for _, called := range c.calledComponents(info) {
			visit(called)
		}
	}
	visit(name)
	return out
}

// transitiveAssets collects the static files a component and everything it calls
// require, over the same call graph as transitiveHead and for the same reason: a
// nested call's stylesheet is required by whoever renders the outer component,
// which is the only value a caller holds before rendering starts.
//
// Identity is the file, so two components referencing one stylesheet collapse
// here exactly as their tags do.
func (c *compiler) transitiveAssets(name string) []Asset {
	var out []Asset
	visited := map[string]bool{}
	emitted := map[string]bool{}
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
		// A component's own head declarations and the builtin elements it writes
		// are two sources of one requirement, so they fold into one set.
		for _, asset := range append(append([]Asset(nil), info.assets...), info.builtinAssets...) {
			if asset.Base == "" || emitted[asset.Base] {
				continue
			}
			emitted[asset.Base] = true
			out = append(out, asset)
		}
		for _, called := range c.calledComponents(info) {
			visit(called)
		}
	}
	visit(name)
	return out
}

// headTags is one component's head contribution split into its individual tags.
// Whitespace between tags is dropped, because the merged head is assembled from
// the tags rather than from the authored block. extractAssets fills the list, so
// a style or inline script block appears here as its reference tag rather than
// as the block itself.
func (c *compiler) headTags(info *componentInfo) []headTag { return info.headTags }

// headSource names the declaring component and the position of the tag, in the
// form a diagnostic can print directly.
func (c *compiler) headSource(info *componentInfo, node Node) string {
	pos := nodePosition(node)
	return fmt.Sprintf("%s (%s:%d:%d)", info.decl.Name, c.filename, pos.Line, pos.Col)
}

// nodePosition reads the source position of a static head node.
func nodePosition(node Node) Position {
	switch node := node.(type) {
	case *ElementNode:
		return node.Pos
	case *CommentNode:
		return node.Pos
	case *TextNode:
		return node.Pos
	}
	return Position{Line: 1, Col: 1}
}

// calledComponents lists the components a component's body invokes.
func (c *compiler) calledComponents(info *componentInfo) []string {
	var names []string
	seen := map[string]bool{}
	var walk func(nodes []Node)
	walk = func(nodes []Node) {
		for _, node := range nodes {
			switch node := node.(type) {
			case *ComponentNode:
				if !seen[node.Name] {
					seen[node.Name] = true
					names = append(names, node.Name)
				}
				walk(node.Children)
			case *ElementNode:
				walk(node.Children)
			case *SlotNode:
				walk(node.Default)
			case *syntax.IfNode:
				walk(node.Then)
				walk(node.Else)
			case *syntax.ForNode:
				walk(node.Body)
			case *syntax.ValNode:
				walk(node.Body)
			case *syntax.AwaitNode:
				walk(node.Primary)
				walk(node.Fallback)
				walk(node.Recover)
			}
		}
	}
	if body, ok := info.decl.Body.([]Node); ok {
		walk(body)
	}
	return names
}

// liveOpParts carries the pieces emitAwaitOp already built, so the live tail
// reads as one call rather than threading several positional arguments.
type liveOpParts struct {
	scopeType   string
	recoverType string
	bindings    []string
	primary     string
	fallback    string
	handler     string
}

// finishLiveOp writes the htmlbind.Live call. It differs from the await tail in
// what it hands the runtime: subscriptions rather than a join, and a scope the
// runtime fills field by field as deliveries arrive rather than one built once
// from a settled set of results.
func (e *goEmitter) finishLiveOp(p *planEmitter, parts liveOpParts) error {
	pumps := make([]string, len(parts.bindings))
	for index, binding := range parts.bindings {
		pumps[index] = indentBlock(binding, "\t\t") + ","
	}
	bindings := fmt.Sprintf("func(ctx context.Context, %s %s) []htmlbind.LiveBinding[%s] {\n\treturn []htmlbind.LiveBinding[%s]{\n%s\n\t}\n}",
		receiverIdent, p.scope.goType, parts.scopeType, parts.scopeType, strings.Join(pumps, "\n"))
	// The scope starts with the outer parameters alone. Each binding fills its
	// own field as it delivers, and the first render waits until every one has.
	scope := fmt.Sprintf("func(%s %s) %s { return %s{Outer: %s} }",
		receiverIdent, p.scope.goType, parts.scopeType, parts.scopeType, receiverIdent)
	build := fmt.Sprintf("func(%s %s, err htmlbind.AsyncError) %s { return %s{Outer: %s, Err: err} }",
		receiverIdent, p.scope.goType, parts.recoverType, parts.recoverType, receiverIdent)
	p.flush()
	p.raw(fmt.Sprintf("htmlbind.Live(\n%s,\n\t%s,\n\t%s,\n%s,\n%s,\n%s)",
		indentBlock(bindings, "\t"), scope, build,
		indentBlock(parts.primary, "\t"),
		indentBlock(parts.fallback, "\t"),
		indentBlock(parts.handler, "\t")))
	return nil
}
