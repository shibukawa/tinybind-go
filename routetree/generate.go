package routetree

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/shibukawa/tinybind-go/internal/contextscan"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// Default base names of the files [Generate] produces.
const (
	DefaultDecoderOutput  = "route_gen.go"
	DefaultRegistryOutput = "routes_gen.go"
	// ComponentSuffix is appended to a template's base name to form its
	// generated file, so page.tb.html and layout.tb.html in one directory do not
	// claim the same output.
	ComponentSuffix = "_gen.go"
)

// Generated is one emitted Go file and where it belongs.
type Generated struct {
	// Path is the absolute destination.
	Path string
	// Source is the formatted Go source.
	Source []byte
	// Registry marks the one file registering every route and endpoint.
	//
	// It is flagged because it is the one file whose write has an order. It
	// names the entry point of every typed server action, and those are emitted
	// by the binding phase, which runs after this one because it type-checks
	// each route package and a package does not type-check until the compiled
	// component is in it. Writing the registry first would leave the root
	// package naming a symbol nothing had written yet.
	//
	// Writing it last costs nothing: the binding phase is required to skip it
	// anyway, per rule:generated-source-not-discovered, so writing it earlier
	// only produces a file that phase must ignore.
	Registry bool
}

// SplitRegistry separates the files a caller writes before the binding phase
// from the registry it writes after, per [Generated.Registry].
func SplitRegistry(files []Generated) (before []Generated, registry []Generated) {
	for _, file := range files {
		if file.Registry {
			registry = append(registry, file)
			continue
		}
		before = append(before, file)
	}
	return before, registry
}

// Result is everything one whole-tree generation run produced.
//
// Go sources and public assets are two lists rather than one tagged list
// because only one of them has a destination this package can compute. A
// generated component belongs beside the template that produced it; an
// extracted asset belongs wherever the caller serves PublicURLBase from, which
// is the caller's layout and not the tree's.
type Result struct {
	// Files are the emitted Go sources: a compiled component per template, a
	// typed decoder per route, and one registry in the route root.
	Files []Generated
	// Assets are the stylesheets and component scripts the tree's templates
	// extracted, deduplicated across the tree. A file name carries the hash of
	// its own bytes, so two templates producing one name produced identical
	// content and the duplicate is dropped rather than returned twice.
	//
	// Nothing writes them. Each carries Base, Extension, Content, and the URL it
	// was compiled against, which is what a caller needs to put the file where
	// that URL resolves. A page declaring a script block whose asset is dropped
	// leaves a reference to a file that answers 404.
	Assets []htmlbind.Asset
	// Actions is every server function the tree discovered, raw and typed
	// alike, in the order the registry registers them.
	//
	// A caller needs the typed ones: the entry point each is registered under
	// is emitted by the binding phase, which runs after this, so the signature
	// read here has to reach that phase. The raw ones are reported beside them
	// because one table is what every other consumer already reads.
	Actions []Action
}

// GenerateOptions configures one whole-tree generation run.
type GenerateOptions struct {
	// Config selects the route root and reserved names.
	Config Config
	// RootPackage is the Go package clause of the route root directory, which is
	// where the registry is emitted. Empty derives it from the directory name.
	RootPackage string
	// Emitter renders the Go source. Nil uses [NewEmitter].
	Emitter *Emitter
	// ComponentSuffix, DecoderOutput, and RegistryOutput override the generated
	// file names.
	ComponentSuffix string
	DecoderOutput   string
	RegistryOutput  string
	// ActionResolver supplies the endpoint URL of a server action this tree does
	// not declare, so a framework can address a handler from its own route table.
	// A handler exported by the template's own route package always wins, which is
	// what keeps a resolver from silently retargeting a discovered action.
	ActionResolver func(name string) (url string, ok bool)
	// ScriptResolver reads the component script blocks of one template and
	// answers what each block exposes and which of the component's parameters to
	// emit onto its root element.
	//
	// It is how a framework that parses JavaScript supplies what this module
	// refuses to read. Configuring one costs a second parse of every template
	// carrying a block, because the blocks have to be reported before the compile
	// that consumes the answers can run; a tree with no resolver parses once, as
	// it always has.
	ScriptResolver func(path string, scripts []htmlbind.ComponentScript) (ScriptAnswers, error)
	// DataAttributePrefix is the boundary attribute prefix compiled into every
	// template in the tree. Empty uses the htmlbind default.
	//
	// A project configuring its runtime with a prefix of its own sets the same
	// value here. Without it a page tree takes the default while a registered
	// template takes the configured one, and two halves of one project disagree
	// about an attribute name they both have to read.
	DataAttributePrefix string
	// PublicURLBase is the URL prefix an extracted asset's reference is computed
	// against. Empty uses the htmlbind default.
	//
	// It has to name where the caller actually serves Result.Assets from, since
	// the URL it produces is written into the generated component and nothing
	// downstream can correct it.
	PublicURLBase string
}

// Generate discovers the tree and emits its Go files, discarding the public
// assets its templates extracted. Callers that write files use [GenerateTree]
// instead.
//
// Discarding them is safe only for a tree whose templates declare no style or
// script block. One that does gets a generated component referencing an asset
// URL, and no bytes for anything to serve there.
func Generate(options GenerateOptions) ([]Generated, error) {
	result, err := GenerateTree(options)
	return result.Files, err
}

// GenerateTree discovers the tree and emits everything it needs: the compiled
// components for each template, a typed decoder per route, one registry in the
// route root carrying the integrated ServeMux, and the public files the
// templates extracted.
//
// Nothing is written to disk; the caller owns that, which is what lets a
// framework post-process or redirect the output.
func GenerateTree(options GenerateOptions) (Result, error) {
	emitter := options.Emitter
	if emitter == nil {
		emitter = NewEmitter()
	}
	componentSuffix := orDefault(options.ComponentSuffix, ComponentSuffix)
	decoderOut := orDefault(options.DecoderOutput, DefaultDecoderOutput)
	registryOut := orDefault(options.RegistryOutput, DefaultRegistryOutput)

	tree, err := Discover(options.Config)
	if err != nil {
		return Result{}, err
	}
	rootPackage := options.RootPackage
	if rootPackage == "" {
		rootPackage = PackageName(filepath.Base(tree.Root))
	}

	if err := ValidateActionPrefix(emitter.ActionPrefix, tree); err != nil {
		return Result{}, err
	}

	var out []Generated
	var assets []htmlbind.Asset
	var errs []error

	// An asset's file name carries the hash of its own bytes, so one name is one
	// content and the second template to extract it adds nothing. A layout
	// shared by twenty routes is compiled once, but a style block two pages
	// happen to write identically still arrives twice.
	seenAsset := map[string]bool{}
	// nativeForm records which handlers a template named from a form, keyed by
	// the declaring directory and the handler name. Only those need a POST on the
	// page's own pattern: an action on a bare button has no native submit to
	// serve, and registering one would claim a pattern an application is
	// documented to be free to register itself.
	nativeForm := map[string]bool{}
	collect := func(relDir string, compiled htmlbind.Result) {
		for _, asset := range compiled.Assets {
			if seenAsset[asset.FileName()] {
				continue
			}
			seenAsset[asset.FileName()] = true
			assets = append(assets, asset)
		}
		for _, ref := range compiled.ActionRefs {
			if ref.Element == "form" {
				nativeForm[relDir+"\x00"+ref.Handler] = true
			}
		}
	}

	// Layout templates are compiled once per directory, and their signatures
	// are keyed by RelDir so every route sharing a layout reads the same one.
	layoutSignatures := map[string]ComponentSignature{}
	seenLayout := map[string]bool{}

	// Server functions are resolved before any template is compiled, because a
	// template names a handler and the compiler needs the URL that name lowers
	// to. Discovery is keyed by the declaring directory, so a package holding
	// both a page and a layout is read once.
	actionsByDir := map[string][]Action{}
	var allActions []Action
	discoverActions := func(dir, relDir, pkg, importPath string) {
		if _, done := actionsByDir[relDir]; done {
			return
		}
		found, err := DiscoverActionsWith(dir, relDir, pkg, importPath, emitter.ActionPrefix, emitter.handlerShape())
		if err != nil {
			errs = append(errs, err)
			actionsByDir[relDir] = nil
			return
		}
		actionsByDir[relDir] = found
		allActions = append(allActions, found...)
	}

	analyses := make([]Analysis, 0, len(tree.Routes))
	for _, route := range tree.Routes {
		for _, layout := range route.Layouts {
			if seenLayout[layout.RelDir] {
				continue
			}
			seenLayout[layout.RelDir] = true
			signature, err := LayoutComponent(layout.File)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			layoutSignatures[layout.RelDir] = signature
			discoverActions(filepath.Dir(layout.File), layout.RelDir, layout.Package, layout.ImportPath)
			compiled, err := compileTemplate(layout.File, layout.Package, emitter, actionsByDir[layout.RelDir], options)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			collect(layout.RelDir, compiled)
			out = append(out, Generated{Path: componentPath(layout.File, componentSuffix), Source: compiled.GoSource})
		}
		discoverActions(route.Dir, route.RelDir, route.Package, route.ImportPath)

		analysis, err := AnalyzeWith(route, emitter.handlerShape())
		if err != nil {
			errs = append(errs, err)
			// A failed analysis still contributes a placeholder, so route and
			// analysis stay index-aligned for the registry below.
			analyses = append(analyses, analysis)
			continue
		}
		analyses = append(analyses, analysis)

		pagePath := componentPath(route.PageFile, componentSuffix)
		if !alreadyEmitted(out, pagePath) {
			compiled, err := compileTemplate(route.PageFile, route.Package, emitter, actionsByDir[route.RelDir], options)
			if err != nil {
				errs = append(errs, err)
			} else {
				collect(route.RelDir, compiled)
				out = append(out, Generated{Path: pagePath, Source: compiled.GoSource})
			}
		}

		decoder, err := emitter.Decoder(route, analysis.Inputs)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, Generated{Path: filepath.Join(route.Dir, decoderOut), Source: decoder})
	}

	if len(errs) > 0 {
		return Result{}, joinErrors(errs)
	}

	// The page POST route is decided by the markup rather than by the Go sources,
	// so the flag is applied once every template has been compiled.
	for i := range allActions {
		allActions[i].NativeForm = nativeForm[allActions[i].RelDir+"\x00"+allActions[i].Name]
	}

	registry, err := emitter.Registry(tree, rootPackage, analyses, layoutSignatures, allActions)
	if err != nil {
		return Result{}, err
	}
	out = append(out, Generated{Path: filepath.Join(tree.Root, registryOut), Source: registry, Registry: true})
	return Result{Files: out, Assets: assets, Actions: allActions}, nil
}

// Write writes generated files to disk, creating directories as needed.
func Write(files []Generated) error {
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file.Path, file.Source, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// componentPath names a template's generated file after the template itself,
// so every template in one directory keeps its own output.
func componentPath(templatePath, suffix string) string {
	base := filepath.Base(templatePath)
	if cut := strings.IndexByte(base, '.'); cut > 0 {
		base = base[:cut]
	}
	return filepath.Join(filepath.Dir(templatePath), base+suffix)
}

// ScriptAnswers is what a [GenerateOptions.ScriptResolver] returns for one
// template: what each component's script block exposes, and which of each
// component's parameters to emit onto its root element.
//
// Both maps are keyed by component declaration name, which is unique within one
// template module. A component absent from Handlers is unchecked, and one absent
// from Parameters emits nothing.
type ScriptAnswers struct {
	Handlers   map[string]htmlbind.ClientHandlerSet
	Parameters map[string][]string
}

func compileTemplate(path, pkg string, emitter *Emitter, actions []Action, options GenerateOptions) (htmlbind.Result, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return htmlbind.Result{}, err
	}
	// A route directory is its own Go package, so an external's implementation
	// sits beside the template that calls it. Scanning that directory is what
	// lets a function declaring a leading context.Context receive one here, the
	// way it already does in a templates package. Without it the same
	// declaration generates a bare call and the route package does not build.
	withContext, err := contextscan.Externals(filepath.Dir(path))
	if err != nil {
		return htmlbind.Result{}, err
	}
	// GenerateModule rather than Generate: the tree's own templates declare
	// style and script blocks, and Generate is the variant that drops what they
	// extract, which leaves the component referencing a file nobody wrote.
	var answers ScriptAnswers
	if options.ScriptResolver != nil {
		// The blocks are reported before the compile that consumes the answers,
		// which is why this parses once more. Only a tree configuring a resolver
		// pays it.
		scripts, err := htmlbind.ComponentScripts(path, source)
		if err != nil {
			return htmlbind.Result{}, err
		}
		if len(scripts) > 0 {
			answers, err = options.ScriptResolver(path, scripts)
			if err != nil {
				return htmlbind.Result{}, err
			}
		}
	}
	return htmlbind.GenerateModule(path, source, htmlbind.GenerateOptions{
		Package:                   pkg,
		ClientHandlers:            answers.Handlers,
		ClientHandlerAttr:         emitter.ClientHandlerAttr,
		ComponentParameters:       answers.Parameters,
		ComponentParameterAttr:    emitter.ComponentParameterAttr,
		ServerActions:             actionURLs(actions),
		ServerActionRefusals:      actionRefusals(actions),
		ServerActionSelectors:     actionSelectors(actions),
		ServerActionSelectorField: emitter.ActionSelectorField,
		ServerActionResolver:      options.ActionResolver,
		ServerActionAttr:          emitter.ActionAttr,
		ContextExternals:          withContext,
		DataAttributePrefix:       options.DataAttributePrefix,
		PublicURLBase:             options.PublicURLBase,
	})
}

func alreadyEmitted(files []Generated, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
