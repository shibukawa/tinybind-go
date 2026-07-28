package routetree

import (
	"bytes"
	"fmt"
	"go/format"
	"path"
	"strings"
)

// TemplateComposer is the name of the template that renders a route composer.
// Pass it to [Emitter.Parse] to replace the emitted shape.
const TemplateComposer = "composer"

// RenderFunc is the name of the generated composer.
const RenderFunc = "Render"

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

// ComposerModel is the data the composer template renders.
type ComposerModel struct {
	Header     string
	Package    string
	Pattern    string
	Imports    []Import
	ParamsType string
	RenderFunc string
	// Component is the page component name, which also prefixes its generated
	// parameter struct.
	Component string
	Layouts   []ComposerLayout
	Symbols   Symbols
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
		Header:     GeneratedHeader,
		Package:    route.Package,
		Pattern:    route.Pattern(),
		ParamsType: e.ParamsType,
		RenderFunc: e.RenderFunc,
		Component:  PageComponentName,
		Symbols:    e.Symbols,
	}

	// Two ancestors may share a directory base name, so selectors are made
	// unique before any of them reaches the import block.
	aliases := newAliasSet(route.Package)
	imports := []Import{{Path: "io"}}
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
			imports = append(imports, Import{Path: layout.ImportPath, Alias: aliasFor(layout.ImportPath, alias), Group: len(imports) == 1})
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
			Group: len(imports) == 1,
		})
	}
	model.Imports = imports
	return model, nil
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
