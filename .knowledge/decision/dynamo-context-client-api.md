---
id: decision:dynamo-context-client-api
type: decision
title: The Client Comes From The Context
---
Carry the DynamoDB client and the deployment table prefix in one Context value, and give no entry of dynamobind a client parameter at all.

```yaml
status: implemented
built:
  runtime: dynamobind/context.go, and every entry of item.go, query.go and batch.go
  emitter: generator/dynamoquery_emit.go
  fixture: internal/dynamofixture
one_surface:
  rule: there is no client-taking form, no suffixed variant and no generation option
  reason: the client is a deployment fact fixed for a process, so a parameter repeats at every call site what one setup line already said
  second_client: a second Context, not a second signature, which is what a test or a second region uses
carried_together:
  what: the client and the table prefix travel in one Context value
  why: both are deployment facts fixed for a process, and splitting them would make a caller set two things to get one working call
runtime:
  setter: "WithClient(ctx, *dynamodb.Client, ...ClientOption) context.Context"
  prefix_option: "WithTablePrefix(string) ClientOption"
  table_resolver: "TableFromContext(ctx, table) (*dynamodb.Client, string, error)", which every entry of the package calls
  client_resolver: "ClientFromContext(ctx) (*dynamodb.Client, error)", the escape hatch for reaching the driver directly
  key: private typed key
  table_resolution: the argument table name with the prefix prepended, whether it came from a declaration or from an item call
  errors: ErrNoClient and ErrNoTablePrefix
signatures:
  item: "Load(ctx, table, key, opts...)" and the rest, still naming a table because they have no declaration to read one from
  declared_query: "<Name>(ctx, params..., opts...)", naming neither, per requirement:dynamo-typed-queries
  where_resolution_happens: inside the runtime entry, so generated code passes its declared table name and holds no resolver call
errors_not_panics:
  rule: a resolver returns an error, and a missing client or prefix fails loudly
  why_the_prefix_matters: a missing client cannot issue a request at all, while a missing prefix would read the unprefixed table and answer with a normal empty page, so a silent fallback is indistinguishable from no data
  consequence: no empty-prefix default; a deployment using the declared names says so with WithTablePrefix("")
  reporting: a function returning an error returns it; an iterator yields it once with the zero value and stops, as a failed page already does
cost:
  binary: +37,971 bytes on tinygo wasip1, per requirement:dynamobind-verification
  cause: context.WithValue and the type assertion that reads it back, which is inherent to the pattern rather than to this implementation
  accepted: the call-site property was the requirement; a size-critical program calls the driver directly with the generated methods and links none of this
no_framework_resolver:
  what: no generation option selects a resolver, unlike decision:sql-context-executor-api
  why: resolution moved into the runtime entries, so there is no generated call site to redirect; a framework installs its own client and prefix with WithClient instead
  later: a per-deployment name mapping stays possible as a ClientOption, which is additive
related:
  - requirement:dynamo-typed-queries
  - api:dynamobind-operations
  - decision:sql-context-executor-api
  - rule:dynamobind-driver-passthrough
```
