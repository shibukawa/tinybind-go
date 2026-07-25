---
id: requirement:template-v1-scope
type: requirement
title: Template V1 Scope
---
Keep the first implementation limited to the minimum language needed for safe HTML and SQL output composition.

```yaml
source: concept:typed-template-language
included:
  - requirement:template-language-core
  - requirement:sql-relation-composition
  - requirement:html-template-v1
  - requirement:html-slot-syntax unnamed slot, default content, and required marker
  - requirement:explicit-output-control
  - requirement:sql-template-v1
  - requirement:sql-generated-api-layers
  - requirement:template-code-generation
deferred:
  - requirement:html-slot-syntax named slots
  - immutable let bindings
  - explicit enum underlying values and field mapping annotations
  - anonymous SQL row types if named rows suffice for the first milestone
  - typed SQL identifier abstraction and affected-row annotations
  - bulk insert and repeated SQL fragment syntax
excluded:
  - general user-defined value functions, lambdas, and pipelines
  - map, filter, reduce, mutable variables, generics, pattern matching, and macros
  - dynamic HTML names and arbitrary attribute spreads
  - block control inside HTML attribute values or attribute lists
  - arbitrary SQL identifier interpolation, general SQL loops, and dynamic result columns
  - async language semantics and runtime interpretation
milestone_order:
  - declaration keywords, naming rules, types, expressions, and signatures
  - HTML structure, escaping, components, if, and for
  - explicit raw output and safe script JSON contexts
  - SQL parameters, result contracts, and static statements
  - private typed relation statements in FROM and JOIN
  - PostgreSQL lowering, generated statement builders, and execution wrappers
  - structured SQL lists and mutation guards
```
