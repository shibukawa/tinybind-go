package sqlbind

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// Parse parses one SQL template module. SQL string literals, quoted
// identifiers, comments, and PostgreSQL dollar-quoted strings are kept
// losslessly and never interpreted as template syntax.
func Parse(filename string, source []byte) (*Module, error) {
	return syntax.ParseModule(filename, string(source), []syntax.RootDeclaration{{
		Keyword: "statement", NodeType: "sql:statement", OutputPrefix: "sql",
		Context: "sql:value", Parser: formatParser{},
	}})
}

type formatParser struct{}

func (formatParser) ParseBody(context *syntax.BodyContext, insertionContext string) ([]syntax.Node, *syntax.Terminator, error) {
	p := sqlParser{context: context, source: context.Source(), pos: context.Offset()}
	nodes, terminator, err := p.parse(insertionContext)
	context.SetOffset(p.pos)
	return nodes, terminator, err
}

type sqlParser struct {
	context *syntax.BodyContext
	source  string
	pos     int
}

func (p *sqlParser) parse(insertionContext string) ([]syntax.Node, *syntax.Terminator, error) {
	var nodes []syntax.Node
	textStart := p.pos
	flush := func(end int) {
		if textStart == end {
			return
		}
		text := p.source[textStart:end]
		if len(nodes) > 0 {
			if previous, ok := nodes[len(nodes)-1].(*TextNode); ok {
				previous.Text += text
				return
			}
		}
		nodes = append(nodes, &TextNode{Kind: "sql:text", Pos: p.context.Position(textStart), Text: text})
	}
	for p.pos < len(p.source) {
		switch {
		case strings.HasPrefix(p.source[p.pos:], "--"):
			p.scanLineComment()
		case strings.HasPrefix(p.source[p.pos:], "/*"):
			if err := p.scanBlockComment(); err != nil {
				return nil, nil, err
			}
		case p.source[p.pos] == '\'' || p.source[p.pos] == '"':
			if err := p.scanQuoted(p.source[p.pos]); err != nil {
				return nil, nil, err
			}
		case p.source[p.pos] == '$':
			if end, ok := p.scanDollarQuote(); ok {
				p.pos = end
			} else if p.pos+1 < len(p.source) && p.source[p.pos+1] >= '0' && p.source[p.pos+1] <= '9' {
				return nil, nil, p.context.ErrorAt(p.pos, "manual SQL placeholders are forbidden; use a template expression")
			} else {
				p.pos++
			}
		case strings.HasPrefix(p.source[p.pos:], "{{"):
			end := strings.Index(p.source[p.pos+2:], "}}")
			if end < 0 {
				return nil, nil, p.context.ErrorAt(p.pos, "unterminated escaped template text")
			}
			flush(p.pos)
			nodes = appendText(nodes, "{"+p.source[p.pos+2:p.pos+2+end]+"}", p.context.Position(p.pos))
			p.pos += end + 4
			textStart = p.pos
		case p.hasSubquery():
			flush(p.pos)
			node, end, err := p.parseSubquery()
			if err != nil {
				return nil, nil, err
			}
			nodes = append(nodes, node)
			p.pos, textStart = end, end
		case p.source[p.pos] == '{':
			flush(p.pos)
			fragment, end, err := p.readEmbedded(p.pos)
			if err != nil {
				return nil, nil, err
			}
			p.pos = end
			p.context.SetOffset(end)
			node, terminator, err := p.context.ParseEmbedded(fragment, insertionContext)
			if err != nil {
				return nil, nil, err
			}
			p.pos = p.context.Offset()
			if terminator != nil {
				return nodes, terminator, nil
			}
			nodes = append(nodes, node)
			textStart = p.pos
		case p.source[p.pos] == '}':
			flush(p.pos)
			terminator := &syntax.Terminator{Kind: syntax.TerminatorRoot, Pos: p.context.Position(p.pos)}
			p.pos++
			return nodes, terminator, nil
		default:
			_, size := utf8.DecodeRuneInString(p.source[p.pos:])
			p.pos += size
		}
	}
	return nil, nil, p.context.ErrorAt(p.pos, "unterminated statement body")
}

func (p *sqlParser) hasSubquery() bool {
	const keyword = "subquery"
	if !strings.HasPrefix(p.source[p.pos:], keyword) {
		return false
	}
	if p.pos > 0 {
		r, _ := utf8.DecodeLastRuneInString(p.source[:p.pos])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return false
		}
	}
	end := p.pos + len(keyword)
	if end >= len(p.source) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(p.source[end:])
	return unicode.IsSpace(r)
}

func (p *sqlParser) parseSubquery() (*RelationNode, int, error) {
	start, pos := p.pos, p.pos+len("subquery")
	skipSpace := func() {
		for pos < len(p.source) {
			r, size := utf8.DecodeRuneInString(p.source[pos:])
			if !unicode.IsSpace(r) {
				break
			}
			pos += size
		}
	}
	skipSpace()
	nameStart := pos
	for pos < len(p.source) {
		r, size := utf8.DecodeRuneInString(p.source[pos:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		pos += size
	}
	name := p.source[nameStart:pos]
	if name == "" || !unicode.IsUpper([]rune(name)[0]) {
		return nil, 0, p.context.ErrorAt(nameStart, "subquery relation name must be PascalCase")
	}
	skipSpace()
	if pos >= len(p.source) || p.source[pos] != '(' {
		return nil, 0, p.context.ErrorAt(pos, "expected '(' after subquery relation name")
	}
	pos++
	var args []Expr
	for {
		skipSpace()
		if pos < len(p.source) && p.source[pos] == ')' {
			pos++
			break
		}
		argStart := pos
		depth := 0
		quote := byte(0)
		for pos < len(p.source) {
			ch := p.source[pos]
			if quote != 0 {
				if ch == '\\' {
					pos += 2
					continue
				}
				pos++
				if ch == quote {
					quote = 0
				}
				continue
			}
			if ch == '\'' || ch == '"' {
				quote = ch
				pos++
				continue
			}
			if ch == '(' || ch == '[' {
				depth++
			} else if ch == ')' && depth > 0 {
				depth--
			} else if depth == 0 && (ch == ',' || ch == ')') {
				break
			}
			pos++
		}
		argEnd := pos
		for argEnd > argStart {
			r, size := utf8.DecodeLastRuneInString(p.source[argStart:argEnd])
			if !unicode.IsSpace(r) {
				break
			}
			argEnd -= size
		}
		if argEnd == argStart {
			return nil, 0, p.context.ErrorAt(argStart, "expected subquery argument")
		}
		expr, err := syntax.ParseExpressionAt(p.context.Filename(), p.source[argStart:argEnd], argStart, p.context.Position(argStart))
		if err != nil {
			return nil, 0, err
		}
		args = append(args, expr)
		if pos >= len(p.source) {
			return nil, 0, p.context.ErrorAt(start, "unterminated subquery invocation")
		}
		if p.source[pos] == ',' {
			pos++
			continue
		}
		pos++
		break
	}
	skipSpace()
	if !strings.HasPrefix(p.source[pos:], "AS") || (pos+2 < len(p.source) && (unicode.IsLetter(rune(p.source[pos+2])) || unicode.IsDigit(rune(p.source[pos+2])) || p.source[pos+2] == '_')) {
		return nil, 0, p.context.ErrorAt(pos, "subquery requires AS and a lower_snake_case alias")
	}
	pos += 2
	skipSpace()
	aliasStart := pos
	for pos < len(p.source) {
		r, size := utf8.DecodeRuneInString(p.source[pos:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		pos += size
	}
	alias := p.source[aliasStart:pos]
	if !lowerSnake(alias) {
		return nil, 0, p.context.ErrorAt(aliasStart, "subquery alias must be lower_snake_case")
	}
	return &RelationNode{Kind: "sql:relation", Pos: p.context.Position(start), Name: name, Arguments: args, Alias: alias}, pos, nil
}

func lowerSnake(value string) bool {
	if value == "" || strings.HasPrefix(value, "_") || strings.HasSuffix(value, "_") || strings.Contains(value, "__") {
		return false
	}
	for i, r := range value {
		if i == 0 && !unicode.IsLower(r) {
			return false
		}
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func (p *sqlParser) scanLineComment() {
	if end := strings.IndexByte(p.source[p.pos:], '\n'); end >= 0 {
		p.pos += end + 1
	} else {
		p.pos = len(p.source)
	}
}

func (p *sqlParser) scanBlockComment() error {
	start, depth := p.pos, 0
	for p.pos < len(p.source) {
		if strings.HasPrefix(p.source[p.pos:], "/*") {
			depth++
			p.pos += 2
			continue
		}
		if strings.HasPrefix(p.source[p.pos:], "*/") {
			depth--
			p.pos += 2
			if depth == 0 {
				return nil
			}
			continue
		}
		p.pos++
	}
	return p.context.ErrorAt(start, "unterminated SQL block comment")
}

func (p *sqlParser) scanQuoted(quote byte) error {
	start := p.pos
	p.pos++
	for p.pos < len(p.source) {
		if p.source[p.pos] == quote {
			if p.pos+1 < len(p.source) && p.source[p.pos+1] == quote {
				p.pos += 2
				continue
			}
			p.pos++
			return nil
		}
		if p.source[p.pos] == '\\' && quote == '\'' && p.pos+1 < len(p.source) {
			p.pos += 2
			continue
		}
		p.pos++
	}
	return p.context.ErrorAt(start, "unterminated SQL quoted value")
}

func (p *sqlParser) scanDollarQuote() (int, bool) {
	i := p.pos + 1
	for i < len(p.source) && (unicode.IsLetter(rune(p.source[i])) || unicode.IsDigit(rune(p.source[i])) || p.source[i] == '_') {
		i++
	}
	if i >= len(p.source) || p.source[i] != '$' {
		return 0, false
	}
	tag := p.source[p.pos : i+1]
	end := strings.Index(p.source[i+1:], tag)
	if end < 0 {
		return 0, false
	}
	return i + 1 + end + len(tag), true
}

func (p *sqlParser) readEmbedded(start int) (syntax.Embedded, int, error) {
	pos, contentStart, depth, quote := start+1, start+1, 0, byte(0)
	for pos < len(p.source) {
		c := p.source[pos]
		if quote != 0 {
			if c == '\\' {
				pos += 2
				continue
			}
			pos++
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			pos++
			continue
		}
		if c == '(' || c == '[' {
			depth++
		} else if c == ')' || c == ']' {
			depth--
		} else if c == '}' && depth == 0 {
			return syntax.Embedded{Text: p.source[contentStart:pos], StartOffset: start, ContentOffset: contentStart}, pos + 1, nil
		}
		pos++
	}
	return syntax.Embedded{}, 0, p.context.ErrorAt(start, "unterminated template expression")
}

func appendText(nodes []syntax.Node, text string, pos Position) []syntax.Node {
	if text == "" {
		return nodes
	}
	if len(nodes) > 0 {
		if previous, ok := nodes[len(nodes)-1].(*TextNode); ok {
			previous.Text += text
			return nodes
		}
	}
	return append(nodes, &TextNode{Kind: "sql:text", Pos: pos, Text: text})
}
