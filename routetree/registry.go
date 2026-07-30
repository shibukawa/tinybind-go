package routetree

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// TemplateRegistry is the name of the template that renders the integrated
// ServeMux. Pass it to [Emitter.Parse] to replace the emitted shape.
const TemplateRegistry = "registry"

// Default names of the registry's generated declarations.
const (
	RegisterFunc   = "Register"
	MuxFunc        = "NewServeMux"
	TableVar       = "Routes"
	ActionTableVar = "Actions"
)

// RegistryAction is one server function endpoint lowered to what the registry
// template writes.
type RegistryAction struct {
	Pattern string
	Path    string
	Hash    string
	Name    string
	RelDir  string
	// Selector qualifies the handler's package, such as "id_." It is empty for
	// a handler in the root package itself.
	Selector string
}

// RegistryRoute is one route lowered to what the registry template writes.
type RegistryRoute struct {
	Pattern string
	Path    string
	RelDir  string
	// ParamNames are the dynamic segment names, in route order.
	ParamNames []string
	// Selector qualifies symbols of the route's package, such as "id_." It is
	// empty for a route in the root package itself.
	Selector string
	// Raw registers the route's own handler directly and generates no body.
	Raw bool
	// Call is set when a typed func Page must run before rendering.
	Call bool
	// CallResults is the assignment prefix for that call, such as "u, err :=".
	CallResults string
	// CallArgs is the argument list for that call, read from the decoded route.
	CallArgs string
	// PageFields fills the page component parameter struct.
	PageFields []ComposerArg
	// Layouts are the ancestor wrappers, outermost first.
	Layouts []ComposerLayout
}

// RegistryModel is the data the registry template renders.
type RegistryModel struct {
	Header         string
	Package        string
	Imports        []Import
	RegisterFunc   string
	MuxFunc        string
	TableVar       string
	ActionTableVar string
	DecodeFunc     string
	Routes         []RegistryRoute
	Actions        []RegistryAction
	Symbols        Symbols
}

// Render describes the render call of one route's generated handler, which the
// registry template hands to the [TemplateRender] block. The request is always
// in scope here, so an override reaches it without any setting.
func (m RegistryModel) Render(route RegistryRoute) RenderCall {
	call := RenderCall{
		Writer:  "w",
		Request: "r",
		Leaf:    route.Selector + PageComponentName + "(params)",
		Options: "options",
		Symbols: m.Symbols,
	}
	if len(route.Layouts) > 0 {
		call.Wrappers = "wrappers"
	}
	return call
}

// Registry renders the integrated ServeMux for a whole tree, into the route
// root package.
//
// Putting it in the root is what makes every generated import point down the
// tree: the registry reaches route and layout packages below it, and none of
// them reaches back up. A per-route composer in a leaf package would have to
// import its ancestors, which with a registry in the root is a cycle.
//
// analyses must hold one entry per route of the tree, layouts maps a layout's
// RelDir to its signature, and actions are the server functions discovered
// across the tree.
func (e *Emitter) Registry(tree *Tree, rootPackage string, analyses []Analysis, layouts map[string]ComponentSignature, actions []Action) ([]byte, error) {
	model, err := e.registryModel(tree, rootPackage, analyses, layouts, actions)
	if err != nil {
		return nil, err
	}
	var raw bytes.Buffer
	if err := e.tmpl.ExecuteTemplate(&raw, TemplateRegistry, model); err != nil {
		return nil, fmt.Errorf("routetree: rendering %s: %w", TemplateRegistry, err)
	}
	source, err := format.Source(raw.Bytes())
	if err != nil {
		return nil, fmt.Errorf("routetree: template %q produced unparsable Go: %w\n%s",
			TemplateRegistry, err, raw.String())
	}
	return source, nil
}

func (e *Emitter) registryModel(tree *Tree, rootPackage string, analyses []Analysis, layouts map[string]ComponentSignature, actions []Action) (RegistryModel, error) {
	if len(analyses) != len(tree.Routes) {
		return RegistryModel{}, fmt.Errorf("routetree: tree has %d route(s) but %d analysis result(s) were supplied",
			len(tree.Routes), len(analyses))
	}

	model := RegistryModel{
		Header:         GeneratedHeader,
		Package:        rootPackage,
		RegisterFunc:   e.RegisterFunc,
		MuxFunc:        e.MuxFunc,
		TableVar:       e.TableVar,
		ActionTableVar: orDefault(e.ActionTableVar, ActionTableVar),
		DecodeFunc:     e.DecodeFunc,
		Symbols:        e.Symbols,
	}

	aliases := newAliasSet(rootPackage)
	// The route packages and the runtime are collected apart from the standard
	// library head, because whether the request package belongs in that head is
	// not known until every route has been read: only a generated handler body
	// names ResponseWriter and Request, so a tree of nothing but raw handlers
	// imports the router alone.
	var tail []Import
	needsRequest := false
	addImport := func(path, preferred string) string {
		if path == "" {
			// Without an ImportBase there is nothing to import; the symbol is
			// assumed to be reachable unqualified, which is only true in the
			// root package itself.
			return ""
		}
		alias := aliases.add(preferred, path)
		if alias == rootPackage && preferred == rootPackage {
			return ""
		}
		for _, existing := range tail {
			if existing.Path == path {
				return alias + "."
			}
		}
		tail = append(tail, Import{Path: path, Alias: aliasFor(path, alias)})
		return alias + "."
	}

	var errs []error
	for i, route := range tree.Routes {
		analysis := analyses[i]
		entry := RegistryRoute{
			Pattern: route.Pattern(),
			Path:    route.Path,
			RelDir:  route.RelDir,
		}
		for _, segment := range route.Params {
			entry.ParamNames = append(entry.ParamNames, segment.Name)
		}
		if route.RelDir != "" {
			entry.Selector = addImport(route.ImportPath, route.Package)
		}

		if analysis.Page != nil && analysis.Page.Rung == RungHandlerPage {
			// The handler owns the whole response, so the registry contributes
			// nothing but the registration.
			entry.Raw = true
			model.Routes = append(model.Routes, entry)
			continue
		}
		needsRequest = true

		fields, callArgs, callResults, err := pageBinding(route, analysis)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		entry.PageFields = fields
		if analysis.Page != nil && analysis.Page.Rung == RungTypedPage {
			entry.Call = true
			entry.CallArgs = callArgs
			entry.CallResults = callResults
		}

		for _, layout := range route.Layouts {
			signature, ok := layouts[layout.RelDir]
			if !ok {
				errs = append(errs, &Error{
					Path:    layout.File,
					Message: fmt.Sprintf("no signature supplied for the layout of %s", route.Path),
				})
				continue
			}
			if !hasSlot(signature) {
				errs = append(errs, &Error{
					Path: layout.File,
					Message: fmt.Sprintf("component %s must declare `%s: html` to wrap a page; "+
						"the template compiler emits a wrapper binder only for that shape",
						signature.Name, SlotParamName),
				})
				continue
			}
			args, argErrs := composerArgs(layout, signature)
			errs = append(errs, argErrs...)
			selector := ""
			if layout.RelDir != "" {
				selector = addImport(layout.ImportPath, layout.Package)
			}
			entry.Layouts = append(entry.Layouts, ComposerLayout{
				Selector:   selector,
				Binder:     "Bind" + signature.Name,
				ParamsType: signature.Name + "Params",
				Args:       args,
			})
		}
		model.Routes = append(model.Routes, entry)
	}

	// Endpoints are registered after the pages, because an action's package is
	// usually one the route loop already imported and reusing that alias keeps
	// the import block stable.
	errs = append(errs, checkActionCollisions(actions)...)
	for _, action := range actions {
		entry := RegistryAction{
			Pattern: action.Pattern(),
			Path:    action.Path,
			Hash:    action.Hash,
			Name:    action.Name,
			RelDir:  action.RelDir,
		}
		if action.RelDir != "" {
			entry.Selector = addImport(action.ImportPath, action.Package)
		}
		model.Actions = append(model.Actions, entry)
	}

	if len(errs) > 0 {
		return RegistryModel{}, joinErrors(errs)
	}

	if e.Symbols.ErrorImport != "" {
		tail = append(tail, Import{Path: e.Symbols.ErrorImport, Alias: aliasFor(e.Symbols.ErrorImport, e.Symbols.ErrorAlias)})
	}
	if e.Symbols.RuntimeImport != "" {
		tail = append(tail, Import{Path: e.Symbols.RuntimeImport, Alias: aliasFor(e.Symbols.RuntimeImport, e.Symbols.RuntimeAlias)})
	}
	model.Imports = groupImports(e.registryHead(needsRequest), tail)
	return model, nil
}

// registryHead lists the standard library packages the registry itself names:
// the router of every registration, and the request pair a generated handler
// body declares. The two are separate symbols, so one may move without the
// other, and the default points both at net/http and emits one line.
func (e *Emitter) registryHead(needsRequest bool) []Import {
	var head []Import
	add := func(path, alias string) {
		if path == "" {
			return
		}
		for _, existing := range head {
			if existing.Path == path {
				return
			}
		}
		head = append(head, Import{Path: path, Alias: aliasFor(path, alias)})
	}
	add(e.Symbols.MuxImport, e.Symbols.MuxAlias)
	if needsRequest {
		add(e.Symbols.HTTPImport, e.Symbols.HTTPAlias)
	}
	sort.Slice(head, func(i, j int) bool { return head[i].Path < head[j].Path })
	return head
}

// groupImports concatenates the two import groups, starting a new group at the
// first entry of the second so the standard library stays separated from the
// generated packages and the runtime.
func groupImports(head, tail []Import) []Import {
	out := append([]Import(nil), head...)
	for i, entry := range tail {
		entry.Group = i == 0 && len(head) > 0
		out = append(out, entry)
	}
	return out
}

// pageBinding works out how the page component's parameter struct is filled.
//
// At RungTemplateOnly every field comes from the decoded route. At
// RungTypedPage the function's results supply them, and the decoded route
// supplies the function's arguments instead.
func pageBinding(route Route, analysis Analysis) (fields []ComposerArg, callArgs, callResults string, err error) {
	component := analysis.Component
	if analysis.Page == nil || analysis.Page.Rung != RungTypedPage {
		for _, input := range component.Inputs {
			name := ExportedName(input.Name)
			fields = append(fields, ComposerArg{Field: name, From: "route." + name})
		}
		return fields, "", "", nil
	}

	if len(analysis.Page.Results) != len(component.Inputs) {
		return nil, "", "", &Error{
			Path: analysis.Page.File,
			Message: fmt.Sprintf("func %s returns %d value(s) before the error, but component %s declares %d parameter(s)",
				PageFuncName, len(analysis.Page.Results), component.Name, len(component.Inputs)),
		}
	}

	args := make([]string, len(analysis.Page.Params))
	for i, param := range analysis.Page.Params {
		args[i] = "route." + ExportedName(param.Name)
	}
	names := make([]string, 0, len(component.Inputs)+1)
	for _, input := range component.Inputs {
		names = append(names, "page"+ExportedName(input.Name))
	}
	names = append(names, "err")
	for i, input := range component.Inputs {
		fields = append(fields, ComposerArg{Field: ExportedName(input.Name), From: names[i]})
	}
	return fields, strings.Join(args, ", "), strings.Join(names, ", ") + " := ", nil
}
