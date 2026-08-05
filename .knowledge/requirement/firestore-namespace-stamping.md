---
id: requirement:firestore-namespace-stamping
type: requirement
title: Stamp A Key The Way The Package Does
---
KeyFor exports the namespace application every wrapped entry performs, so a caller on the ClientFromContext escape hatch places keys identically instead of reimplementing the resolver contract or silently writing to the default namespace.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05, in firestorebind/context.go
source: decision:firestore-framework-requests
signature: "func KeyFor(ctx context.Context, key datastore.Key) datastore.Key"
plural: "func KeysFor(ctx context.Context, keys []datastore.Key) []datastore.Key"
today:
  applied_by: applyNamespace and applyNamespaceAll, both unexported
  resolved_from: the NamespaceResolver stored beside the client by WithClient, per decision:firestore-context-client-api
  escape_hatch: ClientFromContext, documented as applying no namespace, with the hazard stated
  what_a_caller_has: the client and the warning, and no way to act on the warning
the_failure:
  choice_a: reimplement the contract, which is three clauses - resolve, keep a namespace the key already names, ignore an empty result
  choice_b: forget, and write into the default namespace
  why_b_is_quiet: a multi-tenant caller gets a data-placement bug that no test in the default namespace can see, and a test-isolation caller gets a teardown that deletes nothing and reports success
  shared_property: both failures succeed
why_the_operation_rather_than_the_ingredient:
  rejected_alternative: "NamespaceFromContext(ctx) (string, bool)"
  against: it hands back the ingredient and leaves the caller to recombine it, so the clause a caller forgets is the one that keeps an explicitly placed key where it was placed
  for_KeyFor: it cannot be got subtly wrong, and it is the same function the wrapped entries call, so the two cannot drift
  both_could_ship: nothing stops the accessor existing too, but it is not what closes the failure and is not proposed
why_the_plural_too:
  fact: the escape hatch that motivates this is a sweep, and requirement:firestore-key-batch-delete is the batch shape
  allocation: applyNamespaceAll allocates only when a resolver would change something, which a caller looping over KeyFor would not
behaviour:
  no_client_in_context: the key is returned unchanged, with no error, matching what a nil resolver already does; a caller with no client has a bigger problem and will meet ErrNoClient at the operation
  no_resolver: unchanged
  empty_resolved_namespace: unchanged, so a resolver returning "" means the default namespace rather than an error
  key_already_names_one: kept, because an explicitly placed key is not silently moved
  no_error_return: every branch has an answer, and an error would push callers into handling a case that cannot occur
documentation_duty:
  where: the ClientFromContext godoc, whose warning becomes a pointer rather than a dead end
  what_it_says: keys reaching the driver through the escape hatch are sent as built, and KeyFor is how to place them as this package would
acceptance:
  - a key passed through KeyFor and then through the driver lands where the same key passed through Load or Store lands
  - a key that already names a namespace is returned unchanged
  - a Context with a client but no namespace resolver returns the key unchanged
  - a Context carrying no client at all returns the key unchanged rather than panicking
  - KeysFor over a slice allocates nothing when no resolver is installed
related:
  - decision:firestore-context-client-api
  - api:firestorebind-operations
  - requirement:firestore-key-batch-delete
  - decision:firestore-framework-requests
  - decision:firestore-key-identity
```
