package htmlbind

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

type htmlParser struct {
	context    *syntax.BodyContext
	filename   string
	source     string
	baseOffset int
	basePos    Position
	pos        int
	// insideHTMLElement marks that parsing is below an html element, where a
	// head element is the document shell rather than a contribution.
	insideHTMLElement bool
	// insideHeadContribution marks that parsing is inside a contributing head
	// element, where style and script bodies are raw text like a single-file
	// component block rather than template markup.
	insideHeadContribution bool
	// insideShellHead marks that parsing is inside the document shell's own head,
	// which is the one place where a raw-text diagnostic can point at the
	// contributing head element that would have parsed instead.
	insideShellHead bool
}

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

func (p *htmlParser) parseNodes(stopTag, context string) ([]Node, *syntax.Terminator, error) {
	// A script or style body is authored JavaScript or CSS, where a brace is
	// ordinary syntax of that language rather than template punctuation.
	raw := isRawTextContext(context)
	var nodes []Node
	for p.pos < len(p.source) {
		if strings.HasPrefix(p.source[p.pos:], "{{") {
			start := p.pos
			end := strings.Index(p.source[p.pos+2:], "}}")
			if end < 0 {
				return nil, nil, p.errAt(p.pos, "unterminated escaped template text")
			}
			nodes = appendText(nodes, "{"+p.source[p.pos+2:p.pos+2+end]+"}", p.position(start))
			p.pos += end + 4
			continue
		}
		if p.source[p.pos] == '}' {
			// Raw text is always terminated by its closing tag, so no brace here
			// can close a declaration body or a control block; the authored
			// language owns it.
			if raw {
				nodes = appendText(nodes, "}", p.position(p.pos))
				p.pos++
				continue
			}
			if stopTag != "" {
				return nil, nil, p.errAt(p.pos, "missing closing tag </"+stopTag+">")
			}
			terminator := &syntax.Terminator{Kind: syntax.TerminatorRoot, Pos: p.position(p.pos)}
			p.pos++
			return nodes, terminator, nil
		}
		if p.source[p.pos] == '<' {
			if strings.HasPrefix(p.source[p.pos:], "<!--") {
				node, err := p.parseComment()
				if err != nil {
					return nil, nil, err
				}
				nodes = append(nodes, node)
				continue
			}
			if strings.HasPrefix(strings.ToLower(p.source[p.pos:]), "<!doctype") {
				node, err := p.parseDoctype()
				if err != nil {
					return nil, nil, err
				}
				nodes = append(nodes, node)
				continue
			}
			if strings.HasPrefix(p.source[p.pos:], "</") {
				name, err := p.parseClosingTag()
				if err != nil {
					return nil, nil, err
				}
				if stopTag == "" {
					return nil, nil, p.errAt(p.pos, "unexpected closing tag </"+name+">")
				}
				if name != stopTag {
					return nil, nil, p.errAt(p.pos, "expected closing tag </"+stopTag+">, got </"+name+">")
				}
				return nodes, nil, nil
			}
			node, err := p.parseElement(context)
			if err != nil {
				return nil, nil, err
			}
			nodes = append(nodes, node)
			continue
		}
		if p.source[p.pos] == '{' {
			if raw && !p.rawInsertionAhead() {
				nodes = appendText(nodes, "{", p.position(p.pos))
				p.pos++
				continue
			}
			start := p.pos
			content, contentOffset, err := p.readDirective()
			if err != nil {
				return nil, nil, p.annotateRawText(raw, context, err)
			}
			p.context.SetOffset(p.pos)
			node, terminator, err := p.context.ParseEmbedded(syntax.Embedded{
				Text:          content,
				StartOffset:   start,
				ContentOffset: contentOffset,
			}, context)
			if err != nil {
				return nil, nil, p.annotateRawText(raw, context, err)
			}
			p.pos = p.context.Offset()
			if terminator != nil {
				return nodes, terminator, nil
			}
			nodes = append(nodes, node)
			continue
		}
		start := p.pos
		for p.pos < len(p.source) && p.source[p.pos] != '<' && p.source[p.pos] != '{' && p.source[p.pos] != '}' {
			p.pos++
		}
		nodes = appendText(nodes, p.source[start:p.pos], p.position(start))
	}
	if stopTag != "" {
		return nil, nil, p.errAt(p.pos, "missing closing tag </"+stopTag+">")
	}
	return nodes, nil, nil
}

func (p *htmlParser) parseElement(context string) (Node, error) {
	start := p.pos
	p.pos++
	name := p.readName()
	if name == "" {
		return nil, p.errAt(start, "expected HTML element or component name")
	}
	if name == "slot" {
		return p.parseSlot(start, context)
	}
	isComponent := startsUpper(name)
	if isComponent {
		if !isPascal(name) {
			return nil, p.errAt(start+1, "component name must be PascalCase")
		}
	} else if !isHTMLName(name) {
		return nil, p.errAt(start+1, "HTML element name must be lowercase or kebab-case")
	}
	attrs, selfClosing, err := p.parseAttributes(isComponent)
	if err != nil {
		return nil, err
	}
	childContext := "html:child"
	if name == "script" {
		childContext = "html:script"
	} else if name == "style" {
		childContext = "html:style"
	}
	// A style or script body inside a head contribution is authored content,
	// not markup, so braces belong to CSS and JavaScript rather than to the
	// template language. A component script block is the same kind of content
	// declared one level out, so it reads the same way.
	if (p.insideHeadContribution || isComponentScriptBlock(name, attrs)) && (name == "style" || name == "script") && !selfClosing {
		text, err := p.readRawUntilClose(name)
		if err != nil {
			return nil, err
		}
		var children []Node
		if text != "" {
			children = []Node{&TextNode{Kind: "html:text", Pos: p.position(start), Text: text}}
		}
		return &ElementNode{Kind: "html:element", Pos: p.position(start), Name: name, Attributes: attrs, Children: children}, nil
	}
	var children []Node
	if !selfClosing && !voidElements[name] {
		var terminator *syntax.Terminator
		if name == "html" {
			outer := p.insideHTMLElement
			p.insideHTMLElement = true
			defer func() { p.insideHTMLElement = outer }()
		}
		if name == "head" {
			if p.insideHTMLElement {
				outer := p.insideShellHead
				p.insideShellHead = true
				defer func() { p.insideShellHead = outer }()
			} else {
				outer := p.insideHeadContribution
				p.insideHeadContribution = true
				defer func() { p.insideHeadContribution = outer }()
			}
		}
		children, terminator, err = p.parseNodes(name, childContext)
		if err != nil {
			return nil, err
		}
		if terminator != nil {
			return nil, p.errAt(p.pos, "missing closing tag </"+name+">")
		}
	}
	if isComponent {
		return &ComponentNode{Kind: "html:component", Pos: p.position(start), Name: name, Arguments: attrs, Children: children, SelfClosing: selfClosing}, nil
	}
	// A head outside the document shell contributes to the merged head. The
	// shell's own head stays an ordinary element and marks the injection point.
	if name == "head" && !p.insideHTMLElement {
		if len(attrs) > 0 {
			return nil, p.errAt(start, "a contributing head element takes no attributes")
		}
		return &HeadNode{Kind: "html:head", Pos: p.position(start), Children: children}, nil
	}
	return &ElementNode{Kind: "html:element", Pos: p.position(start), Name: name, Attributes: attrs, Children: children, SelfClosing: selfClosing}, nil
}

// parseSlot reads a reserved slot element. The element itself is never
// emitted; only the bound argument or the declared default content is.
func (p *htmlParser) parseSlot(start int, context string) (Node, error) {
	if context != "html:child" {
		return nil, p.errAt(start, "slot is only allowed in child-node position")
	}
	attrs, selfClosing, err := p.parseAttributes(false)
	if err != nil {
		return nil, err
	}
	slot := &SlotNode{Kind: "html:slot", Pos: p.position(start)}
	for _, attribute := range attrs {
		switch attribute.Name {
		case "name":
			value, ok := staticAttributeText(attribute)
			if !ok {
				return nil, p.errAt(start, "slot name must be a static value")
			}
			if !isLowerCamel(value) {
				return nil, p.errAt(start, "slot name must be lowerCamelCase")
			}
			if value == "children" {
				return nil, p.errAt(start, "omit the name attribute to declare the unnamed slot")
			}
			slot.Name = value
		case "required":
			if !attribute.Boolean {
				return nil, p.errAt(start, "required must be a bare attribute")
			}
			slot.Required = true
		default:
			return nil, p.errAt(start, "unknown slot attribute "+attribute.Name)
		}
	}
	if selfClosing {
		return slot, nil
	}
	children, terminator, err := p.parseNodes("slot", context)
	if err != nil {
		return nil, err
	}
	if terminator != nil {
		return nil, p.errAt(p.pos, "missing closing tag </slot>")
	}
	slot.Default = children
	return slot, nil
}

// componentScriptMarker is the bare attribute naming a component's own script,
// the block requirement:component-script-block extracts and binds to the
// component's instances.
//
// The marker is needed because position cannot tell the two apart: a script at
// the top of a component body is equally the shape of markup carrying a
// RawJavaScript or JsonForScript insertion, which is a shipped feature.
const componentScriptMarker = "component"

// isComponentScriptBlock reports whether an element is a component's own script
// block rather than a script it renders.
func isComponentScriptBlock(name string, attrs []Attribute) bool {
	if name != "script" {
		return false
	}
	for _, attr := range attrs {
		if attr.Name == componentScriptMarker && attr.Boolean {
			return true
		}
	}
	return false
}

// isRawTextContext reports whether an insertion context is the body of a
// raw-text element, whose content is authored in another language.
func isRawTextContext(context string) bool {
	return context == "html:script" || context == "html:style"
}

// rawTextElement names the element owning a raw-text insertion context.
func rawTextElement(context string) string {
	return strings.TrimPrefix(context, "html:")
}

// insertionKeywords open a control block. They are recognized before the value
// shapes below because a keyword is never a bare value.
// val is here for the same reason as the rest: without it the shapes below read
// `{val a = f()}` as content, because an identifier followed by another one is
// not a bare value, a member access, or a call. It costs no new ambiguity, since
// no valid JavaScript opens a tight brace with `val` the way `{let x = 1}` does.
var insertionKeywords = map[string]bool{
	"if": true, "else": true, "for": true,
	"await": true, "recover": true, "fallback": true,
	"val": true,
}

// rawInsertionAhead reports whether the brace at the parser position opens a
// template insertion rather than a JavaScript or CSS block.
//
// Inside raw text an insertion must open tight, with the content against the
// brace, and take one of a small set of shapes. A block in either authored
// language opens with a space or a line break far more often than not, so the
// tight rule alone separates `{js}` from `{ this.value = 1 }` and `{ render() }`,
// which the shapes below could not tell apart on their own. Anything an
// insertion cannot express this way is written parenthesized.
// See rule:raw-text-insertion-gate.
func (p *htmlParser) rawInsertionAhead() bool {
	return rawInsertionAt(p.source, p.pos)
}

// rawInsertionAt is the parser's own test, as a function of the text alone, so
// the printer can ask the same question. A brace the parser reads as literal
// text has to be printed as literal text, and one it reads as an insertion has
// to be printed escaped; anything else is a rewrite.
func rawInsertionAt(source string, pos int) bool {
	// A brace after a dollar sign is a JavaScript template literal placeholder,
	// which the browser evaluates. Reading it as an insertion would silently
	// substitute a server value into what the author wrote as client code.
	if pos > 0 && source[pos-1] == '$' {
		return false
	}
	i := pos + 1
	if i >= len(source) {
		return false
	}
	// A closing directive and a parenthesized expression cannot begin a block in
	// either authored language, so they need no further evidence.
	if source[i] == '/' || source[i] == '(' {
		return true
	}
	name, after := readInsertionName(source, i)
	if name == "" {
		return false
	}
	// A control keyword is followed by its clause, so it needs no shape check.
	if insertionKeywords[name] {
		return true
	}
	after = skipBlanks(source, after)
	if after >= len(source) {
		return false
	}
	// A bare value, a member access, and a call are the remaining insertion
	// shapes. A minified declaration such as `{color:red}` or a shorthand such
	// as `{a, b}` stops here, because neither a colon nor a comma can follow.
	switch source[after] {
	case '}', '.', '(':
		return true
	}
	return false
}

// skipBlanks advances over spaces and tabs. It is applied after the insertion
// name and never before it, because leading blanks are what distinguish an
// authored block from an insertion.
func skipBlanks(source string, i int) int {
	for i < len(source) && (source[i] == ' ' || source[i] == '\t') {
		i++
	}
	return i
}

// readInsertionName reads the identifier a raw-text insertion may start with. It
// returns an empty name when the position does not begin one, which is what
// keeps a numeric key such as `{0: 'a'}` out of the recognized set.
func readInsertionName(source string, i int) (string, int) {
	start := i
	first, size := utf8.DecodeRuneInString(source[i:])
	if !unicode.IsLetter(first) && first != '_' {
		return "", i
	}
	i += size
	for i < len(source) {
		r, size := utf8.DecodeRuneInString(source[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		i += size
	}
	return source[start:i], i
}

// rawTextHint names the raw-text context and its escapes. A brace inside script
// or style content is usually authored JavaScript or CSS, so a bare position and
// an expression-grammar message leave an author guessing which language the
// generator was reading.
func rawTextHint(context string, shellHead bool) string {
	element := rawTextElement(context)
	insert := "RawJavaScript or JsonForScript"
	if element == "style" {
		insert = "RawCSS"
	}
	hint := "; this is inside <" + element + "> content, where {...} is a template insertion." +
		" Write {{...}} to keep a literal brace, insert a value with " + insert +
		", or move the " + element + " to a file under the public asset directory"
	// The asymmetry is worth naming, because the same markup in a component head
	// contribution is read verbatim and compiles.
	if shellHead {
		hint += ". A <head> declared outside <html> is a contribution, whose script and style bodies are verbatim"
	}
	return hint
}

// annotateRawText adds the raw-text hint to a parser diagnostic.
func (p *htmlParser) annotateRawText(raw bool, context string, err error) error {
	if !raw {
		return err
	}
	var parseErr *syntax.ParseError
	if !errors.As(err, &parseErr) {
		return err
	}
	parseErr.Message += rawTextHint(context, p.insideShellHead)
	return err
}

// readRawUntilClose consumes the verbatim body of a raw-text element and
// leaves the parser after its closing tag.
func (p *htmlParser) readRawUntilClose(name string) (string, error) {
	closing := "</" + name
	start := p.pos
	for offset := p.pos; ; {
		index := strings.Index(strings.ToLower(p.source[offset:]), closing)
		if index < 0 {
			return "", p.errAt(start, "missing closing tag </"+name+">")
		}
		end := offset + index
		after := end + len(closing)
		// Require a tag terminator so a substring such as </styles> is content.
		rest := p.source[after:]
		trimmed := strings.TrimLeft(rest, " \t\r\n")
		if !strings.HasPrefix(trimmed, ">") {
			offset = after
			continue
		}
		text := p.source[start:end]
		p.pos = after + (len(rest) - len(trimmed)) + 1
		return text, nil
	}
}

// staticAttributeText returns the literal text of an attribute whose value
// contains no embedded expression.
func staticAttributeText(attribute Attribute) (string, bool) {
	if attribute.Boolean {
		return "", false
	}
	var text strings.Builder
	for _, part := range attribute.Value {
		if part.Expression != nil {
			return "", false
		}
		text.WriteString(part.Text)
	}
	return text.String(), true
}

func (p *htmlParser) parseAttributes(component bool) ([]Attribute, bool, error) {
	var attrs []Attribute
	for {
		p.skipSpace()
		if strings.HasPrefix(p.source[p.pos:], "/>") {
			p.pos += 2
			return attrs, true, nil
		}
		if p.pos < len(p.source) && p.source[p.pos] == '>' {
			p.pos++
			return attrs, false, nil
		}
		if p.pos >= len(p.source) {
			return nil, false, p.errAt(p.pos, "unterminated start tag")
		}
		start := p.pos
		name := p.readName()
		validName := isHTMLName(name)
		message := "attribute name must be lowercase or kebab-case"
		if component {
			validName = isLowerCamel(name)
			message = "component argument name must be lowerCamelCase"
		}
		if !validName {
			return nil, false, p.errAt(start, message)
		}
		p.skipSpace()
		if p.pos >= len(p.source) || p.source[p.pos] != '=' {
			attrs = append(attrs, Attribute{Kind: "html:attribute", Pos: p.position(start), Name: name, Boolean: true})
			continue
		}
		p.pos++
		p.skipSpace()
		parts, err := p.parseAttributeValue()
		if err != nil {
			return nil, false, err
		}
		attrs = append(attrs, Attribute{Kind: "html:attribute", Pos: p.position(start), Name: name, Value: parts})
	}
}

func (p *htmlParser) parseAttributeValue() ([]AttributePart, error) {
	if p.pos >= len(p.source) {
		return nil, p.errAt(p.pos, "expected attribute value")
	}
	if p.source[p.pos] == '{' {
		start := p.pos
		content, offset, err := p.readDirective()
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(content)
		if isControl(trimmed) {
			return nil, p.errAt(offset, "control blocks are forbidden in attributes")
		}
		p.context.SetOffset(p.pos)
		node, terminator, err := p.context.ParseEmbedded(syntax.Embedded{Text: content, StartOffset: start, ContentOffset: offset}, "html:attribute")
		if err != nil {
			return nil, err
		}
		p.pos = p.context.Offset()
		expr, ok := node.(*syntax.ExpressionNode)
		if terminator != nil || !ok {
			return nil, p.errAt(offset, "only expressions are allowed in attributes")
		}
		return []AttributePart{{Kind: expr.Kind, Pos: expr.Pos, Context: expr.Context, Expression: expr.Expression}}, nil
	}
	quote := p.source[p.pos]
	if quote != '\'' && quote != '"' {
		start := p.pos
		for p.pos < len(p.source) && !unicode.IsSpace(rune(p.source[p.pos])) && p.source[p.pos] != '>' {
			p.pos++
		}
		return []AttributePart{{Kind: "html:text", Pos: p.position(start), Text: p.source[start:p.pos]}}, nil
	}
	p.pos++
	var parts []AttributePart
	textStart := p.pos
	for p.pos < len(p.source) && p.source[p.pos] != quote {
		if p.source[p.pos] != '{' {
			p.pos++
			continue
		}
		if textStart < p.pos {
			parts = append(parts, AttributePart{Kind: "html:text", Pos: p.position(textStart), Text: p.source[textStart:p.pos]})
		}
		start := p.pos
		content, offset, err := p.readDirective()
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(content)
		if isControl(trimmed) {
			return nil, p.errAt(offset, "control blocks are forbidden in attributes")
		}
		p.context.SetOffset(p.pos)
		node, terminator, err := p.context.ParseEmbedded(syntax.Embedded{Text: content, StartOffset: start, ContentOffset: offset}, "html:attribute")
		if err != nil {
			return nil, err
		}
		p.pos = p.context.Offset()
		expr, ok := node.(*syntax.ExpressionNode)
		if terminator != nil || !ok {
			return nil, p.errAt(offset, "only expressions are allowed in attributes")
		}
		parts = append(parts, AttributePart{Kind: expr.Kind, Pos: expr.Pos, Context: expr.Context, Expression: expr.Expression})
		textStart = p.pos
	}
	if p.pos >= len(p.source) {
		return nil, p.errAt(textStart, "unterminated quoted attribute value")
	}
	if textStart < p.pos {
		parts = append(parts, AttributePart{Kind: "html:text", Pos: p.position(textStart), Text: p.source[textStart:p.pos]})
	}
	p.pos++
	return parts, nil
}

func (p *htmlParser) readDirective() (string, int, error) {
	start := p.pos
	p.pos++
	contentStart := p.pos
	depth := 0
	quote := byte(0)
	for p.pos < len(p.source) {
		c := p.source[p.pos]
		if quote != 0 {
			if c == '\\' {
				p.pos += 2
				continue
			}
			p.pos++
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			p.pos++
			continue
		}
		if c == '(' || c == '[' {
			depth++
		} else if c == ')' || c == ']' {
			depth--
		} else if c == '}' && depth == 0 {
			content := p.source[contentStart:p.pos]
			p.pos++
			return content, contentStart, nil
		}
		p.pos++
	}
	return "", 0, p.errAt(start, "unterminated template expression")
}

func (p *htmlParser) parseClosingTag() (string, error) {
	p.pos += 2
	p.skipSpace()
	name := p.readName()
	p.skipSpace()
	if p.pos >= len(p.source) || p.source[p.pos] != '>' {
		return "", p.errAt(p.pos, "expected '>' after closing tag")
	}
	p.pos++
	return name, nil
}

func (p *htmlParser) parseComment() (Node, error) {
	start := p.pos + 4
	end := strings.Index(p.source[start:], "-->")
	if end < 0 {
		return nil, p.errAt(p.pos, "unterminated HTML comment")
	}
	text := p.source[start : start+end]
	p.pos = start + end + 3
	return &CommentNode{Kind: "html:comment", Pos: p.position(start - 4), Text: text}, nil
}

func (p *htmlParser) parseDoctype() (Node, error) {
	start := p.pos + 2
	end := strings.IndexByte(p.source[start:], '>')
	if end < 0 {
		return nil, p.errAt(p.pos, "unterminated doctype")
	}
	text := strings.TrimSpace(p.source[start : start+end])
	p.pos = start + end + 1
	return &DoctypeNode{Kind: "html:doctype", Pos: p.position(start - 2), Text: text}, nil
}

func (p *htmlParser) readName() string {
	start := p.pos
	for p.pos < len(p.source) {
		c := p.source[p.pos]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == ':' || c == '_' {
			p.pos++
			continue
		}
		break
	}
	return p.source[start:p.pos]
}

func (p *htmlParser) skipSpace() {
	for p.pos < len(p.source) {
		r, size := utf8.DecodeRuneInString(p.source[p.pos:])
		if !unicode.IsSpace(r) {
			return
		}
		p.pos += size
	}
}

func (p *htmlParser) errAt(offset int, message string) error {
	return syntax.ErrorAtPosition(p.filename, p.source, offset, p.baseOffset, p.basePos, message)
}

func appendText(nodes []Node, text string, pos Position) []Node {
	if text == "" {
		return nodes
	}
	if len(nodes) > 0 {
		if previous, ok := nodes[len(nodes)-1].(*TextNode); ok {
			previous.Text += text
			return nodes
		}
	}
	return append(nodes, &TextNode{Kind: "html:text", Pos: pos, Text: text})
}

func (p *htmlParser) position(offset int) Position {
	return positionInHTMLFragment(p.source, offset, p.basePos)
}

func positionInHTMLFragment(source string, offset int, base Position) Position {
	line, col := base.Line, base.Col
	for i := 0; i < offset && i < len(source); {
		r, size := utf8.DecodeRuneInString(source[i:])
		i += size
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return Position{Line: line, Col: col}
}

func startsUpper(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func isPascal(name string) bool {
	return name != "" && startsUpper(name) && !strings.Contains(name, "-")
}

func isLowerCamel(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsLower(r) && !strings.Contains(name, "-")
}

func isHTMLName(name string) bool {
	if name == "" || startsUpper(name) {
		return false
	}
	for _, r := range name {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' && r != ':' && r != '_' {
			return false
		}
	}
	return true
}

func isControl(value string) bool {
	switch {
	// A value binding has no closer, but it still scopes the nodes after it, so
	// it is as much a block here as the rest: an attribute value has no later
	// siblings for it to reach.
	case strings.HasPrefix(value, "if "), strings.HasPrefix(value, "for "),
		strings.HasPrefix(value, "await "), strings.HasPrefix(value, "else if "),
		strings.HasPrefix(value, "recover "), strings.HasPrefix(value, "val "),
		strings.HasPrefix(value, "/"):
		return true
	}
	return value == "else" || value == "fallback" || value == "recover"
}
