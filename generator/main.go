package generator

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/shibukawa/httpbind-go/parser"
)

// Main owns the httpbinder-gen command-line interface for custom generator binaries.
func Main(options Options) {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr, options))
}

// Run executes the generator CLI and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("httpbinder-gen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", ".", "package directory to analyze")
	out := flags.String("out", "", "output directory (default: same as -dir)")
	name := flags.String("name", "httpbinder_gen.go", "binder/writer output file name")
	openapi := flags.Bool("openapi", true, "also generate OpenAPI embed (httpbinder_openapi_gen.go)")
	openapiName := flags.String("openapi-name", "httpbinder_openapi_gen.go", "OpenAPI output file name")
	check := flags.Bool("check", false, "report analysis diagnostics and exit 1 if any undiscoverable route candidates exist")
	generateAll := flags.Bool("generate-all", false, "generate every enabled mapping path for every struct")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	normalized := options.normalized()
	if *check {
		diags, err := parser.CheckPackageWithConfig(*dir, normalized.parserConfig)
		if err != nil {
			fmt.Fprintf(stderr, "httpbinder-gen check: %v\n", err)
			return 1
		}
		for _, diagnostic := range diags {
			fmt.Fprintln(stderr, diagnostic.String())
		}
		if len(diags) > 0 {
			fmt.Fprintf(stderr, "httpbinder-gen check: %d diagnostic(s)\n", len(diags))
			return 1
		}
		fmt.Fprintln(stdout, "ok")
		return 0
	}

	options.GenerateAll = options.GenerateAll || *generateAll
	g := New(options)
	path, err := g.Generate(*dir, *out, *name)
	if err != nil {
		fmt.Fprintf(stderr, "httpbinder-gen: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	if *openapi && normalized.openAPI {
		path, err := g.GenerateOpenAPI(*dir, *out, *openapiName)
		if err != nil {
			fmt.Fprintf(stderr, "httpbinder-gen openapi: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, path)
	}
	return 0
}
