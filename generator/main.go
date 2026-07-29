package generator

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

func runGenerate(ctx context.Context, args []string, streams CommandIO, options Options) int {
	stdout, stderr := streams.Stdout, streams.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", ".", "package directory to analyze")
	out := flags.String("out", "", "output directory (default: same as -dir)")
	name := flags.String("name", "tinybind_gen.go", "binder/writer output file name")
	openapi := flags.Bool("openapi", true, "also generate OpenAPI embed (tinybind_openapi_gen.go)")
	openapiName := flags.String("openapi-name", "tinybind_openapi_gen.go", "OpenAPI output file name")
	templatesName := flags.String("templates-name", DefaultTemplatesName, "HTML/SQL template output file name")
	htmlTemplatePattern := flags.String("html-template-pattern", templatePattern(options.HTMLTemplatePattern, DefaultHTMLTemplatePattern), "HTML template file glob")
	sqlTemplatePattern := flags.String("sql-template-pattern", templatePattern(options.SQLTemplatePattern, DefaultSQLTemplatePattern), "SQL template file glob")
	sqlDialect := flags.String("sql-dialect", options.SQLDialect, "target database for SQL templates: postgresql or mysql (required when SQL templates exist)")
	sqlContextAPI := flags.Bool("sql-context-api", false, "generate Context-resolved SQL template wrappers")
	sqlContextOnlyAPI := flags.Bool("sql-context-only-api", false, "publish only the Context-resolved SQL API under the declared name")
	check := flags.Bool("check", false, "report analysis diagnostics and exit 1 if any undiscoverable route candidates exist")
	generateAll := flags.Bool("generate-all", false, "generate every enabled mapping path for every struct")
	force := flags.Bool("force", false, "regenerate even when the generated files record the current input hash")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if streams.WorkingDirectory != "" {
		if !filepath.IsAbs(*dir) {
			*dir = filepath.Join(streams.WorkingDirectory, *dir)
		}
		if *out != "" && !filepath.IsAbs(*out) {
			*out = filepath.Join(streams.WorkingDirectory, *out)
		}
	}

	result, err := New(options).GeneratePackage(ctx, GenerateRequest{
		Dir: *dir, Out: *out, Name: *name,
		OpenAPI: *openapi, OpenAPIName: *openapiName,
		TemplatesName:       *templatesName,
		HTMLTemplatePattern: *htmlTemplatePattern,
		SQLTemplatePattern:  *sqlTemplatePattern,
		SQLDialect:          *sqlDialect,
		Check:               *check, GenerateAll: *generateAll, Force: *force, SQLContextAPI: *sqlContextAPI,
		SQLContextOnlyAPI: *sqlContextOnlyAPI,
	})
	if err != nil {
		fmt.Fprintf(stderr, "generate: %v\n", err)
		return 1
	}
	if *check {
		for _, diagnostic := range result.Diagnostics {
			fmt.Fprintln(stderr, diagnostic.String())
		}
		if len(result.Diagnostics) > 0 {
			fmt.Fprintf(stderr, "generate check: %d diagnostic(s)\n", len(result.Diagnostics))
			return 1
		}
		fmt.Fprintln(stdout, "ok")
		return 0
	}
	if result.Cached {
		fmt.Fprintf(stderr, "generate: %s is up to date\n", *dir)
	}
	for _, path := range result.Paths() {
		fmt.Fprintln(stdout, path)
	}
	return 0
}

func templatePattern(configured, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}
