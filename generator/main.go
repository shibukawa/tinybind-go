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
	dataAttributePrefix := flags.String("data-attribute-prefix", options.DataAttributePrefix, "data attribute namespace for HTML partial updates")
	publicDir := flags.String("public-dir", "", "directory receiving extracted component assets (default: "+DefaultPublicDir+"; requires -public-url-base)")
	publicURLBase := flags.String("public-url-base", "", "URL path or full URL prefixing extracted asset names (default: "+DefaultPublicURLBase+"; requires -public-dir)")
	sqlDialect := flags.String("sql-dialect", options.SQLDialect, "target database for SQL templates: postgresql, mysql, or sqlite (required when SQL templates exist)")
	sqlContextAPI := flags.Bool("sql-context-api", false, "generate Context-resolved SQL template wrappers")
	sqlContextOnlyAPI := flags.Bool("sql-context-only-api", false, "publish only the Context-resolved SQL API under the declared name")
	dynamoParameterAPI := flags.Bool("dynamo-parameter-api", false, "give generated DynamoDB queries a dynamobind.Handle parameter instead of resolving one from the Context")
	firestoreParameterAPI := flags.Bool("firestore-parameter-api", false, "give generated Firestore queries a firestorebind.Handle parameter instead of resolving one from the Context")
	backend := flags.String("backend", "", "derive a second transport from the net/http source: \"fasthttp\", or empty for none")
	transportName := flags.String("transport-name", defaultTransportOut, "derived transport output file name")
	transportReport := flags.Bool("transport-report", false, "list what the selected backend would refuse, write nothing, and exit 0")
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
		if *publicDir != "" && !filepath.IsAbs(*publicDir) {
			*publicDir = filepath.Join(streams.WorkingDirectory, *publicDir)
		}
	}

	options.DataAttributePrefix = *dataAttributePrefix

	switch *backend {
	case "":
		if *transportReport {
			fmt.Fprintln(stderr, "generate: -transport-report needs -backend")
			return 2
		}
	case "fasthttp":
		transform := DefaultTransformOptions()
		transform.ReportOnly = *transportReport
		options.Transform = &transform
	default:
		fmt.Fprintf(stderr, "generate: unknown -backend %q; the only derived backend is \"fasthttp\"\n", *backend)
		return 2
	}

	result, err := New(options).GeneratePackage(ctx, GenerateRequest{
		Dir: *dir, Out: *out, Name: *name,
		OpenAPI: *openapi, OpenAPIName: *openapiName,
		TemplatesName:       *templatesName,
		HTMLTemplatePattern: *htmlTemplatePattern,
		SQLTemplatePattern:  *sqlTemplatePattern,
		PublicDir:           *publicDir,
		PublicURLBase:       *publicURLBase,
		SQLDialect:          *sqlDialect,
		Check:               *check, GenerateAll: *generateAll, Force: *force, SQLContextAPI: *sqlContextAPI,
		SQLContextOnlyAPI:  *sqlContextOnlyAPI,
		DynamoParameterAPI: *dynamoParameterAPI, FirestoreParameterAPI: *firestoreParameterAPI,
		TransportName: *transportName,
	})
	if err != nil {
		fmt.Fprintf(stderr, "generate: %v\n", err)
		return 1
	}
	if *transportReport {
		for _, diagnostic := range result.Diagnostics {
			fmt.Fprintln(stderr, diagnostic.String())
		}
		fmt.Fprintf(stdout, "%d handler(s) would be refused by the %s backend\n", len(result.Diagnostics), *backend)
		return 0
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
