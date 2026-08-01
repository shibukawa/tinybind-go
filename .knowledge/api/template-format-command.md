---
id: api:template-format-command
type: api
title: Template Format Command
---
A fmt subcommand on the api:generator-main dispatcher formats template sources in place, lists unformatted ones, or filters one source through standard streams.

```yaml
status: implemented 2026-08-02
public_shape:
  - "func FormatCommand(options Options) Command"
library: api:template-formatter-library holds everything this command does; the command adds flag parsing and exit codes only
command:
  name: fmt
  summary: format tinybind template sources
  registration: composed into a CommandSet like every other command, so a framework may omit or replace it under requirement:extensible-generator-command
flags:
  dir: package directory to format; defaults to the working directory and does not descend, matching requirement:configurable-template-file-patterns discovery
  w: write the result back to each source; without it the formatted text goes to stdout
  l: list the paths whose formatting differs and write nothing, for CI
  html-template-pattern: same glob as generate
  sql-template-pattern: same glob as generate
  dynamo-template-pattern: same glob as generate
  width: soft line width for rule:sql-template-layout and rule:html-template-layout; defaults to 100
  preserve-whitespace: mirrors the data:generator-options field of the same meaning and restricts HTML layout to the droppable positions, per rule:template-format-fidelity; the formatter cannot read the generator's configuration, so it has to be told
  as: language of a source read from stdin, one of html, sql, or dynamo
positional_arguments: file names to format instead of the whole directory; each is resolved against -dir
streams:
  stdin: -as reads one source from stdin and writes the result to stdout, which is what an editor filter needs
  format_selection_for_stdin: named by -as, because a stream has no file name to match a glob against
exit_codes:
  0: nothing to do, or every file formatted
  1: at least one file differs under -l, or a source failed to parse
  2: usage error
behavior:
  - a file is rewritten only when its formatted form differs, so timestamps stay put and rule:generation-input-hash does not see a spurious change
  - a parse failure reports the diagnostic and leaves that file untouched, then continues to the remaining files
  - the run never invokes generation, so formatting is safe on a package that does not currently generate
  - templatefmt is the library the command wraps, so a framework or an editor plugin needs no CLI process
  - registration is optional: cmd/tinybind-gen composes fmt beside generate, and a framework may omit or replace it
related:
  - requirement:template-source-formatting
  - decision:template-formatter-architecture
  - rule:template-format-fidelity
  - api:generator-main
  - requirement:extensible-generator-command
  - requirement:configurable-template-file-patterns
  - api:template-formatter-library
  - rule:sql-template-layout
  - rule:html-template-layout
```
