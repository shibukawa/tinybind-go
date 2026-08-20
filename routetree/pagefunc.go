package routetree

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// PageFuncName is the reserved name of the optional Go entry point beside a
// page template.
//
// It is Load rather than Page because the template compiler already emits a
// func Page for the component into the same package, and two declarations
// cannot share a name. The file stays page.go and the component stays Page;
// only the Go entry point moves aside.
const PageFuncName = "Load"

// Rung is how much of the request path a route hands to Go, from a template
// with no Go file at all up to a plain net/http handler.
type Rung uint8

const (
	// RungTemplateOnly is a route with no page.go, or one declaring no Page.
	// The whole handler is generated and the template's own external calls
	// supply the data.
	RungTemplateOnly Rung = iota + 1
	// RungHandlerPage is a Page that is an ordinary http.HandlerFunc and owns
	// the whole response.
	RungHandlerPage
)

func (r Rung) String() string {
	switch r {
	case RungTemplateOnly:
		return "template-only"
	case RungHandlerPage:
		return "handler Page"
	default:
		return "unknown"
	}
}

// Value is one declared parameter or result of a typed Page.
type Value struct {
	// Name is the declared identifier. A result is usually unnamed, leaving
	// this empty.
	Name string
	// Type is the source text of the type expression, such as string or
	// []Order.
	Type string
}

// PageFunc describes the Go entry point of one route.
type PageFunc struct {
	Rung Rung
	// File is the page.go path, empty at RungTemplateOnly with no file.
	File string
	// Line is the line of the func Page declaration, zero when absent.
	Line int
}

// scalarTypes are the parameter types a generated decoder can bind from a URL.
// Anything else is rejected, because a path segment and a query value carry no
// object.
var scalarTypes = map[string]bool{
	"string": true, "bool": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

// bindableType splits a declared input type into the scalar a decoder parses and
// whether it arrives as a pointer.
//
// A pointer is how an optional query parameter tells an absent value from a zero
// one, which a Go parameter cannot otherwise express. It is also what the
// template language already generates for an optional declaration such as
// `page: int?`, so the spelling is the compiler's rather than this package's.
func bindableType(declared string) (base string, optional bool, ok bool) {
	base = declared
	if rest, found := strings.CutPrefix(base, "*"); found {
		base, optional = rest, true
	}
	return base, optional, scalarTypes[base]
}

// optionalPathError explains why an optional path parameter is rejected. A
// single dynamic segment is always present when the route matches at all, and a
// catch-all legally matches an empty remainder, so neither has an absent value
// to report.
func optionalPathError(name, declared string, catchAll bool) string {
	if catchAll {
		return fmt.Sprintf("path parameter %q has type %s; a catch-all matches an empty remainder rather than being absent, so it binds a string",
			name, declared)
	}
	return fmt.Sprintf("path parameter %q has type %s; a dynamic segment is always present when the route matches, so only a query parameter can be optional",
		name, declared)
}

// HandlerShape is the rung 3 signature a page function is recognized by: the
// transport package it names and the parameter types it takes from that
// package.
//
// It is configuration rather than a constant because the shape is what a
// transport is, at this seam: net/http declares a writer and a request, and
// fasthttp declares one value carrying both. A recognizer keyed on net/http
// alone reads a fasthttp handler as a malformed typed page and reports a
// signature error for a declaration that is correct.
type HandlerShape struct {
	// Import is the package path the parameter types come from. The check is
	// syntactic, so the qualifier is resolved from the file's own import of this
	// path rather than from type information.
	Import string
	// Types are the parameter types, in order, written without a qualifier and
	// with a leading * where the parameter is a pointer: ResponseWriter and
	// *Request name the net/http pair.
	Types []string
	// GeneratedHeaders names header prefixes, beside this module's own, whose
	// files action discovery must skip. A framework branding its generated
	// output writes a header nothing here recognizes on its own; this module's
	// own prefix is always recognized, so a caller never lists it.
	GeneratedHeaders []string
	// Declaration names the annotation that admits a typed server action, whose
	// signature this shape says nothing about. A zero value uses the annotation
	// this module ships.
	//
	// It sits here because the two admission rules are read together: a
	// function is an action by having this shape or by being declared, and a
	// caller configuring one transport configures both at once.
	Declaration ActionDeclaration
}

// DefaultHandlerShape is the ordinary http.HandlerFunc signature.
func DefaultHandlerShape() HandlerShape {
	return HandlerShape{Import: "net/http", Types: []string{"ResponseWriter", "*Request"}}
}

// FastHTTPHandlerShape is the fasthttp handler signature, where one value
// carries both halves and there is therefore one parameter rather than two.
// transportImport names the fasthttp package; empty uses
// [DefaultFastHTTPImport].
func FastHTTPHandlerShape(transportImport string) HandlerShape {
	return HandlerShape{
		Import: orDefault(transportImport, DefaultFastHTTPImport),
		Types:  []string{"*RequestCtx"},
	}
}

// describeShape spells the shape the way a page author would write it, for the
// error naming what a near-miss could have been.
func (h HandlerShape) describeShape() string {
	qualifier := h.Import[strings.LastIndex(h.Import, "/")+1:]
	parts := make([]string, len(h.Types))
	for i, typeName := range h.Types {
		parts[i] = qualify(typeName, qualifier)
	}
	return "func(" + strings.Join(parts, ", ") + ")"
}

// qualify inserts the package name into an unqualified type, keeping a pointer
// marker outside it.
func qualify(typeName, qualifier string) string {
	if rest, found := strings.CutPrefix(typeName, "*"); found {
		return "*" + qualifier + "." + rest
	}
	return qualifier + "." + typeName
}

// InspectLogic reads one page.go and classifies its Page declaration. An empty
// path, or a file declaring no Page, yields RungTemplateOnly.
//
// It recognizes the net/http rung 3 signature. Use [InspectLogicWith] for a
// tree whose handlers are written against another transport.
func InspectLogic(path string) (*PageFunc, error) {
	return InspectLogicWith(path, DefaultHandlerShape())
}

// InspectLogicWith is [InspectLogic] against a named rung 3 signature. A zero
// shape uses [DefaultHandlerShape].
func InspectLogicWith(path string, shape HandlerShape) (*PageFunc, error) {
	if len(shape.Types) == 0 {
		shape = DefaultHandlerShape()
	}
	if path == "" {
		return &PageFunc{Rung: RungTemplateOnly}, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	decl := findFunc(file, PageFuncName)
	if decl == nil {
		return &PageFunc{Rung: RungTemplateOnly, File: path}, nil
	}
	fn := &PageFunc{File: path, Line: fset.Position(decl.Pos()).Line}
	params := flattenFields(decl.Type.Params)
	results := flattenFields(decl.Type.Results)

	if isHandlerSignature(file, shape, params, results) {
		fn.Rung = RungHandlerPage
		return fn, nil
	}
	// The handler shape is the only one left. A page that needs Go to decide,
	// combine, or fail writes an external and binds it with {val}: the
	// component names what it needs, and a failing loader chooses the response
	// before anything is written. A near-miss is reported as the shape it is
	// not, rather than accepted into a contract that no longer exists.
	return nil, &Error{
		Path: fmt.Sprintf("%s:%d", path, fn.Line),
		Message: fmt.Sprintf("func %s must be %s; it is declared as func(%s). "+
			"A page that loads its own data declares an external and binds it with {val} in the template",
			PageFuncName, shape.describeShape(), describe(params)),
	}
}

func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name != nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// flattenFields expands grouped declarations, so func Page(org, id string)
// yields two values rather than one.
func flattenFields(list *ast.FieldList) []Value {
	if list == nil {
		return nil
	}
	var out []Value
	for _, field := range list.List {
		typeText := exprString(field.Type)
		if len(field.Names) == 0 {
			out = append(out, Value{Type: typeText})
			continue
		}
		for _, name := range field.Names {
			out = append(out, Value{Name: name.Name, Type: typeText})
		}
	}
	return out
}

// isHandlerSignature reports the transport's handler shape. The check is
// syntactic because it runs before the package compiles, so it resolves the
// transport's import name from the file rather than from type information.
func isHandlerSignature(file *ast.File, shape HandlerShape, params, results []Value) bool {
	if len(params) != len(shape.Types) || len(results) != 0 {
		return false
	}
	name, ok := importName(file, shape.Import)
	if !ok {
		return false
	}
	for i, typeName := range shape.Types {
		if params[i].Type != qualify(typeName, name) {
			return false
		}
	}
	return true
}

// takesLeadingContext reports whether a declaration opens with a
// context.Context. Like isHandlerSignature it resolves the import name from the
// file, because the check runs before the package compiles.
//
// A file that does not import context cannot be declaring one, so the missing
// import is enough to answer no.
func takesLeadingContext(file *ast.File, params []Value) bool {
	if len(params) == 0 {
		return false
	}
	contextName, ok := importName(file, "context")
	if !ok {
		return false
	}
	return params[0].Type == contextName+".Context"
}

// importName returns the identifier a file uses for an imported package,
// honoring an alias.
func importName(file *ast.File, path string) (string, bool) {
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil || value != path {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				return "", false
			}
			return spec.Name.Name, true
		}
		return path[strings.LastIndex(path, "/")+1:], true
	}
	return "", false
}

// exprString renders a type expression as source text. It covers the shapes a
// page signature may legally use and falls back to a marker for anything else,
// which Validate then rejects by name.
func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprString(e.Elt)
		}
		return "[" + exprString(e.Len) + "]" + exprString(e.Elt)
	case *ast.MapType:
		return "map[" + exprString(e.Key) + "]" + exprString(e.Value)
	case *ast.Ellipsis:
		return "..." + exprString(e.Elt)
	case *ast.IndexExpr:
		return exprString(e.X) + "[" + exprString(e.Index) + "]"
	case *ast.InterfaceType:
		if e.Methods == nil || len(e.Methods.List) == 0 {
			return "any"
		}
		return "interface{...}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.BasicLit:
		return e.Value
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func describe(params []Value) string {
	parts := make([]string, len(params))
	for i, param := range params {
		if param.Name == "" {
			parts[i] = param.Type
			continue
		}
		parts[i] = param.Name + " " + param.Type
	}
	return strings.Join(parts, ", ")
}

func paramNames(segments []Segment) string {
	parts := make([]string, len(segments))
	for i, segment := range segments {
		parts[i] = segment.Name
	}
	return strings.Join(parts, ", ")
}
