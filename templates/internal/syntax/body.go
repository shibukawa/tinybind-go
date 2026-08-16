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
	// TerminatorEndMessage closes a rich-text message block. It is discovered
	// rather than opened: `{t id}` is the same leaf either way, and the closer is
	// what says the siblings after it are its holes. The same shape ValNode uses,
	// per .knowledge decision:value-binding-form desugaring.
	TerminatorEndMessage TerminatorKind = "end-message"
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
	case trimmed == "/t":
		return nil, &Terminator{Kind: TerminatorEndMessage, Pos: pos, HeaderOffset: headerOffset, ContentOffset: fragment.ContentOffset}, nil
	case trimmed == "/await":
		return nil, &Terminator{Kind: TerminatorEndAwait, Pos: pos, HeaderOffset: headerOffset, ContentOffset: fragment.ContentOffset}, nil
	case strings.HasPrefix(trimmed, "await "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "await "))
		offset := headerOffset + strings.Index(trimmed, header)
		node, err := c.parseAwait(header, offset, pos, context)
		return node, nil, err
	case strings.HasPrefix(trimmed, "val "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "val "))
		offset := headerOffset + strings.Index(trimmed, header)
		node, err := c.parseVal(header, offset, pos, context)
		return node, nil, err
	case strings.HasPrefix(trimmed, "check "):
		header := strings.TrimSpace(strings.TrimPrefix(trimmed, "check "))
		offset := headerOffset + strings.Index(trimmed, header)
		node, err := c.parseCheck(header, offset, pos, context)
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
		// A message reference is recognized only when the whole body reads as
		// one, so `{t}`, `{t.field}`, `{t(x)}` and `{t == "x"}` keep meaning
		// the parameter. See .knowledge decision:message-reference-syntax.
		if message, ok, err := c.parseMessage(trimmed, headerOffset); ok {
			if err != nil {
				return nil, nil, err
			}
			return &ExpressionNode{Kind: "template:expression", Pos: pos, Context: context, Expression: message}, nil, nil
		}
		expr, err := ParseExpressionAt(c.filename, trimmed, headerOffset, c.Position(headerOffset))
		if err != nil {
			return nil, nil, err
		}
		return &ExpressionNode{Kind: "template:expression", Pos: pos, Context: context, Expression: expr}, nil, nil
	}
}

// CloseMessageBlock turns the node list a format parser has collected into a
// rich-text message, when it reaches `{/t}`.
//
// The block is discovered at its closer rather than opened, because `{t id}`
// has to keep meaning the same thing whether or not holes follow it: a parser
// deciding at the opening would need lookahead, and a second keyword would make
// an author choose between two spellings of one reference. ValNode already
// takes this shape, per .knowledge decision:value-binding-form desugaring.
//
// nameHole is supplied by the format, because what names a hole is markup the
// shared parser cannot read.
func (c *BodyContext) CloseMessageBlock(nodes []Node, closer Position, nameHole func(Node) (string, Position, bool, error)) ([]Node, error) {
	for i := len(nodes) - 1; i >= 0; i-- {
		expression, ok := nodes[i].(*ExpressionNode)
		if !ok {
			continue
		}
		message, ok := expression.Expression.(*MessageExpr)
		if !ok {
			continue
		}
		block := &MessageBlockNode{Kind: "template:message-block", Pos: expression.Pos,
			Context: expression.Context, Message: message}
		seen := map[string]bool{}
		for _, node := range nodes[i+1:] {
			name, pos, bound, err := nameHole(node)
			if err != nil {
				return nil, err
			}
			if !bound {
				// Whitespace between holes is layout; anything else is text the
				// translation owns, so writing it in the template would put the
				// same sentence in two places.
				continue
			}
			if seen[name] {
				return nil, c.errorAtPosition(pos, "duplicate message hole "+name+
					"; two holes sharing a tag need a hole attribute to tell them apart")
			}
			seen[name] = true
			block.Holes = append(block.Holes, MessageHole{Pos: pos, Name: name, Nodes: []Node{node}})
		}
		return append(nodes[:i:i], block), nil
	}
	return nil, c.errorAtPosition(closer, "{/t} closes a message block, but no {t ...} reference opens one here")
}

// errorAtPosition reports against a position a node already carries, for a
// check that runs over parsed nodes rather than over the cursor.
func (c *BodyContext) errorAtPosition(pos Position, message string) error {
	return &ParseError{Filename: c.filename, Line: pos.Line, Column: pos.Col, Message: message}
}

// messageKeyword is the contextual keyword introducing a message reference. It
// is not reserved: every existing directive is recognized the same way, by a
// keyword and a body that reads as that directive's header.
const messageKeyword = "t"

// parseMessage reads `t <id>` and `t <id>, name: expression, ...`.
//
// The second result reports whether this body is a message reference at all. It
// is false for anything whose first part is not an id, which is what keeps an
// ordinary expression over a parameter named t working. Once the id is valid
// the reference is committed, so a malformed argument is reported as a bad
// message reference rather than as a confusing expression.
func (c *BodyContext) parseMessage(trimmed string, headerOffset int) (*MessageExpr, bool, error) {
	rest, ok := strings.CutPrefix(trimmed, messageKeyword)
	if !ok || rest == "" {
		return nil, false, nil
	}
	// The keyword has to be followed by space, or `title` would be read as the
	// tail of a longer identifier.
	if r, _ := utf8.DecodeRuneInString(rest); !unicode.IsSpace(r) {
		return nil, false, nil
	}
	parts := splitTopLevel(rest, ',')
	id := strings.TrimSpace(parts[0].text)
	if !messageID(id) {
		return nil, false, nil
	}
	node := &MessageExpr{Kind: "template:message", Pos: c.Position(headerOffset), Written: id}
	for _, part := range parts[1:] {
		text := strings.TrimSpace(part.text)
		offset := headerOffset + len(messageKeyword) + part.offset +
			(len(part.text) - len(strings.TrimLeftFunc(part.text, unicode.IsSpace)))
		name, valueText, found := strings.Cut(text, ":")
		if !found {
			return nil, true, c.ErrorAt(offset, "message argument syntax is {t "+id+", name: expression}")
		}
		name = strings.TrimSpace(name)
		if !lowerCamelIdentifier(name) {
			return nil, true, c.ErrorAt(offset, "message argument name must be lowerCamelCase")
		}
		for _, existing := range node.Args {
			if existing.Name == name {
				return nil, true, c.ErrorAt(offset, "duplicate message argument "+name)
			}
		}
		valueBody := strings.TrimSpace(valueText)
		if valueBody == "" {
			return nil, true, c.ErrorAt(offset, "message argument "+name+" has no value")
		}
		valueOffset := offset + strings.Index(text, valueBody)
		value, err := ParseExpressionAt(c.filename, valueBody, valueOffset, c.Position(valueOffset))
		if err != nil {
			return nil, true, err
		}
		node.Args = append(node.Args, MessageArg{Pos: c.Position(offset), Name: name, Value: value})
	}
	return node, true, nil
}

// messageID reports whether the token is a message id: dot-separated segments
// of letters, digits, underscores and hyphens, carrying no whitespace, with no
// segment beginning or ending in a hyphen.
//
// The hyphen rule is what keeps `{t -x}` a subtraction. A leading hyphen fails
// the id form, so the body falls through to the expression arm and the
// established meaning survives. See .knowledge
// decision:message-reference-syntax id_lexical_form.
func messageID(value string) bool {
	if value == "" {
		return false
	}
	for _, segment := range strings.Split(value, ".") {
		if segment == "" || strings.HasPrefix(segment, "-") || strings.HasSuffix(segment, "-") {
			return false
		}
		for _, r := range segment {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
				return false
			}
		}
	}
	return true
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

// parseVal reads one value binding. It has no closer, because a binding that
// delimited its own subtree would indent the markup it scopes for a reason the
// markup does not have. The nodes it scopes are its later siblings, so the node
// is parsed as a leaf and Body is filled later by whoever needs a subtree.
//
// Duplicate names are not checked here. A second binding of one name in the
// same block is the same mistake whether it is written in this node's list or
// in the next node's, so DuplicateValBinding reports both against the whole
// list rather than this reporting half of it.
func (c *BodyContext) parseVal(header string, headerOffset int, pos Position, context string) (*ValNode, error) {
	node := &ValNode{Kind: "template:val", Pos: pos, Context: context}
	for _, part := range splitTopLevel(header, ',') {
		text := strings.TrimSpace(part.text)
		offset := headerOffset + part.offset + (len(part.text) - len(strings.TrimLeftFunc(part.text, unicode.IsSpace)))
		name, valueText, found := strings.Cut(text, "=")
		if !found {
			return nil, c.ErrorAt(offset, "val binding syntax is {val name = expression}")
		}
		name = strings.TrimSpace(name)
		if !lowerCamelIdentifier(name) {
			return nil, c.ErrorAt(offset, "val binding name must be lowerCamelCase")
		}
		valueBody := strings.TrimSpace(valueText)
		valueOffset := offset + strings.Index(text, valueBody)
		value, err := ParseExpressionAt(c.filename, valueBody, valueOffset, c.Position(valueOffset))
		if err != nil {
			return nil, err
		}
		// The bindings of one directive are independent, so a value is read
		// against what was in scope before the directive. Go reads a
		// comma-separated declaration the same way — `a, b := f(), g(a)` does
		// not compile — and an await clause has to, because its bindings settle
		// concurrently. One comma cannot mean two things.
		for _, existing := range node.Bindings {
			if ExprReads(value, existing.Name) {
				return nil, c.ErrorAt(offset, "val binding "+name+" reads "+existing.Name+
					", which the same directive binds; the bindings of one directive are independent, so write "+name+" as its own {val}")
			}
		}
		node.Bindings = append(node.Bindings, ValBinding{Pos: c.Position(offset), Name: name, Value: value})
	}
	if len(node.Bindings) == 0 {
		return nil, c.ErrorAt(headerOffset, "val needs at least one binding")
	}
	return node, nil
}

// parseCheck reads one call made for its error alone.
//
// One call per directive, where a val takes a comma list. A comma list buys
// several names on one line, and a check introduces no name, so a second call
// is a second directive and the source says so.
func (c *BodyContext) parseCheck(header string, headerOffset int, pos Position, context string) (*CheckNode, error) {
	if parts := splitTopLevel(header, ','); len(parts) > 1 {
		return nil, c.ErrorAt(headerOffset, "check takes one call; write a second {check} rather than a comma list, because a check binds no name to share the line with")
	}
	call, err := ParseExpressionAt(c.filename, header, headerOffset, c.Position(headerOffset))
	if err != nil {
		return nil, err
	}
	// Refused here rather than by typing, because the position wants a call
	// whatever the callee turns out to be: a field path or a literal has no
	// error to check, and saying so against the syntax names the mistake.
	if _, ok := call.(*CallExpr); !ok {
		return nil, c.ErrorAt(headerOffset, "check syntax is {check Name(...)}; it calls an external for its error and has nothing to do with a value")
	}
	return &CheckNode{Kind: "template:check", Pos: pos, Context: context, Call: call}, nil
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
