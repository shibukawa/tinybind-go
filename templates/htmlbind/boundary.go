package htmlbind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// DefaultDataAttributePrefix names the generated data attributes. A project
// whose markup already uses this prefix overrides it through GenerateOptions.
const DefaultDataAttributePrefix = "tb"

// validatePrefix rejects a prefix that cannot form a data attribute name.
func validatePrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("data attribute prefix must not be empty")
	}
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-':
		default:
			return fmt.Errorf("data attribute prefix %q must use lowercase letters, digits, and hyphens", prefix)
		}
	}
	if strings.HasPrefix(prefix, "-") || strings.HasSuffix(prefix, "-") {
		return fmt.Errorf("data attribute prefix %q must not start or end with a hyphen", prefix)
	}
	return nil
}

// boundaryRoot returns the component's single root element, or nil when the
// body has no single element to carry the instance attribute.
//
// A doctype, a comment, whitespace, and a hoisted head declaration are ignored,
// because none of them can receive an attribute and none of them prevents the
// remaining element from being the boundary root. Anything else, including a
// conditional or a loop at the top level, leaves the node count unknown at
// generation time and therefore yields no boundary.
func boundaryRoot(nodes []Node) *ElementNode {
	var root *ElementNode
	for _, node := range nodes {
		switch node := node.(type) {
		case *TextNode:
			if strings.TrimSpace(node.Text) != "" {
				return nil
			}
		case *CommentNode, *DoctypeNode, *HeadNode:
		case *ElementNode:
			if root != nil {
				return nil
			}
			root = node
		default:
			return nil
		}
	}
	return root
}

// boundaryCandidate reports whether a component can become an update boundary.
//
// Only an exported component can be a chain member, and the document shell is
// excluded because partial navigation retains the shell rather than replacing
// it. A component failing the single root rule is silently not a boundary
// rather than an error: boundaries are automatic here, so a template that
// never opted in must keep compiling.
func (e *goEmitter) boundaryCandidate(component *TemplateDecl) *ElementNode {
	if !component.Exported || e.c.components[component.Name].shell {
		return nil
	}
	return boundaryRoot(component.Body.([]Node))
}

// usesBoundary reports whether any component in the module becomes an update
// boundary, which is what makes the generated code reference the delta package.
func (e *goEmitter) usesBoundary() bool {
	for _, declaration := range e.c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if ok && e.boundaryCandidate(component) != nil {
			return true
		}
	}
	return false
}

// componentKind names a component: its package, its file, and its declaration.
//
// It is deliberately not a digest. Identity and version are separate jobs, and
// a digest of the compiled plan does the second one badly: it misses a change
// in a component this one calls, in an external function, and in the render
// runtime itself. The build identity covers all of those at once, so the kind
// is free to be what it should be, which is readable, stable, and unique by
// construction rather than by collision resistance.
//
// Stability is the point. A template edit leaves the endpoint URL alone, so an
// unrelated deploy does not invalidate it, and a log line names the component
// an operator can go and open.
func componentKind(pkg, filename, name string) string {
	return pkg + "." + fileStem(filename) + "." + name
}

// fileStem is the base name with its extensions removed, so counter.tb.html
// contributes counter.
func fileStem(filename string) string {
	base := filename
	if at := strings.LastIndexAny(base, "/\\"); at >= 0 {
		base = base[at+1:]
	}
	if at := strings.Index(base, "."); at >= 0 {
		base = base[:at]
	}
	return goIdentifier(base)
}

// reloadableIDParameter is the parameter carrying the instance id an author
// writes at the call site. The framework emits it on the root element, so the
// browser can address the region and the replacement stays addressable.
const reloadableIDParameter = "id"

// checkReloadable validates a component published as a redraw endpoint.
//
// Unlike an automatic boundary, this is an explicit opt-in, so a component that
// cannot satisfy the rules is a generation error rather than silently ordinary.
func (e *goEmitter) checkReloadable(component *TemplateDecl) error {
	if !component.Exported {
		return e.c.error(component.Pos, "reloadable component "+component.Name+" must be exported, because registering it publishes an endpoint")
	}
	if e.c.components[component.Name].shell {
		return e.c.error(component.Pos, "reloadable component "+component.Name+" cannot own the document head")
	}
	if boundaryRoot(component.Body.([]Node)) == nil {
		return e.c.error(component.Pos, "reloadable component "+component.Name+" must render exactly one root element, because its id and kind live on that element")
	}
	var hasID bool
	for _, parameter := range component.Parameters {
		t, err := e.c.resolveType(parameter.Type)
		if err != nil {
			return err
		}
		if parameter.Name == reloadableIDParameter {
			if t.kind != kindString || t.optional {
				return e.c.error(parameter.Pos, "reloadable component "+component.Name+" must declare id as a required string")
			}
			hasID = true
			continue
		}
		if !queryDecodable(t) {
			return e.c.error(parameter.Pos, "reloadable component "+component.Name+" cannot accept parameter "+parameter.Name+
				" of type "+t.String()+", because a redraw carries every argument in the query string")
		}
	}
	if !hasID {
		return e.c.error(component.Pos, "reloadable component "+component.Name+" must declare an id parameter naming the instance")
	}
	return nil
}

// queryDecodable reports whether a value can travel in a query string
// deterministically. A record, a slice, and html cannot.
func queryDecodable(t valueType) bool {
	if t.kind == kindArray || t.kind == kindRecord || t.kind == kindHTML || t.async {
		return false
	}
	switch t.kind {
	case kindString, kindBool, kindInt, kindFloat, kindDecimal, kindEnum, kindURL, kindDateTime, kindDate, kindTime:
		return true
	}
	return false
}

// emitReloadable writes the typed query decoder and the registration value.
func (e *goEmitter) emitReloadable(component *TemplateDecl, params, kind string) error {
	name := e.c.componentGoName(component.Name)
	fmt.Fprintf(&e.b, "// %sReloadable publishes %s as a redraw endpoint.\n", name, component.Name)
	e.b.WriteString("// Its parameters arrive from the caller, so it authorizes them itself.\n")
	fmt.Fprintf(&e.b, "const %sKind = %s\n\n", name, strconv.Quote(kind))
	fmt.Fprintf(&e.b, "var %sReloadable = htmlupdate.Reloadable{\n\tKindID: %sKind,\n", name, name)
	e.b.WriteString("\tRender: func(r *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error) {\n")
	fmt.Fprintf(&e.b, "\t\tvar params %s\n\t\tparams.%s = instanceID\n", params, goPublicName(reloadableIDParameter))
	for _, parameter := range component.Parameters {
		if parameter.Name == reloadableIDParameter {
			continue
		}
		t, err := e.c.resolveType(parameter.Type)
		if err != nil {
			return err
		}
		fmt.Fprintf(&e.b, "\t\tif err := %s(values, %s, &params.%s); err != nil {\n\t\t\treturn htmlbind.Fragment{}, err\n\t\t}\n",
			queryDecoder(t), strconv.Quote(parameter.Name), goPublicName(parameter.Name))
	}
	fmt.Fprintf(&e.b, "\t\treturn %s(params), nil\n\t},\n", name)
	// A redraw swaps markup into a page this endpoint never rendered, so unlike
	// a navigation it cannot merge into a head it owns. Publishing what the
	// component contributes is what lets a caller put it in the document shell
	// before anything is swapped, and lets the response install it if the caller
	// did not. Both fields are written only when there is something to say, so a
	// component contributing nothing regenerates byte for byte.
	if tags := e.c.transitiveHead(component.Name); len(tags) > 0 {
		parts := make([]string, 0, len(tags))
		for _, tag := range tags {
			parts = append(parts, strconv.Quote(tag.html))
		}
		fmt.Fprintf(&e.b, "\tHead: []string{%s},\n", strings.Join(parts, ", "))
	}
	if required := e.c.transitiveAssets(component.Name); len(required) > 0 {
		parts := make([]string, 0, len(required))
		for _, asset := range required {
			parts = append(parts, fmt.Sprintf("{ID: %s, Type: %s, URL: %s}",
				strconv.Quote(asset.Base), strconv.Quote(asset.MediaType()), strconv.Quote(asset.URL)))
		}
		fmt.Fprintf(&e.b, "\tAssets: []htmlbind.Asset{%s},\n", strings.Join(parts, ", "))
	}
	e.b.WriteString("}\n\n")
	return nil
}

// queryDecoder names the runtime helper decoding one declared type.
func queryDecoder(t valueType) string {
	if t.optional {
		return "htmlupdate.QueryOptional[" + goType(t.required()) + "]"
	}
	switch t.kind {
	case kindBool:
		return "htmlupdate.QueryBool"
	case kindInt:
		return "htmlupdate.QueryInt"
	case kindFloat:
		return "htmlupdate.QueryFloat"
	case kindURL:
		return "htmlupdate.QueryURL"
	case kindDateTime, kindDate, kindTime:
		return "htmlupdate.QueryTime"
	default:
		// string, decimal, and enums are all ~string.
		return "htmlupdate.QueryString[" + goType(t) + "]"
	}
}

// emitBoundary writes the canonical input encoder and the boundary declaration
// of one component.
func (e *goEmitter) emitBoundary(component *TemplateDecl, prefix, params, kind string) {
	fmt.Fprintf(&e.b, "// %sInput canonically encodes the declared inputs of %s.\n", prefix, component.Name)
	fmt.Fprintf(&e.b, "// Slot arguments are excluded: their content belongs to the child boundary,\n")
	fmt.Fprintf(&e.b, "// so a frame stays comparable when only its child changed.\n")
	fmt.Fprintf(&e.b, "func %sInput(p %s) string {\n\treturn delta.CanonJoin(\n", prefix, params)
	for _, parameter := range component.Parameters {
		t, err := e.c.resolveType(parameter.Type)
		// A slot argument belongs to the child boundary, and an async parameter
		// is a handle rather than a value: neither is an input this component
		// can be compared by. What either one renders shows up in the frame
		// validator instead.
		if err != nil || t.required().kind == kindHTML || t.async {
			continue
		}
		fmt.Fprintf(&e.b, "\t\t%s,\n", canonEncodeCall(t, "p."+goPublicName(parameter.Name)))
	}
	e.b.WriteString("\t)\n}\n\n")
	// A reloadable component names its own instance, from the id an author
	// writes at the call site. That is what makes it an update boundary
	// wherever it renders, so a navigation delta can compare the region a
	// redraw can replace rather than discovering it changed only by replacing
	// an ancestor. Every other candidate leaves it nil and is numbered by its
	// chain position instead, which keeps an ordinary component call out of the
	// manifest.
	instance := ""
	if e.reloadable {
		instance = fmt.Sprintf("\tInstance: func(p %s) string { return p.%s },\n",
			params, goPublicName(reloadableIDParameter))
	}
	fmt.Fprintf(&e.b, "var %sBoundary = &htmlbind.Boundary[%s]{\n", prefix, params)
	fmt.Fprintf(&e.b, "\tComponentID: %s,\n\tAttr: %s,\n%s\tInput: %sInput,\n}\n\n",
		strconv.Quote(kind), strconv.Quote(e.instanceAttr()), instance, prefix)
}

// instanceAttr is the data attribute carrying an instance ID.
func (e *goEmitter) instanceAttr() string { return "data-" + e.prefix + "-id" }

// canonEncodeCall returns an expression encoding code canonically.
func canonEncodeCall(t valueType, code string) string {
	if t.optional {
		return "delta.CanonOptional(" + code + ", " + canonEncoder(t.required()) + ")"
	}
	if t.kind == kindArray && t.elem != nil {
		return "delta.CanonArray(" + code + ", " + canonEncoder(*t.elem) + ")"
	}
	return canonEncoder(t) + "(" + code + ")"
}

// canonEncoder names a function value encoding t.
func canonEncoder(t valueType) string {
	if t.optional {
		return "func(value " + goType(t) + ") string { return " + canonEncodeCall(t, "value") + " }"
	}
	switch t.kind {
	case kindBool:
		return "delta.CanonBool"
	case kindInt:
		return "delta.CanonInt"
	case kindFloat:
		return "delta.CanonFloat"
	case kindBytes:
		return "delta.CanonBytes"
	case kindDateTime, kindDate, kindTime:
		return "delta.CanonTime"
	case kindURL:
		return "delta.CanonURL"
	case kindRecord:
		return canonRecordEncoder(t.name)
	case kindArray:
		return "func(value " + goType(t) + ") string { return " + canonEncodeCall(t, "value") + " }"
	default:
		// string, decimal, enums, and the trusted string types are all ~string.
		return "delta.CanonString[" + goType(t) + "]"
	}
}

// canonRecordEncoder names the generated canonical encoder for a record.
func canonRecordEncoder(name string) string { return "_tinybindCanon" + name }

// collectCanonRecords returns the records reachable from boundary component
// parameters, in a stable order.
func (e *goEmitter) collectCanonRecords() []valueType {
	types := map[string]valueType{}
	for _, declaration := range e.c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok || e.boundaryCandidate(component) == nil {
			continue
		}
		for _, parameter := range component.Parameters {
			t, err := e.c.resolveType(parameter.Type)
			if err != nil || t.async || t.required().kind == kindHTML {
				continue
			}
			collectJSONTypes(types, t, e.c)
		}
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

// emitCanonHelpers writes one canonical encoder per record a boundary reads.
func (e *goEmitter) emitCanonHelpers() {
	for _, record := range e.canonRecords {
		fmt.Fprintf(&e.b, "func %s(value %s) string {\n", canonRecordEncoder(record.name), goType(record))
		e.b.WriteString("\treturn delta.CanonRecord(delta.CanonJoin(\n")
		for _, f := range e.c.records[record.name].Fields {
			ft, _ := e.c.resolveType(f.Type)
			// An async field is a handle, not a value, on the same terms as an
			// async parameter.
			if ft.async || ft.required().kind == kindHTML {
				continue
			}
			fmt.Fprintf(&e.b, "\t\t%s,\n", canonEncodeCall(ft, "value."+goPublicName(f.Name)))
		}
		e.b.WriteString("\t))\n}\n\n")
	}
}
