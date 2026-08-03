package generator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shibukawa/tinybind-go/parser"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// ErrNothingToGenerate reports a package with no enabled artifacts.
var ErrNothingToGenerate = errors.New("generator: nothing to generate")

// GenerateRequest configures one package-local generation execution.
type GenerateRequest struct {
	Dir           string
	Out           string
	Name          string
	OpenAPI       bool
	OpenAPIName   string
	TemplatesName string
	// HTMLTemplatePattern and SQLTemplatePattern override template discovery
	// globs. Empty values retain the generator options.
	HTMLTemplatePattern string
	SQLTemplatePattern  string
	// SQLDialect overrides the generator option for this run. An empty value
	// retains it.
	SQLDialect     string
	ConfigBindName string
	// DynamoName is the DynamoDB item codec output file.
	DynamoName string
	// DynamoQueryName is the generated DynamoDB query output file.
	DynamoQueryName string
	// FirestoreName is the Firestore entity codec output file.
	FirestoreName string
	// FirestoreQueryName is the generated Firestore query output file.
	FirestoreQueryName string
	// PublicDir and PublicURLBase override where extracted static assets are
	// written and how they are referenced. Empty values retain the generator
	// options; setting one requires setting the other.
	PublicDir     string
	PublicURLBase string
	Check         bool
	GenerateAll   bool
	// Force regenerates even when the generated files record the current input
	// hash. Use it after a change the hash does not cover, such as an edit in
	// another package of the module.
	Force         bool
	SQLContextAPI bool
	// SQLContextOnlyAPI enables the context-only SQL API for this run. It can
	// turn the option on, never off.
	SQLContextOnlyAPI bool
}

// GenerateResult records generated artifacts or check diagnostics.
type GenerateResult struct {
	BinderPath         string
	ConfigBindPath     string
	DynamoPath         string
	DynamoQueryPath    string
	FirestorePath      string
	FirestoreQueryPath string
	OpenAPIPath        string
	TemplatesPath      string
	// AssetPaths holds the static files extracted from component style and
	// script blocks, then the files reference hook conversions produced, in
	// generation order.
	AssetPaths []string
	// Rewrites reports what the reference hooks did, including what they
	// declined and why. An author cannot see a build-time rewrite by reading
	// the template, so the build is the only place it is visible.
	//
	// It reports rather than interprets: whether a converted file is small
	// enough is the caller's judgment, and a caller measuring sizes owns its
	// own transform and can measure inside it.
	Rewrites []htmlbind.Rewrite
	// ReadSet holds every authored file the run depended on through a hook: the
	// sources each cache key named, plus whatever each transform reported
	// reading beyond them, sorted. A transform that under-reports produces a
	// stale output on the next run, which is the one correctness property this
	// package cannot verify for the caller.
	ReadSet []string
	// DynamicReferences are the attributes a hook was registered for whose
	// value is a template expression, and so could not be rewritten.
	DynamicReferences []htmlbind.DynamicReference
	// DepsPath is the recorded read set, written only when a transform reported
	// reading something. The next run verifies it before trusting its own skip,
	// because a file read by a transform is not otherwise a hashed input.
	DepsPath    string
	Diagnostics []parser.Diagnostic
	// Cached reports that the paths were left untouched because the generated
	// files already record the current input hash.
	Cached bool
}

// Paths returns non-empty artifact paths in generation order.
func (result GenerateResult) Paths() []string {
	paths := make([]string, 0, 6+len(result.AssetPaths))
	if result.TemplatesPath != "" {
		paths = append(paths, result.TemplatesPath)
	}
	// The extracted assets follow the template file that produced them.
	paths = append(paths, result.AssetPaths...)
	if result.DepsPath != "" {
		paths = append(paths, result.DepsPath)
	}
	for _, path := range []string{result.BinderPath, result.ConfigBindPath, result.DynamoPath, result.DynamoQueryPath, result.FirestorePath, result.FirestoreQueryPath, result.OpenAPIPath} {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// goPaths is Paths without the public assets. Only Go artifacts carry a
// generation stamp, because a stylesheet has no comment syntax in common with
// Go and its name already records the hash of its bytes.
func (result GenerateResult) goPaths() []string {
	paths := make([]string, 0, 6)
	for _, path := range []string{result.TemplatesPath, result.BinderPath, result.ConfigBindPath, result.DynamoPath, result.DynamoQueryPath, result.FirestorePath, result.FirestoreQueryPath, result.OpenAPIPath} {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// GeneratePackage executes every enabled generator phase without CLI or process ownership.
func (g *Generator) GeneratePackage(ctx context.Context, request GenerateRequest) (GenerateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	if request.Dir == "" {
		request.Dir = "."
	}
	if err := request.validate(); err != nil {
		return GenerateResult{}, err
	}
	if request.Name == "" {
		request.Name = "tinybind_gen.go"
	}
	if request.OpenAPIName == "" {
		request.OpenAPIName = "tinybind_openapi_gen.go"
	}
	if request.TemplatesName == "" {
		request.TemplatesName = DefaultTemplatesName
	}
	if request.ConfigBindName == "" {
		request.ConfigBindName = defaultConfigBindOut
	}
	if request.DynamoName == "" {
		request.DynamoName = defaultDynamoOut
	}
	if request.DynamoQueryName == "" {
		request.DynamoQueryName = defaultDynamoQueryOut
	}
	if request.FirestoreName == "" {
		request.FirestoreName = defaultFirestoreOut
	}
	if request.FirestoreQueryName == "" {
		request.FirestoreQueryName = defaultFirestoreQueryOut
	}

	options := request.applyTo(g.Options)
	normalized, err := options.normalized()
	if err != nil {
		return GenerateResult{}, err
	}
	if request.Check {
		diagnostics, err := parser.CheckPackageWithConfig(request.Dir, normalized.parserConfig)
		return GenerateResult{Diagnostics: diagnostics}, err
	}

	outDir := request.Out
	if outDir == "" {
		outDir = request.Dir
	}
	// A fingerprint that cannot be computed only disables the cache; generation
	// itself reports the underlying problem with better context.
	fingerprint, fingerprintErr := generationFingerprint(request.Dir, outDir, request, options)
	// The recorded read set is checked beside the fingerprint rather than
	// folded into it: it describes files a transform read during the previous
	// run, which the fingerprint cannot know before this one starts.
	if fingerprintErr == nil && !request.Force && depsUnchanged(outDir) {
		if cached, ok := cachedGeneration(outDir, fingerprint, request); ok {
			return cached, nil
		}
	}

	runner := New(options)
	result := GenerateResult{}
	templates, err := runner.generateTemplateFiles(request.Dir, request.Out, request.TemplatesName)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate templates: %w", err)
	}
	result.TemplatesPath, result.AssetPaths = templates.goPath, templates.assetPaths
	result.Rewrites, result.ReadSet = templates.rewrites, templates.readSet
	result.DynamicReferences = templates.dynamic
	if err := writeDeps(outDir, templates.readSet); err != nil {
		return GenerateResult{}, fmt.Errorf("record reference hook inputs: %w", err)
	}
	result.DepsPath = depsPath(outDir, templates.readSet)
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	// Generated templates join the package, so the remaining phases share one
	// type check taken after they are written.
	load := newPackageLoad(request.Dir)
	result.BinderPath, err = runner.generate(load, request.Out, request.Name)
	if err != nil {
		if !errors.Is(err, ErrNothingToGenerate) {
			return GenerateResult{}, fmt.Errorf("generate mapping: %w", err)
		}
		result.BinderPath = ""
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	result.ConfigBindPath, err = runner.generateConfigBind(load, request.Out, request.ConfigBindName)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate configbind: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	result.DynamoPath, err = runner.generateDynamoItems(load, request.Out, request.DynamoName)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate dynamobind: %w", err)
	}
	result.DynamoQueryPath, err = runner.generateDynamoQueries(load, request.Out, request.DynamoQueryName)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate dynamobind queries: %w", err)
	}
	result.FirestorePath, err = runner.generateFirestoreEntities(load, request.Out, request.FirestoreName)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate firestorebind: %w", err)
	}
	result.FirestoreQueryPath, err = runner.generateFirestoreQueries(load, request.Out, request.FirestoreQueryName)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate firestorebind queries: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	if request.OpenAPI && normalized.openAPI {
		result.OpenAPIPath, err = runner.generateOpenAPI(load, request.Out, request.OpenAPIName)
		if err != nil {
			if result.BinderPath == "" && result.ConfigBindPath != "" && strings.Contains(err.Error(), "no") {
				result.OpenAPIPath = ""
			} else if result.BinderPath != "" || result.TemplatesPath != "" {
				return GenerateResult{}, fmt.Errorf("generate OpenAPI: %w", err)
			} else if result.ConfigBindPath == "" {
				return GenerateResult{}, fmt.Errorf("generate OpenAPI: %w", err)
			}
		}
	}
	if len(result.Paths()) == 0 {
		return result, fmt.Errorf("%w in %s", ErrNothingToGenerate, request.Dir)
	}
	if fingerprintErr == nil {
		if err := stampGeneration(fingerprint, result); err != nil {
			return GenerateResult{}, fmt.Errorf("stamp generated files: %w", err)
		}
	}
	return result, nil
}
