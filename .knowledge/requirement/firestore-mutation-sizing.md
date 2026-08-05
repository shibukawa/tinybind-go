---
id: requirement:firestore-mutation-sizing
type: requirement
title: Size A Mutation With The Driver's Own Measure
---
Batch writes size every part of a commit through the driver: MutationSize for each mutation and CommitOverheadBytes for the request around them, so no local byte figure remains for rule:firestorebind-driver-passthrough to forbid.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05, in firestorebind/batch.go
completed: 2026-08-06, when the envelope stopped being local too; see as_built.now_that_the_driver_names_the_envelope
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
the_envelope_was_the_one_thing_left:
  fact: MutationSize measures one mutation; a commit request also carries databaseId, mode, an optional transaction handle, the mutations array and one separator per element
  size: small and bounded, but not zero, so summing mutation sizes to exactly datastore.MaxRequestBytes can overshoot by the envelope
  it_was_hidden_before: the 512 per mutation covered it many times over, which is why the problem had not been met
  measured: 2026-08-05, against a stub server; the wrapper is 42 bytes plus one per mutation on a default client, and 75 plus one with an eighteen-character database name
  position: if a margin is wanted it should be the driver's to own, for the reason the limits themselves are
  action: sent upstream as a round-five ask, per system:tinygodriver-firestore round_five
  upstream_answer: Client.CommitOverheadBytes and Tx.CommitOverheadBytes, in v1.1.9. The position held: it is the driver's, and it is measured rather than estimated
as_built:
  now_that_the_driver_names_the_envelope:
    per_mutation: "datastore.Client.MutationSize, unpadded"
    per_commit: "datastore.Client.CommitOverheadBytes, passed to chunksByCommitSize as the overhead function; firestorebind holds no envelope figure at all"
    single_entity_refusal: "CommitOverheadBytes(1) + size against datastore.MaxRequestBytes, which is exactly the smallest request that could carry the entity"
    why_the_signature_takes_a_count: chunking stays a running total. Asking whether one more fits is that one's MutationSize plus the overhead re-read for n+1, rather than re-measuring the batch each step
    the_client_method_not_the_Tx_one: both chunked paths commit outside a transaction; Tx.CommitOverheadBytes is what a transactional commit would need, and nothing here has one
  the_interim_and_why_it_was_defensible:
    what_it_was: "commitEnvelopeReserve = 4096 held back once per commit, measured against a stub server at 42 bytes on a default client and 75 with an eighteen-character database name"
    divergence_from_the_position_above: this requirement first said a replacement constant here would be the same mistake with a smaller number, and one shipped anyway. It was per-commit rather than per-mutation, which is where 512 actually went wrong: that one scaled with the batch, so a thousand small entities lost half a megabyte of budget
    it_was_held_back_not_added: it never inflated a single mutation, so it could not refuse an entity the service would accept - the observable bug of the old constant
    the_separator_moved_first: it shipped as mutationSeparator = 1 added to each mutation's size, counting n commas where an array of n elements has n-1. One byte per commit, and the same shape of error as the reserve beside it: a per-request cost charged per mutation. Moving it onto the request is what made the interim shape match the driver's, so adopting the real figure was a one-line swap
    where_the_reserve_must_not_apply: the single-entity refusal, which asks whether one entity can ever be sent. A first cut measured it against the reduced budget and so refused an entity within 4 KiB of the limit that the service would accept - the old constant's bug at eight times the size. The refusal stayed against the bare limit for as long as the figure was a guess, and only became exact when it stopped being one
  reading: the rule is about numbers that have to track something this package does not own. A provisional one is survivable when it is per-request, held back rather than added, and shaped like the answer it is waiting for
error_handling:
  MutationSize_returns_an_error: it encodes the mutation, so it fails on what the client would refuse to send
  mapping: the existing ValueError path, which already reports a sizing failure; the message names the encoding failure rather than the size
acceptance:
  - mutationOverhead is gone, and no per-mutation padding remains
  - an entity just under datastore.MaxRequestBytes is accepted rather than refused
  - a batch of small entities commits in fewer requests than it did
  - a delete batch sizes through the same call as a write batch, so the two paths cannot drift
  - no size or overhead figure remains in firestorebind/; both come from the client
  - a batch larger than one request splits, every commit's body stays inside datastore.MaxRequestBytes, and no commit leaves more room than one more mutation needs. The last one is what an over-estimated envelope fails, and it fails silently otherwise since the writes still succeed; it is asserted with no allowance, which only a measured envelope can meet
tested: internal/firestorefixture TestStoreAllChunksAgainstTheRequestLimit, which reads each commit's body length from the fake rather than its mutation count, since the whole point of the envelope is that the two differ
related:
  - rule:firestorebind-driver-passthrough
  - api:firestorebind-operations
  - system:tinygodriver-firestore
  - decision:firestore-framework-requests
  - requirement:firestore-key-batch-delete
```
