---
id: requirement:firestore-key-batch-delete
type: requirement
title: Delete A Batch Of Keys
---
RemoveKeys deletes entities named by keys rather than by bound values, closing the one shape QueryKeysPage opens and nothing consumes.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05, in firestorebind/batch.go
source: decision:firestore-framework-requests, first among the four
signature: "func RemoveKeys(ctx context.Context, keys []datastore.Key, opts ...datastore.WriteOption) error"
the_gap:
  what_produces_keys: "QueryKeysPage returns KeyPage{Keys []datastore.Key, ...}, and QueryKeysPageTx the same inside a transaction"
  what_consumes_them: nothing; every batch write entry takes bound values, so RemoveAll[T Keyer] wants a T that a key alone cannot supply
  consequence: find-these-keys-then-delete-them, which is the shape of every cleanup, teardown and administrative sweep, is the one shape that has to be hand-rolled
  what_hand_rolling_costs: dropping to client.Mutate with datastore.DeleteOp and reimplementing the size chunking that sits in the same file
symmetry_is_the_argument:
  statement: QueryKeysPage exists so a caller can work in keys, and then there is nothing a caller can do with keys
  weight: stronger than the motivating use, because it holds for any caller rather than for one
nothing_new_is_needed:
  namespace: applyNamespaceAll already exists, and requirement:firestore-namespace-stamping is what makes the same stamping reachable from outside
  sizing: RemoveAll already sizes a delete mutation, and requirement:firestore-mutation-sizing replaces how
  chunking: chunksBySize already chunks by size against datastore.MaxRequestBytes
  effect: this is a fourth caller of three existing helpers, not a new mechanism
behaviour:
  incomplete_key: refused before anything is sent, as RemoveAll refuses one, since an incomplete key names no entity to delete
  missing_entity: deleting a key that holds nothing succeeds, as it does on the wire, so the result cannot say which of them existed
  not_a_transaction: chunking means a large sweep commits in pieces and a failure leaves the earlier pieces deleted; the godoc says so and points at Run, as StoreAll does
  ordering: keys are sent in the order given, and no result is returned because a delete has none
  empty_input: no request, no error
motivating_use:
  what: a test run isolating itself in its own namespace, whose teardown is a keys-only query per kind and then a batch delete
  why_it_has_to_be_this_shape: there is no API that deletes a namespace, so teardown against a real project cannot be one call
  cost_today: two firestorebind calls with this, one plus a hand-written chunker without it
  ownership: the sweep and the isolation policy are the framework's, per decision:firestore-framework-requests
open:
  a_transaction_form:
    what: RemoveKeysTx, matching how QueryKeysPageTx twins QueryKeysPage
    position: not asked for and not proposed; a transaction is bounded by datastore.MaxTransactionBytes and a sweep large enough to want a batch is too large to be atomic
    revisit: when a caller wants a bounded set of keys deleted as one unit, which the Tx write methods of decision:firestore-transaction-scope already serve one at a time
acceptance:
  - keys from a QueryKeysPage result delete without the caller naming a kind, building a mutation or chunking
  - a batch exceeding datastore.MaxRequestBytes commits in pieces and deletes every key
  - a key that already names a namespace keeps it, and one that does not receives the resolved one
  - an incomplete key is refused with a KeyError before any request is sent
  - deleting a key that holds nothing returns no error
related:
  - api:firestorebind-operations
  - decision:firestore-framework-requests
  - requirement:firestore-namespace-stamping
  - requirement:firestore-mutation-sizing
  - rule:firestorebind-driver-passthrough
  - decision:firestore-context-client-api
```
