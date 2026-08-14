package parser

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
)

type bodyInfo struct {
	Request         string
	Response        string
	Stream          string
	SocketIn        string
	SocketOut       string
	Errors          []string
	SuccessStatuses []int
	Diagnostics     []Diagnostic
}

func (p *packageParser) analyzeBody(body *ast.BlockStmt) bodyInfo {
	var info bodyInfo
	if body == nil {
		return info
	}
	errorSet := map[string]struct{}{}
	statusSet := map[int]struct{}{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		obj := objectOf(p.info, call.Fun)
		if obj == nil {
			return true
		}

		// One call can carry more than one meaning: a socket entry names an
		// inbound type and an outbound one, each configured as its own
		// pattern against the same target.
		for _, pattern := range configuredCalls(obj, p.config.Calls) {
			typeArgs := genericTypeArgExprs(call)
			typeArgStrs := make([]string, 0, len(typeArgs))
			for _, a := range typeArgs {
				typeArgStrs = append(typeArgStrs, typeExprString(a))
			}
			typeIndex := pattern.TypeArgument
			if pattern.TypeValueArgument != nil {
				if typeName := p.callArgumentTypeName(call, *pattern.TypeValueArgument); typeName != "" {
					typeArgs = []ast.Expr{ast.NewIdent(typeName)}
					typeArgStrs = []string{typeName}
					typeIndex = 0
				}
			} else if len(typeArgs) <= typeIndex {
				if typeName := p.instantiatedTypeArgumentName(call.Fun, typeIndex); typeName != "" {
					for len(typeArgs) <= typeIndex {
						typeArgs = append(typeArgs, nil)
						typeArgStrs = append(typeArgStrs, "")
					}
					typeArgs[typeIndex] = ast.NewIdent(typeName)
					typeArgStrs[typeIndex] = typeName
				}
			}
			switch pattern.Operation {
			case CallRequestBind:
				if len(typeArgs) > typeIndex {
					if reason := typeArgIssue(typeArgs[typeIndex]); reason != "" {
						info.Diagnostics = append(info.Diagnostics, p.diagAt(call, reason, "Bind type argument is not a same-package plain named type"))
					} else if info.Request == "" {
						info.Request = typeArgStrs[typeIndex]
					}
				}
			case CallResponseWrite:
				if len(typeArgs) > typeIndex {
					if reason := typeArgIssue(typeArgs[typeIndex]); reason != "" {
						info.Diagnostics = append(info.Diagnostics, p.diagAt(call, reason, "Write type argument is not a same-package plain named type"))
					} else {
						resp, streamElem := parseResponseType(typeArgStrs[typeIndex])
						if info.Response == "" {
							info.Response = resp
						}
						if streamElem != "" && info.Stream == "" {
							info.Stream = streamElem
						}
						statusSet[200] = struct{}{}
					}
				}
			case CallResponseWriteStatus:
				if len(typeArgs) > typeIndex {
					if reason := typeArgIssue(typeArgs[typeIndex]); reason != "" {
						info.Diagnostics = append(info.Diagnostics, p.diagAt(call, reason, "WriteStatus type argument is not a same-package plain named type"))
					} else {
						resp, streamElem := parseResponseType(typeArgStrs[typeIndex])
						if info.Response == "" {
							info.Response = resp
						}
						if streamElem != "" && info.Stream == "" {
							info.Stream = streamElem
						}
						// WriteStatus[T](w, r, status, value) — status is arg index 2
						if pattern.StatusConstant != nil {
							statusSet[*pattern.StatusConstant] = struct{}{}
						} else if pattern.StatusArgument != nil {
							if st, ok := staticIntArgument(call, *pattern.StatusArgument); ok {
								statusSet[st] = struct{}{}
							} else {
								info.Diagnostics = append(info.Diagnostics, p.diagAt(call, ReasonOther, "response status is not a compile-time integer constant; OpenAPI uses 200"))
								statusSet[200] = struct{}{}
							}
						} else if st, ok := staticHTTPStatus(call); ok {
							statusSet[st] = struct{}{}
						} else {
							// dynamic status: still record a success response (default 200 in OpenAPI fallback)
							info.Diagnostics = append(info.Diagnostics, p.diagAt(call, ReasonOther, "response status is not a compile-time integer constant; OpenAPI uses 200"))
							statusSet[200] = struct{}{}
						}
					}
				}
			case CallStreamCreate:
				if len(typeArgs) > typeIndex {
					if reason := typeArgIssue(typeArgs[typeIndex]); reason != "" {
						info.Diagnostics = append(info.Diagnostics, p.diagAt(call, reason, "NewStream type argument is not a same-package plain named type"))
					} else {
						elem := typeArgStrs[typeIndex]
						if info.Stream == "" {
							info.Stream = elem
						}
						if info.Response == "" {
							info.Response = "httpbind.Stream[" + elem + "]"
						}
						statusSet[200] = struct{}{}
					}
				}
			case CallSocketReceive:
				if len(typeArgs) > typeIndex {
					if reason := typeArgIssue(typeArgs[typeIndex]); reason != "" {
						info.Diagnostics = append(info.Diagnostics, p.diagAt(call, reason, "WebSocket inbound type argument is not a same-package plain named type"))
					} else if info.SocketIn == "" {
						info.SocketIn = typeArgStrs[typeIndex]
					}
				}
			case CallSocketSend:
				if len(typeArgs) > typeIndex {
					if reason := typeArgIssue(typeArgs[typeIndex]); reason != "" {
						info.Diagnostics = append(info.Diagnostics, p.diagAt(call, reason, "WebSocket outbound type argument is not a same-package plain named type"))
					} else if info.SocketOut == "" {
						info.SocketOut = typeArgStrs[typeIndex]
					}
				}
			case CallErrorResponse:
				if pattern.ErrorName != "" {
					errorSet[pattern.ErrorName] = struct{}{}
				}
			}
		}
		return true
	})
	if len(errorSet) > 0 {
		info.Errors = make([]string, 0, len(errorSet))
		for k := range errorSet {
			info.Errors = append(info.Errors, k)
		}
		sort.Strings(info.Errors)
	}
	if len(statusSet) > 0 {
		info.SuccessStatuses = make([]int, 0, len(statusSet))
		for s := range statusSet {
			info.SuccessStatuses = append(info.SuccessStatuses, s)
		}
		sort.Ints(info.SuccessStatuses)
	}
	return info
}

func (p *packageParser) callArgumentTypeName(call *ast.CallExpr, index int) string {
	if p.info == nil || p.pkg == nil || p.pkg.Types == nil || call == nil || index < 0 || len(call.Args) <= index {
		return ""
	}
	value := p.info.TypeOf(call.Args[index])
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	named, ok := value.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != p.pkg.Types.Path() {
		return ""
	}
	return named.Obj().Name()
}

func (p *packageParser) instantiatedTypeArgumentName(fun ast.Expr, index int) string {
	if p.info == nil || p.pkg == nil || p.pkg.Types == nil || index < 0 {
		return ""
	}
	for {
		switch expr := stripParens(fun).(type) {
		case *ast.IndexExpr:
			fun = expr.X
		case *ast.IndexListExpr:
			fun = expr.X
		case *ast.SelectorExpr:
			instance, ok := p.info.Instances[expr.Sel]
			if !ok || instance.TypeArgs.Len() <= index {
				return ""
			}
			return localNamedTypeName(instance.TypeArgs.At(index), p.pkg.Types.Path())
		case *ast.Ident:
			instance, ok := p.info.Instances[expr]
			if !ok || instance.TypeArgs.Len() <= index {
				return ""
			}
			return localNamedTypeName(instance.TypeArgs.At(index), p.pkg.Types.Path())
		default:
			return ""
		}
	}
}

func localNamedTypeName(value types.Type, packagePath string) string {
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	named, ok := value.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != packagePath {
		return ""
	}
	return named.Obj().Name()
}

func (p *packageParser) diagAt(call *ast.CallExpr, reason, message string) Diagnostic {
	d := Diagnostic{Reason: reason, Message: message, OmitsOpenAPI: false}
	if p.fset != nil && call != nil {
		pos := p.fset.Position(call.Pos())
		d.File = pos.Filename
		d.Line = pos.Line
		d.Column = pos.Column
	}
	return d
}

// typeArgIssue returns a diagnostic reason if type arg is unsupported for discovery.
func typeArgIssue(expr ast.Expr) string {
	expr = stripParens(expr)
	switch e := expr.(type) {
	case *ast.Ident:
		// same-package plain name
		return ""
	case *ast.SelectorExpr:
		// pkg.Type — cross-package model
		return ReasonCrossPackageModel
	case *ast.StarExpr:
		return ReasonComplexTypeArg
	case *ast.IndexExpr, *ast.IndexListExpr:
		// Stream[T] is handled as response string; nested generics beyond Ident/selector on Write less common
		// Allow Ident element only for simple forms; IndexExpr for Stream[ChatEvent] uses X=Ident/selector
		if idx, ok := e.(*ast.IndexExpr); ok {
			// e.g. Stream[ChatEvent] or httpbind.Stream[ChatEvent]
			if isStreamIndex(idx) {
				if issue := typeArgIssue(idx.Index); issue != "" {
					return issue
				}
				return ""
			}
		}
		return ReasonComplexTypeArg
	default:
		return ReasonComplexTypeArg
	}
}

func isStreamIndex(idx *ast.IndexExpr) bool {
	switch x := stripParens(idx.X).(type) {
	case *ast.Ident:
		return x.Name == "Stream"
	case *ast.SelectorExpr:
		return x.Sel != nil && x.Sel.Name == "Stream"
	}
	return false
}

func genericTypeArgExprs(call *ast.CallExpr) []ast.Expr {
	if call == nil {
		return nil
	}
	switch fun := call.Fun.(type) {
	case *ast.IndexExpr:
		return []ast.Expr{fun.Index}
	case *ast.IndexListExpr:
		return fun.Indices
	default:
		return nil
	}
}

// staticHTTPStatus resolves WriteStatus status argument (index 2) when constant.
func staticHTTPStatus(call *ast.CallExpr) (int, bool) {
	return staticIntArgument(call, 2)
}

func staticIntArgument(call *ast.CallExpr, index int) (int, bool) {
	if call == nil || index < 0 || len(call.Args) <= index {
		return 0, false
	}
	return evalStatusExpr(call.Args[index])
}

func evalStatusExpr(expr ast.Expr) (int, bool) {
	expr = stripParens(expr)
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return 0, false
		}
		n, err := strconv.Atoi(e.Value)
		if err != nil {
			return 0, false
		}
		return n, true
	case *ast.SelectorExpr:
		// http.StatusCreated etc.
		if e.Sel == nil {
			return 0, false
		}
		if id, ok := e.X.(*ast.Ident); ok && (id.Name == "http" || id.Name == "http_") {
			if n, ok := httpStatusName(e.Sel.Name); ok {
				return n, true
			}
		}
		// bare StatusCreated if dot-imported — still accept common names
		if n, ok := httpStatusName(e.Sel.Name); ok {
			return n, true
		}
	case *ast.Ident:
		if n, ok := httpStatusName(e.Name); ok {
			return n, true
		}
	}
	return 0, false
}

func httpStatusName(name string) (int, bool) {
	// Common net/http status name constants used in apps.
	switch name {
	case "StatusOK":
		return 200, true
	case "StatusCreated":
		return 201, true
	case "StatusAccepted":
		return 202, true
	case "StatusNoContent":
		return 204, true
	case "StatusMovedPermanently":
		return 301, true
	case "StatusFound":
		return 302, true
	case "StatusBadRequest":
		return 400, true
	case "StatusUnauthorized":
		return 401, true
	case "StatusForbidden":
		return 403, true
	case "StatusNotFound":
		return 404, true
	case "StatusConflict":
		return 409, true
	case "StatusInternalServerError":
		return 500, true
	default:
		if strings.HasPrefix(name, "Status") {
			// unknown Status* — not static to us
			return 0, false
		}
		return 0, false
	}
}

// parseResponseType turns Write type arg into response name + optional stream element.
// Handles: CreateUserResponse, Stream[ChatEvent], httpbind.Stream[ChatEvent]
func parseResponseType(typeStr string) (response, streamElem string) {
	s := typeStr
	if i := lastIndexStream(s); i >= 0 {
		inner := extractBracketContent(s[i:])
		if inner != "" {
			return s, inner
		}
	}
	return s, ""
}

func lastIndexStream(s string) int {
	for i := 0; i+7 <= len(s); i++ {
		if s[i:i+7] == "Stream[" {
			return i
		}
	}
	return -1
}

func extractBracketContent(s string) string {
	start := -1
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ']':
			depth--
			if depth == 0 && start >= 0 {
				return s[start:i]
			}
		}
	}
	return ""
}

func typeExprString(expr ast.Expr) string {
	expr = stripParens(expr)
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		left := typeExprString(e.X)
		if e.Sel == nil {
			return left
		}
		if left == "" {
			return e.Sel.Name
		}
		return left + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + typeExprString(e.X)
	case *ast.IndexExpr:
		return typeExprString(e.X) + "[" + typeExprString(e.Index) + "]"
	case *ast.IndexListExpr:
		parts := make([]string, 0, len(e.Indices))
		for _, idx := range e.Indices {
			parts = append(parts, typeExprString(idx))
		}
		return typeExprString(e.X) + "[" + joinComma(parts) + "]"
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + typeExprString(e.Elt)
		}
		return "[" + typeExprString(e.Len) + "]" + typeExprString(e.Elt)
	case *ast.MapType:
		return "map[" + typeExprString(e.Key) + "]" + typeExprString(e.Value)
	default:
		return ""
	}
}

func joinComma(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += ", " + parts[i]
	}
	return out
}
