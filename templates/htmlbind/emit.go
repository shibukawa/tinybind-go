package htmlbind

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

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

// planEmitter accumulates the instruction list of one plan.
type planEmitter struct {
	e       *goEmitter
	scope   *emitScope
	ops     []string
	pending strings.Builder
}

func (p *planEmitter) static(text string) { p.pending.WriteString(text) }

func (p *planEmitter) flush() {
	if p.pending.Len() == 0 {
		return
	}
	text := p.pending.String()
	p.pending.Reset()
	p.op("Static(" + strconv.Quote(text) + ")")
}

// op appends a builder call. Static output is flushed first so instruction
// order matches document order.
func (p *planEmitter) op(call string) {
	p.ops = append(p.ops, p.scope.builder+"."+call)
}

func (p *planEmitter) raw(call string) { p.ops = append(p.ops, call) }

// literal renders the instruction list as a Go composite literal.
func (p *planEmitter) literal() string {
	p.flush()
	if len(p.ops) == 0 {
		return "nil"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "[]htmlbind.Op[%s]{\n", p.scope.goType)
	for _, op := range p.ops {
		out.WriteString("\t" + strings.ReplaceAll(op, "\n", "\n\t") + ",\n")
	}
	out.WriteString("}")
	return out.String()
}

// emitComponentPlan writes the plan, builder, and entry points of one component.
func (e *goEmitter) emitComponentPlan(component *TemplateDecl) error {
	info := e.c.components[component.Name]
	e.scope = info.scope
	e.shell = info.shell
	defer func() { e.scope, e.shell = nil, false }()

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
	if err := e.emitOps(plan, component.Body.([]Node)); err != nil {
		return err
	}
	head := "nil"
	if fragment := renderStaticHTML(info.head); strings.TrimSpace(fragment) != "" {
		head = "[]string{" + strconv.Quote(fragment) + "}"
	}
	if transitive := e.c.transitiveHead(component.Name); len(transitive) > 0 {
		head = "[]string{" + strings.Join(transitive, ", ") + "}"
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
	// The Cache field is written only for a cached component, so the generated
	// output of every other component is unchanged.
	cache := ""
	if info.cache != nil {
		cache = fmt.Sprintf("\tCache: &%sCache,\n", prefix)
		if err := e.emitCachePolicy(component, prefix, params, head, ops); err != nil {
			return err
		}
	}
	fmt.Fprintf(&e.b, "var %sPlan = &htmlbind.Plan[%s]{\n\tHead: %s,\n%s%s\tOps: %s,\n}\n\n",
		prefix, params, head, await, cache, indentBlock(ops, "\t"))

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
	fmt.Fprintf(&e.b, "var %sCache = htmlbind.CachePolicy[%s]{\n\tID: %s,\n\tTTL: %d, // %s\n\tKey: func(%s %s) string { return %s },\n}\n\n",
		prefix, params, strconv.Quote(id), int64(info.cache.ttl), info.cache.ttl, receiverIdent, params, key)
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
		if info := e.c.components[component.Name]; info == nil || info.cache == nil {
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

func (e *goEmitter) emitOps(p *planEmitter, nodes []Node) error {
	for _, node := range nodes {
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
	p.static("<" + node.Name)
	for _, attribute := range node.Attributes {
		if err := e.emitAttributeOp(p, attribute); err != nil {
			return err
		}
	}
	if node.SelfClosing {
		p.static(" />")
		return nil
	}
	p.static(">")
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

func (e *goEmitter) emitAttributeOp(p *planEmitter, attribute Attribute) error {
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
			p.op(fmt.Sprintf("BoolAttr(%s, func(%s %s) bool { return %s })",
				strconv.Quote(attribute.Name), receiverIdent, p.scope.goType, condition))
			return nil
		}
	}
	value, optional, err := e.attributeValueCode(attribute, p.scope)
	if err != nil {
		return err
	}
	p.flush()
	p.op(fmt.Sprintf("Attr(%s, func(%s %s) (string, bool) { %s })",
		strconv.Quote(attribute.Name), receiverIdent, p.scope.goType, optional(value)))
	return nil
}

// attributeValueCode builds the escaped attribute value and the body that
// reports whether it is present.
func (e *goEmitter) attributeValueCode(attribute Attribute, scope *emitScope) (string, func(string) string, error) {
	if len(attribute.Value) == 1 && attribute.Value[0].Expression != nil {
		expr := attribute.Value[0].Expression
		t := e.c.exprTypes[expr]
		code, err := e.exprCode(expr, scope)
		if err != nil {
			return "", nil, err
		}
		if t.optional {
			value := "htmlbind.Escape(" + valueString("*("+code+")", t.required()) + ")"
			return value, func(v string) string {
				return "if " + code + " == nil { return \"\", false }; return " + v + ", true"
			}, nil
		}
		return "htmlbind.Escape(" + valueString(code, t) + ")", func(v string) string {
			return "return " + v + ", true"
		}, nil
	}
	var parts []string
	for _, part := range attribute.Value {
		if part.Expression == nil {
			text := part.Text
			if attribute.Name == "class" {
				text = e.scopedClassList(text)
			}
			parts = append(parts, strconv.Quote(text))
			continue
		}
		code, err := e.exprCode(part.Expression, scope)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, "htmlbind.Escape("+valueString(code, e.c.exprTypes[part.Expression])+")")
	}
	if len(parts) == 0 {
		parts = append(parts, `""`)
	}
	return strings.Join(parts, " + "), func(v string) string { return "return " + v + ", true" }, nil
}

func (e *goEmitter) emitValueOp(p *planEmitter, expr Expr, context string) error {
	t := e.c.exprTypes[expr]
	code, err := e.exprCode(expr, p.scope)
	if err != nil {
		return err
	}
	if t.kind == kindHTML {
		p.flush()
		p.op(fmt.Sprintf("Slot(func(%s %s) htmlbind.Fragment { return %s }, nil)", receiverIdent, p.scope.goType, code))
		return nil
	}
	if context == "html:script" && t.kind == kindScriptJSON {
		call := expr.(*CallExpr)
		argument := call.Arguments[0]
		argCode, err := e.exprCode(argument, p.scope)
		if err != nil {
			return err
		}
		p.flush()
		p.op(fmt.Sprintf("Raw(func(%s %s) string { return %s })",
			receiverIdent, p.scope.goType, jsonEncodeCall(e.c.exprTypes[argument], argCode)))
		return nil
	}
	raw := t.required().kind == kindTrustedHTML || t.required().kind == kindTrustedCSS || t.required().kind == kindTrustedJS || t.required().kind == kindScriptJSON
	kind := "Text"
	if raw {
		kind = "Raw"
	}
	body := "return " + valueString(code, t)
	if t.optional {
		body = "if " + code + " == nil { return \"\" }; return " + valueString("*("+code+")", t.required())
	}
	p.flush()
	p.op(fmt.Sprintf("%s(func(%s %s) string { %s })", kind, receiverIdent, p.scope.goType, body))
	return nil
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
	p.op(fmt.Sprintf("If(func(%s %s) bool { return %s },\n%s,\n%s)",
		receiverIdent, p.scope.goType, condition,
		indentBlock(then.literal(), "\t"), indentBlock(otherwise.literal(), "\t")))
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
	p.raw(fmt.Sprintf("htmlbind.For(\n\tfunc(%s %s) []%s { return %s },\n\tfunc(%s %s, item %s, index int) %s { return %s{Outer: %s, Item: item, Index: index} },\n%s)",
		receiverIdent, p.scope.goType, goType(elem), iterable,
		receiverIdent, p.scope.goType, goType(elem), scopeType, scopeType, receiverIdent,
		indentBlock(body.literal(), "\t")))
	return nil
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

	// The bindings run concurrently and each assigns its own scope field, so
	// they share no memory and need no lock.
	var fields, tasks []string
	for _, binding := range node.Bindings {
		call := binding.Call.(*CallExpr)
		t := e.c.exprTypes[binding.Call]
		field := goPublicName(binding.Name)
		primary.paths[binding.Name] = field
		primary.types[binding.Name] = t
		fields = append(fields, "\t"+field+" "+goType(t))
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
		if e.contextExternals[callee] {
			args = append([]string{"ctx"}, args...)
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

	// On failure the scope is discarded rather than returned: a cancelled wait
	// leaves its abandoned tasks still writing those fields.
	resolve := fmt.Sprintf("func(ctx context.Context, %s %s) (%s, error) {\n\tscope := %s{Outer: %s}\n\tif err := htmlbind.Concurrent(ctx,\n%s\n\t); err != nil {\n\t\tvar zero %s\n\t\treturn zero, err\n\t}\n\treturn scope, nil\n}",
		receiverIdent, p.scope.goType, scopeType, scopeType, receiverIdent, strings.Join(tasks, "\n"), scopeType)
	build := fmt.Sprintf("func(%s %s, err htmlbind.AsyncError) %s { return %s{Outer: %s, Err: err} }",
		receiverIdent, p.scope.goType, recoverType, recoverType, receiverIdent)
	p.flush()
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
	p.op(fmt.Sprintf("Component(func(%s %s) htmlbind.Fragment { return %s(%s{%s}) })",
		receiverIdent, p.scope.goType, e.c.componentGoName(node.Name),
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

func indentBlock(value, indent string) string {
	return strings.ReplaceAll(value, "\n", "\n"+indent)
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
		if _, intrinsic := intrinsicResult(callee.Name); intrinsic {
			return args[0], nil
		}
		return callee.Name + "(" + strings.Join(args, ", ") + ")", nil
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

// transitiveHead collects the head contributions of a component and every
// component reachable from it, because a nested call renders after the shell
// head is already written.
func (c *compiler) transitiveHead(name string) []string {
	var out []string
	seen := map[string]bool{}
	var visit func(string)
	visit = func(current string) {
		if seen[current] {
			return
		}
		seen[current] = true
		info, ok := c.components[current]
		if !ok {
			return
		}
		if fragment := renderStaticHTML(info.head); strings.TrimSpace(fragment) != "" {
			out = append(out, strconv.Quote(fragment))
		}
		for _, called := range c.calledComponents(info) {
			visit(called)
		}
	}
	visit(name)
	return out
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
