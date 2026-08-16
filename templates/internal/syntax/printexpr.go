package syntax

import (
	"fmt"
	"strconv"
	"strings"
)

// ExprString renders an expression back to source. Parentheses are reinserted
// from precedence rather than remembered, so a redundant pair the author wrote
// is dropped and a necessary one is never lost.
func ExprString(e Expr) string {
	var b strings.Builder
	writeExpr(&b, e, 0)
	return b.String()
}

// exprPrecedence is the binding strength of an expression's own top operator.
// A primary expression binds tighter than every operator.
func exprPrecedence(e Expr) int {
	switch v := e.(type) {
	case *ConditionalExpr:
		return 0
	case *BinaryExpr:
		if prec, ok := binaryPrecedence[v.Operator]; ok {
			return prec
		}
		return 1
	case *UnaryExpr:
		return 7
	default:
		return 8
	}
}

// writeExpr renders e, parenthesizing it when its own operator binds looser
// than the position it sits in requires.
func writeExpr(b *strings.Builder, e Expr, min int) {
	if exprPrecedence(e) < min {
		b.WriteString("(")
		writeExpr(b, e, 0)
		b.WriteString(")")
		return
	}
	switch v := e.(type) {
	case *IdentifierExpr:
		b.WriteString(v.Name)
	case *LiteralExpr:
		b.WriteString(literalText(v))
	case *MemberExpr:
		writeExpr(b, v.Object, 8)
		b.WriteString(".")
		b.WriteString(v.Member)
	case *IndexExpr:
		writeExpr(b, v.Object, 8)
		b.WriteString("[")
		writeExpr(b, v.Index, 0)
		b.WriteString("]")
	case *CallExpr:
		writeExpr(b, v.Callee, 8)
		b.WriteString("(")
		for i, arg := range v.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			writeExpr(b, arg, 0)
		}
		b.WriteString(")")
	case *UnaryExpr:
		b.WriteString(v.Operator)
		// A word operator needs the space its spelling implies; `not x` and
		// `notx` are different tokens, while `!x` is not.
		if isWordOperator(v.Operator) {
			b.WriteString(" ")
		}
		writeExpr(b, v.Operand, 7)
	case *BinaryExpr:
		prec := exprPrecedence(v)
		writeExpr(b, v.Left, prec)
		b.WriteString(" " + v.Operator + " ")
		// Every binary operator here is left-associative, so an equally binding
		// right operand needs parentheses to stay the tree it was.
		writeExpr(b, v.Right, prec+1)
	case *ConditionalExpr:
		writeExpr(b, v.Condition, 1)
		b.WriteString(" ? ")
		writeExpr(b, v.Then, 0)
		b.WriteString(" : ")
		writeExpr(b, v.Else, 0)
	case *MessageExpr:
		b.WriteString(MessageString(v))
	default:
		b.WriteString(fmt.Sprintf("%v", e))
	}
}

func isWordOperator(op string) bool {
	for _, r := range op {
		if r >= 'a' && r <= 'z' {
			return true
		}
		return false
	}
	return false
}

func literalText(l *LiteralExpr) string {
	switch l.ValueKind {
	case "string":
		if s, ok := l.Value.(string); ok {
			return strconv.Quote(s)
		}
	case "number":
		if s, ok := l.Value.(string); ok {
			return s
		}
	case "bool":
		if v, ok := l.Value.(bool); ok {
			return strconv.FormatBool(v)
		}
	case "null":
		return "null"
	}
	return fmt.Sprintf("%v", l.Value)
}

// ValString renders a value binding without the braces the format owns. It is
// not a control node: the binding has no closer, so a printer writes it as a
// leaf and lets the nodes it scopes stay where the author put them.
func ValString(n *ValNode) string {
	parts := make([]string, 0, len(n.Bindings))
	for _, binding := range n.Bindings {
		parts = append(parts, binding.Name+" = "+ExprString(binding.Value))
	}
	return "val " + strings.Join(parts, ", ")
}

// CheckString renders a check directive without the braces the format owns. Like
// a value binding it is a leaf with no closer, so the nodes after it stay where
// the author put them.
func CheckString(n *CheckNode) string { return "check " + ExprString(n.Call) }

// MessageString renders a message reference as written, without the braces the
// format owns. It prints the authored id rather than the resolved one, because
// printing back a template must not rewrite what the author chose to qualify.
func MessageString(n *MessageExpr) string {
	out := messageKeyword + " " + n.Written
	for _, arg := range n.Args {
		out += ", " + arg.Name + ": " + ExprString(arg.Value)
	}
	return out
}

// ControlOpen renders the opening marker of a shared control node, without the
// braces the format owns. It reports false for a node that is not a control
// node.
func ControlOpen(n Node) (string, bool) {
	switch v := n.(type) {
	case *IfNode:
		return "if " + ExprString(v.Condition), true
	case *ForNode:
		head := "for " + v.Variable
		if v.Index != "" {
			head += ", " + v.Index
		}
		return head + " in " + ExprString(v.Iterable), true
	case *AwaitNode:
		parts := make([]string, 0, len(v.Bindings))
		for _, binding := range v.Bindings {
			parts = append(parts, binding.Name+" = "+ExprString(binding.Call))
		}
		return "await " + strings.Join(parts, ", "), true
	}
	return "", false
}

// ControlClose renders the closing marker of a shared control node.
func ControlClose(n Node) string {
	switch n.(type) {
	case *IfNode:
		return "/if"
	case *ForNode:
		return "/for"
	case *AwaitNode:
		return "/await"
	}
	return ""
}
