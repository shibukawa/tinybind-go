package routetree

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// DefaultActionDeclarationImport and DefaultActionDeclarationName are the
// annotation this module ships. A framework declaring its own supplies its own
// pair through [ActionDeclaration].
const (
	DefaultActionDeclarationImport  = "github.com/shibukawa/tinybind-go"
	DefaultActionDeclarationPackage = "httpbind"
	DefaultActionDeclarationName    = "ServerAction"
)

// ActionDeclaration names the annotation that admits a typed server action.
//
// The spelling belongs to whoever wraps this module: a framework writes
// pw.ServerAction(GetUser) and this module cannot fix that identifier. What it
// fixes is the shape it recognizes, which is a package-level declaration whose
// value is a call taking the function symbol and an optional published name.
type ActionDeclaration struct {
	// Import is the package path declaring the annotation.
	Import string
	// Package is the identifier that path is reached through when the file
	// imports it without an explicit alias.
	//
	// It has to be declared rather than derived, because a path's last element
	// is not always the package name: this module's own path ends in
	// tinybind-go and its package is httpbind. The handler shape check never
	// met that, resolving only net/http, where the two agree.
	Package string
	// Name is the function name within it.
	Name string
}

// DefaultActionDeclaration is the annotation this module ships.
func DefaultActionDeclaration() ActionDeclaration {
	return ActionDeclaration{
		Import:  DefaultActionDeclarationImport,
		Package: DefaultActionDeclarationPackage,
		Name:    DefaultActionDeclarationName,
	}
}

func (d ActionDeclaration) orDefault() ActionDeclaration {
	if d.Import == "" || d.Name == "" {
		return DefaultActionDeclaration()
	}
	return d
}

// alias resolves the identifier this file reaches the annotation through, or
// reports that the file does not import it at all.
func (d ActionDeclaration) alias(file *ast.File) (string, bool) {
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil || value != d.Import {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				return "", false
			}
			return spec.Name.Name, true
		}
		if d.Package != "" {
			return d.Package, true
		}
		return d.Import[strings.LastIndex(d.Import, "/")+1:], true
	}
	return "", false
}

// TypedSignature is what a declared function's signature says, read as source
// text rather than resolved.
//
// Nothing here needs a type checker. The address, the published name and the
// argument list are all readable from the declaration, and the phase that
// builds the argument struct and its codec does type-check, so resolving a
// parameter's type is that phase's job rather than this one's.
type TypedSignature struct {
	// Params are the declared inputs, in order, after any leading context has
	// been trimmed. Each carries its name and the source text of its type.
	Params []Value
	// TakesContext records that the declaration opened with a context.Context,
	// on the terms the typed page entry point already reads one.
	TakesContext bool
	// Result is the type of the single non-error result, empty when the
	// function returns only an error.
	Result string
}

// findTypedActions reads one package's declarations and returns the actions its
// annotations admit.
//
// The declaration is resolved syntactically, by matching the annotation's import
// against this file's import list, exactly as the handler shape check resolves
// net/http and the context check resolves context. A type checker would be
// stronger — another package imported under the same alias would be accepted
// here — but this runs before the package compiles, so it is the same weakness
// both of those already carry. Requiring the argument to name a function
// declared in this same file's package is what keeps a misread from reaching
// outside it.
func findTypedActions(files []*ast.File, filenames []string, fset *token.FileSet, decl ActionDeclaration) ([]typedActionRef, []error) {
	decl = decl.orDefault()
	var out []typedActionRef
	var errs []error
	for i, file := range files {
		alias, ok := decl.alias(file)
		if !ok {
			continue
		}
		funcs := map[string]*ast.FuncDecl{}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name != nil {
				funcs[fn.Name.Name] = fn
			}
		}
		for _, d := range file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, value := range vs.Values {
					call, ok := value.(*ast.CallExpr)
					if !ok || !callsDeclaration(call, alias, decl.Name) {
						continue
					}
					ref, err := readDeclaration(call, funcs, file, filenames[i], fset)
					if err != nil {
						errs = append(errs, err)
						continue
					}
					out = append(out, ref)
				}
			}
		}
	}
	return out, errs
}

type typedActionRef struct {
	Fn        *ast.FuncDecl
	Name      string
	Published string
	File      string
	Line      int
	Signature TypedSignature
}

func callsDeclaration(call *ast.CallExpr, alias, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != name {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == alias
}

// readDeclaration turns one annotation into a reference, or reports why it is
// not one.
func readDeclaration(call *ast.CallExpr, funcs map[string]*ast.FuncDecl, file *ast.File, filename string, fset *token.FileSet) (typedActionRef, error) {
	position := fset.Position(call.Pos())
	where := fmt.Sprintf("%s:%d", filename, position.Line)
	if len(call.Args) == 0 {
		return typedActionRef{}, fmt.Errorf("%s: server action declaration names no function", where)
	}
	ident, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return typedActionRef{}, fmt.Errorf("%s: server action declaration takes a function declared in this package, not an expression", where)
	}
	fn, ok := funcs[ident.Name]
	if !ok {
		return typedActionRef{}, fmt.Errorf("%s: server action declaration names %s, which is not a function declared in this package", where, ident.Name)
	}
	// Load is the page's own entry point at every rung, so it is never an
	// action however it is declared.
	if fn.Name.Name == PageFuncName {
		return typedActionRef{}, fmt.Errorf("%s: %s is the page entry point and cannot be a server action", where, PageFuncName)
	}
	published, err := publishedFromDeclaration(call, fn.Name.Name, where)
	if err != nil {
		return typedActionRef{}, err
	}
	signature, err := readTypedSignature(fn, file, where)
	if err != nil {
		return typedActionRef{}, err
	}
	return typedActionRef{
		Fn:        fn,
		Name:      fn.Name.Name,
		Published: published,
		File:      filename,
		Line:      fset.Position(fn.Pos()).Line,
		Signature: signature,
	}, nil
}

func publishedFromDeclaration(call *ast.CallExpr, goName, where string) (string, error) {
	if len(call.Args) < 2 {
		return PublishedName(goName), nil
	}
	literal, ok := call.Args[1].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", fmt.Errorf("%s: published name must be a string literal, because it is resolved at generation", where)
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil || value == "" {
		return "", fmt.Errorf("%s: published name is not a usable string", where)
	}
	return value, nil
}

// readTypedSignature reads the parameters and results a declared function
// carries.
//
// Parameter types are kept as source text. The phase that builds the argument
// struct type-checks the package and can resolve them; nothing here has to.
func readTypedSignature(fn *ast.FuncDecl, file *ast.File, where string) (TypedSignature, error) {
	var signature TypedSignature
	params := flattenFields(fn.Type.Params)
	if takesLeadingContext(file, params) {
		signature.TakesContext = true
		params = params[1:]
	}
	for _, p := range params {
		if p.Name == "" || p.Name == "_" {
			return TypedSignature{}, fmt.Errorf("%s: every parameter of %s needs a name, because an argument is bound by it", where, fn.Name.Name)
		}
		signature.Params = append(signature.Params, p)
	}
	results := flattenFields(fn.Type.Results)
	switch len(results) {
	case 1:
		if results[0].Type != "error" {
			return TypedSignature{}, fmt.Errorf("%s: %s returns %s alone; a single result must be an error", where, fn.Name.Name, results[0].Type)
		}
	case 2:
		if results[1].Type != "error" {
			return TypedSignature{}, fmt.Errorf("%s: %s returns (%s, %s); the second result must be an error", where, fn.Name.Name, results[0].Type, results[1].Type)
		}
		signature.Result = results[0].Type
	default:
		return TypedSignature{}, fmt.Errorf("%s: %s returns %d values; a server action returns one value and an error, or an error alone", where, fn.Name.Name, len(results))
	}
	return signature, nil
}
