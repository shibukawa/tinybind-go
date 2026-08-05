---
id: requirement:firestore-mutation-sizing
type: requirement
title: Size A Mutation With The Driver's Own Measure
---
Batch writes chunk against a local 512-byte constant that the driver's MutationSize now measures exactly; the constant double-counts, refuses entities the service would accept, and is the number rule:firestorebind-driver-passthrough exists to forbid.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05, in firestorebind/batch.go
source: decision:firestore-framework-requests
where: firestorebind/batch.go, in mutateAll and RemoveAll
before:
  constant: "const mutationOverhead = 512"
  stated_reason: the partitionId a Client adds to every key, and the JSON wrapping one mutation in a commit request
  write_path: "json.Marshal(entity) in mutateAll, then len(encoded) + mutationOverhead"
  delete_path: "len(key.String()) + mutationOverhead in RemoveAll"
  usage_of_the_driver_measure: none; MutationSize appeared nowhere in firestorebind
replacement:
  call: "size, err := client.MutationSize(mutation)"
  what_it_returns: the encoded mutation's length, including the key with its project, database and namespace attached
  why_a_client_method: only the client knows the partition, so an Entity-level figure would understate every mutation by exactly the part the caller cannot see
  available_since: tinygodriver v1.1.6, added at this module's own request
  ordering_change: the mutation must be built before it is sized, where today the entity is sized and the mutation built after; clientFor already runs first, so the client is in hand
  marshal_dropped: mutateAll no longer marshals the entity itself, which is what the driver's godoc names as the reason the method exists
three_reasons:
  the_premise_is_no_longer_true:
    was: sizing cannot see the partition, so allow for it
    now: MutationSize counts the partition, so the 512 bytes are added to a figure that already includes them
  it_contradicts_the_module_rule:
    rule: rule:firestorebind-driver-passthrough, which says the driver exports its limits and no literal appears here
    reading: a fudge factor is not a service limit, but it is a number that has to track the driver's encoding and lives one module away from it, so it is inside the spirit of the rule
  it_has_an_observable_effect:
    false_refusal: an entity within 512 bytes of datastore.MaxRequestBytes is refused locally with "one entity is larger than a request" when it is not
    over_conservative_chunking: up to 512 bytes per mutation of unused budget, which costs an extra commit on a large batch of small entities
    error_text: the refusal message becomes true once the measure is exact, rather than being true of the padded figure only
the_envelope_is_the_one_thing_left:
  fact: MutationSize measures one mutation; a commit request also carries databaseId, mode, an optional transaction handle, the mutations array and one separator per element
  size: small and bounded, but not zero, so summing mutation sizes to exactly datastore.MaxRequestBytes can overshoot by the envelope
  it_was_hidden_before: the 512 per mutation covered it many times over, which is why the problem had not been met
  measured: 2026-08-05, against a stub server; the wrapper is 42 bytes plus one per mutation on a default client, and 75 plus one with an eighteen-character database name
  position: if a margin is wanted it should be the driver's to own, for the reason the limits themselves are
  action: sent upstream as a round-five ask, per system:tinygodriver-firestore round_five_pending
as_built:
  divergence_from_the_position_above: this requirement first said a replacement constant here would be the same mistake with a smaller number, and one shipped anyway; that sentence weighed the wrong thing and this clause is the correction
  what_shipped:
    per_mutation: "datastore.Client.MutationSize plus mutationSeparator = 1, the comma between two array elements"
    per_commit: "commitEnvelopeReserve = 4096, subtracted from datastore.MaxRequestBytes once by commitBudget"
  why_the_separator_is_not_the_same_mistake: it is a property of JSON rather than of the service, so nothing upstream can change it and there is nothing to drift against
  why_the_reserve_is_not_either:
    per_commit_not_per_mutation: the fault in 512 was that it scaled with the batch, so a thousand small entities lost half a megabyte of budget; a fixed reserve costs 4 KiB of a 10 MiB request whatever the batch size
    it_is_held_back_not_added: it never inflates a single mutation, so it cannot refuse an entity the service would accept, which was the observable bug
    it_is_declared_provisional: the constant's comment carries the measurement and says it goes away when the driver names the figure
  what_the_alternative_was: chunk on the summed measure with no reserve at all, which is tight but can overshoot by 42 + n bytes; that trades a guaranteed-safe chunker for one that fails on a batch landing within a kilobyte of the limit, and the failure is a rejected commit
  reading: the rule is about numbers that have to track something this package does not own. One of these two does not, and the other is 4 KiB of slack that is honest about being slack
  where_the_reserve_must_not_apply:
    what: the single-entity refusal, which asks whether one entity can ever be sent
    caught: on review, after a first cut measured it against the reduced budget
    why_that_was_wrong: it refused an entity within 4 KiB of the limit that the service would accept, which is the observable bug of the old constant reproduced at eight times the size
    now: the refusal is against datastore.MaxRequestBytes and only the chunk boundary is against the budget; reserving for a request wrapper is a batching concern and not an entity's
error_handling:
  MutationSize_returns_an_error: it encodes the mutation, so it fails on what the client would refuse to send
  mapping: the existing ValueError path, which already reports a sizing failure; the message names the encoding failure rather than the size
acceptance:
  - mutationOverhead is gone, and no per-mutation padding remains
  - an entity just under datastore.MaxRequestBytes is accepted rather than refused
  - a batch of small entities commits in fewer requests than it did
  - a delete batch sizes through the same call as a write batch, so the two paths cannot drift
  - every constant left in firestorebind/ is either a JSON fact or a declared provisional reserve, and the comment says which
related:
  - rule:firestorebind-driver-passthrough
  - api:firestorebind-operations
  - system:tinygodriver-firestore
  - decision:firestore-framework-requests
  - requirement:firestore-key-batch-delete
```
