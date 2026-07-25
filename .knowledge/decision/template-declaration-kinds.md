---
id: decision:template-declaration-kinds
type: decision
title: Format-Specific Template Declarations
---
Use explicit lowercase declaration keywords instead of calling every output declaration a component.

```yaml
source:
  - concept:typed-template-language
  - user design discussion 2026-07-20
declarations:
  component:
    format: HTML
    required_output: html
    slots: requirement:html-slot-syntax makes a component slot-capable; layouts need no separate keyword
  statement:
    format: SQL
    required_output_prefix: sql
common:
  - optional export modifier controls generated public API visibility
  - PascalCase declaration name
  - typed parameters and explicit output type
  - private declaration when export is absent
semantics:
  - declaration keyword selects a registered format parser through decision:template-parser-delegation
  - output type selects result behavior, insertion contexts, and SQL cardinality
  - keyword and output type mismatch is a compile-time error
compiler_model:
  common: TemplateDecl
  format_nodes: HTMLComponentDecl and SQLStatementDecl
  node_type_ids: namespaced by decision:template-parser-delegation
```
