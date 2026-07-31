---
id: requirement:dynamo-typed-queries
type: requirement
title: Typed DynamoDB Queries
---
Generate one named function per declared access pattern, so a query names no attribute, no placeholder and no expression at the call site.

```yaml
status: implemented
built:
  parser: generator/dynamoquery.go
  checks: generator/dynamoquery_plan.go
  emitter: generator/dynamoquery_emit.go
  wiring: generator/dynamoquery_generate.go, writing dynamoquery_gen.go
  fixture: internal/dynamofixture/readings.tb.dynamo
  context_client: per decision:dynamo-context-client-api, resolved inside the runtime entry rather than in generated code
scope: decision:dynamo-framework-requests
checks: rule:dynamo-query-checks
problem:
  now: "dynamobind.Query[Reading](ctx, table, \"userID = :uid AND ts > :from\", dynamodb.WithExpressionValues(...))"
  strings: the attribute name, the ":uid" placeholder and the value map key are three unrelated strings, and a tag rename breaks none of them at compile time
  scope_of_the_gap: the drift requirement:dynamobind-product-goals closed for the item, still open for the read path
declaration:
  file: a template source discovered beside the package, as the .tb.sql and .tb.html of requirement:configurable-template-file-patterns are
  outer_structure: reused from .tb.sql - export statement, a typed parameter list, a result type after a colon, a braced body
  body: DynamoDB clauses rather than SQL text
  example: "export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> { table readings; key sensor = {sensor} and at > {from} }"
  parameters: named in the caller's vocabulary and bound to attributes where the condition names them, so the two namespaces stay separate
  export_keyword: must agree with the name's own casing, since Go decides visibility by the name; either without the other is an error rather than a silent rename
result_type_slot:
  chooses: the request shape rather than a row count, since a Query always returns many
  page: one request, returning Page[T]
  many: the iterator over every page
  reason: rule:dynamobind-driver-passthrough keeps the request count visible, so the author picks rather than a default
table_clause:
  form: "table <name>", required in every statement body
  effect: the generated function takes no table parameter, and with the client in the Context it takes neither of the two the old call site repeated
  name_check: the DynamoDB rule, three to 255 characters of letters, digits, underscore, hyphen and dot, so a name the service would reject fails generation rather than the first call
  order: either clause may come first, and ";" separates them on one line
  why_the_statement_owns_it:
    a_type_is_not_one_table: the same struct can be stored in a test table and a production one, so binding the table to the type asserts something untrue
    a_statement_is_one_table: an access pattern names exactly one, so the fact is complete where it is written and a reader needs no second file
    direction: the result type is the decode target, an output; the table is an input, and inputs belong in the body with the key clause and the parameters
  required_not_optional: one declaration form must yield one signature; an optional clause would produce two, which is the surprise this codebase avoids elsewhere
  item_operations: Load, Store and the rest keep their table parameter, having no declaration to read it from; that is the absence of a declaration rather than an inconsistency
  deployment_prefix: resolved at run time by the runtime entry, per decision:dynamo-context-client-api
generated:
  one_function_per_declaration: named by the declaration, returning the page or iterator form its result type selects
  signature: context, the declared parameters, then variadic driver query options, and nothing else; the generated names and values are appended last so a caller option cannot replace the condition
  what_is_absent: the table, which the declaration names, and the client, which the Context carries; the signature holds only what neither can supply
  expression: a constant, with the attribute aliases and the placeholder names fixed at generation time
  table: a constant beside the expression, so the declared name is one string in one place
  values: built per call from the typed parameters, through the same attribute encoders the codec uses
  no_builder: the function embeds its condition directly; no per-type condition builder is generated
counts_as_usage:
  what: a declaration is a use of its result type, feeding DynamoDecode into the item pass
  why: the generated function instantiates dynamobind.Query with that type, which does not compile without the decoder
  effect: a package whose only DynamoDB use is a declaration still gets a codec, per rule:usage-directed-generation
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
