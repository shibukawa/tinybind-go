package routetree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultActionPrefix is the reserved path every server function endpoint hangs
// below.
//
// It is safe by construction only because [Discover] ignores directories
// beginning with an underscore, so a route tree can never produce this path. A
// configured prefix has no such guarantee, which is why [ValidateActionPrefix]
// takes the discovered routes.
const DefaultActionPrefix = "/_action"

// ActionHashLength is how many hexadecimal characters of the digest an endpoint
// carries. It is fixed rather than configurable.
const ActionHashLength = 12

// Action is one server function reachable from client code.
//
// Every exported function in a route package with the handler signature becomes
// one, whether or not a template references it. A route package is imported by
// nothing but the generated registry, so an exported symbol in it is that
// route's surface rather than a general API; lower-casing the function is what
// keeps it private, because generated code in another package cannot reach an
// unexported symbol.
type Action struct {
	// Name is the exported Go function name.
	Name string
	// RelDir is the declaring package's directory relative to the route root,
	// using slashes. The route root package itself has an empty RelDir.
	RelDir string
	// Package and ImportPath name the declaring Go package. ImportPath is empty
	// unless Config.ImportBase was set.
	Package    string
	ImportPath string
	// File and Line locate the declaration, for diagnostics.
	File string
	Line int
	// Hash is the first ActionHashLength hexadecimal characters of the digest of
	// the declaring directory and the name.
	Hash string
	// Path is the endpoint path, such as /_action/9f3c2ab1e4d7/Rename.
	Path string
}

// Pattern returns the stdlib ServeMux pattern for the endpoint, which is always
// POST.
func (a Action) Pattern() string { return "POST " + a.Path }

// DiscoverActions reads the Go sources of one route or layout package and
// returns its server functions, ordered by name.
//
// dir is the package directory, relDir its path relative to the route root, and
// prefix the reserved endpoint prefix; an empty prefix uses
// [DefaultActionPrefix].
//
// A server function is recognized by the same signature a rung 3 page is, so
// this reads the net/http shape. Use [DiscoverActionsWith] for a tree whose
// handlers are written against another transport.
func DiscoverActions(dir, relDir, pkg, importPath, prefix string) ([]Action, error) {
	return DiscoverActionsWith(dir, relDir, pkg, importPath, prefix, DefaultHandlerShape())
}

// DiscoverActionsWith is [DiscoverActions] against a named handler signature. A
// zero shape uses [DefaultHandlerShape].
func DiscoverActionsWith(dir, relDir, pkg, importPath, prefix string, shape HandlerShape) ([]Action, error) {
	if prefix == "" {
		prefix = DefaultActionPrefix
	}
	if len(shape.Types) == 0 {
		shape = DefaultHandlerShape()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	fset := token.NewFileSet()
	var out []Action
	var errs []error
	for _, name := range names {
		filename := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, filename, nil, parser.SkipObjectResolution)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil {
				continue
			}
			// Load is the page's own entry point and shares this signature at
			// rung 3, so it is never a server function.
			if !fn.Name.IsExported() || fn.Name.Name == PageFuncName {
				continue
			}
			if !isHandlerSignature(file, shape, flattenFields(fn.Type.Params), flattenFields(fn.Type.Results)) {
				continue
			}
			hash := ActionHash(relDir, fn.Name.Name)
			out = append(out, Action{
				Name:       fn.Name.Name,
				RelDir:     relDir,
				Package:    pkg,
				ImportPath: importPath,
				File:       filename,
				Line:       fset.Position(fn.Pos()).Line,
				Hash:       hash,
				Path:       ActionPath(prefix, hash, fn.Name.Name),
			})
		}
	}
	if len(errs) > 0 {
		return nil, joinErrors(errs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ActionHash derives the stable half of an endpoint URL from the declaring
// package's route-relative directory and the handler name.
//
// The mount prefix is deliberately not hashed, so remounting an application
// changes the URL without changing the identity underneath. There is no build
// salt either: regenerating an unchanged project reproduces the same value, so
// a client that cached the URL keeps working across a deploy.
//
// The declaring directory rather than the serving route path is what goes in,
// because a layout is compiled once and its handler must have one URL no matter
// which page renders it.
func ActionHash(relDir, name string) string {
	digest := sha256.Sum256([]byte(relDir + "\x00" + name))
	return hex.EncodeToString(digest[:])[:ActionHashLength]
}

// ActionPath builds the endpoint path for one handler.
func ActionPath(prefix, hash, name string) string {
	return strings.TrimSuffix(prefix, "/") + "/" + hash + "/" + name
}

// ValidateActionPrefix reports whether a configured endpoint prefix is usable
// with the discovered routes.
//
// The default prefix cannot collide, because a route directory beginning with
// an underscore is ignored by discovery. A configured one has no such
// protection, so it is checked against every discovered pattern here rather
// than surfacing as a ServeMux panic at startup.
func ValidateActionPrefix(prefix string, tree *Tree) error {
	if prefix == "" {
		return nil
	}
	if !strings.HasPrefix(prefix, "/") {
		return &Error{Path: prefix, Message: "action prefix must begin with /"}
	}
	if strings.ContainsAny(prefix, "{}") {
		return &Error{Path: prefix, Message: "action prefix must be a literal path; it cannot contain a dynamic segment"}
	}
	if cleaned := path.Clean(prefix); cleaned != prefix {
		return &Error{Path: prefix, Message: fmt.Sprintf("action prefix is not a clean path; write %s", cleaned)}
	}
	if tree == nil {
		return nil
	}
	// An endpoint hangs one hash and one name below the prefix, so a route
	// matching the prefix itself or anything under it would answer requests the
	// endpoints own, or be shadowed by them.
	for _, route := range tree.Routes {
		if route.Path == prefix || strings.HasPrefix(route.Path, prefix+"/") {
			return &Error{
				Path: route.PageFile,
				Message: fmt.Sprintf("route %s lies under the action prefix %s; choose a prefix no route occupies",
					route.Path, prefix),
			}
		}
	}
	return nil
}

// checkActionCollisions reports two actions sharing a hash. Every input is
// known at generation, so this can never surface at runtime.
func checkActionCollisions(actions []Action) []error {
	seen := make(map[string]Action, len(actions))
	var errs []error
	for _, action := range actions {
		if first, ok := seen[action.Hash]; ok {
			errs = append(errs, &Error{
				Path: fmt.Sprintf("%s:%d", action.File, action.Line),
				Message: fmt.Sprintf("server action %s hashes to %s, which %s:%d already claims",
					action.Name, action.Hash, first.File, first.Line),
			})
			continue
		}
		seen[action.Hash] = action
	}
	return errs
}

// actionURLs is the resolution map the template compiler needs: handler name to
// endpoint URL, for the actions declared beside that template.
func actionURLs(actions []Action) map[string]string {
	if len(actions) == 0 {
		return nil
	}
	out := make(map[string]string, len(actions))
	for _, action := range actions {
		out[action.Name] = action.Path
	}
	return out
}
