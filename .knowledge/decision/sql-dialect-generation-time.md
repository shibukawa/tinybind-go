---
id: decision:sql-dialect-generation-time
type: decision
title: SQL Dialect Fixed at Generation Time
---
Select SQL dialect and placeholder style when running the code generator, never when executing generated application APIs.

```yaml
source:
  - concept:typed-template-language
  - user design discussion 2026-07-20
  - user design discussion 2026-07-29
generator_options:
  dialect: data:sql-dialect, one value covering every engine difference
  selection: requirement:sql-dialect-selection; explicit, with no implicit default
  placeholder_style: derived from the dialect, not configured separately
reconsidered_runtime_option:
  proposal: pass the dialect at execution time the way requirement:sql-read-only-executor passes the read-only flag
  feasible: yes, once rule:sql-dialect-syntax-rejection removed SQL translation from scope, the placeholder style became the only dialect-divergent thing generated code emits
  rejected_on_cost_not_feasibility: true
  rejected_because:
    - asymmetry; a generation-time choice can gain a runtime escape hatch later, but a runtime parameter cannot be removed from a published signature
    - cost distribution; every call site of every generated function would carry a value that is constant in almost every program, to serve the one case of a binary speaking two engines
    - requirement:sql-generated-api-layers sells Build<Component> as a database-free deterministic call; a dialect argument degrades the layer with the most value
    - the read-only flag is consumed by the Context resolver, which only the Context API layer has; the placeholder style is consumed by sqlbind.NewBuilder inside Build<Component>, which takes neither context.Context nor an executor, so a context-carried dialect would leave the lowest layer uncovered
    - the accident-prevention argument for a runtime value is already answered; requirement:sql-dialect-selection makes an unset dialect a loud generation error
future_escape_hatch:
  need: one binary querying both PostgreSQL and MySQL, which also requires SQL text portable without translation
  design: Builder.Arg records the byte offset of each placeholder instead of only writing it; Statement keeps its generation-time SQL and gains a re-render to another style
  properties:
    - no generated signature changes
    - splicing recorded offsets never rescans SQL text, so a question mark inside a literal cannot be mistaken for a placeholder
    - the other style must be asked for explicitly, so no silent fallback exists
  status: not built; add only when a real caller needs it
pipeline:
  - parse to dialect-neutral typed SQL IR
  - validate the selected dialect against rule:sql-dialect-syntax-rejection
  - bake placeholder appender into generated code
  - emit requirement:sql-generated-api-layers
not_in_pipeline: lowering dialect-specific syntax or types; author SQL text reaches the output verbatim
runtime:
  receives:
    - component parameters
    - runtime structural condition values
    - database executor for high-level API
  excludes:
    - dialect argument
    - placeholder-style argument
    - driver-based dialect detection
multi_dialect: generate separate packages or artifacts for each dialect
enforcement: an unset dialect is a generation error, so the option cannot stay unreachable the way the earlier placeholder-style option did
benefits:
  - deterministic SQL and golden tests
  - generation-time unsupported-feature diagnostics
  - no per-query dialect branching
  - stable generated public APIs
  - a generated signature identical under every dialect, so switching engines is a diff in SQL text only
```
