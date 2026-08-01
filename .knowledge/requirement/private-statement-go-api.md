---
id: requirement:private-statement-go-api
type: requirement
title: A Private Statement Is Callable From Its Own Package
---
A statement without export generates the same API an exported one does, under unexported Go names, so application code in the package calls it by the name it was declared under.

```yaml
status: implemented 2026-08-02
source: user requirement 2026-08-02
problem:
  was: a private executable statement generated only the prefixed fragment builder, `_tinybindBuild<Name>(b *Builder, ...) error`
  consequence: nothing ran the query, so a handler had to name a generator-internal symbol and then assemble the Builder and execute by hand
  root_cause: requirement:template-file-scope made file visibility and Go symbol visibility one axis, which left no state for "usable inside this Go package but not part of its public API"
  observation: the missing state is the common one; most statements a package runs are not meant to be its public surface
rule:
  every executable statement generates the execution API, exported or not
  the execution function is named exactly as the declaration is, so its case is what decides Go visibility
  the export modifier and the name's case therefore have to agree, per decision:declaration-name-policy
generated_names:
  execution: the declaration name verbatim, both directions; FindUser and findUser
  context_api: the declaration name plus Context, when decision:sql-context-executor-api is on
  builder_wrapper: Build<Name> when exported, build<Name> when not; it is the one name that cannot be the declaration's own, because the execution API already holds that
  fragment_builder: _tinybindBuild<Name>, unchanged, and never written by hand
fragments_excluded:
  which: sql.predicate and sql.relation
  why: they are embedded into a caller's builder rather than run, so the fragment builder is their whole API and no Go identifier carries their name
  consequence: their own name case is unconstrained, which is why an existing private PascalCase predicate keeps working
export_meaning:
  keeps: publishing to other template files, and publishing to the package's Go API
  gains_nothing: it no longer decides whether the statement is usable at all
acceptance:
  - a package whose statements are all private generates code that compiles and runs
  - a handler calls findUser(ctx, db, id) with no generator-internal name at the call site
  - an exported statement is still callable as FindUser, unchanged
  - a private sql.predicate still generates only its fragment builder
related:
  - decision:declaration-name-policy
  - requirement:template-file-scope
  - requirement:sql-generated-api-layers
  - decision:sql-context-executor-api
  - decision:template-declaration-kinds
```
