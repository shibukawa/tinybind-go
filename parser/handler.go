package parser

import (
	"go/ast"
	"go/token"

	"github.com/shibukawa/tinybind-go/internal/godoc"
)

// resolveHandler classifies the leaf expression and returns its body for analysis.
func (p *packageParser) resolveHandler(leaf ast.Expr) (Handler, *ast.BlockStmt) {
	leaf = stripParens(leaf)
	switch e := leaf.(type) {
	case *ast.FuncLit:
		return Handler{Form: "inline"}, e.Body
	case *ast.Ident:
		if fd, ok := p.funcs[e.Name]; ok {
			return Handler{Form: "named", Name: e.Name, Doc: godoc.Text(fd.Doc)}, fd.Body
		}
		// unknown named reference in package — still record as named without body
		return Handler{Form: "named", Name: e.Name}, nil
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return p.resolveStructHandler(e.X)
		}
	case *ast.CompositeLit:
		return p.resolveStructHandler(e)
	case *ast.CallExpr:
		// leftover call that wasn't unwrapped — fail
		return Handler{}, nil
	case *ast.SelectorExpr:
		if e.Sel != nil {
			return Handler{Form: "named", Name: e.Sel.Name}, nil
		}
	}
	return Handler{}, nil
}

func (p *packageParser) resolveStructHandler(expr ast.Expr) (Handler, *ast.BlockStmt) {
	expr = stripParens(expr)
	typeName := ""
	switch e := expr.(type) {
	case *ast.CompositeLit:
		typeName = typeNameOf(e.Type)
	case *ast.Ident:
		typeName = e.Name
	case *ast.SelectorExpr:
		if e.Sel != nil {
			typeName = e.Sel.Name
		}
	}
	if typeName == "" {
		return Handler{}, nil
	}
	serve := p.findServeHTTP(typeName)
	handler := Handler{Form: "struct", Name: typeName, Doc: p.typeDocs[typeName]}
	if handler.Doc == "" && serve != nil {
		handler.Doc = godoc.Text(serve.Doc)
	}
	if serve == nil {
		return handler, nil
	}
	return handler, serve.Body
}

func typeNameOf(expr ast.Expr) string {
	expr = stripParens(expr)
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if e.Sel != nil {
			return e.Sel.Name
		}
	case *ast.StarExpr:
		return typeNameOf(e.X)
	}
	return ""
}

func (p *packageParser) findServeHTTP(typeName string) *ast.FuncDecl {
	for _, f := range p.files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Name == nil || fd.Name.Name != "ServeHTTP" {
				continue
			}
			if recvTypeName(fd.Recv) == typeName {
				return fd
			}
		}
	}
	return nil
}

func recvTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
