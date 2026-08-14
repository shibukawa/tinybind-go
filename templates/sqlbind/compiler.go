package sqlbind

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

type valueKind string

const (
	kindInvalid   valueKind = "invalid"
	kindString    valueKind = "string"
	kindBool      valueKind = "bool"
	kindInt       valueKind = "int"
	kindFloat     valueKind = "float"
	kindDecimal   valueKind = "decimal"
	kindDateTime  valueKind = "datetime"
	kindDate      valueKind = "date"
	kindTime      valueKind = "time"
	kindURL       valueKind = "url"
	kindBytes     valueKind = "bytes"
	kindRecord    valueKind = "record"
	kindEnum      valueKind = "enum"
	kindArray     valueKind = "array"
	kindPredicate valueKind = "sql.predicate"
)

type valueType struct {
	kind     valueKind
	name     string
	elem     *valueType
	optional bool
}

func (t valueType) required() valueType { t.optional = false; return t }
func (t valueType) String() string {
	base := t.name
	if base == "" {
		base = string(t.kind)
	}
	if t.kind == kindArray && t.elem != nil {
		base = "[]" + t.elem.String()
	}
	if t.optional {
		base += "?"
	}
	return base
}

type functionSig struct {
	params []valueType
	result valueType
}
type statementInfo struct {
	decl        *TemplateDecl
	params      map[string]valueType
	result      valueType
	cardinality string
	// readOnly is the access mode of rule:sql-statement-access-mode. It selects
	// the Context resolver a write statement must use, so a read-only executor
	// rejects the statement before it reaches the database.
	readOnly bool
}

type compiler struct {
	filename      string
	module        *Module
	records       map[string]*TypeDecl
	enums         map[string]*EnumDecl
	enumMembers   map[string]valueType
	externals     map[string]functionSig
	statements    map[string]*statementInfo
	exprTypes     map[Expr]valueType
	relationCalls map[string][]string
}

type CompileError struct {
	Filename string
	Pos      Position
	Message  string
}

func (e *CompileError) Error() string {
	name := e.Filename
	if name == "" {
		name = "<template>"
	}
	return fmt.Sprintf("%s:%d:%d: %s", name, e.Pos.Line, e.Pos.Col, e.Message)
}

func newCompiler(filename string, module *Module) *compiler {
	return &compiler{filename: filename, module: module, records: map[string]*TypeDecl{}, enums: map[string]*EnumDecl{}, enumMembers: map[string]valueType{}, externals: map[string]functionSig{}, statements: map[string]*statementInfo{}, exprTypes: map[Expr]valueType{}, relationCalls: map[string][]string{}}
}

func (c *compiler) analyze() error {
	for _, declaration := range c.module.Declarations {
		switch d := declaration.(type) {
		case *TypeDecl:
			if c.nameExists(d.Name) {
				return c.error(d.Pos, "duplicate declaration "+d.Name)
			}
			c.records[d.Name] = d
		case *EnumDecl:
			if c.nameExists(d.Name) {
				return c.error(d.Pos, "duplicate declaration "+d.Name)
			}
			c.enums[d.Name] = d
			for _, member := range d.Members {
				if _, exists := c.enumMembers[member.Name]; exists {
					return c.error(member.Pos, "duplicate enum member "+member.Name)
				}
				c.enumMembers[member.Name] = valueType{kind: kindEnum, name: d.Name}
			}
		}
	}
	for _, declaration := range c.module.Declarations {
		switch d := declaration.(type) {
		case *TypeDecl:
			seen := map[string]bool{}
			for _, field := range d.Fields {
				if seen[field.Name] {
					return c.error(field.Pos, "duplicate field "+field.Name)
				}
				seen[field.Name] = true
				if _, err := c.resolveType(field.Type); err != nil {
					return err
				}
			}
		case *ExternalDecl:
			if c.nameExists(d.Name) {
				return c.error(d.Pos, "duplicate declaration "+d.Name)
			}
			var sig functionSig
			for _, p := range d.Parameters {
				t, err := c.resolveType(p.Type)
				if err != nil {
					return err
				}
				sig.params = append(sig.params, t)
			}
			result, err := c.resolveType(d.Result)
			if err != nil {
				return err
			}
			sig.result = result
			c.externals[d.Name] = sig
		case *TemplateDecl:
			if d.Kind != "sql:statement" {
				return c.error(d.Pos, "SQL generator only accepts sql:statement declarations")
			}
			if c.nameExists(d.Name) {
				return c.error(d.Pos, "duplicate declaration "+d.Name)
			}
			cardinality, result, err := c.resolveOutput(d.Output)
			if err != nil {
				return err
			}
			if d.Exported && (cardinality == "predicate" || cardinality == "relation") {
				return c.error(d.Pos, "sql."+cardinality+" statements must be private")
			}
			if err := c.checkStatementName(d, cardinality); err != nil {
				return err
			}
			info := &statementInfo{decl: d, params: map[string]valueType{}, result: result, cardinality: cardinality}
			for _, p := range d.Parameters {
				if _, exists := info.params[p.Name]; exists {
					return c.error(p.Pos, "duplicate parameter "+p.Name)
				}
				t, err := c.resolveType(p.Type)
				if err != nil {
					return err
				}
				info.params[p.Name] = t
			}
			c.statements[d.Name] = info
		}
	}
	for _, declaration := range c.module.Declarations {
		d, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		body, ok := d.Body.([]Node)
		if !ok {
			return c.error(d.Pos, "invalid SQL statement body")
		}
		if err := c.analyzeNodes(body, copyScope(c.statements[d.Name].params), d.Name); err != nil {
			return err
		}
		c.statements[d.Name].readOnly = isReadOnly(body)
		if err := c.checkMutationSafety(d, body); err != nil {
			return err
		}
		if err := c.validateStaticResultShape(d, body); err != nil {
			return err
		}
	}
	if err := c.checkRelationCycles(); err != nil {
		return err
	}
	return nil
}

func (c *compiler) resolveOutput(ref TypeRef) (string, valueType, error) {
	if !strings.HasPrefix(ref.Name, "sql.") {
		return "", valueType{}, c.error(ref.Pos, "unknown SQL output "+ref.Name)
	}
	cardinality := strings.TrimPrefix(ref.Name, "sql.")
	switch cardinality {
	case "exec", "predicate":
		if len(ref.Arguments) != 0 {
			return "", valueType{}, c.error(ref.Pos, ref.Name+" does not accept a result type")
		}
		return cardinality, valueType{}, nil
	case "one", "optional", "many", "relation":
		if len(ref.Arguments) != 1 {
			return "", valueType{}, c.error(ref.Pos, ref.Name+" requires one result type")
		}
		result, err := c.resolveType(ref.Arguments[0])
		if err != nil {
			return "", valueType{}, err
		}
		if result.kind != kindRecord || result.optional || result.elem != nil {
			return "", valueType{}, c.error(ref.Arguments[0].Pos, ref.Name+" result must be a named record")
		}
		return cardinality, result, nil
	default:
		return "", valueType{}, c.error(ref.Pos, "unsupported SQL output "+ref.Name)
	}
}

func (c *compiler) analyzeNodes(nodes []Node, scope map[string]valueType, owner string) error {
	// A binding scopes the nodes after it. Unlike the HTML lowering, nothing is
	// rewritten to say so: generation emits straight-line Go where a control
	// body is already a Go block, so the binding is a local and the target's own
	// block scoping is the scope. The caller always hands this a copy, so adding
	// to it reaches the following nodes and nothing beyond them.
	if binding, ok := syntax.DuplicateValBinding(nodes); ok {
		return c.error(binding.Pos, "duplicate value binding "+binding.Name+
			"; a second binding of one name in the same block is a redeclaration, so rename it or move it inside a nested block to shadow deliberately")
	}
	for index, node := range nodes {
		// What a binding scopes is what follows it, which is also what decides
		// whether anything reads it.
		rest := nodes[index+1:]
		switch n := node.(type) {
		case *TextNode:
		case *ExpressionNode:
			t, err := c.infer(n.Expression, scope)
			if err != nil {
				return err
			}
			if t.kind == kindRecord || t.kind == kindInvalid {
				return c.error(n.Pos, "cannot bind "+t.String()+" as a SQL value")
			}
			if t.kind == kindPredicate {
				call := n.Expression.(*CallExpr)
				id := call.Callee.(*IdentifierExpr)
				c.relationCalls[owner] = append(c.relationCalls[owner], id.Name)
			}
		case *RelationNode:
			target, ok := c.statements[n.Name]
			if !ok || target.cardinality != "relation" {
				return c.error(n.Pos, "unknown sql.relation "+n.Name)
			}
			if len(n.Arguments) != len(target.decl.Parameters) {
				return c.error(n.Pos, fmt.Sprintf("%s expects %d arguments", n.Name, len(target.decl.Parameters)))
			}
			for i, argument := range n.Arguments {
				got, err := c.infer(argument, scope)
				if err != nil {
					return err
				}
				want := target.params[target.decl.Parameters[i].Name]
				if !assignable(want, got) {
					return c.error(exprPos(argument), fmt.Sprintf("argument %d expects %s, got %s", i+1, want, got))
				}
			}
			c.relationCalls[owner] = append(c.relationCalls[owner], n.Name)
		case *IfNode:
			t, err := c.infer(n.Condition, scope)
			if err != nil {
				return err
			}
			if t.kind != kindBool || t.optional {
				return c.error(n.Pos, "if condition must be bool")
			}
			if err := c.analyzeNodes(n.Then, copyScope(scope), owner); err != nil {
				return err
			}
			if err := c.analyzeNodes(n.Else, copyScope(scope), owner); err != nil {
				return err
			}
		case *ValNode:
			for _, binding := range n.Bindings {
				t, err := c.infer(binding.Value, scope)
				if err != nil {
					return err
				}
				// A binding nothing reads still calls its external every time
				// the statement is built, and the result goes nowhere. Left to
				// generation it would surface as an unused Go local, which
				// names a line of emitted code rather than the template line
				// that caused it, so it is refused here instead.
				//
				if !valueRead(rest, binding.Name) {
					return c.error(binding.Pos, "val binding "+binding.Name+
						" is never read; its call would run every time the statement is built and be discarded, so read it or remove it")
				}
				// What reaches a statement is a value. A predicate call is a
				// fragment of the statement itself, so naming one would promise
				// a value position it cannot fill.
				if t.kind == kindPredicate {
					return c.error(binding.Pos, "val binding "+binding.Name+" is "+t.String()+
						", which composes the statement rather than producing a value; call it in place instead of binding it")
				}
				if t.kind == kindInvalid {
					return c.error(binding.Pos, "cannot bind "+t.String()+" as a SQL value")
				}
				scope[binding.Name] = t
			}
		case *ForNode:
			return c.error(n.Pos, "general SQL loops are forbidden; bind an array expression to expand a value list")
		default:
			return c.error(Position{Line: 1, Col: 1}, fmt.Sprintf("unsupported SQL node %T", node))
		}
	}
	return nil
}

// valueRead reports whether anything in a binding's extent reads name. The
// extent is the nodes that follow it: a sibling binding of the same directive
// cannot read it, because parseVal refuses a directive whose bindings depend on
// each other.
//
// A block that rebinds the name is not scanned past that point: a reference
// there resolves to the inner local and leaves the outer one still unread. That
// makes the answer exact rather than conservative, which it has to be, because
// a name wrongly reported unread refuses a working statement.
func valueRead(nodes []Node, name string) bool {
	for _, node := range nodes {
		switch n := node.(type) {
		case *ExpressionNode:
			if syntax.ExprReads(n.Expression, name) {
				return true
			}
		case *RelationNode:
			for _, argument := range n.Arguments {
				if syntax.ExprReads(argument, name) {
					return true
				}
			}
		case *IfNode:
			if syntax.ExprReads(n.Condition, name) || valueRead(n.Then, name) || valueRead(n.Else, name) {
				return true
			}
		case *ValNode:
			for _, binding := range n.Bindings {
				if syntax.ExprReads(binding.Value, name) {
					return true
				}
				if binding.Name == name {
					return false
				}
			}
		}
	}
	return false
}

func (c *compiler) infer(expr Expr, scope map[string]valueType) (valueType, error) {
	if known, ok := c.exprTypes[expr]; ok {
		return known, nil
	}
	var result valueType
	var err error
	switch x := expr.(type) {
	case *IdentifierExpr:
		if t, ok := scope[x.Name]; ok {
			result = t
		} else if t, ok := c.enumMembers[x.Name]; ok {
			result = t
		} else {
			err = c.error(x.Pos, "unknown identifier "+x.Name)
		}
	case *LiteralExpr:
		switch x.ValueKind {
		case "string":
			result.kind = kindString
		case "bool":
			result.kind = kindBool
		case "number":
			if strings.Contains(x.Value.(string), ".") {
				result.kind = kindFloat
			} else {
				result.kind = kindInt
			}
		case "null":
			result = valueType{kind: kindInvalid, optional: true}
		default:
			err = c.error(x.Pos, "unknown literal type")
		}
	case *MemberExpr:
		object, e := c.infer(x.Object, scope)
		err = e
		if err == nil {
			if object.optional {
				err = c.error(x.Pos, "member access on optional "+object.String())
			} else if object.kind != kindRecord {
				err = c.error(x.Pos, "member access requires a record")
			} else if f, ok := findField(c.records[object.name], x.Member); !ok {
				err = c.error(x.Pos, "unknown field "+x.Member+" on "+object.name)
			} else {
				result, err = c.resolveType(f.Type)
			}
		}
	case *IndexExpr:
		object, e := c.infer(x.Object, scope)
		err = e
		var index valueType
		if err == nil {
			index, err = c.infer(x.Index, scope)
		}
		if err == nil && (object.kind != kindArray || object.optional) {
			err = c.error(x.Pos, "indexing requires an array")
		}
		if err == nil && index.kind != kindInt {
			err = c.error(x.Pos, "array index must be int")
		}
		if err == nil {
			result = *object.elem
		}
	case *CallExpr:
		result, err = c.inferCall(x, scope)
	case *UnaryExpr:
		operand, e := c.infer(x.Operand, scope)
		err = e
		if err == nil {
			switch x.Operator {
			case "!", "not":
				if operand.kind != kindBool || operand.optional {
					err = c.error(x.Pos, "not requires bool")
				} else {
					result = operand
				}
			case "+", "-":
				if !numeric(operand) {
					err = c.error(x.Pos, "numeric unary operator requires number")
				} else {
					result = operand
				}
			default:
				err = c.error(x.Pos, "unsupported unary operator "+x.Operator)
			}
		}
	case *BinaryExpr:
		left, e := c.infer(x.Left, scope)
		err = e
		var right valueType
		if err == nil {
			right, err = c.infer(x.Right, scope)
		}
		if err == nil {
			result, err = c.binaryType(x, left, right)
		}
	case *ConditionalExpr:
		condition, e := c.infer(x.Condition, scope)
		err = e
		if err == nil && (condition.kind != kindBool || condition.optional) {
			err = c.error(x.Pos, "conditional condition must be bool")
		}
		var a, b valueType
		if err == nil {
			a, err = c.infer(x.Then, scope)
		}
		if err == nil {
			b, err = c.infer(x.Else, scope)
		}
		if err == nil {
			if !assignable(a, b) || !assignable(b, a) {
				err = c.error(x.Pos, "conditional branches must have the same type")
			} else {
				result = a
			}
		}
	default:
		err = c.error(Position{Line: 1, Col: 1}, fmt.Sprintf("unsupported expression %T", expr))
	}
	if err != nil {
		return valueType{}, err
	}
	c.exprTypes[expr] = result
	return result, nil
}

func (c *compiler) inferCall(call *CallExpr, scope map[string]valueType) (valueType, error) {
	id, ok := call.Callee.(*IdentifierExpr)
	if !ok {
		return valueType{}, c.error(call.Pos, "only named functions can be called")
	}
	if statement, ok := c.statements[id.Name]; ok && statement.cardinality == "predicate" {
		if len(call.Arguments) != len(statement.decl.Parameters) {
			return valueType{}, c.error(call.Pos, fmt.Sprintf("%s expects %d arguments", id.Name, len(statement.decl.Parameters)))
		}
		for i, argument := range call.Arguments {
			got, err := c.infer(argument, scope)
			if err != nil {
				return valueType{}, err
			}
			want := statement.params[statement.decl.Parameters[i].Name]
			if !assignable(want, got) {
				return valueType{}, c.error(exprPos(argument), fmt.Sprintf("argument %d expects %s, got %s", i+1, want, got))
			}
		}
		return valueType{kind: kindPredicate}, nil
	}
	sig, ok := c.externals[id.Name]
	if !ok {
		return valueType{}, c.error(call.Pos, "unknown function "+id.Name)
	}
	if len(call.Arguments) != len(sig.params) {
		return valueType{}, c.error(call.Pos, fmt.Sprintf("%s expects %d arguments", id.Name, len(sig.params)))
	}
	for i, arg := range call.Arguments {
		got, err := c.infer(arg, scope)
		if err != nil {
			return valueType{}, err
		}
		if !assignable(sig.params[i], got) {
			return valueType{}, c.error(exprPos(arg), fmt.Sprintf("argument %d expects %s, got %s", i+1, sig.params[i], got))
		}
	}
	return sig.result, nil
}

func (c *compiler) checkRelationCycles() error {
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return c.error(c.statements[name].decl.Pos, "recursive SQL composition involving "+name)
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		for _, next := range c.relationCalls[name] {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[name] = 2
		return nil
	}
	for name := range c.statements {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) validateStaticResultShape(statement *TemplateDecl, nodes []Node) error {
	info := c.statements[statement.Name]
	if info.cardinality != "one" && info.cardinality != "optional" && info.cardinality != "many" && info.cardinality != "relation" {
		return nil
	}
	resultContext := ""
	var walk func([]Node) error
	walk = func(items []Node) error {
		for _, item := range items {
			switch n := item.(type) {
			case *TextNode:
				for _, word := range strings.Fields(strings.ToUpper(n.Text)) {
					word = strings.Trim(word, "(),;\n\t")
					switch word {
					case "SELECT":
						resultContext = "SELECT"
					case "FROM":
						if resultContext == "SELECT" {
							resultContext = ""
						}
					case "RETURNING":
						resultContext = "RETURNING"
					}
				}
			case *IfNode:
				if resultContext != "" {
					return c.error(n.Pos, "runtime-conditional "+strings.ToLower(resultContext)+" columns are forbidden")
				}
				if err := walk(n.Then); err != nil {
					return err
				}
				if err := walk(n.Else); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(nodes); err != nil {
		return err
	}
	items, found := resultColumns(staticSQL(nodes))
	if !found {
		return nil
	}
	if len(items) == 1 && (items[0] == "*" || strings.HasSuffix(items[0], ".*")) {
		return nil
	}
	record := c.records[info.result.name]
	if len(items) != len(record.Fields) {
		return c.error(statement.Pos, fmt.Sprintf("SQL result has %d columns, but %s has %d fields", len(items), record.Name, len(record.Fields)))
	}
	for i, item := range items {
		name, known := staticColumnName(item)
		if known && name != record.Fields[i].Name {
			return c.error(statement.Pos, fmt.Sprintf("SQL result column %q does not match field %q", name, record.Fields[i].Name))
		}
	}
	return nil
}

// resultListTerminators are the top-level keywords that can end a select list
// when the statement has no FROM clause, or end a RETURNING list.
var resultListTerminators = map[string]bool{
	"FROM": true, "WHERE": true, "GROUP": true, "HAVING": true, "WINDOW": true,
	"ORDER": true, "LIMIT": true, "OFFSET": true, "FETCH": true, "FOR": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
}

// resultColumns returns the top-level result columns of a statement, per
// rule:sql-top-level-keyword-scan. Only keywords at the statement's own nesting
// level are read, so a subquery's select list is never mistaken for the outer
// one, and a WITH statement is resolved to its tail.
func resultColumns(query string) ([]string, bool) {
	tokens, ok := scanSQLTokens(query)
	if !ok || len(tokens) == 0 {
		return nil, false
	}
	from := 0
	if tokens[0].word && tokens[0].text == "WITH" {
		if _, from, ok = splitWith(tokens); !ok {
			return nil, false
		}
	}
	begin, list := -1, -1
	for i := from; i < len(tokens); i++ {
		if !tokens[i].word || tokens[i].depth != 0 {
			continue
		}
		if tokens[i].text == "SELECT" || tokens[i].text == "RETURNING" {
			begin = i
			break
		}
	}
	if begin < 0 {
		return nil, false
	}
	list = skipSelectQualifiers(tokens, begin+1)
	end := len(tokens)
	for i := list; i < len(tokens); i++ {
		if tokens[i].word && tokens[i].depth == 0 && resultListTerminators[tokens[i].text] {
			end = i
			break
		}
	}
	// The list spans the raw text between the tokens that bracket it, so an
	// item opening with a literal keeps its full text.
	start := tokens[list-1].end
	stop := len(query)
	if end < len(tokens) {
		stop = tokens[end].start
	}
	if strings.TrimSpace(query[start:stop]) == "" {
		return nil, false
	}
	return splitColumns(query, start, stop, tokens[list:end]), true
}

// skipSelectQualifiers steps over ALL, DISTINCT, and DISTINCT ON (...), which
// precede the first result column without being one.
func skipSelectQualifiers(tokens []sqlToken, i int) int {
	if i >= len(tokens) || !tokens[i].word {
		return i
	}
	switch tokens[i].text {
	case "ALL":
		return i + 1
	case "DISTINCT":
		i++
		if i < len(tokens) && tokens[i].word && tokens[i].text == "ON" {
			i++
			if i < len(tokens) && tokens[i].text == "(" {
				if closing := matchParen(tokens, i); closing >= 0 {
					return closing + 1
				}
			}
		}
	}
	return i
}

// splitColumns cuts the list at its own top-level commas and returns the raw
// text of each item, so alias and column names stay readable. A comma inside a
// literal or a nested expression is not a separator.
func splitColumns(query string, start, stop int, list []sqlToken) []string {
	var out []string
	for _, token := range list {
		if token.text == "," && !token.word && token.depth == 0 {
			out = append(out, strings.TrimSpace(query[start:token.start]))
			start = token.end
		}
	}
	return append(out, strings.TrimSpace(query[start:stop]))
}
func staticColumnName(item string) (string, bool) {
	item = strings.TrimSpace(item)
	upper := strings.ToUpper(item)
	if index := strings.LastIndex(upper, " AS "); index >= 0 {
		return trimSQLIdentifier(strings.TrimSpace(item[index+4:])), true
	}
	simple := true
	for _, r := range item {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' && r != '"' {
			simple = false
			break
		}
	}
	if !simple || item == "" {
		return "", false
	}
	parts := strings.Split(item, ".")
	name := trimSQLIdentifier(parts[len(parts)-1])
	if name == "" || unicode.IsDigit([]rune(name)[0]) {
		return "", false
	}
	return name, true
}
func trimSQLIdentifier(value string) string { return strings.Trim(strings.TrimSpace(value), "\"") }

func (c *compiler) binaryType(expr *BinaryExpr, left, right valueType) (valueType, error) {
	switch expr.Operator {
	case "and", "&&", "or", "||":
		if left.kind != kindBool || right.kind != kindBool || left.optional || right.optional {
			return valueType{}, c.error(expr.Pos, "boolean operator requires bool")
		}
		return valueType{kind: kindBool}, nil
	case "==", "!=":
		if left.kind == kindInvalid && left.optional {
			return valueType{kind: kindBool}, nil
		}
		if right.kind == kindInvalid && right.optional {
			return valueType{kind: kindBool}, nil
		}
		if !assignable(left, right) && !assignable(right, left) {
			return valueType{}, c.error(expr.Pos, "incompatible comparison")
		}
		return valueType{kind: kindBool}, nil
	case "<", "<=", ">", ">=":
		if !numeric(left) || !numeric(right) {
			return valueType{}, c.error(expr.Pos, "ordered comparison requires numbers")
		}
		return valueType{kind: kindBool}, nil
	case "+":
		if left.kind == kindString && right.kind == kindString && !left.optional && !right.optional {
			return valueType{kind: kindString}, nil
		}
		fallthrough
	case "-", "*", "/", "%":
		if !numeric(left) || !numeric(right) || left.kind != right.kind {
			return valueType{}, c.error(expr.Pos, "arithmetic operands must have the same numeric type")
		}
		return left, nil
	}
	return valueType{}, c.error(expr.Pos, "unsupported binary operator "+expr.Operator)
}

func (c *compiler) resolveType(ref TypeRef) (valueType, error) {
	var result valueType
	switch ref.Name {
	case "string":
		result.kind = kindString
	case "bool":
		result.kind = kindBool
	case "int":
		result.kind = kindInt
	case "float":
		result.kind = kindFloat
	case "decimal":
		result.kind = kindDecimal
	case "datetime":
		result.kind = kindDateTime
	case "date":
		result.kind = kindDate
	case "time":
		result.kind = kindTime
	case "url":
		result.kind = kindURL
	case "bytes":
		result.kind = kindBytes
	default:
		if _, ok := c.records[ref.Name]; ok {
			result = valueType{kind: kindRecord, name: ref.Name}
		} else if _, ok := c.enums[ref.Name]; ok {
			result = valueType{kind: kindEnum, name: ref.Name}
		} else {
			return valueType{}, c.error(ref.Pos, "unknown type "+ref.Name)
		}
	}
	if ref.Array {
		elem := result
		result = valueType{kind: kindArray, elem: &elem}
	}
	result.optional = ref.Optional
	return result, nil
}

// checkStatementName applies decision:declaration-name-policy. The constraint
// follows from what is emitted: an executable statement's function is named
// exactly as the declaration is, so the name's case decides Go visibility and
// has to agree with the export modifier. A predicate or relation emits only the
// prefixed fragment builder, so its own case reaches no Go identifier and is
// left alone.
func (c *compiler) checkStatementName(d *TemplateDecl, cardinality string) error {
	if cardinality == "predicate" || cardinality == "relation" {
		return nil
	}
	exportedName := startsUpper(d.Name)
	if d.Exported && !exportedName {
		return c.error(d.Pos, "statement "+d.Name+" is declared export but its name is unexported; capitalize it or drop export")
	}
	if !d.Exported && exportedName {
		return c.error(d.Pos, "statement "+d.Name+" has an exported name; write \"export statement "+d.Name+"\" or lowercase it")
	}
	return nil
}

func startsUpper(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func (c *compiler) nameExists(name string) bool {
	_, a := c.records[name]
	_, b := c.enums[name]
	_, d := c.externals[name]
	_, e := c.statements[name]
	return a || b || d || e
}
func (c *compiler) usesKind(kind valueKind) bool {
	for _, declaration := range c.module.Declarations {
		switch d := declaration.(type) {
		case *TypeDecl:
			for _, field := range d.Fields {
				t, _ := c.resolveType(field.Type)
				if containsKind(t, kind) {
					return true
				}
			}
		case *ExternalDecl:
			for _, parameter := range d.Parameters {
				t, _ := c.resolveType(parameter.Type)
				if containsKind(t, kind) {
					return true
				}
			}
			t, _ := c.resolveType(d.Result)
			if containsKind(t, kind) {
				return true
			}
		case *TemplateDecl:
			for _, parameter := range d.Parameters {
				t, _ := c.resolveType(parameter.Type)
				if containsKind(t, kind) {
					return true
				}
			}
		}
	}
	return false
}
func containsKind(t valueType, kind valueKind) bool {
	return t.kind == kind || (t.elem != nil && containsKind(*t.elem, kind))
}
func (c *compiler) error(pos Position, message string) error {
	return &CompileError{Filename: c.filename, Pos: pos, Message: message}
}
func assignable(want, got valueType) bool {
	if got.kind == kindInvalid && got.optional {
		return want.optional
	}
	if want.kind != got.kind || want.name != got.name || want.optional != got.optional {
		return false
	}
	if want.kind == kindArray {
		return want.elem != nil && got.elem != nil && assignable(*want.elem, *got.elem)
	}
	return true
}
func numeric(t valueType) bool { return !t.optional && (t.kind == kindInt || t.kind == kindFloat) }
func findField(record *TypeDecl, name string) (Field, bool) {
	if record != nil {
		for _, field := range record.Fields {
			if field.Name == name {
				return field, true
			}
		}
	}
	return Field{}, false
}
func copyScope(in map[string]valueType) map[string]valueType {
	out := make(map[string]valueType, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func staticSQL(nodes []Node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n := n.(type) {
		case *TextNode:
			b.WriteString(n.Text)
		case *IfNode:
			b.WriteString(staticSQL(n.Then))
			b.WriteString(staticSQL(n.Else))
		}
	}
	return b.String()
}

func exprPos(expr Expr) Position {
	switch x := expr.(type) {
	case *IdentifierExpr:
		return x.Pos
	case *LiteralExpr:
		return x.Pos
	case *MemberExpr:
		return x.Pos
	case *IndexExpr:
		return x.Pos
	case *CallExpr:
		return x.Pos
	case *UnaryExpr:
		return x.Pos
	case *BinaryExpr:
		return x.Pos
	case *ConditionalExpr:
		return x.Pos
	}
	return Position{Line: 1, Col: 1}
}
