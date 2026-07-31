---
id: requirement:dynamo-typed-queries
type: requirement
title: Typed DynamoDB Queries
---
Generate one named function per declared access pattern, so a query names no attribute, no placeholder and no expression at the call site.

```yaml
status: implemented 2026-07-31
implementation:
  parser: generator/dynamoquery.go
  checks: generator/dynamoquery_plan.go
  emitter: generator/dynamoquery_emit.go
  wiring: generator/dynamoquery_generate.go, writing dynamoquery_gen.go
  fixture: internal/dynamofixture/readings.tb.dynamo
scope: decision:dynamo-framework-requests
checks: rule:dynamo-query-checks
problem:
  now: "dynamobind.Query[Reading](ctx, c, table, \"userID = :uid AND ts > :from\", dynamodb.WithExpressionValues(...))"
  strings: the attribute name, the ":uid" placeholder and the value map key are three unrelated strings, and a tag rename breaks none of them at compile time
  scope_of_the_gap: the drift requirement:dynamobind-product-goals closed for the item, still open for the read path
declaration:
  file: a template source discovered beside the package, as the .tb.sql and .tb.html of requirement:configurable-template-file-patterns are
  outer_structure: reused from .tb.sql - export statement, a typed parameter list, a result type after a colon, a braced body
  body: DynamoDB clauses rather than SQL text
  example: "export statement ReadingsBySensor(sensor: string, from: int64): dynamo.many<Reading> { key sensor = {sensor} and at > {from} }"
  parameters: named in the caller's vocabulary and bound to attributes where the condition names them, so the two namespaces stay separate
result_type_slot:
  chooses: the request shape rather than a row count, since a Query always returns many
  page: one request, returning Page[T]
  many: the iterator over every page
  reason: rule:dynamobind-driver-passthrough keeps the request count visible, so the author picks rather than a default
no_table_clause:
  shape: the generated function takes the table name, as every other dynamobind entry does
  why: validation needs the attribute names and the key definitions, which come from the bound type's tags; the table name contributes nothing to it
  deployment_prefix: unchanged and not this requirement's problem, because Load and Store already take the table name
  later: an optional table clause could omit the parameter, but one declaration form yielding two signatures is the surprise this codebase avoids elsewhere; wait for a request
generated:
  one_function_per_declaration: named by the declaration, returning the page or iterator form its result type selects
  expression: a constant, with the attribute aliases and the placeholder names fixed at generation time
  values: built per call from the typed parameters, through the same attribute encoders the codec uses
  no_builder: the function embeds its condition directly; no per-type condition builder is generated
scope:
  in: key condition, limit, scan direction, consistent read
  out: filter, projection, condition and update expressions
  reason: what is left out is the open grammar api:dynamobind-operations already defers; a filter joins this declaration when it lands, needing no second mechanism
depends_on:
  - requirement:dynamobind-generated-item-codec, for the key attribute types
  - decision:dynamo-single-table-scope, so a query is defined against a table one type owns
related:
  - api:dynamobind-operations
  - rule:dynamo-tag-options
  - concept:typed-template-language
  - requirement:sql-generated-api-layers
```
