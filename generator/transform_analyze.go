package generator

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

const (
	netHTTPResponseWriter = "net/http.ResponseWriter"
	netHTTPRequest        = "*net/http.Request"
)

// TransformCandidate is one function the transform considered.
type TransformCandidate struct {
	Name string
	Decl *ast.FuncDecl
	// TransportParams are the parameter names holding the writer and the
	// request, in declaration order. They collapse into one context parameter.
	TransportParams []string
	// transportIndexes are the same parameters as flattened positions, which is
	// what a caller's argument list is matched against.
	transportIndexes map[int]bool
	objects          map[types.Object]bool
	calls            map[string]bool
}

// TransformPlan is the analysis result: what can be rewritten and what cannot.
type TransformPlan struct {
	Admitted []*TransformCandidate
	Refusals TransformRefusals
}

// AnalyzeTransform classifies every same-package function taking a transport
// value, per rule:transform-eligibility.
//
// Every such function is a candidate, not only the discovered handlers: with no
// adapter to fall back to, a shared helper that stays net/http would refuse
// every handler that calls it, so the admission set closes over the call graph.
func AnalyzeTransform(pkg *packages.Package, options TransformOptions) (*TransformPlan, error) {
	options, err := options.normalized()
	if err != nil {
		return nil, err
	}
	if pkg == nil || pkg.TypesInfo == nil {
		return &TransformPlan{}, nil
	}
	a := &transformAnalyzer{
		pkg:        pkg,
		info:       pkg.TypesInfo,
		fset:       pkg.Fset,
		options:    options,
		patterns:   indexCallPatterns(options.Calls),
		candidates: map[string]*TransformCandidate{},
	}
	a.collectCandidates()
	a.classify()
	a.propagate()
	return a.plan(), nil
}

type transformAnalyzer struct {
	pkg        *packages.Package
	info       *types.Info
	fset       *token.FileSet
	options    TransformOptions
	patterns   map[string]CallPattern
	candidates map[string]*TransformCandidate
	order      []string
	refusals   map[string]TransformRefusal
}

func indexCallPatterns(patterns []CallPattern) map[string]CallPattern {
	out := map[string]CallPattern{}
	for _, pattern := range patterns {
		if pattern.Target.Function == nil {
			continue
		}
		out[pattern.Target.Function.PackagePath+"."+pattern.Target.Function.Name] = pattern
	}
	return out
}

func (a *transformAnalyzer) collectCandidates() {
	for _, file := range a.pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Type.Params == nil {
				continue
			}
			candidate := &TransformCandidate{
				Name:             fn.Name.Name,
				Decl:             fn,
				transportIndexes: map[int]bool{},
				objects:          map[types.Object]bool{},
				calls:            map[string]bool{},
			}
			index := 0
			for _, field := range fn.Type.Params.List {
				names := field.Names
				if len(names) == 0 {
					names = []*ast.Ident{nil}
				}
				for _, name := range names {
					if a.isTransportType(field.Type) {
						candidate.transportIndexes[index] = true
						if name != nil {
							candidate.TransportParams = append(candidate.TransportParams, name.Name)
							if obj := a.info.Defs[name]; obj != nil {
								candidate.objects[obj] = true
							}
						}
					}
					index++
				}
			}
			if len(candidate.transportIndexes) == 0 {
				continue
			}
			a.candidates[candidate.Name] = candidate
			a.order = append(a.order, candidate.Name)
		}
	}
}

func (a *transformAnalyzer) isTransportType(expr ast.Expr) bool {
	tv, ok := a.info.Types[expr]
	if !ok || tv.Type == nil {
		return false
	}
	switch tv.Type.String() {
	case netHTTPResponseWriter, netHTTPRequest:
		return true
	}
	return false
}

// classify walks each candidate and records what its transport values are used
// for. An occurrence the walk does not recognize is a refusal; the default is
// never "probably fine".
func (a *transformAnalyzer) classify() {
	a.refusals = map[string]TransformRefusal{}
	for _, name := range a.order {
		candidate := a.candidates[name]
		accounted := map[*ast.Ident]bool{}
		classified := map[*ast.Ident]TransformRefusal{}
		// forced outranks accounted. A closure body is full of perfectly
		// ordinary runtime calls, and looking only at those would admit the
		// capture that is the actual problem.
		forced := map[*ast.Ident]TransformRefusal{}

		// A transport value captured by a closure outlives the statement that
		// created it, so the whole literal is refused rather than inspected.
		ast.Inspect(candidate.Decl.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.FuncLit)
			if !ok {
				return true
			}
			ast.Inspect(lit.Body, func(inner ast.Node) bool {
				if id, ok := inner.(*ast.Ident); ok && a.isTransportIdent(candidate, id) {
					forced[id] = a.refuse(candidate.Name, RefusalEscapes, id,
						"captures "+id.Name+" in a function literal")
				}
				return true
			})
			return true
		})

		ast.Inspect(candidate.Decl.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				// "_ = r" discards the value and is common enough in real
				// handlers that refusing it would refuse most of them.
				if !allBlank(node.Lhs) {
					return true
				}
				for _, rhs := range node.Rhs {
					if id, ok := stripIdent(rhs); ok && a.isTransportIdent(candidate, id) {
						accounted[id] = true
					}
				}
			case *ast.CallExpr:
				a.classifyCall(candidate, node, accounted, classified)
			case *ast.SelectorExpr:
				id, ok := stripIdent(node.X)
				if !ok || !a.isTransportIdent(candidate, id) {
					return true
				}
				if _, ok := a.options.RequestSelectorRewrites[node.Sel.Name]; ok {
					accounted[id] = true
					return true
				}
				classified[id] = a.refuse(candidate.Name, RefusalUnknownSelector, node,
					"reads "+id.Name+"."+node.Sel.Name+", which no rewrite covers")
			case *ast.TypeAssertExpr:
				id, ok := stripIdent(node.X)
				if ok && a.isTransportIdent(candidate, id) {
					classified[id] = a.refuse(candidate.Name, RefusalTypeAssertion, node,
						"type-asserts "+id.Name+", a capability the other transport presents differently")
				}
			case *ast.UnaryExpr:
				if node.Op != token.AND {
					return true
				}
				if id, ok := stripIdent(node.X); ok && a.isTransportIdent(candidate, id) {
					classified[id] = a.refuse(candidate.Name, RefusalEscapes, node,
						"takes the address of "+id.Name)
				}
			}
			return true
		})

		// Anything still unexplained escapes: assigned, stored, returned or
		// compared. The classification is coarse on purpose, because the
		// remedy is the same and precision here would be guesswork.
		ast.Inspect(candidate.Decl.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || !a.isTransportIdent(candidate, id) {
				return true
			}
			if refusal, ok := forced[id]; ok {
				a.record(refusal)
				return true
			}
			if accounted[id] {
				return true
			}
			if existing, ok := classified[id]; ok {
				a.record(existing)
				return true
			}
			a.record(a.refuse(candidate.Name, RefusalEscapes, id,
				"uses "+id.Name+" somewhere the rewrite cannot follow"))
			return true
		})
	}
}

func (a *transformAnalyzer) classifyCall(candidate *TransformCandidate, call *ast.CallExpr, accounted map[*ast.Ident]bool, classified map[*ast.Ident]TransformRefusal) {
	obj := transformObjectOf(a.info, call.Fun)
	pattern, hasPattern := a.patternFor(obj)
	callee, isLocal := a.localCallee(obj)
	if isLocal {
		candidate.calls[callee.Name] = true
	}
	for index, arg := range call.Args {
		id, ok := stripIdent(arg)
		if !ok || !a.isTransportIdent(candidate, id) {
			continue
		}
		if hasPattern && pattern.Transport.Drops(index) {
			accounted[id] = true
			continue
		}
		if isLocal && callee.transportIndexes[index] {
			accounted[id] = true
			continue
		}
		classified[id] = a.refuse(candidate.Name, RefusalUnknownCall, arg,
			"passes "+id.Name+" to "+describeCallee(obj, call)+", whose transport arguments are undeclared")
	}
}

func (a *transformAnalyzer) patternFor(obj types.Object) (CallPattern, bool) {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil {
		return CallPattern{}, false
	}
	pattern, ok := a.patterns[fn.Pkg().Path()+"."+fn.Name()]
	return pattern, ok
}

func (a *transformAnalyzer) localCallee(obj types.Object) (*TransformCandidate, bool) {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || a.pkg.Types == nil || fn.Pkg().Path() != a.pkg.Types.Path() {
		return nil, false
	}
	candidate, ok := a.candidates[fn.Name()]
	return candidate, ok
}

func (a *transformAnalyzer) isTransportIdent(candidate *TransformCandidate, id *ast.Ident) bool {
	if id == nil {
		return false
	}
	obj := a.info.Uses[id]
	if obj == nil {
		obj = a.info.Defs[id]
	}
	return obj != nil && candidate.objects[obj]
}

func (a *transformAnalyzer) refuse(function string, kind TransformRefusalKind, node ast.Node, detail string) TransformRefusal {
	return TransformRefusal{
		Function: function,
		Kind:     kind,
		Position: a.fset.Position(node.Pos()),
		Detail:   detail,
	}
}

func (a *transformAnalyzer) record(refusal TransformRefusal) {
	if _, exists := a.refusals[refusal.Function]; exists {
		return
	}
	a.refusals[refusal.Function] = refusal
}

// propagate carries a refusal to every caller that hands the refused function a
// transport value, repeating until nothing changes. A refusal found late
// invalidates callers already walked, which is why this is a fixpoint rather
// than one pass.
func (a *transformAnalyzer) propagate() {
	for {
		changed := false
		for _, name := range a.order {
			if _, refused := a.refusals[name]; refused {
				continue
			}
			candidate := a.candidates[name]
			for callee := range candidate.calls {
				downstream, ok := a.refusals[callee]
				if !ok {
					continue
				}
				chain := append([]TransformRefusalHop{{
					Function: downstream.Function,
					Position: downstream.Position,
					Detail:   downstream.Detail,
				}}, downstream.Chain...)
				a.refusals[name] = TransformRefusal{
					Function: name,
					Kind:     RefusalInheritedFromCallee,
					Position: a.fset.Position(candidate.Decl.Pos()),
					Detail:   "calls " + callee + ", which is not transformable",
					Chain:    chain,
				}
				changed = true
				break
			}
		}
		if !changed {
			return
		}
	}
}

func (a *transformAnalyzer) plan() *TransformPlan {
	plan := &TransformPlan{}
	for _, name := range a.order {
		if refusal, refused := a.refusals[name]; refused {
			plan.Refusals = append(plan.Refusals, refusal)
			continue
		}
		plan.Admitted = append(plan.Admitted, a.candidates[name])
	}
	sortRefusals(plan.Refusals)
	return plan
}

func allBlank(exprs []ast.Expr) bool {
	if len(exprs) == 0 {
		return false
	}
	for _, expr := range exprs {
		id, ok := expr.(*ast.Ident)
		if !ok || id.Name != "_" {
			return false
		}
	}
	return true
}

func stripIdent(expr ast.Expr) (*ast.Ident, bool) {
	for {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			expr = e.X
		case *ast.Ident:
			return e, true
		default:
			return nil, false
		}
	}
}

func transformObjectOf(info *types.Info, fun ast.Expr) types.Object {
	if info == nil || fun == nil {
		return nil
	}
	switch e := fun.(type) {
	case *ast.ParenExpr:
		return transformObjectOf(info, e.X)
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
		return transformObjectOf(info, e.X)
	case *ast.IndexListExpr:
		return transformObjectOf(info, e.X)
	}
	return nil
}

func describeCallee(obj types.Object, call *ast.CallExpr) string {
	if fn, ok := obj.(*types.Func); ok {
		if fn.Pkg() != nil {
			return fn.Pkg().Name() + "." + fn.Name()
		}
		return fn.Name()
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel != nil {
		return sel.Sel.Name
	}
	if id, ok := call.Fun.(*ast.Ident); ok {
		return id.Name
	}
	return "an unrecognized call"
}
