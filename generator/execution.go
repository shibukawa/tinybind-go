package generator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shibukawa/tinybind-go/parser"
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
	DynamoName  string
	Check       bool
	GenerateAll bool
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
	BinderPath     string
	ConfigBindPath string
	DynamoPath     string
	OpenAPIPath    string
	TemplatesPath  string
	Diagnostics    []parser.Diagnostic
	// Cached reports that the paths were left untouched because the generated
	// files already record the current input hash.
	Cached bool
}

// Paths returns non-empty artifact paths in generation order.
func (result GenerateResult) Paths() []string {
	paths := make([]string, 0, 5)
	for _, path := range []string{result.TemplatesPath, result.BinderPath, result.ConfigBindPath, result.DynamoPath, result.OpenAPIPath} {
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
	if fingerprintErr == nil && !request.Force {
		if cached, ok := cachedGeneration(outDir, fingerprint, request); ok {
			return cached, nil
		}
	}

	runner := New(options)
	result := GenerateResult{}
	if result.TemplatesPath, err = runner.GenerateTemplates(request.Dir, request.Out, request.TemplatesName); err != nil {
		return GenerateResult{}, fmt.Errorf("generate templates: %w", err)
	}
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
