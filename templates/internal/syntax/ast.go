package syntax

// Module is the format-neutral root of a template source file.
type Module struct {
	Pos          Position      `json:"pos"`
	Package      *PackageDecl  `json:"package,omitempty"`
	Imports      []ImportDecl  `json:"imports,omitempty"`
	Declarations []Declaration `json:"declarations"`
}

type PackageDecl struct {
	Kind string   `json:"kind"`
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
}

type ImportDecl struct {
	Pos   Position `json:"pos"`
	Path  string   `json:"path"`
	Alias string   `json:"alias,omitempty"`
}

// Declaration is implemented by all root declarations.
type Declaration interface {
	declarationNode()
}

type Field struct {
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
	Type TypeRef  `json:"type"`
}

type TypeDecl struct {
	Kind   string   `json:"kind"`
	Pos    Position `json:"pos"`
	Name   string   `json:"name"`
	Fields []Field  `json:"fields"`
}

func (*TypeDecl) declarationNode() {}

type EnumDecl struct {
	Kind    string       `json:"kind"`
	Pos     Position     `json:"pos"`
	Name    string       `json:"name"`
	Members []EnumMember `json:"members"`
}

func (*EnumDecl) declarationNode() {}

type EnumMember struct {
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
}

type ExternalDecl struct {
	Kind       string      `json:"kind"`
	Pos        Position    `json:"pos"`
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters,omitempty"`
	Result     TypeRef     `json:"result"`
}

func (*ExternalDecl) declarationNode() {}

type TemplateDecl struct {
	Kind     string   `json:"kind"`
	Pos      Position `json:"pos"`
	Exported bool     `json:"exported"`
	// Reloadable marks a component the browser may re-render on its own. It is
	// an opt-in because it publishes an HTTP endpoint whose parameters the
	// caller supplies.
	Reloadable bool        `json:"reloadable,omitempty"`
	Name       string      `json:"name"`
	Parameters []Parameter `json:"parameters,omitempty"`
	Output     TypeRef     `json:"output"`
	Body       any         `json:"body"`
}

func (*TemplateDecl) declarationNode() {}

// Node is a body AST node produced either by the shared parser or a registered
// format parser. Type IDs are namespaced as <language>:<node-type>.
type Node interface {
	NodeType() string
}

type ExpressionNode struct {
	Kind       string   `json:"kind"`
	Pos        Position `json:"pos"`
	Context    string   `json:"context"`
	Expression Expr     `json:"expression"`
}

func (n *ExpressionNode) NodeType() string { return n.Kind }

type IfNode struct {
	Kind      string   `json:"kind"`
	Pos       Position `json:"pos"`
	Context   string   `json:"context"`
	Condition Expr     `json:"condition"`
	Then      []Node   `json:"then"`
	Else      []Node   `json:"else,omitempty"`
}

func (n *IfNode) NodeType() string { return n.Kind }

type ForNode struct {
	Kind     string   `json:"kind"`
	Pos      Position `json:"pos"`
	Context  string   `json:"context"`
	Variable string   `json:"variable"`
	Index    string   `json:"index,omitempty"`
	Iterable Expr     `json:"iterable"`
	Body     []Node   `json:"body"`
}

func (n *ForNode) NodeType() string { return n.Kind }

type Parameter struct {
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
	Type TypeRef  `json:"type"`
}

// TypeRef represents named, generic, array, and optional types without binding
// them to Go types during parsing.
type TypeRef struct {
	Pos       Position  `json:"pos"`
	Name      string    `json:"name"`
	Arguments []TypeRef `json:"arguments,omitempty"`
	Array     bool      `json:"array,omitempty"`
	Optional  bool      `json:"optional,omitempty"`
}

// Expr is the shared expression AST embedded by every output format.
type Expr interface {
	exprNode()
}

type IdentifierExpr struct {
	Kind string   `json:"kind"`
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
}

func (*IdentifierExpr) exprNode() {}

type LiteralExpr struct {
	Kind      string   `json:"kind"`
	Pos       Position `json:"pos"`
	ValueKind string   `json:"valueKind"`
	Value     any      `json:"value"`
}

func (*LiteralExpr) exprNode() {}

type MemberExpr struct {
	Kind   string   `json:"kind"`
	Pos    Position `json:"pos"`
	Object Expr     `json:"object"`
	Member string   `json:"member"`
}

func (*MemberExpr) exprNode() {}

type IndexExpr struct {
	Kind   string   `json:"kind"`
	Pos    Position `json:"pos"`
	Object Expr     `json:"object"`
	Index  Expr     `json:"index"`
}

func (*IndexExpr) exprNode() {}

type CallExpr struct {
	Kind      string   `json:"kind"`
	Pos       Position `json:"pos"`
	Callee    Expr     `json:"callee"`
	Arguments []Expr   `json:"arguments,omitempty"`
}

func (*CallExpr) exprNode() {}

type UnaryExpr struct {
	Kind     string   `json:"kind"`
	Pos      Position `json:"pos"`
	Operator string   `json:"operator"`
	Operand  Expr     `json:"operand"`
}

func (*UnaryExpr) exprNode() {}

type BinaryExpr struct {
	Kind     string   `json:"kind"`
	Pos      Position `json:"pos"`
	Operator string   `json:"operator"`
	Left     Expr     `json:"left"`
	Right    Expr     `json:"right"`
}

func (*BinaryExpr) exprNode() {}

type ConditionalExpr struct {
	Kind      string   `json:"kind"`
	Pos       Position `json:"pos"`
	Condition Expr     `json:"condition"`
	Then      Expr     `json:"then"`
	Else      Expr     `json:"else"`
}

func (*ConditionalExpr) exprNode() {}
