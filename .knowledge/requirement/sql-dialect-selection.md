---
id: requirement:sql-dialect-selection
type: requirement
title: Explicit SQL Dialect Selection
---
A generation run containing SQL templates must name its target dialect, and that name must reach the emitted placeholder style.

```yaml
source:
  - decision:sql-dialect-generation-time
  - user design discussion 2026-07-29
defect:
  symptom: a MySQL target receives PostgreSQL $1 placeholders, which MySQL rejects
  cause: templates/sqlbind GenerateOptions.PlaceholderStyle exists and works, but no caller outside its own unit test ever sets it
  evidence: generator/templates.go builds GenerateOptions from Package, ContextAPI, ContextOnly, and ExecutorResolver only
  consequence: every generated file emits Dollar regardless of target engine, and the unit test that passes "question" hides the gap
value: data:sql-dialect
selection:
  layer: generation only; no generated signature gains a dialect parameter
  requirement: explicit; there is no implicit default
  scope: required only when the run discovers at least one SQL template
plumbing:
  - Options.SQLDialect on the generator option surface, listed in data:generator-options
  - GenerateRequest.SQLDialect overriding the base value in GenerateRequest.applyTo
  - a -sql-dialect CLI flag alongside the existing SQL flags
  - generator template emission passing the value into templates/sqlbind GenerateOptions.Dialect, which replaces the unreachable PlaceholderStyle option
  - templates/sqlbind mapping the dialect to the sqlbind.NewBuilder style constant
validation:
  shared_check: templates/sqlbind ValidateDialect, exported so the discovering caller and the compiler apply one rule
  unknown_dialect: generation error naming the rejected value and listing the choices
  missing_dialect: generation error before any file is written, reported as a configuration error naming the option rather than a per-template diagnostic
  low_level: templates/sqlbind Generate also rejects an empty dialect, so the guarantee does not depend on which caller invokes it
  discovery_paths: both GenerateTemplates and templateArtifacts check, because either can be the entry point
  breaking: existing projects with SQL templates must add the option; this is intended, because a silent default is the defect above
regeneration:
  mechanism: rule:generation-input-hash already serializes normalized data:generator-options into the fingerprint
  effect: changing the dialect invalidates the data:generation-stamp with no new hashed input
  condition: the value must live on Options, not only on the per-run request, for the base configuration to be hashed
deferred:
  - the rule:sql-dialect-syntax-rejection checks; the validation_attributes of data:sql-dialect hold their place
  - sqlite selection
non_goals:
  - translating author SQL between engines; rule:sql-dialect-syntax-rejection rejects instead
  - a runtime dialect argument; decision:sql-dialect-generation-time keeps generated APIs dialect-free
  - dialect inference from a driver name, DSN, or database handle
  - one generated package serving more than one dialect
acceptance:
  - a run with SQL templates and no dialect fails with a configuration error naming the option
  - a run with no SQL template succeeds without the option
  - an unrecognized dialect fails and names the value
  - dialect mysql emits sqlbind.NewBuilder(sqlbind.Question) and 'WHERE id = ?'
  - dialect postgresql emits sqlbind.NewBuilder(sqlbind.Dollar) and 'WHERE id = $1'
  - an expanded value list follows the selected style for both dialects
  - changing only the dialect makes previously stamped files report stale
  - the generated public API is identical under both dialects except for placeholder text
  - no generated signature gains a dialect parameter
```
