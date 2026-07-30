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
	Kind string   `json:"kind"`
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
	// Async marks a function that runs concurrently and may fail. It is a
	// keyword rather than an annotation because it changes the Go signature the
	// package must provide.
	Async bool `json:"async,omitempty"`
	// Live marks a function that yields many values over time rather than
	// settling once. It is a keyword for the same reason Async is: the Go
	// signature becomes an iter.Seq2 over the result type, with a leading
	// context that is mandatory rather than optional, because an endless source
	// has to be stoppable.
	Live       bool        `json:"live,omitempty"`
	Parameters []Parameter `json:"parameters,omitempty"`
	Result     TypeRef     `json:"result"`
}

func (*ExternalDecl) declarationNode() {}

// Annotation is one `@name(key: "value")` line attached to the declaration
// below it. The shared parser owns the grammar; each output format decides
// which names it accepts, so an unknown name is a generation error rather than
// a silently ignored line.
type Annotation struct {
	Pos  Position        `json:"pos"`
	Name string          `json:"name"`
	Args []AnnotationArg `json:"args,omitempty"`
}

// Argument returns the value of a named annotation argument.
func (a Annotation) Argument(name string) (AnnotationArg, bool) {
	for _, arg := range a.Args {
		if arg.Name == name {
			return arg, true
		}
	}
	return AnnotationArg{}, false
}

type AnnotationArg struct {
	Pos   Position `json:"pos"`
	Name  string   `json:"name"`
	Value string   `json:"value"`
}

type TemplateDecl struct {
	Kind        string       `json:"kind"`
	Pos         Position     `json:"pos"`
	Exported    bool         `json:"exported"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Name        string       `json:"name"`
	Parameters  []Parameter  `json:"parameters,omitempty"`
	Output      TypeRef      `json:"output"`
	Body        any          `json:"body"`
}

// Annotation returns the declaration's annotation with the given name.
func (d *TemplateDecl) Annotation(name string) (Annotation, bool) {
	for _, annotation := range d.Annotations {
		if annotation.Name == name {
			return annotation, true
		}
	}
	return Annotation{}, false
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

// AwaitNode is one asynchronous boundary. Its bindings run concurrently; the
// primary subtree reads them, the fallback subtree is emitted while they are
// pending, and the optional recover subtree replaces the fallback on failure.
//
// How many times the boundary renders is a property of what its bindings name,
// not of the clause: a binding on a settle-once source produces one render, and
// a binding on a live source produces one per delivery. The clause says which
// values the subtree waits for, and the declarations say how those values
// arrive, so nothing here has to be repeated at the wait site.
type AwaitNode struct {
	Kind     string         `json:"kind"`
	Pos      Position       `json:"pos"`
	Context  string         `json:"context"`
	Bindings []AwaitBinding `json:"bindings"`
	Primary  []Node         `json:"primary"`
	Fallback []Node         `json:"fallback"`
	// HasRecover distinguishes a declared but empty recover subtree from an
	// omitted one, which keeps the committed fallback instead.
	HasRecover bool     `json:"hasRecover,omitempty"`
	Recover    []Node   `json:"recover,omitempty"`
	ErrorName  string   `json:"errorName,omitempty"`
	ErrorPos   Position `json:"errorPos,omitempty"`
}

func (n *AwaitNode) NodeType() string { return n.Kind }

// AwaitBinding names one asynchronous call whose result the primary subtree
// reads.
type AwaitBinding struct {
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
	Call Expr     `json:"call"`
}

type Parameter struct {
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
	Type TypeRef  `json:"type"`
}

// TypeRef represents named, generic, array, optional, and asynchronous types
// without binding them to Go types during parsing.
type TypeRef struct {
	Pos       Position  `json:"pos"`
	Name      string    `json:"name"`
	Arguments []TypeRef `json:"arguments,omitempty"`
	Array     bool      `json:"array,omitempty"`
	Optional  bool      `json:"optional,omitempty"`
	// Async marks a value the caller starts and the template waits for in an
	// await clause. It modifies the whole type expression, so `async Order[]`
	// is one pending array rather than an array of pending values.
	Async bool `json:"async,omitempty"`
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
