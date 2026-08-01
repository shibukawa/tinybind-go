---
id: api:template-formatter-library
type: api
title: templatefmt Formatting Library
---
Package templates/templatefmt formats template sources as a library, so an editor plugin, a build step, or a framework needs no CLI process to get the canonical form.

```yaml
status: implemented 2026-08-02
package: github.com/shibukawa/tinybind-go/templates/templatefmt
public_shape:
  - "type Format string; const HTML, SQL, Dynamo Format"
  - "type Options struct { Width int; Indent string; PreserveWhitespace bool; HTMLPattern, SQLPattern, DynamoPattern string }"
  - "func Identify(name string, options Options) (Format, error)"
  - "func Source(filename string, source []byte, options Options) ([]byte, error)"
  - "func SourceAs(format Format, filename string, source []byte, options Options) ([]byte, error)"
  - "func Dir(dir string, options Options) ([]Result, error)"
  - "type Result struct { Path string; Format Format; Source, Formatted []byte; Changed bool; Err error }"
  - "func (r Result) Write() error"
  - "var ErrUnknownFormat error"
design:
  zero_value: Options{} is valid and uses every default, so the common call is Source(name, src, Options{})
  purity: Source and SourceAs touch no filesystem; Dir and Result.Write are the only entries that do, because "format this package" is the request everything else is built from
  no_writes_by_default: Dir returns results and writes nothing, so a caller decides between reporting, diffing, and rewriting
  identification: Identify applies the requirement:configurable-template-file-patterns globs, and reports a name matching two patterns rather than picking one
  per_format_entry: SourceAs takes the language explicitly, which is what a caller with a buffer and no file name needs
failure_isolation:
  - a parse failure is carried on Result.Err with Formatted left nil, so a broken file is left exactly as it is
  - one failing file does not end a Dir run
  - Result.Write writes only when Changed, so an already formatted file keeps its timestamp and rule:generation-input-hash sees no change
composition:
  cli: api:template-format-command is a thin wrapper, holding only flag parsing and process exit codes
  printers: each format package exports RootPrinter or Format; templatefmt selects among them and owns no layout of its own
related:
  - requirement:template-source-formatting
  - api:template-format-command
  - decision:template-formatter-architecture
  - rule:template-format-fidelity
  - requirement:configurable-template-file-patterns
  - decision:template-package-boundaries
```
