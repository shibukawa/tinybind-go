---
id: decision:template-package-boundaries
type: decision
title: Template Package Boundaries
---
Expose format-specific template APIs below templates while keeping the language core shared and non-public where practical.

```yaml
source:
  - concept:typed-template-language
  - decision:template-parser-delegation
module: github.com/shibukawa/tinybind-go
packages:
  templates/htmlbind:
    owns: HTML parsing, validation, escaping policy, and Go emission
  templates/sqlbind:
    owns: SQL parsing, typed IR, relation expansion, structured clauses, dialect lowering, parameterization, result contracts, and Go emission
  templates/internal:
    owns: declarations, type system, expressions, symbols, delegated control orchestration, structural control AST, and shared diagnostics
constraints:
  - public users select a format package explicitly
  - templates/sqlbind remains distinct from existing root sqlbind row-scanning runtime
  - generated SQL code uses the root sqlbind runtime and declares no runtime of its own, per decision:generated-runtime-in-module
  - generated HTML code uses the root htmlbind runtime on the same terms
  - generated SQL exposes data:sql-statement builders and requirement:sql-generated-api-layers wrappers
  - shared core does not import HTML- or database-specific runtime dependencies
  - format parsers discover embedded boundaries and supply opaque namespaced contexts through decision:template-parser-delegation
  - shared-parser tests use a lossless raw dummy parser instead of HTML or SQL parsing
  - package imports preserve decision:runtime-package-boundaries and requirement:tinygo-wasm
```
