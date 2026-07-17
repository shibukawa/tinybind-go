package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

const (
	netHTTPPath     = "net/http"
	httpbinderPath  = "github.com/shibukawa/httpbind-go"
	serveMuxTypeStr = "net/http.ServeMux"
)

type FunctionSymbol struct{ PackagePath, Name string }

type RouteSymbol struct {
	PackagePath, Name                 string
	ReceiverPackagePath, ReceiverType string
}

// Config customizes symbols explored by the parser. Every slice is authoritative.
type Config struct {
	RouteRegistrations []RouteSymbol
	GenericFunctions   []FunctionSymbol
	ErrorFunctions     []FunctionSymbol
}

func DefaultConfig() Config {
	return Config{
		RouteRegistrations: []RouteSymbol{
			{PackagePath: netHTTPPath, Name: "Handle"}, {PackagePath: netHTTPPath, Name: "HandleFunc"},
			{PackagePath: netHTTPPath, Name: "Handle", ReceiverPackagePath: netHTTPPath, ReceiverType: "ServeMux"},
			{PackagePath: netHTTPPath, Name: "HandleFunc", ReceiverPackagePath: netHTTPPath, ReceiverType: "ServeMux"},
		},
		GenericFunctions: namesToSymbols(httpbinderPath, httpbinderGenericFns),
		ErrorFunctions:   namesToSymbols(httpbinderPath, httpbinderErrorFns),
	}
}

func namesToSymbols(path string, names map[string]struct{}) []FunctionSymbol {
	out := make([]FunctionSymbol, 0, len(names))
	for name := range names {
		out = append(out, FunctionSymbol{path, name})
	}
	return out
}

// Fixed allowlist of httpbinder function names used in discovery.
var (
	httpbinderGenericFns = map[string]struct{}{
		"Bind":        {},
		"Write":       {},
		"WriteStatus": {},
		"NewStream":   {},
		"DecodeJSON":  {},
		"EncodeJSON":  {},
	}
	httpbinderErrorFns = map[string]struct{}{
		"BadRequest":      {},
		"Unauthorized":    {},
		"Forbidden":       {},
		"NotFound":        {},
		"Conflict":        {},
		"PayloadTooLarge": {},
		"Internal":        {},
		"Validation":      {},
	}
	httpbinderOtherFns = map[string]struct{}{
		"WriteError": {},
	}
)

// loadPackage type-checks the package in dir (host-side only).
func loadPackage(dir string) (*packages.Package, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedModule |
			packages.NeedDeps,
		Dir: abs,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("packages.Load %s: %w", abs, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("packages.Load %s: no packages", abs)
	}
	pkg := pkgs[0]
	// Prefer the package that matches the directory; skip test packages.
	for _, p := range pkgs {
		if p.Name != "" && !strings.HasSuffix(p.ID, ".test") && !strings.HasSuffix(p.Name, "_test") {
			pkg = p
			break
		}
	}
	if pkg.Types == nil || pkg.TypesInfo == nil {
		return nil, fmt.Errorf("packages.Load %s: type-check failed: %v", abs, pkg.Errors)
	}
	return pkg, nil
}

// objectOf resolves the function/method object for a call expression's Fun.
func objectOf(info *types.Info, fun ast.Expr) types.Object {
	if info == nil || fun == nil {
		return nil
	}
	fun = stripParens(fun)
	switch e := fun.(type) {
	case *ast.Ident:
		return info.Uses[e]
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[e]; ok && sel != nil {
			return sel.Obj()
		}
		if e.Sel != nil {
			return info.Uses[e.Sel]
		}
	case *ast.IndexExpr:
		return objectOf(info, e.X)
	case *ast.IndexListExpr:
		return objectOf(info, e.X)
	}
	return nil
}

func isFuncNamed(obj types.Object, pkgPath, name string) bool {
	f, ok := obj.(*types.Func)
	if !ok || f.Name() != name {
		return false
	}
	if f.Pkg() == nil || f.Pkg().Path() != pkgPath {
		return false
	}
	return true
}

// isNetHTTPRegistration reports whether obj is net/http.Handle, HandleFunc,
// or (*net/http.ServeMux).Handle / HandleFunc.
func isNetHTTPRegistration(obj types.Object) bool {
	return isRouteRegistration(obj, DefaultConfig().RouteRegistrations)
}

func isRouteRegistration(obj types.Object, symbols []RouteSymbol) bool {
	f, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	if f.Pkg() == nil {
		return false
	}
	sig, ok := f.Type().(*types.Signature)
	if !ok {
		return false
	}
	for _, s := range symbols {
		if f.Pkg().Path() != s.PackagePath || f.Name() != s.Name {
			continue
		}
		recv := sig.Recv()
		if s.ReceiverType == "" {
			if recv == nil {
				return true
			}
			continue
		}
		if recv == nil {
			continue
		}
		t := recv.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		n, ok := t.(*types.Named)
		if ok && n.Obj() != nil && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == s.ReceiverPackagePath && n.Obj().Name() == s.ReceiverType {
			return true
		}
	}
	return false
}

func isConfiguredFunc(obj types.Object, symbols []FunctionSymbol) bool {
	f, ok := obj.(*types.Func)
	if !ok || f.Pkg() == nil {
		return false
	}
	if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
		return false
	}
	for _, s := range symbols {
		if f.Pkg().Path() == s.PackagePath && f.Name() == s.Name {
			return true
		}
	}
	return false
}

func isServeMuxType(t types.Type) bool {
	if t == nil {
		return false
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := n.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == netHTTPPath && obj.Name() == "ServeMux"
}

// isHTTPBinderFunc reports whether obj is a package-level function in httpbind-go
// with one of the allowed names in allowed.
func isHTTPBinderFunc(obj types.Object, allowed map[string]struct{}) bool {
	f, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	if f.Pkg() == nil || f.Pkg().Path() != httpbinderPath {
		return false
	}
	// Methods are not expected; only package functions.
	if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
		return false
	}
	_, ok = allowed[f.Name()]
	return ok
}

func isHTTPBinderGenericCall(obj types.Object) bool {
	return isHTTPBinderFunc(obj, httpbinderGenericFns)
}

func isHTTPBinderErrorCall(obj types.Object) bool {
	return isHTTPBinderFunc(obj, httpbinderErrorFns)
}

// callFuncName returns the simple function name if obj is *types.Func.
func callFuncName(obj types.Object) string {
	if f, ok := obj.(*types.Func); ok {
		return f.Name()
	}
	return ""
}

// orderedSyntaxFiles returns package syntax files sorted by filename, excluding
// generated binders/openapi embeds and _test.go when present.
func orderedSyntaxFiles(pkg *packages.Package) []*ast.File {
	if pkg == nil {
		return nil
	}
	type pair struct {
		name string
		file *ast.File
	}
	var pairs []pair
	fset := pkg.Fset
	for _, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		name := ""
		if fset != nil {
			name = fset.File(f.Pos()).Name()
		}
		base := filepath.Base(name)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		if strings.HasSuffix(base, "_httpbinder_gen.go") ||
			strings.HasSuffix(base, "_openapi_gen.go") ||
			base == "httpbinder_gen.go" ||
			base == "httpbinder_openapi_gen.go" {
			continue
		}
		pairs = append(pairs, pair{name: name, file: f})
	}
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].name < pairs[i].name {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	out := make([]*ast.File, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.file)
	}
	return out
}

// fileSetFromPackage returns the FileSet used by packages.Load when available.
func fileSetFromPackage(pkg *packages.Package) *token.FileSet {
	if pkg != nil && pkg.Fset != nil {
		return pkg.Fset
	}
	return token.NewFileSet()
}
