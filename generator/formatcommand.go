package generator

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

// FormatCommand creates the tinybind fmt subcommand, per
// api:template-format-command. Everything it does is available as a library
// through templatefmt; this is the process boundary around it.
func FormatCommand(options Options) Command {
	return Command{
		Name:    "fmt",
		Summary: "format tinybind template sources",
		Run: func(ctx context.Context, args []string, streams CommandIO) int {
			return runFormat(ctx, args, streams, options)
		},
	}
}

func runFormat(_ context.Context, args []string, streams CommandIO, options Options) int {
	flags := flag.NewFlagSet("fmt", flag.ContinueOnError)
	flags.SetOutput(streams.Stderr)
	dir := flags.String("dir", "", "package directory to format; defaults to the working directory")
	write := flags.Bool("w", false, "write the result back to each source instead of to stdout")
	list := flags.Bool("l", false, "list the paths whose formatting differs and write nothing")
	width := flags.Int("width", 0, "soft line width; 0 uses the default")
	preserve := flags.Bool("preserve-whitespace", options.PreserveTemplateWhitespace,
		"static whitespace is not collapsed at generation time, so HTML layout stays inside the positions the parser discards")
	htmlPattern := flags.String("html-template-pattern", options.HTMLTemplatePattern, "base-name glob for HTML templates")
	sqlPattern := flags.String("sql-template-pattern", options.SQLTemplatePattern, "base-name glob for SQL templates")
	dynamoPattern := flags.String("dynamo-template-pattern", options.DynamoTemplatePattern, "base-name glob for DynamoDB declarations")
	firestorePattern := flags.String("firestore-template-pattern", options.FirestoreTemplatePattern, "base-name glob for Firestore declarations")
	as := flags.String("as", "", "language of a source read from stdin: html, sql, dynamo, or firestore")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	formatOptions := templatefmt.Options{
		Width:              *width,
		PreserveWhitespace: *preserve,
		HTMLPattern:        *htmlPattern,
		SQLPattern:         *sqlPattern,
		DynamoPattern:      *dynamoPattern,
		FirestorePattern:   *firestorePattern,
	}

	if *as != "" {
		return formatStdin(templatefmt.Format(*as), formatOptions, streams)
	}
	target := *dir
	if target == "" {
		target = streams.WorkingDirectory
	}
	if target == "" {
		target = "."
	}
	return formatDir(target, flags.Args(), formatOptions, *write, *list, streams)
}

// formatStdin filters one source. An editor integration needs exactly this, and
// a stream has no file name to match a pattern against, which is why the
// language is named explicitly.
func formatStdin(format templatefmt.Format, options templatefmt.Options, streams CommandIO) int {
	if streams.Stdin == nil {
		fmt.Fprintln(streams.Stderr, "fmt: -as was given but stdin is not readable")
		return 2
	}
	source, err := io.ReadAll(streams.Stdin)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "fmt: %v\n", err)
		return 1
	}
	formatted, err := templatefmt.SourceAs(format, "<stdin>", source, options)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "fmt: %v\n", err)
		return 1
	}
	_, _ = streams.Stdout.Write(formatted)
	return 0
}

func formatDir(dir string, extra []string, options templatefmt.Options, write, list bool, streams CommandIO) int {
	results, err := collectFormatTargets(dir, extra, options)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "fmt: %v\n", err)
		return 2
	}
	status := 0
	for _, result := range results {
		if result.Err != nil {
			// A source that does not parse is left exactly as it is; the rest of
			// the run continues, because one broken file is not a reason to stop
			// formatting the others.
			fmt.Fprintf(streams.Stderr, "%v\n", result.Err)
			status = 1
			continue
		}
		switch {
		case list:
			if result.Changed {
				fmt.Fprintln(streams.Stdout, result.Path)
				status = 1
			}
		case write:
			if err := result.Write(); err != nil {
				fmt.Fprintf(streams.Stderr, "fmt: %v\n", err)
				status = 1
				continue
			}
			if result.Changed {
				fmt.Fprintln(streams.Stdout, result.Path)
			}
		default:
			_, _ = streams.Stdout.Write(result.Formatted)
		}
	}
	return status
}

// collectFormatTargets resolves the run's inputs: named files when there are
// any, and otherwise every template directly inside the directory.
func collectFormatTargets(dir string, named []string, options templatefmt.Options) ([]templatefmt.Result, error) {
	if len(named) == 0 {
		return templatefmt.Dir(dir, options)
	}
	results := make([]templatefmt.Result, 0, len(named))
	for _, name := range named {
		path := name
		if !filepath.IsAbs(path) && dir != "" {
			path = filepath.Join(dir, name)
		}
		format, err := templatefmt.Identify(path, options)
		if err != nil {
			if errors.Is(err, templatefmt.ErrUnknownFormat) {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return nil, err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			results = append(results, templatefmt.Result{Path: path, Format: format, Err: err})
			continue
		}
		result := templatefmt.Result{Path: path, Format: format, Source: source}
		formatted, err := templatefmt.SourceAs(format, path, source, options)
		if err != nil {
			result.Err = err
		} else {
			result.Formatted = formatted
			result.Changed = string(formatted) != string(source)
		}
		results = append(results, result)
	}
	return results, nil
}
