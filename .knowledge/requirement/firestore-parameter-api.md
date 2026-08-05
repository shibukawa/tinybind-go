---
id: requirement:firestore-parameter-api
type: requirement
title: A Firestore Call That Takes Its Client
---
Give firestorebind the same Handle and the same parameter-taking twins requirement:dynamo-parameter-api gives dynamobind, carrying the namespace rather than a table naming, because that is the only Context fact this package resolves.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05
built:
  handle: firestorebind/context.go, with NewHandle, WithHandle, HandleFromContext, KeyForOn and KeysForOn
  entries: the On twins in firestorebind/entity.go, query.go, batch.go and tx.go
  option: generator/options.go, FirestoreParameterAPI, and -firestore-parameter-api on the CLI
  emitter: generator/firestorequery_emit.go
  tests: internal/firestorefixture/handle_test.go, against the same fake the Context form is tested on, and generator/firestorequery_modes_test.go
source: user request 2026-08-05
decided_by: decision:nosql-client-supply-modes
default: unchanged; a run setting no option generates the Context form of decision:firestore-context-client-api
handle:
  what: the exported form of the clientEntry WithClient already builds in firestorebind/context.go
  shape: "type Handle struct" carrying the client and an optional NamespaceResolver, opaque rather than a literal
  constructor: "func NewHandle(c *datastore.Client, options ...ClientOption) Handle", reusing WithNamespace unchanged
  zero_value: ErrNoClient at the operation, per errors_not_panics of decision:firestore-context-client-api
  ctx_still_first: kept for cancellation, and the NamespaceResolver still takes a Context because a per-request tenant is read from one even when the client is not
what_differs_from_dynamodb:
  no_table_argument: an item entry names no kind, since the key carries it, per decision:firestore-key-identity
  namespace_instead: the Handle carries what WithTableNames carries there, and it is about tenancy rather than naming, per what_is_absent_that_dynamodb_needed of decision:firestore-context-client-api
  key_stamping: KeyFor and KeysFor of requirement:firestore-namespace-stamping need Handle-taking twins too, or the escape hatch loses its placement in parameter mode; that is the one entry outside api:firestorebind-operations this requirement touches
  transactions:
    entry_points: Run and RunReadOnly resolve a client, so they gain RunOn and RunReadOnlyOn
    inside: LoadTx, LoadAllTx, QueryPageTx, QueryKeysPageTx and CountTx take a *Tx that already carries the client and the tenancy, so they look nothing up and gain no twin
    generated: the transactional twin of a declaration is unchanged in every mode, for the same reason
runtime_entries:
  rule: one parameter-taking twin per entry of api:firestorebind-operations, with the Handle after ctx
  suffix: "On", matching requirement:dynamo-parameter-api
  item: "LoadOn[T, PT](ctx, h Handle, key datastore.Key) (T, error)", and the same shift for the rest
  keys: KeyForOn and KeysForOn, which return the key unchanged for a zero Handle exactly as the Context forms do for a Context carrying no client
  direction: these hold the implementation and the Context entries delegate to them
generation_option:
  name: FirestoreParameterAPI
  type: bool
  behavior: a declared query of requirement:firestore-typed-queries generates as "<Name>(ctx, h Handle, params..., opts...)"
  name_unchanged: the declared name, in either mode
  scope: package-wide and fixed at generation time
  no_both_surfaces_mode: as in requirement:dynamo-parameter-api, and for the same reason
verification:
  size: no counterpart to requirement:dynamobind-verification exists yet for this package, so parameter mode is the occasion to take the first measurement rather than to keep inheriting an expectation
  golden: one generated declared query per mode, compared byte for byte
  equivalence: the same declaration in both modes issues the same request, and a key stamped through KeyForOn lands where KeyFor lands
acceptance:
  - a run setting no option generates output identical byte for byte to today's
  - NewHandle and WithClient produce the same behaviour for the same client and options
  - a zero Handle returns ErrNoClient rather than panicking, in every entry including the iterators
  - a Handle built with WithNamespace stamps a key with no Context carrying a client
  - a key that already names a namespace is returned unchanged by KeyForOn, as by KeyFor
  - FirestoreParameterAPI changes the generated signature and not the generated name
related:
  - decision:nosql-client-supply-modes
  - decision:firestore-context-client-api
  - api:firestorebind-operations
  - requirement:firestore-typed-queries
  - requirement:firestore-namespace-stamping
  - requirement:dynamo-parameter-api
  - decision:firestore-key-identity
  - decision:firestore-transaction-scope
```
