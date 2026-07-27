package htmlbind

import (
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
	return base
}

func (t valueType) required() valueType { t.optional = false; return t }

type functionSig struct {
	params []valueType
	result valueType
	// async marks a function that takes a context and returns an error. It may
	// only be called in an await binding.
	async bool
}

// cachePolicy is the validated form of a component's cache annotation.
type cachePolicy struct {
	ttl time.Duration
}

type componentInfo struct {
	decl   *TemplateDecl
	params map[string]valueType
	order  []Parameter
	// head holds the nodes contributed by head elements declared outside the
	// document shell, already scoped and ready to merge.
	head []Node
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
				sig.params = append(sig.params, t)
			}
			result, err := c.resolveType(declaration.Result)
			if err != nil {
				return err
			}
			sig.result = result
			sig.async = declaration.Async
			c.externals[declaration.Name] = sig
		case *TemplateDecl:
			if declaration.Kind != "html:component" || declaration.Output.Name != "html" {
				return c.error(declaration.Pos, "HTML generator only accepts html:component declarations")
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
		default:
			return c.error(annotation.Pos, "unknown annotation @"+annotation.Name)
		}
	}
	return nil
}

// cacheAnnotation reads @cache(ttl: "5m"). The TTL is required and parsed here,
// so a malformed duration is reported with its own position instead of failing
// at run time.
func (c *compiler) cacheAnnotation(declaration *TemplateDecl, annotation Annotation) (*cachePolicy, error) {
	for _, argument := range annotation.Args {
		if argument.Name != "ttl" {
			return nil, c.error(argument.Pos, "unknown @cache argument "+argument.Name)
		}
	}
	argument, ok := annotation.Argument("ttl")
	if !ok {
		return nil, c.error(annotation.Pos, "@cache requires a ttl argument, for example @cache(ttl: \"5m\")")
	}
	ttl, err := time.ParseDuration(argument.Value)
	if err != nil {
		return nil, c.error(argument.Pos, "@cache ttl is not a duration: "+argument.Value)
	}
	if ttl <= 0 {
		return nil, c.error(argument.Pos, "@cache ttl must be positive")
	}
	// An html parameter is a bound continuation rather than a value, so it
	// cannot take part in the cache key.
	for _, parameter := range declaration.Parameters {
		t, err := c.resolveType(parameter.Type)
		if err != nil {
			return nil, err
		}
		if t.kind == kindHTML {
			return nil, c.error(parameter.Pos, "cached component "+declaration.Name+" cannot declare the html parameter "+parameter.Name)
		}
	}
	return &cachePolicy{ttl: ttl}, nil
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
	}
	return nil
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
	return result, nil
}

func (c *compiler) analyzeNodes(nodes []syntax.Node, scope map[string]valueType) error {
	for _, node := range nodes {
		switch node := node.(type) {
		case *TextNode, *CommentNode, *DoctypeNode:
		case *syntax.ExpressionNode:
			t, err := c.infer(node.Expression, scope)
			if err != nil {
				return err
			}
			if err := c.validateInsertion(node.Context, t, exprPos(node.Expression)); err != nil {
				return err
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
			for _, attribute := range node.Attributes {
				if err := c.analyzeAttribute(node.Name, attribute, scope); err != nil {
					return err
				}
			}
			if err := c.analyzeNodes(node.Children, scope); err != nil {
				return err
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

// validateHeadChild keeps head contributions static, because the merged head
// is written before any body byte and cannot wait for request data.
func (c *compiler) validateHeadChild(node Node) error {
	switch node := node.(type) {
	case *TextNode, *CommentNode:
		return nil
	case *ElementNode:
		switch node.Name {
		case "link", "meta", "style", "script", "title":
		default:
			return c.error(node.Pos, "head contribution cannot contain "+node.Name)
		}
		for _, attribute := range node.Attributes {
			if _, static := staticAttributeText(attribute); !static && !attribute.Boolean {
				return c.error(attribute.Pos, "head contribution attributes must be static")
			}
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
		call, ok := binding.Call.(*CallExpr)
		if !ok {
			return c.error(binding.Pos, "await binding "+binding.Name+" must call an async external function")
		}
		identifier, ok := call.Callee.(*IdentifierExpr)
		if !ok {
			return c.error(binding.Pos, "await binding "+binding.Name+" must call an async external function")
		}
		sig, ok := c.externals[identifier.Name]
		if !ok {
			return c.error(binding.Pos, "unknown function "+identifier.Name)
		}
		if !sig.async {
			return c.error(binding.Pos, identifier.Name+" is not async; declare it as external async to await it")
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
		if err := c.validateInsertion("html:attribute", t, part.Pos); err != nil {
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
func isURLAttribute(name string) bool {
	switch name {
	case "href", "src", "action", "formaction", "poster":
		return true
	}
	return false
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
