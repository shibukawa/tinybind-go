package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// unwrapHandler peels stdlib wrappers and best-effort custom middleware until
// a statically analyzable leaf handler expression is found.
func unwrapHandler(expr ast.Expr, meta WrapperMeta) (ast.Expr, WrapperMeta, bool) {
	expr = stripParens(expr)
	switch e := expr.(type) {
	case *ast.CallExpr:
		return unwrapCall(e, meta)
	case *ast.UnaryExpr:
		// &UserHandler{} or &UserHandler{...}
		if e.Op == token.AND {
			return e, meta, true
		}
		return nil, meta, false
	case *ast.CompositeLit:
		return e, meta, true
	case *ast.Ident:
		return e, meta, true
	case *ast.FuncLit:
		return e, meta, true
	case *ast.SelectorExpr:
		// pkg.Handler variable — not a leaf we can analyze as body, but treat as named if possible
		return e, meta, true
	default:
		return nil, meta, false
	}
}

func unwrapCall(call *ast.CallExpr, meta WrapperMeta) (ast.Expr, WrapperMeta, bool) {
	// http.HandlerFunc(fn)
	if isNetHTTPCall(call, "HandlerFunc") && len(call.Args) >= 1 {
		return unwrapHandler(call.Args[0], meta)
	}
	// http.AllowQuerySemicolons(h)
	if isNetHTTPCall(call, "AllowQuerySemicolons") && len(call.Args) >= 1 {
		return unwrapHandler(call.Args[0], meta)
	}
	// http.MaxBytesHandler(h, n)
	if isNetHTTPCall(call, "MaxBytesHandler") && len(call.Args) >= 2 {
		if n, ok := evalInt64(call.Args[1]); ok {
			meta.MaxRequestBodyBytes = &n
		}
		return unwrapHandler(call.Args[0], meta)
	}
	// http.StripPrefix(prefix, h)
	if isNetHTTPCall(call, "StripPrefix") && len(call.Args) >= 2 {
		return unwrapHandler(call.Args[1], meta)
	}
	// http.TimeoutHandler(h, dt, msg)
	if isNetHTTPCall(call, "TimeoutHandler") && len(call.Args) >= 3 {
		if d, ok := evalDuration(call.Args[1]); ok {
			meta.Timeout = d
		}
		return unwrapHandler(call.Args[0], meta)
	}

	// Best-effort custom middleware: Foo(h) or Foo(h, ...) where one arg
	// looks like a handler expression (HandlerFunc, nested call, func lit, &Struct{}).
	if leaf, m, ok := unwrapCustomMiddleware(call, meta); ok {
		return leaf, m, true
	}
	return nil, meta, false
}

func unwrapCustomMiddleware(call *ast.CallExpr, meta WrapperMeta) (ast.Expr, WrapperMeta, bool) {
	// Avoid treating route registration as middleware.
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel != nil {
		switch sel.Sel.Name {
		case "Handle", "HandleFunc", "ListenAndServe", "ListenAndServeTLS":
			return nil, meta, false
		}
	}
	// Prefer the last argument that unwraps successfully as a handler.
	for i := len(call.Args) - 1; i >= 0; i-- {
		arg := stripParens(call.Args[i])
		if !looksLikeHandlerExpr(arg) {
			continue
		}
		if leaf, m, ok := unwrapHandler(arg, meta); ok {
			return leaf, m, true
		}
	}
	return nil, meta, false
}

func looksLikeHandlerExpr(expr ast.Expr) bool {
	expr = stripParens(expr)
	switch e := expr.(type) {
	case *ast.Ident, *ast.FuncLit, *ast.SelectorExpr:
		return true
	case *ast.UnaryExpr:
		return e.Op == token.AND
	case *ast.CompositeLit:
		return true
	case *ast.CallExpr:
		if isNetHTTPCall(e, "HandlerFunc") ||
			isNetHTTPCall(e, "AllowQuerySemicolons") ||
			isNetHTTPCall(e, "MaxBytesHandler") ||
			isNetHTTPCall(e, "StripPrefix") ||
			isNetHTTPCall(e, "TimeoutHandler") {
			return true
		}
		// nested middleware call
		return true
	default:
		return false
	}
}

func isNetHTTPCall(call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != name {
		return false
	}
	// X may be Ident("http") or any alias; accept selector name only for stdlib wrappers.
	// For HandlerFunc / wrappers we require the package-like ident to be "http"
	// OR any selector (in case of import alias). We accept Ident named http, or any Ident.
	switch x := sel.X.(type) {
	case *ast.Ident:
		// common case: http.Xxx
		_ = x
		return true
	default:
		return false
	}
}

func stripParens(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

func evalInt64(expr ast.Expr) (int64, bool) {
	expr = stripParens(expr)
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return 0, false
		}
		n, err := strconv.ParseInt(e.Value, 0, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	case *ast.UnaryExpr:
		if e.Op == token.SUB {
			n, ok := evalInt64(e.X)
			if !ok {
				return 0, false
			}
			return -n, true
		}
		if e.Op == token.ADD {
			return evalInt64(e.X)
		}
	case *ast.BinaryExpr:
		l, ok1 := evalInt64(e.X)
		r, ok2 := evalInt64(e.Y)
		if !ok1 || !ok2 {
			return 0, false
		}
		switch e.Op {
		case token.ADD:
			return l + r, true
		case token.SUB:
			return l - r, true
		case token.MUL:
			return l * r, true
		case token.QUO:
			if r == 0 {
				return 0, false
			}
			return l / r, true
		case token.SHL:
			return l << uint64(r), true
		case token.SHR:
			return l >> uint64(r), true
		}
	}
	return 0, false
}

// evalDuration evaluates simple duration expressions like 30*time.Second.
func evalDuration(expr ast.Expr) (string, bool) {
	expr = stripParens(expr)
	// time.Second, time.Minute, etc. → normalize as 1s / 1m / ...
	if unit, ok := timeUnit(expr); ok {
		return formatDuration(1, unit)
	}
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok || bin.Op != token.MUL {
		// bare integer nanoseconds? skip
		if n, ok := evalInt64(expr); ok {
			return fmt.Sprintf("%dns", n), true
		}
		return "", false
	}
	// n * time.Unit  or  time.Unit * n
	if n, ok := evalInt64(bin.X); ok {
		if unit, ok := timeUnit(bin.Y); ok {
			return formatDuration(n, unit)
		}
	}
	if n, ok := evalInt64(bin.Y); ok {
		if unit, ok := timeUnit(bin.X); ok {
			return formatDuration(n, unit)
		}
	}
	return "", false
}

func timeUnit(expr ast.Expr) (string, bool) {
	expr = stripParens(expr)
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil {
		return "", false
	}
	switch sel.Sel.Name {
	case "Nanosecond", "Microsecond", "Millisecond", "Second", "Minute", "Hour":
		// require time-like package
		if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "time" || id.Name != "") {
			return strings.ToLower(sel.Sel.Name), true
		}
	}
	return "", false
}

func formatDuration(n int64, unit string) (string, bool) {
	// normalize to common short form: 30s, 5m, etc.
	switch unit {
	case "nanosecond":
		return fmt.Sprintf("%dns", n), true
	case "microsecond":
		return fmt.Sprintf("%dµs", n), true
	case "millisecond":
		return fmt.Sprintf("%dms", n), true
	case "second":
		return fmt.Sprintf("%ds", n), true
	case "minute":
		return fmt.Sprintf("%dm", n), true
	case "hour":
		return fmt.Sprintf("%dh", n), true
	default:
		return "", false
	}
}
