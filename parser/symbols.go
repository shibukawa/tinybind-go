package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/shibukawa/tinybind-go/internal/gensource"
)

const (
	netHTTPPath  = "net/http"
	httpbindPath = "github.com/shibukawa/tinybind-go"
)

type RouteSymbol struct {
	PackagePath, Name                 string
	ReceiverPackagePath, ReceiverType string
}

// CallOperation is the static-analysis meaning of a configured function.
type CallOperation string

const (
	CallRequestBind         CallOperation = "request_bind"
	CallResponseWrite       CallOperation = "response_write"
	CallResponseWriteStatus CallOperation = "response_write_status"
	CallStreamCreate        CallOperation = "stream_create"
	CallRouteRegister       CallOperation = "route_register"
	CallErrorResponse       CallOperation = "error_response"
	// A socket carries two type arguments in opposite directions, so it takes
	// two operations against one target rather than one carrying both: a
	// pattern holds a single TypeArgument, and the inbound type is decoded
	// where the outbound one is encoded.
	CallSocketReceive CallOperation = "socket_receive"
	CallSocketSend    CallOperation = "socket_send"
)

// CallPattern maps a resolved function or method to handler-body semantics.
type CallPattern struct {
	Target            RouteSymbol
	Operation         CallOperation
	TypeArgument      int
	TypeValueArgument *int
	StatusArgument    *int
	StatusConstant    *int
	ErrorName         string
	PatternArgument   int
	PatternConstant   *string
	HandlerArgument   int
}

// Config provides the authoritative semantic calls explored by the parser.
type Config struct {
	Calls []CallPattern
	// GeneratedHeaders names header prefixes, beside this module's own, whose
	// files discovery must skip. A framework generating routes with tinybind and
	// branding the output writes a header nothing here recognizes, and an
	// unrecognized generated registry is read as if a user had written it: its
	// page registrations become routes, and an HTML page enters an OpenAPI
	// document. Naming the prefix here is what prevents that.
	//
	// Each entry still requires the conventional "DO NOT EDIT." ending.
	GeneratedHeaders []string
}

func DefaultConfig() Config {
	config := Config{}
	for _, target := range []RouteSymbol{
		{PackagePath: netHTTPPath, Name: "Handle"}, {PackagePath: netHTTPPath, Name: "HandleFunc"},
		{PackagePath: netHTTPPath, Name: "Handle", ReceiverPackagePath: netHTTPPath, ReceiverType: "ServeMux"},
		{PackagePath: netHTTPPath, Name: "HandleFunc", ReceiverPackagePath: netHTTPPath, ReceiverType: "ServeMux"},
	} {
		config.Calls = append(config.Calls, CallPattern{
			Target: target, Operation: CallRouteRegister, PatternArgument: 0, HandlerArgument: 1,
		})
	}
	for name, operation := range map[string]CallOperation{
		"Bind": CallRequestBind, "Write": CallResponseWrite,
		"WriteStatus": CallResponseWriteStatus,
		// The stream entry takes a callback, and its element type is usually
		// inferred from the closure rather than spelled, so discovery reaches it
		// through the recorded instantiation.
		"WriteStream": CallStreamCreate,
	} {
		pattern := CallPattern{
			Target:    RouteSymbol{PackagePath: httpbindPath, Name: name},
			Operation: operation,
		}
		if operation == CallResponseWriteStatus {
			index := 2
			pattern.StatusArgument = &index
		}
		config.Calls = append(config.Calls, pattern)
	}
	// The socket entry needs two patterns against one target: the type
	// arguments run in opposite directions, so neither operation can stand for
	// both. Like the stream entry, neither is usually spelled at the call
	// site, so both are recovered from the recorded instantiation.
	for _, name := range []string{"WebSocket", "WebSocketWith"} {
		target := RouteSymbol{PackagePath: httpbindPath, Name: name}
		config.Calls = append(config.Calls,
			CallPattern{Target: target, Operation: CallSocketReceive, TypeArgument: 0},
			CallPattern{Target: target, Operation: CallSocketSend, TypeArgument: 1},
		)
	}
	for _, name := range []string{
		"BadRequest", "Unauthorized", "Forbidden", "NotFound",
		"Conflict", "PayloadTooLarge", "Internal", "Validation",
	} {
		config.Calls = append(config.Calls, CallPattern{
			Target:    RouteSymbol{PackagePath: httpbindPath, Name: name},
			Operation: CallErrorResponse, ErrorName: name,
		})
	}
	return config
}

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

// configuredCalls returns every pattern whose target is obj, in configuration
// order.
//
// It yields all of them rather than the first, because one call can carry more
// than one meaning: a socket entry names an inbound type and an outbound one,
// and each direction is a pattern of its own. Stopping at the first match made
// the second direction silently undiscovered — no error, no diagnostic, just a
// missing codec at runtime.
func configuredCalls(obj types.Object, patterns []CallPattern) []CallPattern {
	var matched []CallPattern
	for _, pattern := range patterns {
		if isRouteRegistration(obj, []RouteSymbol{pattern.Target}) {
			matched = append(matched, pattern)
		}
	}
	return matched
}

// orderedSyntaxFiles returns package syntax files sorted by filename, excluding
// _test.go and anything tinybind generated.
//
// The generated header is what settles the exclusion, so a file named by a
// framework's own output setting is skipped too. That matters for the generated
// registry of a route tree: it registers every page, and discovering those
// registrations would document an HTML page as an API route.
func orderedSyntaxFiles(pkg *packages.Package, generatedHeaders []string) []*ast.File {
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
			// A file the parser could not read has no FileSet handle, so the
			// lookup returns nil rather than a file to name.
			if handle := fset.File(f.Pos()); handle != nil {
				name = handle.Name()
			}
		}
		base := filepath.Base(name)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		if strings.HasSuffix(base, "_httpbind_gen.go") ||
			strings.HasSuffix(base, "_openapi_gen.go") ||
			base == "httpbind_gen.go" ||
			base == "httpbind_openapi_gen.go" ||
			base == "tinybind_gen.go" ||
			base == "tinybind_openapi_gen.go" ||
			gensource.IsGenerated(f, generatedHeaders...) {
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
