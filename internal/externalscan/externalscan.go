// Package externalscan reads the Go signatures of the package-level functions a
// template may call as externals, and reports the two things generation cannot
// learn from the template: which take a leading context.Context, and which
// return a trailing error.
//
// Both are properties of the implementation rather than the declaration, so a
// template says the same thing either way and the choice belongs to whoever
// writes the function.
//
// Both paths that compile a template use this: the generator, for a templates
// package, and routetree, for a route package. They share it so one definition
// decides what a signature means, and so a declaration means the same thing
// wherever it is written.
package externalscan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Signatures reports what a scan found. A name is present in a map only when
// the corresponding property holds, so an absent name simply means the plain
// shape.
type Signatures struct {
	// Context names the functions declaring a leading context.Context.
	//
	// An async external gets the boundary's context, which is what lets a call
	// that can abort observe cancellation. A synchronous one gets the render
	// context at the position it occupies, which is what lets a value belonging
	// to the request — a CSRF token, a nonce — render inline without travelling
	// through the parameter struct of every page that needs it.
	Context map[string]bool
	// Error names the functions whose last result is an error.
	//
	// A synchronous external is otherwise total: it answers or it does not
	// return. Declaring an error gives a call that can fail somewhere to say so,
	// and the template has to give that failure a place to go, which is why the
	// compiler restricts where such a function may be called.
	Error map[string]bool
}

// Scan reads every Go file in dir and reports the signatures generation needs.
//
// Detection is syntactic on purpose. It runs before the package compiles, so a
// file that does not parse is skipped rather than failing generation; a call
// shape that then does not match is an ordinary Go compile error at the
// generated call site.
func Scan(dir string) (Signatures, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Signatures{}, err
	}
	found := Signatures{Context: map[string]bool{}, Error: map[string]bool{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		// A file that does not import context cannot be declaring one, so the
		// import answers for every function in it at once. An error result needs
		// no import, so the file is still read for that.
		contextName, takesContext := importName(file, "context")
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			// A method cannot be an external, so a receiver rules it out.
			if !ok || function.Recv != nil || function.Name == nil {
				continue
			}
			if takesContext && takesLeadingContext(function.Type, contextName) {
				found.Context[function.Name.Name] = true
			}
			if returnsTrailingError(function.Type) {
				found.Error[function.Name.Name] = true
			}
		}
	}
	return found, nil
}

// returnsTrailingError reports whether signature's last result is an error.
//
// error is a predeclared identifier rather than a selector, so unlike the
// context check there is no import to read it through. A package shadowing the
// name would defeat this, and would defeat far more than this.
func returnsTrailingError(signature *ast.FuncType) bool {
	if signature.Results == nil || len(signature.Results.List) == 0 {
		return false
	}
	last := signature.Results.List[len(signature.Results.List)-1]
	identifier, ok := last.Type.(*ast.Ident)
	return ok && identifier.Name == "error"
}

// takesLeadingContext reports whether signature declares a context.Context
// first, under the name the file gives that import.
//
// The check is on the parsed type rather than a resolved one, because it runs
// before the package compiles. Reading the name from the imports rather than
// matching the literal "context" is what makes it right in both directions: a
// file aliasing the import still opts in, and a file aliasing some other package
// to the name context does not opt in by accident.
func takesLeadingContext(signature *ast.FuncType, contextName string) bool {
	if signature.Params == nil || len(signature.Params.List) == 0 {
		return false
	}
	selector, ok := signature.Params.List[0].Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Context" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == contextName
}

// importName returns the identifier a file uses for an imported package,
// honoring an alias. A blank or dot import gives the package no usable name.
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
