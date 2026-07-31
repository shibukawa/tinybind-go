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
	ConfigBindName      string
	// PublicDir and PublicURLBase override where extracted static assets are
	// written and how they are referenced. Empty values retain the generator
	// options; setting one requires setting the other.
	PublicDir     string
	PublicURLBase string
	Check         bool
	GenerateAll   bool
	SQLContextAPI bool
	// SQLContextOnlyAPI enables the context-only SQL API for this run. It can
	// turn the option on, never off.
	SQLContextOnlyAPI bool
}

// GenerateResult records generated artifacts or check diagnostics.
type GenerateResult struct {
	BinderPath     string
	ConfigBindPath string
	OpenAPIPath    string
	TemplatesPath  string
	// AssetPaths holds the static files extracted from component style and
	// script blocks, in generation order.
	AssetPaths  []string
	Diagnostics []parser.Diagnostic
}

// Paths returns non-empty artifact paths in generation order.
func (result GenerateResult) Paths() []string {
	paths := make([]string, 0, 4+len(result.AssetPaths))
	if result.TemplatesPath != "" {
		paths = append(paths, result.TemplatesPath)
	}
	paths = append(paths, result.AssetPaths...)
	for _, path := range []string{result.BinderPath, result.ConfigBindPath, result.OpenAPIPath} {
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

	options := request.applyTo(g.Options)
	normalized, err := options.normalized()
	if err != nil {
		return GenerateResult{}, err
	}
	if request.Check {
		diagnostics, err := parser.CheckPackageWithConfig(request.Dir, normalized.parserConfig)
		return GenerateResult{Diagnostics: diagnostics}, err
	}

	runner := New(options)
	result := GenerateResult{}
	if result.TemplatesPath, result.AssetPaths, err = runner.generateTemplateFiles(request.Dir, request.Out, request.TemplatesName); err != nil {
		return GenerateResult{}, fmt.Errorf("generate templates: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	result.BinderPath, err = runner.Generate(request.Dir, request.Out, request.Name)
	if err != nil {
		if !strings.Contains(err.Error(), "no generatable structs") {
			return GenerateResult{}, fmt.Errorf("generate mapping: %w", err)
		}
		result.BinderPath = ""
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	result.ConfigBindPath, err = runner.GenerateConfigBind(request.Dir, request.Out, request.ConfigBindName)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate configbind: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return GenerateResult{}, err
	}
	if request.OpenAPI && normalized.openAPI {
		result.OpenAPIPath, err = runner.GenerateOpenAPI(request.Dir, request.Out, request.OpenAPIName)
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
	return result, nil
}
