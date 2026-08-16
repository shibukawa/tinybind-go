package routetree

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"

	templatehtml "github.com/shibukawa/tinybind-go/templates/htmlbind"
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
	// Symbol is what the registration names: the handler itself for a raw
	// action, the generated entry point for a typed one.
	Symbol string
	// Published is the identifier client script calls the action through, and
	// Typed reports which admission rule let it in.
	Published string
	Typed     bool
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
	// PageFields fills the page component parameter struct.
	PageFields []ComposerArg
	// Layouts are the ancestor wrappers, outermost first.
	Layouts []ComposerLayout
	// PostPattern is the pattern the page's own POST route registers under,
	// empty where the route reaches no server function. A form carrying
	// server-action posts here rather than to the handler's own address, because
	// a form declaring no action submits to the document URL and that is what
	// carries the path parameters a handler serving /users/{id} needs.
	PostPattern string
	// FormActions are the server functions a native submit on this page can
	// reach: the ones declared in the route's own package and in every layout
	// wrapping it. One POST registration serves them all and the generated
	// dispatcher branches on the selector.
	FormActions []RegistryFormAction
}

// RegistryFormAction is one server function a page's POST route dispatches to.
type RegistryFormAction struct {
	// Selector is the opaque value the form's hidden field carries.
	Selector string
	// Package qualifies the handler symbol, such as "id_." It is empty for a
	// handler in the root package itself.
	Package string
	// Name is the exported Go function name.
	Name string
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
	// ActionSelectorField is the hidden field the generated dispatcher reads the
	// selector out of. The template compiler wrote it into the form.
	ActionSelectorField string
}

// Render describes the render call of one route's generated handler, which the
// registry template hands to the [TemplateRender] block. The request is always
// in scope here, so an override reaches it without any setting.
func (m RegistryModel) Render(route RegistryRoute) RenderCall {
	call := RenderCall{
		Writer:  m.Symbols.Writer,
		Request: m.Symbols.Request,
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
		Header:         e.header(),
		Package:        rootPackage,
		RegisterFunc:   e.RegisterFunc,
		MuxFunc:        e.MuxFunc,
		TableVar:       e.TableVar,
		ActionTableVar: orDefault(e.ActionTableVar, ActionTableVar),
		DecodeFunc:     e.DecodeFunc,
		Symbols:        e.symbols(),

		ActionSelectorField: orDefault(e.ActionSelectorField, DefaultActionSelectorField),
	}
	// Server functions are grouped by declaring directory, so a route can collect
	// the ones its own package and its layouts declare without rescanning.
	actionsByDir := map[string][]Action{}
	for _, action := range actions {
		actionsByDir[action.RelDir] = append(actionsByDir[action.RelDir], action)
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
			Pattern: model.Symbols.RoutePattern(route),
			Path:    route.Path,
			RelDir:  route.RelDir,
		}
		for _, segment := range route.Params {
			entry.ParamNames = append(entry.ParamNames, segment.Name)
		}
		if route.RelDir != "" {
			entry.Selector = addImport(route.ImportPath, route.Package)
		}

		// A native form submit posts to the page rather than to the handler's own
		// address, so the page pattern gains a POST beside its GET. Reachable
		// means declared in the route's own package or in a layout wrapping it,
		// which is exactly the set a template rendered on this page can name.
		//
		// This runs before the raw-handler branch below, because a handler owning
		// its whole response still renders a template that can carry a form.
		seen := map[string]bool{}
		collectActions := func(relDir string) {
			for _, action := range actionsByDir[relDir] {
				// A handler no form names has no native submit to serve, and
				// registering for it would claim a pattern an application is
				// documented to be able to register itself.
				if !action.NativeForm || seen[action.Selector()] {
					continue
				}
				seen[action.Selector()] = true
				qualifier := ""
				if action.RelDir != "" {
					qualifier = addImport(action.ImportPath, action.Package)
				}
				entry.FormActions = append(entry.FormActions, RegistryFormAction{
					Selector: action.Selector(),
					Package:  qualifier,
					Name:     action.Name,
				})
			}
		}
		collectActions(route.RelDir)
		for _, layout := range route.Layouts {
			collectActions(layout.RelDir)
		}
		if len(entry.FormActions) > 0 {
			entry.PostPattern = model.Symbols.RoutePostPattern(route)
			// The dispatcher body names both transport values, so a tree of
			// nothing but raw handlers still imports the request package once one
			// of its pages can be posted to.
			needsRequest = true
		}

		if analysis.Page != nil && analysis.Page.Rung == RungHandlerPage {
			// The handler owns the whole response, so the registry contributes
			// nothing but the registration.
			entry.Raw = true
			model.Routes = append(model.Routes, entry)
			continue
		}
		needsRequest = true

		entry.PageFields = pageBinding(analysis)

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
			Pattern:   action.Pattern(),
			Path:      action.Path,
			Hash:      action.Hash,
			Name:      action.Name,
			RelDir:    action.RelDir,
			Symbol:    orDefault(action.Wrapper, action.Name),
			Published: orDefault(action.Published, PublishedName(action.Name)),
			Typed:     action.Typed,
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

// pageBinding fills the page component's parameter struct from the decoded
// route. Every field comes from there: a page declares its inputs on the
// component and loads what it needs with a {val} binding, so nothing is
// threaded in from a Go entry point.
func pageBinding(analysis Analysis) []ComposerArg {
	var fields []ComposerArg
	for _, input := range analysis.Component.Inputs {
		// Two structs, two spellings. The decoded route is this package's, so
		// its field is ExportedName and reads id as ID; the component's
		// parameter struct is the template compiler's, which uppercases the
		// first rune and reads it as Id. Using one name for both compiles only
		// while no input is an initialism.
		fields = append(fields, ComposerArg{
			Field: templatehtml.FieldName(input.Name),
			From:  "route." + ExportedName(input.Name),
		})
	}
	return fields
}
