package syntax

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RootDeclaration registers one format-specific declaration with the shared
// root driver.
type RootDeclaration struct {
	Keyword      string
	NodeType     string
	OutputPrefix string
	Context      string
	Parser       FormatParser
}

// ParseModule parses standard declarations plus the registered format roots.
func ParseModule(filename, source string, roots []RootDeclaration) (*Module, error) {
	rootByKeyword := make(map[string]RootDeclaration, len(roots))
	for _, root := range roots {
		if root.Keyword == "" || root.NodeType == "" || root.Context == "" || root.Parser == nil {
			return nil, fmt.Errorf("invalid root declaration registration")
		}
		if _, exists := rootByKeyword[root.Keyword]; exists {
			return nil, fmt.Errorf("duplicate root declaration %q", root.Keyword)
		}
		rootByKeyword[root.Keyword] = root
	}
	p := &moduleParser{filename: filename, source: source, roots: rootByKeyword}
	return p.parse()
}

type moduleParser struct {
	filename string
	source   string
	pos      int
	roots    map[string]RootDeclaration
}

func (p *moduleParser) parse() (*Module, error) {
	module := &Module{Pos: Position{Line: 1, Col: 1}, Declarations: []Declaration{}}
	for {
		if err := p.skipSpaceAndComments(); err != nil {
			return nil, err
		}
		if p.eof() {
			return module, nil
		}
		start := p.pos
		keyword, err := p.identifier()
		if err != nil {
			return nil, err
		}
		switch keyword {
		case "package", "module":
			if module.Package != nil {
				return nil, p.errAt(start, "package or module may only be declared once")
			}
			name, err := p.qualifiedName()
			if err != nil {
				return nil, err
			}
			module.Package = &PackageDecl{Kind: "template:" + keyword, Pos: positionAt(p.source, start), Name: name}
			p.optionalSemicolon()
		case "import":
			path, err := p.stringLiteral()
			if err != nil {
				return nil, err
			}
			p.skipSpaceAndComments()
			alias := ""
			if p.peekIdentifier() == "as" {
				_, _ = p.identifier()
				alias, err = p.identifier()
				if err != nil {
					return nil, err
				}
			}
			module.Imports = append(module.Imports, ImportDecl{Pos: positionAt(p.source, start), Path: path, Alias: alias})
			p.optionalSemicolon()
		case "type", "record":
			decl, err := p.parseTypeDecl(start)
			if err != nil {
				return nil, err
			}
			module.Declarations = append(module.Declarations, decl)
		case "enum":
			decl, err := p.parseEnumDecl(start)
			if err != nil {
				return nil, err
			}
			module.Declarations = append(module.Declarations, decl)
		case "external":
			decl, err := p.parseExternalDecl(start)
			if err != nil {
				return nil, err
			}
			module.Declarations = append(module.Declarations, decl)
		case "export":
			rootKeyword, err := p.identifier()
			if err != nil {
				return nil, err
			}
			// A reloadable component is published as an endpoint, so the
			// modifier follows export rather than standing on its own.
			reloadable := rootKeyword == "reloadable"
			if reloadable {
				if rootKeyword, err = p.identifier(); err != nil {
					return nil, err
				}
			}
			root, ok := p.roots[rootKeyword]
			if !ok {
				return nil, p.errAt(start, "unsupported exported declaration "+strconv.Quote(rootKeyword))
			}
			decl, err := p.parseTemplateDecl(root, true, start)
			if err != nil {
				return nil, err
			}
			if reloadable {
				decl.Reloadable = true
			}
			module.Declarations = append(module.Declarations, decl)
		default:
			root, ok := p.roots[keyword]
			if !ok {
				return nil, p.errAt(start, "unknown root declaration "+strconv.Quote(keyword))
			}
			decl, err := p.parseTemplateDecl(root, false, start)
			if err != nil {
				return nil, err
			}
			module.Declarations = append(module.Declarations, decl)
		}
	}
}

func (p *moduleParser) parseTypeDecl(start int) (*TypeDecl, error) {
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if err := requirePascalCase(name); err != nil {
		return nil, p.errAt(p.pos-len(name), err.Error())
	}
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	decl := &TypeDecl{Kind: "template:type", Pos: positionAt(p.source, start), Name: name}
	for {
		if err := p.skipSpaceAndComments(); err != nil {
			return nil, err
		}
		if p.accept('}') {
			break
		}
		fieldStart := p.pos
		fieldName, err := p.identifier()
		if err != nil {
			return nil, err
		}
		if err := requireLowerCamel(fieldName); err != nil {
			return nil, p.errAt(p.pos-len(fieldName), err.Error())
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		typeRef, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		decl.Fields = append(decl.Fields, Field{Pos: positionAt(p.source, fieldStart), Name: fieldName, Type: typeRef})
		p.acceptSeparator()
	}
	p.optionalSemicolon()
	return decl, nil
}

func (p *moduleParser) parseEnumDecl(start int) (*EnumDecl, error) {
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if err := requirePascalCase(name); err != nil {
		return nil, p.errAt(p.pos-len(name), err.Error())
	}
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	decl := &EnumDecl{Kind: "template:enum", Pos: positionAt(p.source, start), Name: name}
	for {
		if err := p.skipSpaceAndComments(); err != nil {
			return nil, err
		}
		if p.accept('}') {
			break
		}
		memberStart := p.pos
		member, err := p.identifier()
		if err != nil {
			return nil, err
		}
		if err := requirePascalCase(member); err != nil {
			return nil, p.errAt(p.pos-len(member), err.Error())
		}
		decl.Members = append(decl.Members, EnumMember{Pos: positionAt(p.source, memberStart), Name: member})
		p.acceptSeparator()
	}
	p.optionalSemicolon()
	return decl, nil
}

func (p *moduleParser) parseExternalDecl(start int) (*ExternalDecl, error) {
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if err := requirePascalCase(name); err != nil {
		return nil, p.errAt(p.pos-len(name), err.Error())
	}
	params, err := p.parseParameters()
	if err != nil {
		return nil, err
	}
	if err := p.expect(':'); err != nil {
		return nil, err
	}
	result, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	p.optionalSemicolon()
	return &ExternalDecl{Kind: "template:external", Pos: positionAt(p.source, start), Name: name, Parameters: params, Result: result}, nil
}

func (p *moduleParser) parseTemplateDecl(root RootDeclaration, exported bool, start int) (*TemplateDecl, error) {
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if err := requirePascalCase(name); err != nil {
		return nil, p.errAt(p.pos-len(name), err.Error())
	}
	params, err := p.parseParameters()
	if err != nil {
		return nil, err
	}
	if err := p.expect(':'); err != nil {
		return nil, err
	}
	output, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	if output.Name != root.OutputPrefix && !strings.HasPrefix(output.Name, root.OutputPrefix+".") {
		return nil, p.errAt(p.pos, fmt.Sprintf("%s declaration requires %s output", root.Keyword, root.OutputPrefix))
	}
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	context := newBodyContext(p.filename, p.source, p.pos, root.Parser)
	body, terminator, err := root.Parser.ParseBody(context, root.Context)
	if err != nil {
		return nil, err
	}
	if terminator == nil || terminator.Kind != TerminatorRoot {
		return nil, p.errAt(context.Offset(), "format parser did not terminate the declaration body")
	}
	p.pos = context.Offset()
	p.optionalSemicolon()
	return &TemplateDecl{Kind: root.NodeType, Pos: positionAt(p.source, start), Exported: exported, Name: name, Parameters: params, Output: output, Body: body}, nil
}

func (p *moduleParser) parseParameters() ([]Parameter, error) {
	if err := p.expect('('); err != nil {
		return nil, err
	}
	var params []Parameter
	if p.accept(')') {
		return params, nil
	}
	for {
		p.skipSpaceAndComments()
		paramStart := p.pos
		name, err := p.identifier()
		if err != nil {
			return nil, err
		}
		if err := requireLowerCamel(name); err != nil {
			return nil, p.errAt(p.pos-len(name), err.Error())
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		typeRef, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		params = append(params, Parameter{Pos: positionAt(p.source, paramStart), Name: name, Type: typeRef})
		if p.accept(')') {
			return params, nil
		}
		if err := p.expect(','); err != nil {
			return nil, err
		}
	}
}

func (p *moduleParser) parseTypeRef() (TypeRef, error) {
	p.skipSpaceAndComments()
	start := p.pos
	if p.accept('[') {
		inner, err := p.parseTypeRef()
		if err != nil {
			return TypeRef{}, err
		}
		if err := p.expect(']'); err != nil {
			return TypeRef{}, err
		}
		inner.Array = true
		if p.accept('?') {
			inner.Optional = true
		}
		inner.Pos = positionAt(p.source, start)
		return inner, nil
	}
	name, err := p.qualifiedName()
	if err != nil {
		return TypeRef{}, err
	}
	typeRef := TypeRef{Pos: positionAt(p.source, start), Name: name}
	if p.accept('<') {
		for {
			arg, err := p.parseTypeRef()
			if err != nil {
				return TypeRef{}, err
			}
			typeRef.Arguments = append(typeRef.Arguments, arg)
			if p.accept('>') {
				break
			}
			if err := p.expect(','); err != nil {
				return TypeRef{}, err
			}
		}
	}
	if p.accept('[') {
		if err := p.expect(']'); err != nil {
			return TypeRef{}, err
		}
		typeRef.Array = true
	}
	if p.accept('?') {
		typeRef.Optional = true
	}
	return typeRef, nil
}

func (p *moduleParser) skipSpaceAndComments() error {
	for {
		for !p.eof() {
			r, size := utf8.DecodeRuneInString(p.source[p.pos:])
			if !unicode.IsSpace(r) {
				break
			}
			p.pos += size
		}
		if strings.HasPrefix(p.source[p.pos:], "//") {
			if end := strings.IndexByte(p.source[p.pos:], '\n'); end >= 0 {
				p.pos += end + 1
				continue
			}
			p.pos = len(p.source)
			return nil
		}
		if strings.HasPrefix(p.source[p.pos:], "/*") {
			end := strings.Index(p.source[p.pos+2:], "*/")
			if end < 0 {
				return p.errAt(p.pos, "unterminated block comment")
			}
			p.pos += end + 4
			continue
		}
		return nil
	}
}

func (p *moduleParser) identifier() (string, error) {
	p.skipSpaceAndComments()
	if p.eof() {
		return "", p.errAt(p.pos, "expected identifier")
	}
	start := p.pos
	r, size := utf8.DecodeRuneInString(p.source[p.pos:])
	if !unicode.IsLetter(r) && r != '_' {
		return "", p.errAt(p.pos, "expected identifier")
	}
	p.pos += size
	for !p.eof() {
		r, size = utf8.DecodeRuneInString(p.source[p.pos:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			break
		}
		p.pos += size
	}
	return p.source[start:p.pos], nil
}

func (p *moduleParser) qualifiedName() (string, error) {
	name, err := p.identifier()
	if err != nil {
		return "", err
	}
	for p.accept('.') {
		part, err := p.identifier()
		if err != nil {
			return "", err
		}
		name += "." + part
	}
	return name, nil
}

func (p *moduleParser) stringLiteral() (string, error) {
	p.skipSpaceAndComments()
	if p.eof() || (p.source[p.pos] != '"' && p.source[p.pos] != '\'') {
		return "", p.errAt(p.pos, "expected string literal")
	}
	start := p.pos
	quote := p.source[p.pos]
	p.pos++
	for !p.eof() {
		c := p.source[p.pos]
		p.pos++
		if c == '\\' && !p.eof() {
			p.pos++
			continue
		}
		if c == quote {
			raw := p.source[start:p.pos]
			if quote == '\'' {
				return raw[1 : len(raw)-1], nil
			}
			value, err := strconv.Unquote(raw)
			if err != nil {
				return "", p.errAt(start, "invalid string literal")
			}
			return value, nil
		}
	}
	return "", p.errAt(start, "unterminated string literal")
}

func (p *moduleParser) expect(want byte) error {
	p.skipSpaceAndComments()
	if p.eof() || p.source[p.pos] != want {
		return p.errAt(p.pos, "expected "+strconv.QuoteRune(rune(want)))
	}
	p.pos++
	return nil
}

func (p *moduleParser) accept(want byte) bool {
	p.skipSpaceAndComments()
	if p.eof() || p.source[p.pos] != want {
		return false
	}
	p.pos++
	return true
}

func (p *moduleParser) optionalSemicolon() { p.accept(';') }

func (p *moduleParser) acceptSeparator() {
	p.skipSpaceAndComments()
	if !p.eof() && (p.source[p.pos] == ',' || p.source[p.pos] == ';') {
		p.pos++
	}
}

func (p *moduleParser) peekIdentifier() string {
	saved := p.pos
	name, _ := p.identifier()
	p.pos = saved
	return name
}

func (p *moduleParser) eof() bool { return p.pos >= len(p.source) }

func (p *moduleParser) errAt(offset int, message string) error {
	return errorAt(p.filename, p.source, offset, message)
}

func requirePascalCase(name string) error {
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(r) {
		return fmt.Errorf("%q must be PascalCase", name)
	}
	return nil
}

func requireLowerCamel(name string) error {
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsLower(r) {
		return fmt.Errorf("%q must be lowerCamelCase", name)
	}
	return nil
}
