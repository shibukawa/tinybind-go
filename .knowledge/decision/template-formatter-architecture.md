---
id: decision:template-formatter-architecture
type: decision
title: Delegated Template Printer Architecture
---
Print a template source by mirroring decision:template-parser-delegation: the shared package prints everything it parses, and each format package prints only its own body nodes.

```yaml
status: implemented 2026-08-02
source:
  - requirement:template-source-formatting
  - decision:template-parser-delegation
  - decision:template-package-boundaries
shared_printer:
  package: templates/internal/syntax
  owns:
    - module, package, import, type, enum, and external declarations
    - annotations and declaration headers, including parameters and output types
    - expression printing, which is the same grammar in all three formats
    - if/else/for headers and the recursion back into the active format printer
  excludes:
    - any HTML, SQL, or DynamoDB token, because the shared parser already refuses to know them
format_printer:
  interface: one PrintBody entry per format, symmetric with the ParseBody a format parser already implements
  brace_ownership:
    rule: the body printer owns the line break after the opening brace and before the closing one, not the module printer
    reason: in HTML a line break is content, so whether a body may open on its own line is a property of the format; a fragment body glued to its brace stays glued, and a SQL body always opens
  entry: syntax.BodyPrinter, registered as syntax.RootPrinter against the declaration kind its parser produced
  owns:
    - its own body node types and their line and indentation policy
    - re-escaping whatever its parser decoded, per rule:template-format-fidelity
    - calling back into the shared printer for embedded expressions and control nodes
  registration: the same lowercase root declaration keyword the parser registers, so a format cannot be parseable and unprintable
why_not_per_package_printers:
  rejected: give templates/htmlbind, templates/sqlbind, and the dynamo package a whole printer each
  reason: the header is the majority of a short declaration file and is identical across formats, so three copies would drift and each drift shows up as a diff in a user's repository
input:
  from: the AST, extended by requirement:template-comment-retention
  not_from: the source text, because a source-text rewriter would need a second, independent understanding of the grammar
positions:
  use: ordering and comment attachment only
  note: syntax.Position carries line and column but no offset, so the printer works from node order rather than by slicing the original bytes
composition:
  cli: api:template-format-command dispatches by file pattern and never contains format knowledge itself
  library: api:template-formatter-library is the public entry; each format package exports its own RootPrinter beside its Parse
related:
  - requirement:template-source-formatting
  - decision:template-parser-delegation
  - decision:template-package-boundaries
  - rule:template-format-fidelity
  - requirement:template-comment-retention
  - api:template-format-command
  - api:template-formatter-library
```
