package parser

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ParsePackage analyzes Go sources in dir (same package only) and returns
// statically discoverable httpbind route IR.
//
// Symbol identity for route registration and httpbind calls is resolved with
// go/types (host-side only); see rule:go-types-symbol-identity.
func ParsePackage(dir string) (*Result, error) {
	return ParsePackageWithConfig(dir, DefaultConfig())
}

// ParsePackageWithConfig analyzes dir with customizable discovery symbols.
func ParsePackageWithConfig(dir string, config Config) (*Result, error) {
	pkg, err := loadPackage(dir)
	if err != nil {
		return nil, err
	}
	return parseLoadedPackage(pkg, config)
}

// ParseLoadedPackage analyzes a package that the caller already type-checked.
// Loading dominates the cost of analysis, so a generator run that needs both
// routes and type plans loads the package once and shares it.
func ParseLoadedPackage(pkg *packages.Package, config Config) (*Result, error) {
	if pkg == nil {
		return nil, fmt.Errorf("parser: no loaded package")
	}
	return parseLoadedPackage(pkg, config)
}

// ParsePackageFiles is like ParsePackage but accepts an explicit file list
// (used by tests when embedding small snippets). Each path must exist.
func ParsePackageFiles(files []string) (*Result, error) {
	if len(files) == 0 {
		return &Result{Routes: []Route{}}, nil
	}
	dir := filepath.Dir(files[0])
	return ParsePackage(dir)
}

func parseLoadedPackage(pkg *packages.Package, config Config) (*Result, error) {
	fset := fileSetFromPackage(pkg)
	files := orderedSyntaxFiles(pkg)
	p := &packageParser{
		fset:   fset,
		pkg:    pkg,
		info:   pkg.TypesInfo,
		files:  files,
		config: config,
		funcs:  map[string]*ast.FuncDecl{},
		types:  map[string]*ast.TypeSpec{},
	}
	p.indexDecls()
	routes, diags := p.discoverRoutes()
	res := &Result{Routes: routes, Diagnostics: diags}
	res.Normalize()
	return res, nil
}

// CheckPackage runs analysis and returns diagnostics for undiscoverable route candidates.
// Non-empty diagnostics mean OpenAPI would omit incomplete candidates.
func CheckPackage(dir string) ([]Diagnostic, error) {
	return CheckPackageWithConfig(dir, DefaultConfig())
}

// CheckPackageWithConfig runs diagnostics with an authoritative symbol config.
func CheckPackageWithConfig(dir string, config Config) ([]Diagnostic, error) {
	res, err := ParsePackageWithConfig(dir, config)
	if err != nil {
		return nil, err
	}
	return res.Diagnostics, nil
}

type packageParser struct {
	fset   *token.FileSet
	pkg    *packages.Package
	info   *types.Info
	files  []*ast.File
	config Config
	funcs  map[string]*ast.FuncDecl // name -> func (non-method)
	types  map[string]*ast.TypeSpec
	diags  []Diagnostic
}

func (p *packageParser) indexDecls() {
	for _, f := range p.files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Recv == nil && fd.Name != nil {
				p.funcs[fd.Name.Name] = fd
			}
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if ok && ts.Name != nil {
					p.types[ts.Name.Name] = ts
				}
			}
		}
	}
}

func (p *packageParser) discoverRoutes() ([]Route, []Diagnostic) {
	var routes []Route
	for _, f := range p.files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if route, ok := p.tryRouteCall(call); ok {
				routes = append(routes, route)
			}
			return true
		})
	}
	return routes, p.diags
}

func (p *packageParser) addDiag(call *ast.CallExpr, reason, message string) {
	d := Diagnostic{
		Reason:       reason,
		Message:      message,
		OmitsOpenAPI: true,
	}
	if p.fset != nil && call != nil {
		pos := p.fset.Position(call.Pos())
		d.File = pos.Filename
		d.Line = pos.Line
		d.Column = pos.Column
	}
	p.diags = append(p.diags, d)
}

func (p *packageParser) tryRouteCall(call *ast.CallExpr) (Route, bool) {
	obj := objectOf(p.info, call.Fun)
	patternArg, handlerArg := 0, 1
	var fixedPattern *string
	if pattern, ok := configuredCall(obj, p.config.Calls); ok && pattern.Operation == CallRouteRegister {
		patternArg, handlerArg = pattern.PatternArgument, pattern.HandlerArgument
		fixedPattern = pattern.PatternConstant
	} else {
		return Route{}, false
	}
	if handlerArg < 0 || len(call.Args) <= handlerArg || fixedPattern == nil && (patternArg < 0 || len(call.Args) <= patternArg) {
		p.addDiag(call, ReasonOther, "Handle/HandleFunc call has fewer than 2 arguments")
		return Route{}, false
	}

	patternLit := ""
	if fixedPattern != nil {
		patternLit = *fixedPattern
	} else {
		var ok bool
		patternLit, ok = p.staticString(call.Args[patternArg])
		if !ok {
			p.addDiag(call, ReasonDynamicPattern, "route pattern is not a compile-time string constant; OpenAPI will omit this registration")
			return Route{}, false
		}
	}
	method, path, ok := splitPattern(patternLit)
	if !ok {
		p.addDiag(call, ReasonDynamicPattern, "route pattern could not be split into method/path")
		return Route{}, false
	}

	leaf, meta, ok := unwrapHandler(call.Args[handlerArg], WrapperMeta{})
	if !ok {
		p.addDiag(call, ReasonOpaqueMiddleware, "handler wrapper chain could not be unwrapped to a leaf handler")
		return Route{}, false
	}

	handler, body := p.resolveHandler(leaf)
	if handler.Form == "" {
		p.addDiag(call, ReasonOther, "could not classify handler leaf expression")
		return Route{}, false
	}

	route := Route{
		Method:   method,
		Path:     path,
		Handler:  handler,
		Wrappers: meta,
	}
	if body != nil {
		info := p.analyzeBody(body)
		route.Request = info.Request
		route.Response = info.Response
		route.Stream = info.Stream
		route.Errors = info.Errors
		route.SuccessStatuses = info.SuccessStatuses
		// Promote body-level model diagnostics onto the registration site.
		for _, d := range info.Diagnostics {
			d.OmitsOpenAPI = false // route still discovered; model may be incomplete
			if d.File == "" && p.fset != nil {
				pos := p.fset.Position(call.Pos())
				d.File = pos.Filename
				d.Line = pos.Line
				d.Column = pos.Column
			}
			p.diags = append(p.diags, d)
		}
	}
	if len(route.SuccessStatuses) == 0 && route.Response != "" && route.Stream == "" {
		// Write-less response name should not happen; default 200 if response known from Write
	}
	return route, true
}

func (p *packageParser) staticString(expr ast.Expr) (string, bool) {
	if value, ok := stringLiteral(expr); ok {
		return value, true
	}
	if p.info == nil {
		return "", false
	}
	typed := p.info.Types[expr]
	if typed.Value == nil || typed.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(typed.Value), true
}

func splitPattern(pattern string) (method, path string, ok bool) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", "", false
	}
	// Go 1.22+ "METHOD /path" or path-only.
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		m := strings.TrimSpace(pattern[:i])
		p := strings.TrimSpace(pattern[i+1:])
		if m == "" || p == "" {
			return "", "", false
		}
		return m, p, true
	}
	// path-only patterns are still valid registrations
	return "", pattern, true
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconvUnquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func strconvUnquote(s string) (string, error) {
	return unquote(s)
}

// TypesInfo exposes the type-checked info for tests/helpers.
func (p *packageParser) TypesInfo() *types.Info { return p.info }
