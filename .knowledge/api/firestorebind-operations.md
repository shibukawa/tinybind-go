---
id: api:firestorebind-operations
type: api
title: firestorebind Operations
---
Typed wrappers over the driver entity calls: the caller passes and receives application types, and every driver result the wrapper cannot carry stays reachable.

```yaml
status: implemented
proposed: 2026-08-03
implemented: 2026-08-04, in firestorebind/
package: github.com/shibukawa/tinybind-go/firestorebind
shape_source: api:dynamobind-operations, with the arguments the key model removes
constraints:
  EntityEncoder: "interface{ EncodeEntity() datastore.Entity }"
  EntityDecoder: "interface{ DecodeEntity(datastore.Entity) error }"
  Keyer: "interface{ EntityKey() datastore.Key }"
  dispatch: by pointer constraint, as decision:dynamobind-static-dispatch established; no registry, no init entry
single:
  Load: "func Load[T any, PT interface{*T; EntityDecoder}](ctx, key datastore.Key, opts ...datastore.ReadOption) (T, error)"
  Store: "func Store[T EntityEncoder](ctx, v T, opts ...datastore.WriteOption) (datastore.Key, error)"
  version_precondition: Store and Update append datastore.WithBaseVersion when the value implements Versioner and reports a non-zero version, per decision:firestore-transaction-scope
  Insert: "func Insert[T EntityEncoder](ctx, v T, opts ...datastore.WriteOption) (datastore.Key, error)"
  Update: "func Update[T EntityEncoder](ctx, v T, opts ...datastore.WriteOption) error"
  Remove: "func Remove[T Keyer](ctx, v T, opts ...datastore.WriteOption) error"
  no_table_argument: identity is complete in the key, so none of these names a kind; contrast api:dynamobind-operations, where every entry carries a table string
  store_returns_a_key: an incomplete key is completed by the server, so the write has something to give back; Store on a complete key returns the same key, which costs the caller nothing
  three_write_verbs: Store upserts, Insert and Update carry their preconditions in the name, per decision:firestore-transaction-scope
no_returning_forms:
  fact: the wire returns no prior entity from a commit, so there is no ALL_OLD to decode
  effect: StoreReturning and RemoveReturning of api:dynamobind-operations have no counterpart
  what_a_caller_does_instead: read inside a transaction, which is the honest cost of wanting the old value
paged:
  QueryPage: "func QueryPage[T any, PT ...](ctx, q *datastore.Query, opts ...datastore.ReadOption) (Page[T], error)"
  Page: "type Page[T any] struct { Values []T; EndCursor datastore.Cursor; More datastore.MoreResults; SkippedResults int32 }"
  HasMore: "func (p Page[T]) HasMore() bool", delegating to the driver's own rule
  more_is_not_a_bool: MoreResults says why a batch ended, and the reason is kept rather than flattened, per rule:firestorebind-driver-passthrough
iterated:
  Query: "func Query[T any, PT ...](ctx, q *datastore.Query, opts ...datastore.ReadOption) iter.Seq2[T, error]"
  continuation: each batch after the first re-sends the query with Start set from the previous EndCursor; a caller-supplied Start is the starting point, not an override
  stop: an early break ends the iteration without a further request
  error: a failed batch yields one zero value with the error and ends
  cost_disclosure: the godoc states that one range can issue many requests, and that a kind-only query walks every entity of the kind
  discards: SkippedResults and the final EndCursor; a caller who needs to resume or to diagnose an offset uses QueryPage
  no_scan: a kind-only Query is what Scan would have been, so there is no second entry point
count:
  Count: "func Count(ctx, q *datastore.Query, opts ...datastore.ReadOption) (int64, error)"
  ungenericized: a count returns no entities, so there is nothing to decode and no type parameter to infer
  why_it_is_here_at_all: paging keys to count them costs a read per entity, so a wrapper that omitted it would push callers to the expensive thing
batch:
  MaxLookupKeys: not redeclared here; the driver exports it and rule:firestorebind-driver-passthrough forbids a parallel constant that could drift from it
  LoadAll: "func LoadAll[T any, PT ...](ctx, keys []datastore.Key, opts ...datastore.ReadOption) (values []T, missing []datastore.Key, deferred []datastore.Key, err error)"
  StoreAll: "func StoreAll[T EntityEncoder](ctx, vs []T, opts ...datastore.WriteOption) ([]datastore.Key, error)"
  InsertAll: "func InsertAll[T EntityEncoder](ctx, vs []T, opts ...datastore.WriteOption) ([]datastore.Key, error)"
  RemoveAll: "func RemoveAll[T Keyer](ctx, vs []T, opts ...datastore.WriteOption) error"
  not_a_transaction: chunking means a large batch commits in pieces, so a failure leaves the earlier pieces written; the godoc says so and points at Run for all-or-nothing
  three_results_from_a_lookup: found, missing and deferred are three different facts, and collapsing missing into an absent value would lose the difference between "not stored" and "not read yet"
  deferred: returned, never looped, matching how the driver treats them and what rule:firestorebind-driver-passthrough requires
  chunking:
    reads: at datastore.MaxLookupKeys, the driver's own constant since v1.1.5; the driver also checks it and answers ErrTooManyKeys, so the chunking here is what keeps a caller from meeting that error rather than what prevents a bad request
    writes: by encoded size against datastore.MaxRequestBytes, and datastore.MaxTransactionBytes inside a transaction
    why_not_by_count: there is no per-commit mutation count to chunk against; Google documents none and system:tinygodriver-firestore records the driver declining to invent one, so a count-based chunker would be a made-up number
    cost_of_sizing: StoreAll encodes before it chunks, which it does anyway, and sums the encoded lengths; that is one pass, not a second encode
    contrast: api:dynamobind-operations chunks at fixed counts of 25 and 100, because DynamoDB publishes them; the shapes differ because the services differ, not because one is newer
    no_hardcoded_numbers: every limit comes from a driver constant, so a service change is a driver bump rather than an edit here
  mixed_verbs: not offered; a caller needing insert and delete in one commit uses a transaction or the driver's Mutate directly
transactions: decision:firestore-transaction-scope
context_client:
  where: no entry takes a client; every one resolves through ClientFromContext, per decision:firestore-context-client-api
  namespace: applied by the runtime entry, so a generated key carries none
errors:
  passthrough: errors.Is against every driver sentinel and errors.As to *datastore.Error keep working through every helper
  miss: a Load matching nothing stays ErrNoSuchEntity and is never converted to a zero value
  decode: field-level; property name, expected kind, got kind
  type: "firestorebind.Error{Property, Expected, Got, Message}", built by TypeError for a wrong kind and ValueError for a value the field cannot hold
  finding_it: firestorebind.AsError walks the chain by type assertion, as jsonbind.AsError does, because errors.As needs reflection
untyped_query_escape_hatch:
  status: the query is built with the driver's own builder, so a property name is a string until requirement:firestore-typed-queries lands
  unchecked: a renamed property still compiles and returns an empty batch, which is the quiet failure that requirement makes loud
deferred:
  - a batch-level iterator yielding Page[T]
  - a typed query builder; the declaration of requirement:firestore-typed-queries is the answer, not a fluent API
  - AllocateIDs, until a caller needs a key before the write
related:
  - requirement:firestorebind-product-goals
  - system:tinygodriver-firestore
  - decision:firestorebind-runtime-package
  - decision:firestore-context-client-api
  - decision:firestore-transaction-scope
  - decision:firestore-key-identity
  - rule:firestorebind-driver-passthrough
  - requirement:firestorebind-generated-entity-codec
  - api:dynamobind-operations
```
