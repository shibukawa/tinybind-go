// Package htmlbind parses typed HTML template sources into an AST.
package htmlbind

import "github.com/shibukawa/tinybind-go/templates/internal/syntax"

type Module = syntax.Module
type PackageDecl = syntax.PackageDecl
type ImportDecl = syntax.ImportDecl
type Declaration = syntax.Declaration
type TypeDecl = syntax.TypeDecl
type EnumDecl = syntax.EnumDecl
type EnumMember = syntax.EnumMember
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
type MessageExpr = syntax.MessageExpr
type MessageArg = syntax.MessageArg
type ParseError = syntax.ParseError
type ExpressionNode = syntax.ExpressionNode
type IfNode = syntax.IfNode
type ForNode = syntax.ForNode

type AwaitNode = syntax.AwaitNode
type AwaitBinding = syntax.AwaitBinding
type ValNode = syntax.ValNode
type ValBinding = syntax.ValBinding
type Annotation = syntax.Annotation

type Node = syntax.Node

// Body is the body stored in TemplateDecl.Body.
type Body = []Node

// messageInnerNode marks where a rich-text hole's translated text goes. It is
// synthesized at emission from the bound element's empty content, so no author
// writes one and no analysis sees one.
type messageInnerNode struct{ Pos Position }

func (n *messageInnerNode) NodeType() string { return "html:message-inner" }

type TextNode struct {
	Kind string   `json:"kind"`
	Pos  Position `json:"pos"`
	Text string   `json:"text"`
	// Start and End are file-global byte offsets of the source this text came
	// from, for a tool that rewrites a template in place. They are excluded
	// from the serialized AST because they are a tool-facing detail rather than
	// part of the parse's published shape, and because adding them there would
	// move every parser fixture.
	//
	// The range is source rather than content: an escaped brace contributes one
	// character to Text and two to the range, so a rewriter replacing the range
	// replaces the escape as well, which is what an extractor wants. See
	// .knowledge requirement:template-parse-introspection.
	Start int `json:"-"`
	End   int `json:"-"`
}

func (n *TextNode) NodeType() string { return n.Kind }

type CommentNode struct {
	Kind string   `json:"kind"`
	Pos  Position `json:"pos"`
	Text string   `json:"text"`
}

func (n *CommentNode) NodeType() string { return n.Kind }

type DoctypeNode struct {
	Kind string   `json:"kind"`
	Pos  Position `json:"pos"`
	Text string   `json:"text"`
}

func (n *DoctypeNode) NodeType() string { return n.Kind }

type ElementNode struct {
	Kind        string      `json:"kind"`
	Pos         Position    `json:"pos"`
	Name        string      `json:"name"`
	Attributes  []Attribute `json:"attributes,omitempty"`
	Children    []Node      `json:"children,omitempty"`
	SelfClosing bool        `json:"selfClosing,omitempty"`
}

func (n *ElementNode) NodeType() string { return n.Kind }

// HeadNode is a head element declared outside the document shell. Its children
// are hoisted into the merged document head instead of being emitted in place.
type HeadNode struct {
	Kind     string   `json:"kind"`
	Pos      Position `json:"pos"`
	Children []Node   `json:"children,omitempty"`
}

func (n *HeadNode) NodeType() string { return n.Kind }

// SlotNode marks where a bound html parameter is inserted. Name is empty for
// the reserved children parameter. Default holds the content rendered when the
// bound argument is absent.
type SlotNode struct {
	Kind     string   `json:"kind"`
	Pos      Position `json:"pos"`
	Name     string   `json:"name,omitempty"`
	Required bool     `json:"required,omitempty"`
	Default  []Node   `json:"default,omitempty"`
}

func (n *SlotNode) NodeType() string { return n.Kind }

// Parameter reports the component parameter this slot binds to.
func (n *SlotNode) Parameter() string {
	if n.Name == "" {
		return "children"
	}
	return n.Name
}

type ComponentNode struct {
	Kind        string      `json:"kind"`
	Pos         Position    `json:"pos"`
	Name        string      `json:"name"`
	Arguments   []Attribute `json:"arguments,omitempty"`
	Children    []Node      `json:"children,omitempty"`
	SelfClosing bool        `json:"selfClosing,omitempty"`
}

func (n *ComponentNode) NodeType() string { return n.Kind }

type Attribute struct {
	Kind    string          `json:"kind"`
	Pos     Position        `json:"pos"`
	Name    string          `json:"name"`
	Boolean bool            `json:"boolean,omitempty"`
	Value   []AttributePart `json:"value,omitempty"`
}

type AttributePart struct {
	Kind       string   `json:"kind"`
	Pos        Position `json:"pos"`
	Context    string   `json:"context,omitempty"`
	Text       string   `json:"text,omitempty"`
	Expression Expr     `json:"expression,omitempty"`
	// Start and End are file-global byte offsets of the source this part came
	// from, on the same terms as TextNode.Start: excluded from the serialized
	// AST, and a range of source rather than of content.
	Start int `json:"-"`
	End   int `json:"-"`
}
