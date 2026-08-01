package htmlbind

import "github.com/shibukawa/tinybind-go/templates/internal/syntax"

// Parse parses one template source using the shared declarations and the HTML
// component root declaration.
func Parse(filename string, source []byte) (*Module, error) {
	return syntax.ParseModule(filename, string(source), []syntax.RootDeclaration{{
		Keyword:      "component",
		NodeType:     "html:component",
		OutputPrefix: "html",
		Context:      "html:child",
		Parser:       formatParser{},
		// A component is called as a tag whose name starts uppercase, so the
		// name form is the call syntax rather than a convention.
		Names: syntax.NameRule{PascalCase: true, ExportedNameIsGo: true},
	}})
}

type formatParser struct{}

func (formatParser) ParseBody(context *syntax.BodyContext, insertionContext string) ([]syntax.Node, *syntax.Terminator, error) {
	p := &htmlParser{
		context:    context,
		filename:   context.Filename(),
		source:     context.Source(),
		baseOffset: 0,
		basePos:    syntax.Position{Line: 1, Col: 1},
		pos:        context.Offset(),
	}
	nodes, terminator, err := p.parseNodes("", insertionContext)
	context.SetOffset(p.pos)
	return nodes, terminator, err
}
