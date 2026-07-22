package generator

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shibukawa/tinybind-go/parser"
)

// Main owns the tinybind-gen command-line interface for custom generator binaries.
func Main(options Options) {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr, options))
}

// Run executes the generator CLI and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("tinybind-gen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", ".", "package directory to analyze")
	out := flags.String("out", "", "output directory (default: same as -dir)")
	name := flags.String("name", "tinybind_gen.go", "binder/writer output file name")
	openapi := flags.Bool("openapi", true, "also generate OpenAPI embed (tinybind_openapi_gen.go)")
	openapiName := flags.String("openapi-name", "tinybind_openapi_gen.go", "OpenAPI output file name")
	templatesName := flags.String("templates-name", DefaultTemplatesName, "HTML/SQL template output file name")
	sqlContextAPI := flags.Bool("sql-context-api", false, "generate Context-resolved SQL template wrappers")
	check := flags.Bool("check", false, "report analysis diagnostics and exit 1 if any undiscoverable route candidates exist")
	generateAll := flags.Bool("generate-all", false, "generate every enabled mapping path for every struct")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	normalized := options.normalized()
	if *check {
		diags, err := parser.CheckPackageWithConfig(*dir, normalized.parserConfig)
		if err != nil {
			fmt.Fprintf(stderr, "tinybind-gen check: %v\n", err)
			return 1
		}
		for _, diagnostic := range diags {
			fmt.Fprintln(stderr, diagnostic.String())
		}
		if len(diags) > 0 {
			fmt.Fprintf(stderr, "tinybind-gen check: %d diagnostic(s)\n", len(diags))
			return 1
		}
		fmt.Fprintln(stdout, "ok")
		return 0
	}

	options.GenerateAll = options.GenerateAll || *generateAll
	options.SQLContextAPI = options.SQLContextAPI || *sqlContextAPI
	g := New(options)
	templatePath, err := g.GenerateTemplates(*dir, *out, *templatesName)
	if err != nil {
		fmt.Fprintf(stderr, "tinybind-gen templates: %v\n", err)
		return 1
	}
	if templatePath != "" {
		fmt.Fprintln(stdout, templatePath)
	}

	binderPath := ""
	path, err := g.Generate(*dir, *out, *name)
	if err != nil {
		if !strings.Contains(err.Error(), "no generatable structs") {
			fmt.Fprintf(stderr, "tinybind-gen: %v\n", err)
			return 1
		}
		// packages may only define configbind.Bind targets
	} else {
		binderPath = path
		fmt.Fprintln(stdout, path)
	}

	cfgPath, err := g.GenerateConfigBind(*dir, *out, defaultConfigBindOut)
	if err != nil {
		fmt.Fprintf(stderr, "tinybind-gen configbind: %v\n", err)
		return 1
	}
	if cfgPath != "" {
		fmt.Fprintln(stdout, cfgPath)
	}

	if *openapi && normalized.openAPI {
		path, err := g.GenerateOpenAPI(*dir, *out, *openapiName)
		if err != nil {
			// soft-skip when package has no OpenAPI-capable types
			if binderPath == "" && cfgPath != "" && strings.Contains(err.Error(), "no") {
				// configbind-only package: ignore openapi failure
			} else if binderPath != "" || templatePath != "" {
				fmt.Fprintf(stderr, "tinybind-gen openapi: %v\n", err)
				return 1
			} else if cfgPath == "" {
				fmt.Fprintf(stderr, "tinybind-gen openapi: %v\n", err)
				return 1
			}
		} else {
			fmt.Fprintln(stdout, path)
		}
	}

	if binderPath == "" && cfgPath == "" && templatePath == "" {
		fmt.Fprintf(stderr, "tinybind-gen: nothing to generate in %s\n", *dir)
		return 1
	}
	return 0
}
