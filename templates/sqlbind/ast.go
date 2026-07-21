// Package sqlbind parses and compiles typed SQL template sources.
package sqlbind

import "github.com/shibukawa/tinybind-go/templates/internal/syntax"

type Module = syntax.Module
type Declaration = syntax.Declaration
type TypeDecl = syntax.TypeDecl
type EnumDecl = syntax.EnumDecl
type ExternalDecl = syntax.ExternalDecl
type TemplateDecl = syntax.TemplateDecl
type Field = syntax.Field
type Parameter = syntax.Parameter
type TypeRef = syntax.TypeRef
type Position = syntax.Position
type Expr = syntax.Expr
type IdentifierExpr = syntax.IdentifierExpr
type LiteralExpr = syntax.LiteralExpr
type MemberExpr = syntax.MemberExpr
type IndexExpr = syntax.IndexExpr
type CallExpr = syntax.CallExpr
type UnaryExpr = syntax.UnaryExpr
type BinaryExpr = syntax.BinaryExpr
type ConditionalExpr = syntax.ConditionalExpr
type ExpressionNode = syntax.ExpressionNode
type IfNode = syntax.IfNode
type ForNode = syntax.ForNode
type ParseError = syntax.ParseError
type Node = syntax.Node
type Body = []Node

type TextNode struct {
	Kind string   `json:"kind"`
	Pos  Position `json:"pos"`
	Text string   `json:"text"`
}

func (n *TextNode) NodeType() string { return n.Kind }

// RelationNode is a structurally embedded private sql.relation<T> statement.
// It is emitted into the caller's builder, so placeholders remain globally
// ordered after runtime conditions are resolved.
type RelationNode struct {
	Kind      string   `json:"kind"`
	Pos       Position `json:"pos"`
	Name      string   `json:"name"`
	Arguments []Expr   `json:"arguments,omitempty"`
	Alias     string   `json:"alias"`
}

func (n *RelationNode) NodeType() string { return n.Kind }
