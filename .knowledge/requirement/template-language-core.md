---
id: requirement:template-language-core
type: requirement
title: Template Language Core
---
Provide one typed declaration and expression core shared by HTML and SQL body parsers.

```yaml
source: concept:typed-template-language
parser_architecture: decision:template-parser-delegation
declarations:
  - optional package or module and imports
  - primitive, record, array, optional, and basic enum types
  - typed external functions and external components through requirement:template-file-scope
  - HTML component and SQL statement declarations from decision:template-declaration-kinds
  - exported and private typed declarations; visibility uses export, not capitalization
naming: rule:template-name-casing
primitives: [string, bool, int, float, decimal, datetime, date, time, url, bytes]
expressions:
  - variables, field access, array indexing, and literals
  - typed ordinary function calls, including nesting
  - comparisons, boolean operators, basic arithmetic, null checks, and ternary
control:
  if: bool condition; else and else-if supported
  for: collection item iteration; optional index
  binding: proposed requirement:template-value-binding names one or more expression results for its own subtree, which is the only binder a synchronous external has
  recognition: active format parser discovers boundaries only in its valid lexical contexts
  recursion: shared parser parses headers and delegates each control body back to the active format parser
functions:
  standard: portable semantics defined by the language
  external:
    flat_template_mode: same-package backend-mapped and statically checked
    filesystem_route_mode: statically dispatched through data:html-route-dependencies
  template_declaration: reusable typed output composition
  intrinsic: compiler-known context-sensitive functions from requirement:explicit-output-control
opaque_output_types: [trusted_html, trusted_css, trusted_javascript, script_json]
non_value_types:
  html: continuation usable only at a requirement:html-slot-syntax slot element; never an expression operand
validation:
  - resolve types, declarations, and functions at compile time
  - reject invalid insertion and structural contexts
  - select format parser from declared output type
  - keep format-supplied context IDs opaque to the shared parser
```
