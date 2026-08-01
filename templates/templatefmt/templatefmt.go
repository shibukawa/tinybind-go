// Package templatefmt formats tinybind template sources.
//
// It is the library entry point for requirement:template-source-formatting: the
// fmt subcommand is a thin wrapper over it, so an editor plugin, a build step,
// or a framework's own tooling needs no CLI process. Formatting is a pure
// function of a source and a set of options; nothing here reads or writes the
// filesystem except Dir, which exists because "format this package" is the
// request everything else is built from.
package templatefmt

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	"github.com/shibukawa/tinybind-go/templates/dynamobind"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// Format identifies one template language.
type Format string

const (
	// HTML is the .tb.html component language.
	HTML Format = "html"
	// SQL is the .tb.sql statement language.
	SQL Format = "sql"
	// Dynamo is the .tb.dynamo access-pattern language.
	Dynamo Format = "dynamo"
)

// Default file patterns, matching the generator's own discovery defaults.
const (
	DefaultHTMLPattern   = "*.tb.html"
	DefaultSQLPattern    = "*.tb.sql"
	DefaultDynamoPattern = dynamobind.DefaultTemplatePattern
)

// DefaultWidth is the soft line width used when Options.Width is zero.
const DefaultWidth = syntax.DefaultWidth

// Options configures formatting. The zero value is valid and uses every
// default, so templatefmt.Source(name, src, templatefmt.Options{}) is the whole
// API for the common case.
type Options struct {
	// Width is the soft line width; zero uses DefaultWidth. It is soft because a
	// construct that cannot break without changing meaning stays long.
	Width int
	// Indent is one indentation level; empty uses two spaces.
	Indent string
	// PreserveWhitespace mirrors the generator option of the same name. With it
	// set, generation no longer collapses static whitespace, so HTML layout is
	// restricted to the positions the HTML parser discards runs from.
	PreserveWhitespace bool
	// HTMLPattern, SQLPattern, and DynamoPattern are base-name globs used by Dir
	// and by Source when it identifies a file. Empty values use the defaults.
	HTMLPattern   string
	SQLPattern    string
	DynamoPattern string
}

func (o Options) print() syntax.PrintOptions {
	return syntax.PrintOptions{Width: o.Width, Indent: o.Indent, PreserveWhitespace: o.PreserveWhitespace}
}

func (o Options) pattern(format Format) string {
	switch format {
	case HTML:
		if o.HTMLPattern != "" {
			return o.HTMLPattern
		}
		return DefaultHTMLPattern
	case SQL:
		if o.SQLPattern != "" {
			return o.SQLPattern
		}
		return DefaultSQLPattern
	default:
		if o.DynamoPattern != "" {
			return o.DynamoPattern
		}
		return DefaultDynamoPattern
	}
}

// ErrUnknownFormat reports a file name that matches no template pattern.
var ErrUnknownFormat = errors.New("templatefmt: file name matches no template pattern")

// Identify reports which language a file name belongs to. A name matching two
// patterns is ambiguous and reported rather than guessed, on the same terms
// generation refuses it.
func Identify(name string, options Options) (Format, error) {
	base := filepath.Base(name)
	var found Format
	for _, format := range []Format{HTML, SQL, Dynamo} {
		ok, err := filepath.Match(options.pattern(format), base)
		if err != nil {
			return "", fmt.Errorf("templatefmt: invalid %s pattern: %w", format, err)
		}
		if !ok {
			continue
		}
		if found != "" {
			return "", fmt.Errorf("templatefmt: %s matches both the %s and %s patterns", base, found, format)
		}
		found = format
	}
	if found == "" {
		return "", fmt.Errorf("%w: %s", ErrUnknownFormat, base)
	}
	return found, nil
}

// Source formats one source, choosing the language from the file name.
func Source(filename string, source []byte, options Options) ([]byte, error) {
	format, err := Identify(filename, options)
	if err != nil {
		return nil, err
	}
	return SourceAs(format, filename, source, options)
}

// SourceAs formats one source in a named language. It is what a caller with no
// file name uses, such as an editor filtering a buffer through standard input.
func SourceAs(format Format, filename string, source []byte, options Options) ([]byte, error) {
	source, err := normalizeEncoding(filename, source)
	if err != nil {
		return nil, err
	}
	switch format {
	case HTML:
		module, err := htmlbind.Parse(filename, source)
		if err != nil {
			return nil, err
		}
		return printModule(module, htmlbind.RootPrinter(), options)
	case SQL:
		module, err := sqlbind.Parse(filename, source)
		if err != nil {
			return nil, err
		}
		return printModule(module, sqlbind.RootPrinter(), options)
	case Dynamo:
		return dynamobind.Format(filename, source, options.print())
	default:
		return nil, fmt.Errorf("templatefmt: unknown format %q", format)
	}
}

func printModule(module *syntax.Module, root syntax.RootPrinter, options Options) ([]byte, error) {
	out, err := syntax.PrintModule(module, []syntax.RootPrinter{root}, options.print())
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// Result is the outcome for one file.
type Result struct {
	Path   string
	Format Format
	// Source is the file as it was read.
	Source []byte
	// Formatted is the canonical form. It is nil when Err is set, because a
	// source that does not parse is left exactly as it is.
	Formatted []byte
	// Changed reports that Formatted differs from Source.
	Changed bool
	// Err is the parse or print failure for this file. Other files in the same
	// run are unaffected.
	Err error
}

// Dir formats every template directly inside one directory. Discovery does not
// descend, matching requirement:configurable-template-file-patterns: a template
// belongs to the Go package it sits in.
//
// Nothing is written; the caller decides what to do with each Result.
func Dir(dir string, options Options) ([]Result, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		format, err := Identify(entry.Name(), options)
		if err != nil {
			if errors.Is(err, ErrUnknownFormat) {
				continue
			}
			return nil, err
		}
		path := filepath.Join(dir, entry.Name())
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			results = append(results, Result{Path: path, Format: format, Err: readErr})
			continue
		}
		result := Result{Path: path, Format: format, Source: source}
		formatted, formatErr := SourceAs(format, path, source, options)
		if formatErr != nil {
			result.Err = formatErr
		} else {
			result.Formatted = formatted
			result.Changed = string(formatted) != string(source)
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results, nil
}

// Write applies a result to disk. It writes only when the file would change, so
// an already formatted file keeps its timestamp and rule:generation-input-hash
// sees no reason to regenerate.
func (r Result) Write() error {
	if r.Err != nil || !r.Changed {
		return r.Err
	}
	info, err := os.Stat(r.Path)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(r.Path, r.Formatted, mode)
}

// utf8BOM is the byte order mark some editors put at the head of a UTF-8 file.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// normalizeEncoding settles the two questions a formatter can answer once for
// the whole file: a template source is UTF-8 without a byte order mark, and its
// lines end with LF.
//
// The normalization happens before parsing rather than after printing, so a
// region copied byte for byte - a script or style body, a SQL literal - comes
// out with LF too. Printing alone could not reach inside those.
func normalizeEncoding(filename string, source []byte) ([]byte, error) {
	if !utf8.Valid(source) {
		return nil, fmt.Errorf("templatefmt: %s is not valid UTF-8", filename)
	}
	source = bytes.TrimPrefix(source, utf8BOM)
	if !bytes.Contains(source, []byte("\r")) {
		return source, nil
	}
	// A lone carriage return is not a line ending anyone writes today, so it is
	// left where it is rather than guessed at; only the CRLF pair is a line.
	return bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n")), nil
}
