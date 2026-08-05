---
id: requirement:framework-context-bundle
type: requirement
title: One Context Value For The Whole Framework
---
Let a framework carry every value it puts in a Context in one struct under one key, by giving dynamobind and firestorebind the resolver option decision:sql-context-executor-api already gives SQL, so tinybind reads the framework's node instead of installing a second one.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05
built:
  options: generator/options.go, DynamoHandleResolver and FirestoreHandleResolver
  check: generator/handle_resolver.go, shared by both emitters
  emitters: generator/dynamoquery_emit.go and generator/firestorequery_emit.go
  tests: generator/dynamoquery_modes_test.go and generator/firestorequery_modes_test.go, both compiling the generated package
  benchmark: dynamobind/handle_test.go, BenchmarkContextDepth
source: user request 2026-08-05
decided_by: decision:nosql-client-supply-modes
the_ask: one struct holding the values a framework manages, reused for every purpose, because a Context with many values resolves slowly
today:
  nodes_tinybind_installs: three, one each from sqlbind/context.go, dynamobind/context.go and firestorebind/context.go
  each: a private key and a private entry type, so nothing can share a node with anything else
  plus: whatever the framework adds beside them, which is the session, pool and tracer a page reaches through the request Context
  lookup: context.Value walks a linked list from the newest node outward, so a miss walks past every node installed after the one it wants
  assertions: one type assertion per node read, and requirement:dynamobind-verification measures a bare WithValue plus one assertion at 48,409 bytes on tinygo wasip1 with nothing else linked
magnitude:
  honest: the walk is a pointer chase per level, so against a DynamoDB or Datastore round trip it is invisible; the ask is right that the cost is O(depth) and wrong to expect the operation path to feel it
  where_it_is_real: a hot path doing no IO, a deep chain built per request, and the per-binary size of each additional key and assertion, which is the largest documented number in this catalog
  therefore: the requirement is written for node count and assertion count, which are bounded and measurable, rather than for a latency claim
  benchmark: resolve at depth 1, 5 and 20 with and without the bundle, reported as ns/op, so the ask stops resting on an estimate
  measured_2026_08_05:
    machine: darwin/arm64, Apple M3, go1.26.5
    depth_1: 5.588 ns/op, 0 allocs
    depth_5: 10.56 ns/op, 0 allocs
    depth_20: 23.82 ns/op, 0 allocs
    handle: 2.075 ns/op, 0 allocs, which is the parameter form doing no lookup at all
    per_level: about 1.2 ns, so the ask's O(depth) reading is exact and the absolute numbers are nanoseconds
    reading: the bundle collapses any depth to the depth-1 number, and the parameter form removes even that; neither is visible against a Datastore or DynamoDB round trip, so the case for both rests on binary size and on node count rather than on request latency
why_tinybind_cannot_own_the_struct:
  shape_that_fails: one exported struct with typed fields for the SQL executor, the DynamoDB client and the Datastore client
  what_breaks: that package imports database/sql, system:tinygodriver-dynamodb and system:tinygodriver-firestore, so a JSON-only binary links all three
  forbidden_by: decision:runtime-package-boundaries, whose forbidden list already names shared runtime code importing net/http or database/sql for every mode
  not_a_style_objection: it is the rule the package split exists to enforce, and the whole point of dependency_direction there
  rejected_workaround:
    slots: a leaf package holding "[]any" with registered indices, imported by everything and importing nothing
    against: it costs two assertions per read where there is one today, so it is slower per read and larger per binary until the framework carries many values, and it puts index coordination in a leaf no subsystem owns
    verdict: it makes tinybind own a registry to avoid owning an import, which is a worse trade than owning neither
the_answer: the framework owns the struct
  who: a framework already imports every driver it exposes, so a typed bundle costs it nothing it was not already paying
  what_tinybind_supplies: a way for its call sites to read that struct, and nothing else; no new type, no new key, no leaf package
  shape: "type Values struct { ... }" in the framework, one key, one assertion, every field reached from it
  generated_call_sites: the resolver options below
  hand_written_call_sites: parameter mode, per requirement:dynamo-parameter-api and requirement:firestore-parameter-api, where the framework reads its bundle once in middleware and passes the handle down
  composition: neither half covers the other, which is why decision:nosql-client-supply-modes decides them together
resolver_options:
  precedent: SQLExecutorResolver, an existing SymbolPattern selecting a framework function, per decision:sql-context-executor-api
  DynamoHandleResolver:
    type: optional SymbolPattern
    signature: "func(context.Context) (dynamobind.Handle, error)"
  FirestoreHandleResolver:
    type: optional SymbolPattern
    signature: "func(context.Context) (firestorebind.Handle, error)"
  nil: uses this module's own Context key, so the default output is unchanged
  precedence: the ParameterAPI of requirement:dynamo-parameter-api wins, since a signature already carrying the Handle resolves nothing; no resolver import is emitted in that mode
  import: the resolver package is imported as _tinybindresolver, the alias the SQL emitter already uses, which cannot collide with a package the declaration's own types come from
  errors: a resolver returns an error rather than panicking, and the generated body reports it in the shape the declaration returns, which for an iterator means yielding it once with the zero value
  checked_at_generation: a resolver naming no package, or a name that is not an exported identifier, fails generation rather than emitting an unbuildable file whose cause is one setting in a config the reader is not looking at
signature_correction:
  proposed_was: "func(ctx, declared string) (*dynamodb.Client, string, error)", TableFromContext's own shape
  is: the Handle-returning shape above
  why: a resolver handing back a loose client and name gives generated code two values no runtime entry accepts, so it would have needed a third entry form taking a raw client; returning the Handle feeds the entries requirement:dynamo-parameter-api already adds
  effect: one resolver contract serves both packages, it carries the table naming and the tenancy with the client rather than beside it, and the framework builds it with the same NewHandle a parameter-mode caller uses
what_the_resolver_cannot_reach:
  fact: the option redirects generated call sites, and the item entries of api:dynamobind-operations are library functions a human calls
  stated_by: no_framework_resolver in decision:dynamo-context-client-api, which read this as the reason no resolver option should exist
  now: the reason survives, and it bounds the option rather than refuting it; parameter mode is what covers the entries a generation option cannot touch
  rejected_alternative:
    global_hook: "dynamobind.SetClientResolver(fn)" at process level
    against: init-order dependent, invisible at the call site, and untestable in parallel, which contradicts "a second Context, not a second signature" being how a test gets a second client
seam_test:
  rule: decision:framework-integration-seams accepts a request that widens a seam already present and whose default output stays identical
  present: SQLExecutorResolver is the same seam, one subsystem over
  identical: a run setting none of these options emits the same bytes
  verdict: accept
acceptance:
  - a run setting no resolver option generates output identical byte for byte to today's
  - a framework carrying one struct under one key serves SQL, DynamoDB and Firestore generated calls with one lookup and one assertion per operation
  - no package of this module gains an import it does not have today
  - a resolver returning an error surfaces it as ErrNoClient does, and no generated path panics
  - the benchmark reports the depth-1, depth-5 and depth-20 numbers rather than an estimate
related:
  - decision:nosql-client-supply-modes
  - decision:sql-context-executor-api
  - decision:runtime-package-boundaries
  - decision:framework-integration-seams
  - decision:dynamo-context-client-api
  - decision:firestore-context-client-api
  - requirement:dynamo-parameter-api
  - requirement:firestore-parameter-api
  - requirement:dynamobind-verification
  - requirement:custom-framework-generation-profile
```
