---
id: requirement:template-source-formatting
type: requirement
title: Canonical Template Source Formatting
---
One formatter gives every hand-authored template source a canonical form, so a `.tb.html`, `.tb.sql`, or `.tb.dynamo` file is as unarguable as gofmt output.

```yaml
status: implemented 2026-08-02
priority: should
motivation:
  - the module invented these file formats, so no editor or external tool will ever format them; the obligation follows the invention
  - the generator already owns the only parser for all three, so the cost is a printer rather than a language
  - the value is review noise removed from hand-written declarations, which is where the formats are actually edited
scope:
  formats:
    - '*.tb.html' through templates/htmlbind
    - '*.tb.sql' through templates/sqlbind
    - '*.tb.dynamo' through decision:dynamo-template-shared-parser
  discovery: the same requirement:configurable-template-file-patterns globs, so a custom suffix formats on the same terms it generates
  out_of_scope:
    - sorting or deduplicating declarations, imports, attributes, or fields
    - rewriting one construct into another: SQL keyword case, placeholder spelling, and HTML self-closing syntax are left as authored
    - formatting generated Go output, which go/format already owns
file_conventions:
  encoding: UTF-8; a byte order mark is stripped and a source that is not valid UTF-8 is reported rather than reformatted
  line_endings: LF; a CRLF source is normalized before parsing rather than after printing, because a region copied byte for byte is out of a printer's reach
  indentation: two spaces per level, and a declaration body opens exactly one level
  trailing: exactly one newline at end of file, and no trailing whitespace on any line
  raw_text_exception: a script or style body keeps its own indentation, per rule:whitespace-preserving-contexts; re-indenting it would change served bytes, and inside a JavaScript template literal the leading whitespace is data
shared_header:
  applies_to: all three formats, because decision:template-parser-delegation gives them one declaration grammar
  form:
    - package first, then imports, then declarations, one blank line between top-level declarations
    - one annotation per line directly above the declaration it attaches to
    - the signature on one line as `export statement Name(p: T, q: U): out {`, with the brace on that line
    - a type declaration inline when it fits the width, one field per line otherwise, never both in one file for one declaration
    - two spaces per nesting level; a tab is avoided because inside an HTML body the indentation is character data, and a tab there renders differently from spaces
    - exactly one trailing newline and no trailing spaces on any line
body_form:
  html: rule:html-template-layout lays out elements, attributes, and closing tags within the whitespace freedom rule:template-format-fidelity leaves
  sql: rule:sql-template-layout lays out CTEs, clauses, joins, and subqueries over the existing rule:sql-top-level-keyword-scan token stream
  dynamo: fully canonical, because the clause grammar is closed and small enough that every declaration has one spelling
  common: all three are layout passes over a token or node stream, so none of them needs a second grammar
line_width:
  default: 100 columns, as a soft target rather than a hard limit
  soft: a construct that cannot break without changing meaning stays long; a glued HTML run and a multi-line SQL literal are the two that do this
  configurable: through api:template-format-command, since a project's own review width is not this module's decision
acceptance:
  - formatting every template in the repository, then formatting again, changes nothing on the second run
  - generation output is byte-identical before and after formatting, per rule:template-format-fidelity
  - a comment survives formatting in all three formats, per requirement:template-comment-retention
  - a statement whose body is one long line of WITH, joins, and a correlated subquery comes back with one clause per line and the subquery indented under its own SELECT
  - every child of a head element ends up on its own line
  - a glued inline run such as `<b>a</b><i>b</i>` stays on one line, and its rendered output is unchanged
  - a CRLF source, including the line endings inside its script and style bodies, comes back with LF only
  - a source that fails to parse is left untouched and reported with file, line, and column, per requirement:analysis-diagnostics
  - formatting is never a precondition for generation; an unformatted source still generates
related:
  - api:template-format-command
  - decision:template-formatter-architecture
  - rule:template-format-fidelity
  - requirement:template-comment-retention
  - decision:dynamo-template-shared-parser
  - rule:sql-template-layout
  - rule:html-template-layout
  - concept:typed-template-language
  - requirement:configurable-template-file-patterns
  - requirement:static-whitespace-normalization
```
