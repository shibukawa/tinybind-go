---
id: decision:firestore-context-client-api
type: decision
title: The Client And The Namespace Come From The Context
---
Carry the Datastore client in the Context, give no entry of firestorebind a client parameter, and let the namespace be a Context fact rather than a tag or an argument.

```yaml
status: implemented
proposed: 2026-08-03
implemented: 2026-08-04, in firestorebind/context.go
follows: decision:dynamo-context-client-api, whose call-site property is the requirement being restated
one_surface:
  rule: there is no client-taking form, no suffixed variant and no generation option
  reason: the client is a deployment fact fixed for a process, so a parameter repeats at every call site what one setup line already said
  second_client: a second Context, not a second signature, which is what a test or a second project uses
runtime:
  setter: "WithClient(ctx, *datastore.Client, ...ClientOption) context.Context"
  resolver: "ClientFromContext(ctx) (*datastore.Client, error)", which every entry of the package calls
  key: private typed key
  errors: ErrNoClient
what_is_absent_that_dynamodb_needed:
  table_resolver: none, because there is no table name to map
  reason: decision:dynamo-context-client-api needed WithTableNames since a declared table name and a deployed one differ; a kind is intrinsic to the type per decision:firestore-key-identity, and a deployment does not rename it
  effect: the ClientOption list is shorter, and the one option that remains is about tenancy rather than naming
namespace:
  where_it_belongs: the Context, alongside the client
  driver_side: "datastore.WithNamespace(string)" is a client option, so a per-process namespace needs nothing here
  per_request: "WithNamespace(func(ctx context.Context) string) ClientOption", optional, for a tenant that varies per request
  why_a_function: a multi-tenant service picks its namespace from the request, which is exactly the case the driver's process-wide option cannot serve
  why_not_a_tag: a namespace is who is asking, not what the type is; putting it on the type would make one struct unusable for a second tenant
  nil_resolver: ignored, so a mistaken nil behaves as no resolver rather than panicking
  key_interaction: a generated EntityKey carries no namespace, and the resolved one is applied by the runtime entry, so a key value stays portable
database:
  named_databases: "datastore.WithDatabase(string)" on the driver client; a second database is a second Context, as a second client is
  not_here: firestorebind adds no database option of its own, since nothing generated names one
signatures:
  item: "Load[T](ctx, key)" and the rest, naming neither a client nor a kind, since the key carries the kind and the type carries the codec
  declared_query: "<Name>(ctx, params..., opts...)", per requirement:firestore-typed-queries
  comparison: an item operation here loses the table argument its DynamoDB counterpart keeps, because identity is complete in the key
errors_not_panics:
  rule: a missing client is ErrNoClient rather than a panic, so every entry stays an ordinary error-returning function
  reporting: a function returning an error returns it; an iterator yields it once with the zero value and stops, as a failed page already does
cost:
  unmeasured: no counterpart to requirement:dynamobind-verification exists yet
  expectation: about what decision:dynamo-context-client-api measured, since it is the same context.WithValue and type assertion, and the same reasoning applies to accepting it
transactions:
  tx_is_not_in_the_context: a *datastore.Tx is passed explicitly, per decision:firestore-transaction-scope
  why: a Context-carried transaction makes a write silently transactional or not depending on which Context reached it, which is the opposite of the property this decision exists for
related:
  - api:firestorebind-operations
  - decision:dynamo-context-client-api
  - decision:firestore-key-identity
  - decision:firestore-transaction-scope
  - requirement:firestore-typed-queries
  - rule:firestorebind-driver-passthrough
```
