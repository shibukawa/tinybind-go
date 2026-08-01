---
id: concept:typed-template-language
type: concept
title: Compact Typed Template Language
---
Small statically typed DSL for HTML composition and parameterized SQL generation. It is not a general-purpose language.

```yaml
evidence:
  source: user-supplied Compact Typed Template Language Specification
  received: 2026-07-20
review_gate: requirements remain proposed until user approval
outputs:
  - html
  - sql.exec
  - sql.one<T>
  - sql.optional<T>
  - sql.many<T>
  - sql.predicate
  - sql.relation<T>
principles:
  - parse output structure instead of interpolated raw strings
  - output type selects body parser, insertion rules, generated API, and SQL cardinality
  - keep exported component signatures explicit and stable
  - share typed declarations, expressions, and structural control across output formats
  - let each format parser discover embedded boundaries and contexts while the shared parser owns their grammar
requirements:
  - requirement:template-language-core
  - requirement:sql-relation-composition
  - requirement:html-template-v1
  - requirement:explicit-output-control
  - requirement:sql-template-v1
  - requirement:sql-generated-api-layers
  - requirement:template-code-generation
  - requirement:template-v1-scope
boundary: decision:template-package-boundaries
safety: rule:template-context-safety
naming: rule:template-name-casing
declarations: decision:template-declaration-kinds
parser: decision:template-parser-delegation
formatter: requirement:template-source-formatting, since a module that invents a file format owns the tool that formats it
sql_dialect: decision:sql-dialect-generation-time
```
