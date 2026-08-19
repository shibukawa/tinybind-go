---
id: requirement:cbor-state-delta-generation
type: requirement
title: CBOR State Delta Generation
---
Emit a diff and an apply over the world-state struct set, so a game declares struct types and never hand-writes the code that finds what changed.

```yaml
priority: should
status: implemented 2026-08-19
as_built:
  declaration: cborbind GenerateWireDelta and GenerateWorldDelta, each implying the codec for its profile in both directions since the set group carries whole entities
  emitted: generator/cborbind_delta.go for the type and the diff, cborbind_delta_apply.go for the apply and the delta codec, cborbind_delta_list.go for an identified collection
  feature: FeatureCBORDelta turns the delta off and leaves the codecs standing
  helpers_are_name_ordered: the equality, lookup and sort functions are keyed by name and written sorted, so the file does not depend on the order the walk met them in
  tests: generator/cborbind_test.go over a three-level hierarchy, arrival and departure, a mismatched baseline, allocation in the steady state, a collection carried whole, and the identity declaration errors
  tinygo_verified: 2026-08-19, testdata/cmd/tinygo-cbor-smoke carries a two-level identified hierarchy; tinygo 0.41.1 runs it on the host and on wasip1 and builds it for js/wasm, and the protocol digest is the same there as under host Go
  what_that_covers: the generated insertion sort, the clear over the scratch index, and the delta codec all link under a second compiler
the_diff_imposes_no_ordering_on_the_game:
  how: an entity is found through a scratch index map the delta retains, so a collection held in spawn order diffs the same as one held sorted
  determinism_is_unaffected: every loop walks a slice; the map is only ever looked up in, never ranged, so rule:cbor-deterministic-types still holds
  cost: one map per collection per delta, reused across ticks
apply_order_is_removals_then_patches_then_arrivals:
  why: a patch names an entity the baseline held, and applying it after an arrival of the same identity would patch the wrong value
  and_then_sorted: identity order, so a receiver fed deltas and a sender holding the same entities encode to the same bytes
  sort_is_generated: an insertion sort, because a collection is nearly sorted after a tick that changed a few entities, and because it allocates nothing and needs no import on a wasm target
the_steady_state_was_nearly_lost:
  found: 2026-08-19, by the allocation test, at 5 allocations per tick
  what: the patched group appended a fresh delta per changed entity, and the collection delta nested inside it carried the scratch index, so every tick allocated a new map per changed entity
  fix: the patch slot is grown into rather than appended to, so the nested delta -- and the index inside it -- is the one the last tick used
  worth_keeping: the shape looked allocation-free and was not, and only AllocsPerRun over a reused delta said so
not_built:
  patch_or_replace_heuristic:
    what: an entity that exists in both is always patched, never sent whole
    why_that_is_safe: the receiver handles both groups whatever it is sent, so the choice needs no protocol agreement and adding it later changes no version
    when_it_will_matter: an entity whose every field changed costs a mask plus every value plus the framing, where the whole entity costs the values alone
  lookup_is_linear: apply finds an entity by scanning, so a delta touching many entities in a large collection is quadratic; a tick changing a few is not
source:
  - downstream game framework CBOR requirements 2026-08-19, its section 8
  - maintainer ask 2026-08-19, raising it from a Phase 3a placeholder to a wanted feature; world synchronization is what it is for
why_a_generator_can_do_this_well:
  structs_are_the_whole_input: a field either changed or did not, and the framework holding world state as plain Go structs means every world type is walkable at generation
  the_traversal_already_exists: requirement:cbor-wire-codec walks the same reachable type set, so the diff is a second emission over one analysis rather than a second analysis
  the_one_hard_part: collections, where rule:cbor-entity-identity is what makes an element diffable at all
emitted_per_type:
  delta_type: 'a named struct, WorldStateDelta for WorldState, per requirement:cbor-delta-codec'
  diff: 'DiffWorldState(baseline, current WorldState) WorldStateDelta'
  diff_into: 'DiffWorldStateInto(dst *WorldStateDelta, baseline, current WorldState), for a caller reusing the delta across ticks'
  apply: 'ApplyWorldStateDelta(v *WorldState, d WorldStateDelta) error'
  why_apply_returns_an_error: a delta naming an identity the baseline does not hold is a receiver whose baseline is not the one the sender diffed against, which is recoverable only by saying so
comparison:
  scalar: Go equality on the field
  nested_struct: recursion, and the parent's bit is set when the child's mask is non-zero
  self_encoding_type: Go equality if the type is comparable, and otherwise a generation error naming the type and asking for a comparable one
  why_not_compare_encoded_bytes: encoding both sides to compare them is the cost of a full snapshot per tick, which is what a delta exists to avoid
  identified_collection: set, removed and patched by identity, per rule:cbor-entity-identity and data:cbor-state-delta
  unidentified_collection: replaced whole under one bit
patch_or_replace_is_the_sender_s_alone:
  the_choice: an element that changed may be sent as a struct delta or as a whole replacement, and both are legal in the same message
  why_it_needs_no_agreement: the receiver handles both groups whatever it is sent, so which one the sender picks is a local size optimization rather than part of the protocol; two senders may differ and both be correct
  which_matters_because: an entity whose every field changed costs a mask plus every value plus the framing at each level, where the whole element costs the values alone
  the_heuristic: send whole when more than half the element's encodable fields changed, or when the element holds a collection that was itself replaced
  stated_rather_than_tuned: it is written down so a size regression can be read off the rule rather than measured, and a later change to it is not a protocol change
apply_is_exact:
  property: applying the diff of two states to the first yields the second
  the_form_that_is_actually_checked: re-encoding the result gives the bytes the second state encodes to
  why_not_the_Go_value: the wire carries no difference between a nil slice and an empty one, so a collection emptied by a delta comes back as an empty slice where the sender held nil; the bytes agree and reflect.DeepEqual does not
  therefore: the bytes are the contract, which is also what a replay comparing digests needs
steady_state_is_allocation_free:
  diff: through the Into form, with the delta and its collection slices reused across ticks
  apply: reuses the target's existing slice capacity, as requirement:cbor-composite-field-kinds already does for a decode
  the_exception: a delta whose added-entity list grows beyond the retained capacity allocates once, which is a join rather than a tick
determinism:
  traversal: declaration order, then identity order inside a collection
  falls_out_of: rule:cbor-deterministic-types, as long as no Go map is ever ranged
  two_senders_agree: the same pair of states produces byte-identical deltas anywhere, which is what makes a delta comparable in a replay
protocol_version:
  covered_by: requirement:cbor-protocol-version, whose schema description must gain the identity field and the mask width
  why: both are wire-observable; a type whose identity field moves produces deltas an old receiver keys wrongly
boundary_with_the_framework:
  this_module: the delta type, its codec, the diff and the apply
  the_framework: retaining baselines, choosing one, the message header, ack tracking, and filtering a delta by what an agent may see
  why_the_split: retention cost and visibility are session policy, and a generator that decided them would be deciding the game
acceptance:
  - a three-level hierarchy diffs and applies, with one field changed at the deepest level and with entities added and removed at each
  - a hierarchy deep enough to have exceeded the old profile limit generates and round trips, per decision:cbor-delta-nesting-limit
  - an element sent whole and the same element sent as a patch apply to the same result
  - a diff of two world states names every changed field, every created entity and every deleted one
  - applying that diff to the baseline reproduces the current state exactly, and re-encodes to the same bytes
  - two runs over the same pair of states produce byte-identical deltas
  - a delta encodes under either profile, chosen by the caller
  - a collection with no declared identity is replaced whole, and the run reports that it was
  - a delta naming an identity the baseline does not hold is refused rather than applied
  - the steady state of diff and apply allocates nothing with a reused delta
related:
  - data:cbor-state-delta
  - rule:cbor-entity-identity
  - requirement:cbor-delta-codec
  - requirement:cbor-world-codec
  - requirement:cbor-wire-codec
  - requirement:cbor-composite-field-kinds
  - rule:cbor-deterministic-types
  - requirement:cbor-protocol-version
open_questions:
  - whether a delta of a delta is ever wanted, for a client that missed one message and holds the one before it; the framework's baseline policy currently answers that with a resend
```
