---
id: decision:dynamo-context-client-api
type: decision
title: The Client Comes From The Context
---
Carry the DynamoDB client in the Context by default, give no entry of dynamobind a client parameter unless requirement:dynamo-parameter-api is generated, and map declared table names onto the deployment's with an optional resolver function.

```yaml
status: implemented
built:
  runtime: dynamobind/context.go, and every entry of item.go, query.go and batch.go
  emitter: generator/dynamoquery_emit.go
  fixture: internal/dynamofixture
one_surface:
  rule: superseded 2026-08-05 by decision:nosql-client-supply-modes; this is the default rather than the only form
  was: there is no client-taking form, no suffixed variant and no generation option
  reason: the client is a deployment fact fixed for a process, so a parameter repeats at every call site what one setup line already said
  reason_still_holds: it is why the Context form stays the default and what a run adding no option generates
  second_client: a second Context, not a second signature, which is what a test or a second region uses
  what_reopened_it: the rule priced the call-site property against a parameter and never against the ctx.Value lookup, per requirement:framework-context-bundle
table_names:
  default: the declared name is sent unchanged, so a deployment named as declared configures nothing
  resolver: "WithTableNames(func(ctx context.Context, declared string) string) ClientOption", optional
  why_a_function_not_a_prefix:
    prefix_is_only_a_convention: DynamoDB table names have no structure the API reads, unlike an S3 key prefix, which ListObjectsV2, IAM and lifecycle rules all understand; a table prefix is just a string deployment tooling happens to prepend
    what_a_prefix_cannot_say: a CDK generated physical name carries a suffix, "orders-prod" puts the environment last, and a name read from an environment variable shares nothing with the declared one
    one_function_covers_all: prefix, suffix, lookup table and unrelated name cost the same
  why_it_takes_a_context: the mapping can depend on the request rather than only the process, so a per-tenant table is the same one function, and configuration bound to the Context is reachable from inside it
  nil_resolver: ignored, so a mistaken nil behaves as no resolver rather than panicking
runtime:
  setter: "WithClient(ctx, *dynamodb.Client, ...ClientOption) context.Context"
  names_option: "WithTableNames(TableResolver) ClientOption"
  table_resolver: "TableFromContext(ctx, table) (*dynamodb.Client, string, error)", which every entry of the package calls
  client_resolver: "ClientFromContext(ctx) (*dynamodb.Client, error)", the escape hatch for reaching the driver directly
  key: private typed key
  table_resolution: the argument name through the resolver, or unchanged when none is installed, whether it came from a declaration or from an item call
  errors: ErrNoClient
signatures:
  item: "Load(ctx, table, key, opts...)" and the rest, still naming a table because they have no declaration to read one from
  declared_query: "<Name>(ctx, params..., opts...)", naming neither, per requirement:dynamo-typed-queries
  where_resolution_happens: inside the runtime entry, so generated code passes its declared table name and holds no resolver call
errors_not_panics:
  rule: a missing client is ErrNoClient rather than a panic, so every entry stays an ordinary error-returning function
  reporting: a function returning an error returns it; an iterator yields it once with the zero value and stops, as a failed page already does
cost:
  binary: +37,812 bytes on tinygo wasip1, per requirement:dynamobind-verification
  cause: context.WithValue and the type assertion that reads it back, which is inherent to the pattern rather than to this implementation
  accepted: the call-site property was the requirement; a size-critical program calls the driver directly with the generated methods and links none of this
no_framework_resolver:
  what: no generation option selects a resolver, unlike decision:sql-context-executor-api
  why: resolution moved into the runtime entries, so there is no generated call site to redirect; a framework installs its own client and prefix with WithClient instead
  later: a per-deployment name mapping stays possible as a ClientOption, which is additive
  revisited: 2026-08-05 by requirement:framework-context-bundle, which proposes DynamoTableResolver
  what_survives: the observation, which is exact for the item entries and bounds the option to generated call sites
  what_changed: the declared queries of requirement:dynamo-typed-queries are generated call sites, so "there is no generated call site to redirect" was true of the runtime and never of them; the entries the option cannot reach are covered by requirement:dynamo-parameter-api instead
related:
  - requirement:dynamo-typed-queries
  - api:dynamobind-operations
  - decision:sql-context-executor-api
  - rule:dynamobind-driver-passthrough
```
