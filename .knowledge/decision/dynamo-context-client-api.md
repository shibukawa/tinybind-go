---
id: decision:dynamo-context-client-api
type: decision
title: Optional Context Client And Table Prefix
---
Keep the explicit client parameter as the stable generated API, and optionally add Context-resolved wrappers that also carry the deployment table prefix.

```yaml
status: implemented
built:
  runtime: dynamobind/context.go
  emitter: generator/dynamoquery_emit.go
  options: generator.Options DynamoContextAPI, DynamoContextOnlyAPI, DynamoClientResolver, and the -dynamo-context-api and -dynamo-context-only-api flags
  fixture: internal/dynamofixture, generated with the Context API on so both surfaces run against the fake server
model: decision:sql-context-executor-api, whose problem and answer are the same one layer over
default:
  explicit_api: always generated
  context_api: disabled
carried_together:
  what: the client and the table prefix travel in one Context value
  why: both are deployment facts fixed for a process, and splitting them would make a caller set two things to get one working call
runtime:
  setter: "WithClient(ctx, *dynamodb.Client, ...ClientOption) context.Context"
  prefix_option: "WithTablePrefix(string) ClientOption"
  client_resolver: "ClientFromContext(ctx) (*dynamodb.Client, error)", the client alone, for a table name no prefix applies to
  table_resolver: "TableFromContext(ctx, table) (*dynamodb.Client, string, error)", what generated wrappers call
  key: private typed key
  table_resolution: the declared table name of requirement:dynamo-typed-queries with the prefix prepended
  errors: ErrNoClient and ErrNoTablePrefix
generation_options:
  DynamoContextAPI: generate "<Name>Context" wrappers taking no client
  DynamoContextOnlyAPI: publish only the context-resolved surface under the declared name, leaving "<Name>Context" free
  DynamoClientResolver: an optional framework resolver, implying DynamoContextAPI
resolver_contract:
  signature: "func(context.Context, string) (*dynamodb.Client, string, error)"
  argument: the declared table name, so a framework maps it onto a physical one however it likes rather than only prepending
  shape: the same as TableFromContext, which is the default
error_shapes:
  page: returns the resolver error
  many: yields it once and stops, as a failed page already does
name_collision:
  when: the Context mode is on and one statement is named what another statement's wrapper would take
  result: a generation error naming both, since the mode must not silently take a declared name
naming:
  explicit: "<Name>"
  context: "<Name>Context"
  context_only: "<Name>" becomes the context form and the explicit one becomes unexported
errors_not_panics:
  rule: a resolver returns an error, and a missing client or prefix fails loudly
  why_it_matters_more_here_than_for_sql: a missing SQL executor cannot execute at all, while a missing prefix would read the unprefixed table and answer with a normal empty page, so a silent fallback is indistinguishable from no data
  consequence: no empty-prefix default; an unset prefix on a wrapper that needs one is an error
item_operations:
  problem: Load, Store and the rest are runtime generics rather than generated, so a Context variant of each would double the exported surface
  chosen: export ClientFromContext and let a caller resolve in one line, keeping the resolution visible
  later: per-operation variants stay possible and additive, once handlers are seen calling item operations directly often enough to pay for them
constraints:
  - the context mode never replaces or changes the explicit signature
  - the mode is fixed at generation time and applies to the whole package
  - generated wrappers resolve and delegate; they open nothing and close nothing
related:
  - requirement:dynamo-typed-queries
  - api:dynamobind-operations
  - decision:sql-context-executor-api
  - rule:dynamobind-driver-passthrough
```
