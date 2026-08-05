// Package contextscan names the package-level functions in a directory whose
// first parameter is a context.Context.
//
// Both paths that compile a template use it: the generator, for a templates
// package, and routetree, for a route package. They share it so one definition
// decides what counts as a context-taking external, and so a declaration means
// the same thing wherever it is written.
package contextscan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Externals names the package-level functions in dir whose first parameter is a
// context.Context.
//
// An external is an ordinary Go function, so the template declaration says
// nothing about a context. Reading the implementation lets a function that needs
// one receive it without a second declaration form: write the parameter and it
// is passed, leave it out and the function is called plainly.
//
// An async external gets the boundary's context, which is what lets a call that
// can abort observe cancellation. A synchronous one gets the render context at
// the position it occupies, which is what lets a value belonging to the request
// — a CSRF token, a nonce — render inline without travelling through the
// parameter struct of every page that needs it.
//
// Detection is syntactic on purpose. It runs before the package compiles, so a
// file that does not parse is skipped rather than failing generation; a call
// shape that then does not match is an ordinary Go compile error at the
// generated call site.
func Externals(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		// A file that does not import context cannot be declaring one, so the
		// import answers for every function in it at once.
		contextName, ok := importName(file, "context")
		if !ok {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			// A method cannot be an external, so a receiver rules it out.
			if !ok || function.Recv != nil || function.Name == nil {
				continue
			}
			if takesLeadingContext(function.Type, contextName) {
				found[function.Name.Name] = true
			}
		}
	}
	return found, nil
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
