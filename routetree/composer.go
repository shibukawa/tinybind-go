package routetree

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"sort"
	"strings"
)

// TemplateComposer is the name of the template that renders a route composer.
// Pass it to [Emitter.Parse] to replace the emitted shape.
const TemplateComposer = "composer"

// TemplateRender is the name of the block that writes the render call itself,
// in both the composer and the registry handler. Pass it to [Emitter.Parse] to
// send the request, or anything else in scope, to a framework's own entry:
//
//	e.Parse(routetree.TemplateRender,
//		`web.WriteHTML({{ .Writer }}, {{ .Request }}, {{ .Chain }}, {{ .Leaf }})`)
//
// The block writes an expression of type error, so the caller keeps deciding
// what a failure does. Its data is [RenderCall].
//
// An override may only name packages the generated file already imports: the
// runtime and error packages of [Symbols], the request package, and io. Pointing
// Symbols.RuntimeImport at the package holding the entry is what covers the usual
// case, where the entry and the option type ship together.
const TemplateRender = "render"

// RenderFunc is the name of the generated composer.
const RenderFunc = "Render"

// DefaultRenderWriterType is the writer the generated composer takes when
// [Emitter.RenderWriterType] is empty. It is an io.Writer rather than a
// ResponseWriter because a handler that renders into a buffer can still choose
// its status, which is what the script-free mode and a static export need.
const DefaultRenderWriterType = "io.Writer"

// SlotParamName is the parameter a layout must declare to become a chain
// wrapper. The template compiler emits a Bind wrapper only for an exported
// component carrying an html parameter with this name, so the convention is
// the compiler's rather than this package's.
const SlotParamName = "children"

// ComposerLayout is one wrapper in a generated composer, already resolved to
// the selector, binder, and argument list the template writes.
type ComposerLayout struct {
	// Selector is the qualifier for symbols of this layout's package, such as
	// "users." It is empty when the layout lives in the page's own package.
	Selector string
	// Binder is the generated wrapper constructor, such as BindLayout.
	Binder string
	// ParamsType is the generated parameter struct, such as LayoutParams.
	ParamsType string
	// Args maps each declared layout input to the route field feeding it.
	Args []ComposerArg
}

// ComposerArg is one layout input filled from the decoded route.
type ComposerArg struct {
	// Field is the layout parameter struct field.
	Field string
	// From is the route parameter struct field it reads.
	From string
}

// RenderCall is the data the [TemplateRender] block renders. Every field but
// Symbols is the name of something already in scope at the call, so an override
// composes its own call out of them rather than guessing what it may reference.
type RenderCall struct {
	// Writer is the writer identifier, always present.
	Writer string
	// Request is the request identifier, empty where none is in scope. The
	// registry handler always has one; the composer has one only when
	// [Emitter.RenderRequestParam] declared it.
	Request string
	// Wrappers is the layout chain slice identifier, empty when the page has no
	// ancestor layout and renders on its own.
	Wrappers string
	// Leaf is the expression building the page fragment, such as Page(params).
	Leaf string
	// Options is the render option slice identifier, empty where none is in
	// scope.
	Options string
	// Symbols are the identities generated code calls.
	Symbols Symbols
}

// Chain is Wrappers, or nil when the page has no ancestor layout. It is what an
// entry point that always takes a chain argument writes, so such an override
// needs no branch of its own:
//
//	web.WriteHTML({{ .Writer }}, {{ .Request }}, {{ .Chain }}, {{ .Leaf }})
func (c RenderCall) Chain() string {
	if c.Wrappers == "" {
		return "nil"
	}
	return c.Wrappers
}

// ComposerModel is the data the composer template renders.
type ComposerModel struct {
	Header     string
	Package    string
	Pattern    string
	Imports    []Import
	ParamsType string
	RenderFunc string
	// WriterType is the writer the generated composer takes.
	WriterType string
	// RequestParam is the request parameter name, empty when the composer
	// declares none.
	RequestParam string
	// Component is the page component name, which also prefixes its generated
	// parameter struct.
	Component string
	Layouts   []ComposerLayout
	Symbols   Symbols
}

// Render describes the render call for this composer, which the template hands
// to the [TemplateRender] block.
func (m ComposerModel) Render() RenderCall {
	call := RenderCall{
		Writer:  "w",
		Request: m.RequestParam,
		Leaf:    m.Component + "(params)",
		Options: "options",
		Symbols: m.Symbols,
	}
	if len(m.Layouts) > 0 {
		call.Wrappers = "wrappers"
	}
	return call
}

// Composer renders the per-route Render function.
//
// layouts must hold one signature per entry of route.Layouts, in the same
// order. A layout that declares no slot parameter is rejected, because the
// template compiler emits no wrapper binder for it and the generated call
// would not compile.
func (e *Emitter) Composer(route Route, layouts []ComponentSignature) ([]byte, error) {
	model, err := e.composerModel(route, layouts)
	if err != nil {
		return nil, err
	}
	var raw bytes.Buffer
	if err := e.tmpl.ExecuteTemplate(&raw, TemplateComposer, model); err != nil {
		return nil, fmt.Errorf("routetree: rendering %s: %w", TemplateComposer, err)
	}
	source, err := format.Source(raw.Bytes())
	if err != nil {
		return nil, fmt.Errorf("routetree: template %q produced unparsable Go for %s: %w\n%s",
			TemplateComposer, route.Path, err, raw.String())
	}
	return source, nil
}

func (e *Emitter) composerModel(route Route, layouts []ComponentSignature) (ComposerModel, error) {
	if len(layouts) != len(route.Layouts) {
		return ComposerModel{}, &Error{
			Path: route.PageFile,
			Message: fmt.Sprintf("route %s has %d layout(s) but %d signature(s) were supplied",
				route.Path, len(route.Layouts), len(layouts)),
		}
	}

	model := ComposerModel{
		Header:       GeneratedHeader,
		Package:      route.Package,
		Pattern:      route.Pattern(),
		ParamsType:   e.ParamsType,
		RenderFunc:   e.RenderFunc,
		WriterType:   orDefault(e.RenderWriterType, DefaultRenderWriterType),
		RequestParam: e.RenderRequestParam,
		Component:    PageComponentName,
		Symbols:      e.Symbols,
	}

	// Two ancestors may share a directory base name, so selectors are made
	// unique before any of them reaches the import block.
	aliases := newAliasSet(route.Package)
	var imports []Import
	var errs []error

	for i, layout := range route.Layouts {
		signature := layouts[i]
		if !hasSlot(signature) {
			errs = append(errs, &Error{
				Path: layout.File,
				Message: fmt.Sprintf("component %s must declare `%s: html` to wrap a page; "+
					"the template compiler emits a wrapper binder only for that shape",
					signature.Name, SlotParamName),
			})
			continue
		}
		entry := ComposerLayout{
			Binder:     "Bind" + signature.Name,
			ParamsType: signature.Name + "Params",
		}
		if layout.ImportPath != "" && layout.RelDir != route.RelDir {
			alias := aliases.add(layout.Package, layout.ImportPath)
			entry.Selector = alias + "."
			imports = append(imports, Import{Path: layout.ImportPath, Alias: aliasFor(layout.ImportPath, alias)})
		}
		args, argErrs := composerArgs(layout, signature)
		errs = append(errs, argErrs...)
		entry.Args = args
		model.Layouts = append(model.Layouts, entry)
	}
	if len(errs) > 0 {
		return ComposerModel{}, joinErrors(errs)
	}

	if e.Symbols.RuntimeImport != "" {
		imports = append(imports, Import{
			Path:  e.Symbols.RuntimeImport,
			Alias: aliasFor(e.Symbols.RuntimeImport, e.Symbols.RuntimeAlias),
		})
	}
	model.Imports = groupImports(e.composerHead(model), imports)
	return model, nil
}

// composerHead lists the standard library packages the composer's own signature
// names. The writer type is configurable, so what it needs is read off the type
// rather than assumed: a framework whose writer lives in the runtime package
// needs no head import at all, because the runtime is imported anyway.
func (e *Emitter) composerHead(model ComposerModel) []Import {
	var head []Import
	qualifier := typeQualifier(model.WriterType)
	if qualifier == "io" {
		head = append(head, Import{Path: "io"})
	}
	needsRequest := model.RequestParam != "" || qualifier == e.Symbols.HTTPAlias
	if needsRequest && e.Symbols.HTTPImport != "" {
		head = append(head, Import{
			Path:  e.Symbols.HTTPImport,
			Alias: aliasFor(e.Symbols.HTTPImport, e.Symbols.HTTPAlias),
		})
	}
	sort.Slice(head, func(i, j int) bool { return head[i].Path < head[j].Path })
	return head
}

// typeQualifier reports the package selector a type expression begins with, so a
// generated signature can be matched against the packages a file imports.
func typeQualifier(expr string) string {
	expr = strings.TrimPrefix(expr, "*")
	if index := strings.IndexByte(expr, '.'); index > 0 {
		return expr[:index]
	}
	return ""
}

// composerArgs matches each declared layout input to a route parameter. A
// layout may only name a dynamic segment at or above its own directory, which
// is what keeps an ancestor wrapper reusable when a deeper segment changes.
func composerArgs(layout Layout, signature ComponentSignature) ([]ComposerArg, []error) {
	inScope := make(map[string]bool, len(layout.Params))
	for _, segment := range layout.Params {
		inScope[segment.Name] = true
	}

	var args []ComposerArg
	var errs []error
	for _, input := range signature.Inputs {
		if !inScope[input.Name] {
			errs = append(errs, &Error{
				Path: layout.File,
				Message: fmt.Sprintf("component %s declares %q, which is not a dynamic segment at or above %s; "+
					"a layout may only read segments it encloses",
					signature.Name, input.Name, layoutLocation(layout)),
			})
			continue
		}
		args = append(args, ComposerArg{Field: ExportedName(input.Name), From: ExportedName(input.Name)})
	}
	return args, errs
}

func layoutLocation(layout Layout) string {
	if layout.RelDir == "" {
		return "the route root"
	}
	return layout.RelDir
}

func hasSlot(signature ComponentSignature) bool {
	for _, slot := range signature.Slots {
		if slot.Name == SlotParamName {
			return true
		}
	}
	return false
}

// aliasSet hands out import selectors that collide with neither each other nor
// the package being generated into.
type aliasSet struct {
	taken map[string]string // alias -> import path
}

func newAliasSet(own string) *aliasSet {
	return &aliasSet{taken: map[string]string{own: ""}}
}

func (s *aliasSet) add(preferred, importPath string) string {
	if existing, ok := s.taken[preferred]; ok && existing == importPath {
		return preferred
	}
	candidate := preferred
	if candidate == "" {
		candidate = PackageName(path.Base(importPath))
	}
	for i := 1; ; i++ {
		if existing, ok := s.taken[candidate]; !ok {
			s.taken[candidate] = importPath
			return candidate
		} else if existing == importPath {
			return candidate
		}
		candidate = fmt.Sprintf("%s%d", strings.TrimRight(preferred, "0123456789"), i)
	}
}
