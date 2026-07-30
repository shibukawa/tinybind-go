package routetree

import (
	"os"
	"path/filepath"
	"strings"

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

// Generated is one emitted file and where it belongs.
type Generated struct {
	// Path is the absolute destination.
	Path string
	// Source is the formatted Go source.
	Source []byte
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
}

// Generate discovers the tree and emits every file it needs: the compiled
// components for each template, a typed decoder per route, and one registry in
// the route root carrying the integrated ServeMux.
//
// Nothing is written to disk; the caller owns that, which is what lets a
// framework post-process or redirect the output.
func Generate(options GenerateOptions) ([]Generated, error) {
	emitter := options.Emitter
	if emitter == nil {
		emitter = NewEmitter()
	}
	componentSuffix := orDefault(options.ComponentSuffix, ComponentSuffix)
	decoderOut := orDefault(options.DecoderOutput, DefaultDecoderOutput)
	registryOut := orDefault(options.RegistryOutput, DefaultRegistryOutput)

	tree, err := Discover(options.Config)
	if err != nil {
		return nil, err
	}
	rootPackage := options.RootPackage
	if rootPackage == "" {
		rootPackage = PackageName(filepath.Base(tree.Root))
	}

	if err := ValidateActionPrefix(emitter.ActionPrefix, tree); err != nil {
		return nil, err
	}

	var out []Generated
	var errs []error

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
		found, err := DiscoverActions(dir, relDir, pkg, importPath, emitter.ActionPrefix)
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
			source, err := compileTemplate(layout.File, layout.Package, emitter, actionsByDir[layout.RelDir], options.ActionResolver)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			out = append(out, Generated{Path: componentPath(layout.File, componentSuffix), Source: source})
		}
		discoverActions(route.Dir, route.RelDir, route.Package, route.ImportPath)

		analysis, err := Analyze(route)
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
			source, err := compileTemplate(route.PageFile, route.Package, emitter, actionsByDir[route.RelDir], options.ActionResolver)
			if err != nil {
				errs = append(errs, err)
			} else {
				out = append(out, Generated{Path: pagePath, Source: source})
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
		return nil, joinErrors(errs)
	}

	registry, err := emitter.Registry(tree, rootPackage, analyses, layoutSignatures, allActions)
	if err != nil {
		return nil, err
	}
	out = append(out, Generated{Path: filepath.Join(tree.Root, registryOut), Source: registry})
	return out, nil
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

func compileTemplate(path, pkg string, emitter *Emitter, actions []Action, resolver func(string) (string, bool)) ([]byte, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return htmlbind.Generate(path, source, htmlbind.GenerateOptions{
		Package:              pkg,
		ServerActions:        actionURLs(actions),
		ServerActionResolver: resolver,
		ServerActionAttr:     emitter.ActionAttr,
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
