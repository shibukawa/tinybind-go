package syntax

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type exprTokenKind int

const (
	tokenEOF exprTokenKind = iota
	tokenIdent
	tokenNumber
	tokenString
	tokenOperator
	tokenPunct
)

type exprToken struct {
	kind   exprTokenKind
	text   string
	offset int
}

type exprParser struct {
	filename string
	source   string
	base     int
	basePos  Position
	tokens   []exprToken
	pos      int
}

// ParseExpression parses a complete shared template expression.
func ParseExpression(filename, source string, baseOffset int) (Expr, error) {
	return ParseExpressionAt(filename, source, baseOffset, Position{Line: 1, Col: 1})
}

// ParseExpressionAt parses an expression whose first byte starts at baseOffset
// and basePos in its containing template file.
func ParseExpressionAt(filename, source string, baseOffset int, basePos Position) (Expr, error) {
	tokens, err := lexExpression(filename, source, baseOffset, basePos)
	if err != nil {
		return nil, err
	}
	p := &exprParser{filename: filename, source: source, base: baseOffset, basePos: basePos, tokens: tokens}
	expr, err := p.parseConditional()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokenEOF {
		return nil, p.err(p.peek(), "unexpected token "+strconv.Quote(p.peek().text))
	}
	return expr, nil
}

func lexExpression(filename, source string, base int, basePos Position) ([]exprToken, error) {
	var out []exprToken
	for i := 0; i < len(source); {
		r, size := utf8.DecodeRuneInString(source[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		start := i
		if unicode.IsLetter(r) || r == '_' {
			i += size
			for i < len(source) {
				r, size = utf8.DecodeRuneInString(source[i:])
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
					break
				}
				i += size
			}
			out = append(out, exprToken{kind: tokenIdent, text: source[start:i], offset: start})
			continue
		}
		if unicode.IsDigit(r) {
			i += size
			dot := false
			for i < len(source) {
				r, size = utf8.DecodeRuneInString(source[i:])
				if unicode.IsDigit(r) {
					i += size
					continue
				}
				if r == '.' && !dot {
					dot = true
					i += size
					continue
				}
				break
			}
			out = append(out, exprToken{kind: tokenNumber, text: source[start:i], offset: start})
			continue
		}
		if r == '\'' || r == '"' {
			quote := r
			i += size
			for i < len(source) {
				r, size = utf8.DecodeRuneInString(source[i:])
				i += size
				if r == '\\' && i < len(source) {
					_, size = utf8.DecodeRuneInString(source[i:])
					i += size
					continue
				}
				if r == quote {
					out = append(out, exprToken{kind: tokenString, text: source[start:i], offset: start})
					break
				}
			}
			if len(out) == 0 || out[len(out)-1].offset != start {
				return nil, ErrorAtPosition(filename, source, start, base, basePos, "unterminated string literal")
			}
			continue
		}
		two := ""
		if i+size < len(source) {
			_, nextSize := utf8.DecodeRuneInString(source[i+size:])
			two = source[i : i+size+nextSize]
		}
		if two == "==" || two == "!=" || two == "<=" || two == ">=" || two == "&&" || two == "||" {
			out = append(out, exprToken{kind: tokenOperator, text: two, offset: start})
			i += len(two)
			continue
		}
		text := source[i : i+size]
		switch text {
		case "+", "-", "*", "/", "%", "!", "<", ">":
			out = append(out, exprToken{kind: tokenOperator, text: text, offset: start})
		case "(", ")", "[", "]", ",", ".", "?", ":":
			out = append(out, exprToken{kind: tokenPunct, text: text, offset: start})
		default:
			return nil, ErrorAtPosition(filename, source, start, base, basePos, "invalid expression character "+strconv.Quote(text))
		}
		i += size
	}
	out = append(out, exprToken{kind: tokenEOF, offset: len(source)})
	return out, nil
}

func (p *exprParser) parseConditional() (Expr, error) {
	condition, err := p.parseBinary(1)
	if err != nil {
		return nil, err
	}
	if !p.accept("?") {
		return condition, nil
	}
	thenExpr, err := p.parseConditional()
	if err != nil {
		return nil, err
	}
	if !p.accept(":") {
		return nil, p.err(p.peek(), "expected ':' in conditional expression")
	}
	elseExpr, err := p.parseConditional()
	if err != nil {
		return nil, err
	}
	return &ConditionalExpr{Kind: "template:conditional", Pos: expressionPosition(condition), Condition: condition, Then: thenExpr, Else: elseExpr}, nil
}

var binaryPrecedence = map[string]int{
	"||": 1, "or": 1,
	"&&": 2, "and": 2,
	"==": 3, "!=": 3,
	"<": 4, "<=": 4, ">": 4, ">=": 4,
	"+": 5, "-": 5,
	"*": 6, "/": 6, "%": 6,
}

func (p *exprParser) parseBinary(min int) (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		prec, ok := binaryPrecedence[tok.text]
		if !ok || prec < min {
			break
		}
		p.pos++
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Kind: "template:binary", Pos: expressionPosition(left), Operator: tok.text, Left: left, Right: right}
	}
	return left, nil
}

func (p *exprParser) parseUnary() (Expr, error) {
	if tok := p.peek(); tok.text == "!" || tok.text == "-" || tok.text == "+" || tok.text == "not" {
		p.pos++
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Kind: "template:unary", Pos: p.position(tok), Operator: tok.text, Operand: operand}, nil
	}
	return p.parsePostfix()
}

func (p *exprParser) parsePostfix() (Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.accept("."):
			name := p.peek()
			if name.kind != tokenIdent {
				return nil, p.err(name, "expected member name")
			}
			p.pos++
			expr = &MemberExpr{Kind: "template:member", Pos: expressionPosition(expr), Object: expr, Member: name.text}
		case p.accept("["):
			index, err := p.parseConditional()
			if err != nil {
				return nil, err
			}
			if !p.accept("]") {
				return nil, p.err(p.peek(), "expected ']'")
			}
			expr = &IndexExpr{Kind: "template:index", Pos: expressionPosition(expr), Object: expr, Index: index}
		case p.accept("("):
			var args []Expr
			if !p.accept(")") {
				for {
					arg, err := p.parseConditional()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.accept(")") {
						break
					}
					if !p.accept(",") {
						return nil, p.err(p.peek(), "expected ',' or ')' in argument list")
					}
				}
			}
			expr = &CallExpr{Kind: "template:call", Pos: expressionPosition(expr), Callee: expr, Arguments: args}
		default:
			return expr, nil
		}
	}
}

func (p *exprParser) parsePrimary() (Expr, error) {
	tok := p.peek()
	p.pos++
	switch tok.kind {
	case tokenIdent:
		switch tok.text {
		case "true", "false":
			return &LiteralExpr{Kind: "template:literal", Pos: p.position(tok), ValueKind: "bool", Value: tok.text == "true"}, nil
		case "null":
			return &LiteralExpr{Kind: "template:literal", Pos: p.position(tok), ValueKind: "null", Value: nil}, nil
		default:
			return &IdentifierExpr{Kind: "template:identifier", Pos: p.position(tok), Name: tok.text}, nil
		}
	case tokenNumber:
		return &LiteralExpr{Kind: "template:literal", Pos: p.position(tok), ValueKind: "number", Value: tok.text}, nil
	case tokenString:
		value, err := strconv.Unquote(tok.text)
		if err != nil && strings.HasPrefix(tok.text, "'") {
			value = tok.text[1 : len(tok.text)-1]
			err = nil
		}
		if err != nil {
			return nil, p.err(tok, "invalid string literal")
		}
		return &LiteralExpr{Kind: "template:literal", Pos: p.position(tok), ValueKind: "string", Value: value}, nil
	case tokenPunct:
		if tok.text == "(" {
			expr, err := p.parseConditional()
			if err != nil {
				return nil, err
			}
			if !p.accept(")") {
				return nil, p.err(p.peek(), "expected ')'")
			}
			return expr, nil
		}
	}
	return nil, p.err(tok, "expected expression")
}

func (p *exprParser) peek() exprToken { return p.tokens[p.pos] }

func (p *exprParser) accept(text string) bool {
	if p.peek().text != text {
		return false
	}
	p.pos++
	return true
}

func (p *exprParser) err(tok exprToken, message string) error {
	return ErrorAtPosition(p.filename, p.source, tok.offset, p.base, p.basePos, message)
}

func (p *exprParser) position(tok exprToken) Position {
	return positionInFragment(p.source, tok.offset, p.basePos)
}

func expressionPosition(expr Expr) Position {
	switch expr := expr.(type) {
	case *IdentifierExpr:
		return expr.Pos
	case *LiteralExpr:
		return expr.Pos
	case *MemberExpr:
		return expr.Pos
	case *IndexExpr:
		return expr.Pos
	case *CallExpr:
		return expr.Pos
	case *UnaryExpr:
		return expr.Pos
	case *BinaryExpr:
		return expr.Pos
	case *ConditionalExpr:
		return expr.Pos
	case *MessageExpr:
		return expr.Pos
	default:
		return Position{Line: 1, Col: 1}
	}
}
