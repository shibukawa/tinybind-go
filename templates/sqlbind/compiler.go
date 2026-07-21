package sqlbind

import (
	"fmt"
	"strings"
	"unicode"
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
		if (c.statements[d.Name].cardinality == "exec") && isMutation(body) && !hasWhere(body) {
			return c.error(d.Pos, "UPDATE and DELETE statements require a WHERE clause")
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
	for _, node := range nodes {
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
		case *ForNode:
			return c.error(n.Pos, "general SQL loops are forbidden; bind an array expression to expand a value list")
		default:
			return c.error(Position{Line: 1, Col: 1}, fmt.Sprintf("unsupported SQL node %T", node))
		}
	}
	return nil
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
	query := strings.TrimSpace(staticSQL(nodes))
	upper := strings.ToUpper(query)
	if strings.HasPrefix(upper, "WITH ") {
		return nil
	}
	start, end := -1, -1
	if selectAt := keywordIndex(upper, "SELECT", 0); selectAt >= 0 {
		start = selectAt + len("SELECT")
		end = keywordIndex(upper, "FROM", start)
	} else if returningAt := keywordIndex(upper, "RETURNING", 0); returningAt >= 0 {
		start = returningAt + len("RETURNING")
		end = len(query)
	}
	if start < 0 || end < start {
		return nil
	}
	list := strings.TrimSpace(query[start:end])
	if strings.HasPrefix(strings.ToUpper(list), "DISTINCT ") {
		list = strings.TrimSpace(list[len("DISTINCT "):])
	}
	items := splitSQLList(list)
	if len(items) == 1 && (strings.TrimSpace(items[0]) == "*" || strings.HasSuffix(strings.TrimSpace(items[0]), ".*")) {
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

func keywordIndex(upper, keyword string, from int) int {
	for from <= len(upper)-len(keyword) {
		index := strings.Index(upper[from:], keyword)
		if index < 0 {
			return -1
		}
		index += from
		before := index == 0 || !sqlWordByte(upper[index-1])
		after := index+len(keyword) == len(upper) || !sqlWordByte(upper[index+len(keyword)])
		if before && after {
			return index
		}
		from = index + len(keyword)
	}
	return -1
}
func sqlWordByte(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}
func splitSQLList(value string) []string {
	var out []string
	start, depth := 0, 0
	quote := byte(0)
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if quote != 0 {
			if ch == quote {
				if i+1 < len(value) && value[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	if tail := strings.TrimSpace(value[start:]); tail != "" {
		out = append(out, tail)
	}
	return out
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

func isMutation(nodes []Node) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(staticSQL(nodes))), "UPDATE ") || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(staticSQL(nodes))), "DELETE ")
}
func hasWhere(nodes []Node) bool {
	words := strings.Fields(strings.ToUpper(staticSQL(nodes)))
	for _, word := range words {
		if strings.Trim(word, "(),;") == "WHERE" {
			return true
		}
	}
	return false
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
