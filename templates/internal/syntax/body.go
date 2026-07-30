package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FormatParser owns format tokenization and discovers embedded template
// boundaries. The shared BodyContext parses those boundaries and recursively
// calls the same format parser for control bodies.
type FormatParser interface {
	ParseBody(*BodyContext, string) ([]Node, *Terminator, error)
}

type TerminatorKind string

const (
	TerminatorRoot     TerminatorKind = "root"
	TerminatorElse     TerminatorKind = "else"
	TerminatorElseIf   TerminatorKind = "else-if"
	TerminatorEndIf    TerminatorKind = "end-if"
	TerminatorEndFor   TerminatorKind = "end-for"
	TerminatorFallback TerminatorKind = "fallback"
	TerminatorRecover  TerminatorKind = "recover"
	TerminatorEndAwait TerminatorKind = "end-await"
)

// Terminator is discovered by a format parser and interpreted by the shared
// control parser.
type Terminator struct {
	Kind          TerminatorKind
	Pos           Position
	Header        string
	HeaderOffset  int
	ContentOffset int
}

// Embedded is one brace-delimited template fragment discovered by a format
// parser. Offsets are file-global byte offsets.
type Embedded struct {
	Text          string
	StartOffset   int
	ContentOffset int
}

// BodyContext is the shared cursor and control orchestrator for one declaration
// body. Format parsers must use it instead of owning an independent cursor.
type BodyContext struct {
	filename string
	source   string
	offset   int
	format   FormatParser
}

func newBodyContext(filename, source string, offset int, format FormatParser) *BodyContext {
	return &BodyContext{filename: filename, source: source, offset: offset, format: format}
}

func (c *BodyContext) Filename() string { return c.filename }
func (c *BodyContext) Source() string   { return c.source }
func (c *BodyContext) Offset() int      { return c.offset }

func (c *BodyContext) SetOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(c.source) {
		offset = len(c.source)
	}
	c.offset = offset
}

func (c *BodyContext) Position(offset int) Position { return positionAt(c.source, offset) }

func (c *BodyContext) ErrorAt(offset int, message string) error {
	return errorAt(c.filename, c.source, offset, message)
}

// ParseEmbedded parses one fragment after the active format parser has found
// its boundaries. It returns either a shared node or a control terminator.
func (c *BodyContext) ParseEmbedded(fragment Embedded, context string) (Node, *Terminator, error) {
	trimmed := strings.TrimSpace(fragment.Text)
	leading := len(fragment.Text) - len(strings.TrimLeftFunc(fragment.Text, unicode.IsSpace))
	headerOffset := fragment.ContentOffset + leading
	pos := c.Position(fragment.StartOffset)
	switch {
	case trimmed == "else":
		return nil, &Terminator{Kind: TerminatorElse, Pos: pos, HeaderOffset: headerOffset, ContentOffset: fragment.ContentOffset}, nil
	case strings.HasPrefix(trimmed, "else if "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "else if "))
		offset := headerOffset + strings.Index(trimmed, header)
		return nil, &Terminator{Kind: TerminatorElseIf, Pos: pos, Header: header, HeaderOffset: offset, ContentOffset: fragment.ContentOffset}, nil
	case trimmed == "/if":
		return nil, &Terminator{Kind: TerminatorEndIf, Pos: pos, HeaderOffset: headerOffset, ContentOffset: fragment.ContentOffset}, nil
	case trimmed == "/for":
		return nil, &Terminator{Kind: TerminatorEndFor, Pos: pos, HeaderOffset: headerOffset, ContentOffset: fragment.ContentOffset}, nil
	case trimmed == "fallback":
		return nil, &Terminator{Kind: TerminatorFallback, Pos: pos, HeaderOffset: headerOffset, ContentOffset: fragment.ContentOffset}, nil
	case trimmed == "recover" || strings.HasPrefix(trimmed, "recover "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "recover"))
		offset := headerOffset + len(trimmed) - len(header)
		return nil, &Terminator{Kind: TerminatorRecover, Pos: pos, Header: header, HeaderOffset: offset, ContentOffset: fragment.ContentOffset}, nil
	case trimmed == "/await":
		return nil, &Terminator{Kind: TerminatorEndAwait, Pos: pos, HeaderOffset: headerOffset, ContentOffset: fragment.ContentOffset}, nil
	case strings.HasPrefix(trimmed, "await "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "await "))
		offset := headerOffset + strings.Index(trimmed, header)
		node, err := c.parseAwait(header, offset, pos, context)
		return node, nil, err
	case strings.HasPrefix(trimmed, "if "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "if "))
		offset := headerOffset + strings.Index(trimmed, header)
		node, err := c.parseIf(header, offset, pos, context)
		return node, nil, err
	case strings.HasPrefix(trimmed, "for "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "for "))
		offset := headerOffset + strings.Index(trimmed, header)
		node, err := c.parseFor(header, offset, pos, context)
		return node, nil, err
	default:
		expr, err := ParseExpressionAt(c.filename, trimmed, headerOffset, c.Position(headerOffset))
		if err != nil {
			return nil, nil, err
		}
		return &ExpressionNode{Kind: "template:expression", Pos: pos, Context: context, Expression: expr}, nil, nil
	}
}

func (c *BodyContext) parseIf(header string, headerOffset int, pos Position, context string) (*IfNode, error) {
	condition, err := ParseExpressionAt(c.filename, header, headerOffset, c.Position(headerOffset))
	if err != nil {
		return nil, err
	}
	thenNodes, terminator, err := c.format.ParseBody(c, context)
	if err != nil {
		return nil, err
	}
	node := &IfNode{Kind: "template:if", Pos: pos, Context: context, Condition: condition, Then: thenNodes}
	if terminator == nil {
		return nil, c.ErrorAt(c.offset, "missing {/if}")
	}
	switch terminator.Kind {
	case TerminatorEndIf:
		return node, nil
	case TerminatorElse:
		elseNodes, end, err := c.format.ParseBody(c, context)
		if err != nil {
			return nil, err
		}
		if end == nil || end.Kind != TerminatorEndIf {
			return nil, c.ErrorAt(c.offset, "expected {/if} after {else}")
		}
		node.Else = elseNodes
		return node, nil
	case TerminatorElseIf:
		nested, err := c.parseIf(terminator.Header, terminator.HeaderOffset, terminator.Pos, context)
		if err != nil {
			return nil, err
		}
		node.Else = []Node{nested}
		return node, nil
	default:
		return nil, c.ErrorAt(c.offset, "expected {else} or {/if}")
	}
}

func (c *BodyContext) parseFor(header string, headerOffset int, pos Position, context string) (*ForNode, error) {
	parts := strings.SplitN(header, " in ", 2)
	if len(parts) != 2 {
		return nil, c.ErrorAt(headerOffset, "for syntax is {for item[, index] in collection}")
	}
	bindings := strings.Split(parts[0], ",")
	if len(bindings) > 2 {
		return nil, c.ErrorAt(headerOffset, "for accepts an item and optional index")
	}
	variable := strings.TrimSpace(bindings[0])
	if !lowerCamelIdentifier(variable) {
		return nil, c.ErrorAt(headerOffset, "for variable must be lowerCamelCase")
	}
	index := ""
	if len(bindings) == 2 {
		index = strings.TrimSpace(bindings[1])
		if !lowerCamelIdentifier(index) {
			return nil, c.ErrorAt(headerOffset, "for index must be lowerCamelCase")
		}
	}
	iterableText := strings.TrimSpace(parts[1])
	iterableOffset := headerOffset + strings.Index(header, iterableText)
	iterable, err := ParseExpressionAt(c.filename, iterableText, iterableOffset, c.Position(iterableOffset))
	if err != nil {
		return nil, err
	}
	body, terminator, err := c.format.ParseBody(c, context)
	if err != nil {
		return nil, err
	}
	if terminator == nil || terminator.Kind != TerminatorEndFor {
		return nil, c.ErrorAt(c.offset, "expected {/for}")
	}
	return &ForNode{Kind: "template:for", Pos: pos, Context: context, Variable: variable, Index: index, Iterable: iterable, Body: body}, nil
}

// parseAwait reads one boundary. The clause binds its own asynchronous calls,
// so the primary subtree's dependencies are readable at the wait site instead
// of inferred from everything the subtree reaches.
//
// One clause covers a settle-once source and a live one alike, because how often
// a value arrives is what the source declares rather than what the wait site
// asks for.
func (c *BodyContext) parseAwait(header string, headerOffset int, pos Position, context string) (*AwaitNode, error) {
	keyword, end := "await", TerminatorEndAwait
	node := &AwaitNode{Kind: "template:await", Pos: pos, Context: context}
	for _, part := range splitTopLevel(header, ',') {
		text := strings.TrimSpace(part.text)
		offset := headerOffset + part.offset + (len(part.text) - len(strings.TrimLeftFunc(part.text, unicode.IsSpace)))
		name, callText, found := strings.Cut(text, "=")
		if !found {
			return nil, c.ErrorAt(offset, keyword+" binding syntax is {"+keyword+" name = Call(args)}")
		}
		name = strings.TrimSpace(name)
		if !lowerCamelIdentifier(name) {
			return nil, c.ErrorAt(offset, keyword+" binding name must be lowerCamelCase")
		}
		for _, existing := range node.Bindings {
			if existing.Name == name {
				return nil, c.ErrorAt(offset, "duplicate "+keyword+" binding "+name)
			}
		}
		callBody := strings.TrimSpace(callText)
		callOffset := offset + strings.Index(text, callBody)
		call, err := ParseExpressionAt(c.filename, callBody, callOffset, c.Position(callOffset))
		if err != nil {
			return nil, err
		}
		node.Bindings = append(node.Bindings, AwaitBinding{Pos: c.Position(offset), Name: name, Call: call})
	}
	if len(node.Bindings) == 0 {
		return nil, c.ErrorAt(headerOffset, keyword+" needs at least one binding")
	}

	primary, terminator, err := c.format.ParseBody(c, context)
	if err != nil {
		return nil, err
	}
	node.Primary = primary
	// The fallback subtree is what commits first, so a boundary without one
	// would have nothing to show while its bindings run.
	if terminator == nil || terminator.Kind != TerminatorFallback {
		return nil, c.ErrorAt(c.offset, "expected {fallback} inside {"+keyword+"}")
	}
	fallback, terminator, err := c.format.ParseBody(c, context)
	if err != nil {
		return nil, err
	}
	node.Fallback = fallback
	if terminator == nil {
		return nil, c.ErrorAt(c.offset, "expected {recover} or {/"+keyword+"}")
	}
	switch terminator.Kind {
	case end:
		return node, nil
	case TerminatorRecover:
		if terminator.Header != "" {
			if !lowerCamelIdentifier(terminator.Header) {
				return nil, c.ErrorAt(terminator.HeaderOffset, "recover error name must be lowerCamelCase")
			}
			node.ErrorName = terminator.Header
			node.ErrorPos = c.Position(terminator.HeaderOffset)
		}
		node.HasRecover = true
		recovery, closing, err := c.format.ParseBody(c, context)
		if err != nil {
			return nil, err
		}
		if closing == nil || closing.Kind != end {
			return nil, c.ErrorAt(c.offset, "expected {/"+keyword+"} after {recover}")
		}
		node.Recover = recovery
		return node, nil
	default:
		return nil, c.ErrorAt(c.offset, "expected {recover} or {/"+keyword+"}")
	}
}

// segment is one comma-separated piece of an await header with its offset in
// that header.
type segment struct {
	text   string
	offset int
}

// splitTopLevel splits on a separator that is not nested in brackets, quotes,
// or a call's argument list, so {await a = F(x, y)} stays one binding.
func splitTopLevel(value string, separator byte) []segment {
	var out []segment
	depth := 0
	quote := byte(0)
	start := 0
	for i := 0; i < len(value); i++ {
		c := value[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case separator:
			if depth == 0 {
				out = append(out, segment{text: value[start:i], offset: start})
				start = i + 1
			}
		}
	}
	return append(out, segment{text: value[start:], offset: start})
}

func lowerCamelIdentifier(value string) bool {
	if value == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(value)
	if !unicode.IsLower(r) {
		return false
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
