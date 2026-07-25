---
id: requirement:template-code-generation
type: requirement
title: Template Code Generation
---
Compile templates to small Go APIs without runtime interpretation or reflection.

```yaml
source: concept:typed-template-language
inherits:
  - decision:reflection-free
  - requirement:tinygo-wasm
parser_architecture: decision:template-parser-delegation
compiler_pipeline:
  - parse modules, types, enums, external functions, and component signatures
  - validate declaration kinds and symbol casing through decision:template-declaration-kinds and rule:template-name-casing
  - build symbols and select a registered body parser from the declaration keyword and output type
  - let the format parser discover embedded boundaries and attach namespaced contexts
  - delegate expression and control headers to the shared parser; recursively return branch bodies to the format parser
  - parse format structure without introducing format-specific nodes into the shared parser
  - type-check calls and validate format contexts
  - classify HTML components through data:component-render-capabilities and decision:component-capability-lowering
  - validate feature combinations through rule:component-capability-combinations
  - validate HTML escaping and SQL parameters, identifiers, result shape, and cardinality
  - lower typed SQL IR using decision:sql-dialect-generation-time
  - expand typed SQL relations before dialect lowering and placeholder emission
  - generate context-checked raw output and typed JsonForScript serialization from requirement:explicit-output-control
  - coalesce static output and emit Go
html_api:
  default: func Component(w http.ResponseWriter, r *http.Request, typed parameters...) error
  writer_mode: requirement:html-writer-api-mode
sql_api: requirement:sql-generated-api-layers
artifact_attribution: requirement:per-source-generation-artifacts
runtime_constraints:
  - no runtime template parsing
  - no reflection or dynamic type lookup
  - no runtime string evaluation
  - no virtual DOM
  - preserve write, query, scan, and cardinality errors
```
