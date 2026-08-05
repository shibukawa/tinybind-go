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
	// RungTypedPage is a Page taking the route's inputs and returning the
	// values the template renders, followed by an error.
	RungTypedPage
	// RungHandlerPage is a Page that is an ordinary http.HandlerFunc and owns
	// the whole response.
	RungHandlerPage
)

func (r Rung) String() string {
	switch r {
	case RungTemplateOnly:
		return "template-only"
	case RungTypedPage:
		return "typed Page"
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
	// Params and Results are populated at RungTypedPage only. Params excludes a
	// leading context.Context and Results excludes the trailing error.
	Params  []Value
	Results []Value
	// TakesContext records that the declaration opened with a context.Context.
	//
	// It is trimmed out of Params rather than counted in them, so "Params are
	// the URL inputs, in route order" stays true and neither the route-order
	// check nor the generated decoder has to carry an offset. A context is not
	// a URL input: it arrives from the request rather than from the address,
	// which is why it cannot be spelled as one.
	TakesContext bool
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

// InspectLogic reads one page.go and classifies its Page declaration. An empty
// path, or a file declaring no Page, yields RungTemplateOnly.
func InspectLogic(path string) (*PageFunc, error) {
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

	if isHandlerSignature(file, params, results) {
		fn.Rung = RungHandlerPage
		return fn, nil
	}
	// Anything that is not the handler shape is read as the typed shape, so a
	// near-miss reports what is wrong with it rather than silently falling back
	// to the other contract.
	if len(results) == 0 || results[len(results)-1].Type != "error" {
		return nil, &Error{
			Path: fmt.Sprintf("%s:%d", path, fn.Line),
			Message: fmt.Sprintf("func %s must be either func(%s) (values..., error) or func(http.ResponseWriter, *http.Request); "+
				"its last result is not error", PageFuncName, describe(params)),
		}
	}
	fn.Rung = RungTypedPage
	// A leading context.Context is taken off the input list, after the shape
	// check above so a near-miss still reports the signature as written. Only
	// the first position counts: a context anywhere else keeps the ordinary
	// not-a-URL-value error, which is the right answer there.
	if takesLeadingContext(file, params) {
		fn.TakesContext = true
		params = params[1:]
	}
	fn.Params = params
	fn.Results = results[:len(results)-1]
	return fn, nil
}

// Validate cross-checks a typed Page against the route it serves. It returns
// every problem it finds so one run reports more than the first.
//
// componentParams is the page component's declared parameter list, which a
// typed Page must reproduce as its results. Passing nil skips that half of the
// check, which is what a caller does before the template has been compiled.
func Validate(route Route, fn *PageFunc, componentParams []Value) []error {
	if fn == nil || fn.Rung != RungTypedPage {
		return nil
	}
	where := fmt.Sprintf("%s:%d", fn.File, fn.Line)
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, &Error{Path: where, Message: fmt.Sprintf(format, args...)})
	}

	// The leading parameters are the route's dynamic segments, in route order.
	// Position carries the mapping, so nothing has to be annotated.
	if len(fn.Params) < len(route.Params) {
		fail("func %s must begin with the %d path parameter(s) of %s (%s); it declares %d parameter(s)",
			PageFuncName, len(route.Params), route.Path, paramNames(route.Params), len(fn.Params))
		return errs
	}
	for i, want := range route.Params {
		got := fn.Params[i]
		if got.Name != want.Name {
			fail("parameter %d of func %s is %q, but %s binds %q at that position",
				i+1, PageFuncName, got.Name, route.Path, want.Name)
		}
	}
	for i, param := range fn.Params {
		_, optional, ok := bindableType(param.Type)
		if !ok {
			kind := "query parameter"
			if i < len(route.Params) {
				kind = "path parameter"
			}
			fail("%s %q has type %s; a page input must be a scalar the decoder can bind from a URL",
				kind, param.Name, param.Type)
			continue
		}
		if optional && i < len(route.Params) {
			fail("%s", optionalPathError(param.Name, param.Type, route.Params[i].Kind == CatchAllSegment))
		}
	}

	if componentParams == nil {
		return errs
	}
	if len(fn.Results) != len(componentParams) {
		fail("func %s returns %d value(s) before the error, but the page component declares %d parameter(s) (%s)",
			PageFuncName, len(fn.Results), len(componentParams), valueTypes(componentParams))
		return errs
	}
	for i, want := range componentParams {
		if got := fn.Results[i]; got.Type != want.Type {
			fail("result %d of func %s is %s, but page component parameter %q is %s",
				i+1, PageFuncName, got.Type, want.Name, want.Type)
		}
	}
	return errs
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

// isHandlerSignature reports the ordinary http.HandlerFunc shape. The check is
// syntactic because it runs before the package compiles, so it resolves the
// net/http import name from the file rather than from type information.
func isHandlerSignature(file *ast.File, params, results []Value) bool {
	if len(params) != 2 || len(results) != 0 {
		return false
	}
	httpName, ok := importName(file, "net/http")
	if !ok {
		return false
	}
	return params[0].Type == httpName+".ResponseWriter" && params[1].Type == "*"+httpName+".Request"
}

// takesLeadingContext reports whether a typed entry point opens with a
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

func valueTypes(values []Value) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = value.Type
	}
	return strings.Join(parts, ", ")
}
