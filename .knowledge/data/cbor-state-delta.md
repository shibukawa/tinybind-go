---
id: data:cbor-state-delta
type: data
title: CBOR State Delta
---
A presence mask followed by the changed values, applied recursively, so a change one field deep and a change a thousand entities wide are the same shape.

```yaml
status: implemented 2026-08-19
source:
  - maintainer ask 2026-08-19, that the delta be a CBOR document of its own, the way JSON Patch is a JSON document
  - maintainer question 2026-08-19, whether a world/city/house hierarchy can be expressed at all, and whether a one-field change stays small
  - the downstream framework's state-delta message, whose content is changed entity fields, entity creation and entity deletion
struct_delta:
  form: 'an array whose head is an unsigned presence mask and whose tail is the changed values, in declaration order'
  example: '[0b0101, tick, entities] for a four-field type whose first and third fields changed'
  identity_is_not_in_the_mask: an element's identity field takes no bit and is carried by the collection that holds it, since an identity that changed is a different entity rather than a changed one
  why_a_mask_and_not_a_path: both ends have already agreed on requirement:cbor-protocol-version, so a field is named by its position; a path re-derives at every message what the handshake settled once
  why_not_a_field_number_per_value: a number per changed field costs a byte per field where a mask costs one integer for up to 64 of them
  mask_width: one CBOR unsigned integer, so a type of more than 64 encodable fields cannot carry a mask and is a generation error naming the count
collection_delta:
  form: 'the same shape one level down: a mask, then the groups it names'
  groups:
    set: 'identity and whole element, alternating; covers both an entity that appeared and one replaced outright'
    removed: identity alone
    patched: 'identity and a struct delta, alternating'
  why_set_covers_replacement: an added entity and a wholly replaced one carry the same payload, so a fourth group would be a distinction only the sender cares about
  a_swap_is_two_groups: an entity leaving and another arriving is removed plus set; the same identity arriving with new contents is set alone
  flat_not_paired: 'identity and payload alternate in one array rather than sitting in a two-element array each, which saves a byte per element and, more importantly, a nesting level per hierarchy level'
  empty_groups_are_absent: a mask bit is what says a group is present, so the ordinary tick -- entities changed, none created or destroyed -- carries one group and no empty arrays
  requires: rule:cbor-entity-identity, without which a collection is replaced whole as one value under one bit
hierarchy:
  question_asked: whether world, city and house compose
  answer: yes, and by construction rather than by a special case; a collection delta holds struct deltas and a struct delta holds collection deltas, so depth is recursion rather than a feature
  worked_example:
    change: one int32 field of one house, in one city, in a world of many
    bytes: 'WorldDelta [2, CitiesDelta] where CitiesDelta is [4, cityID, CityDelta], CityDelta is [2, HousesDelta], HousesDelta is [4, houseID, HouseDelta], HouseDelta is [2, value]'
    size: 17 bytes measured 2026-08-19, of which 3 are the value and 14 are the mask and identity chain that addresses it
    bytes: 82028204820182028204820b82013903e7
    what_the_14_bytes_buy: the same message with a hundred houses changing two fields each costs about 1010 bytes, where a path per changed field would cost about 1800
  cost_is_bounded_by_depth_not_by_width: each hierarchy level adds a mask, an identity and two array heads, whatever the collection holds
the_path_form_was_considered_and_refused:
  what_it_is: 'one entry per changed leaf, each carrying the identity chain reaching it: [[cityID, houseID, fieldNumber, value], ...]'
  where_it_wins: the single deep change, at about 9 bytes against 18
  where_it_loses: every entity changing more than one field, since the chain repeats per field rather than per entity; measured at roughly 1.8 times larger for a hundred entities changing two fields each
  which_regime_this_is_for: a world synchronized per tick, where many entities move a little, so the dense case is the steady state and the sparse one is the exception
  and_its_one_advantage_is_gone: a path form nests a fixed three levels whatever the hierarchy depth, which mattered while a profile limit was in the way and stopped mattering with decision:cbor-delta-nesting-limit
what_is_not_in_it:
  - the sequence number, the baseline version id and the target tick, which are the framework's message header rather than the delta body
  - which baseline it was computed against; the delta says what changed and the framework says from what
profile_independent:
  what: the same shape is legal under both profiles, being arrays and integers with no map and no text key
  therefore: the profile is chosen at the call site by size and schema stability, and requirement:cbor-state-delta-generation emits one delta type rather than two
  and_no_depth_asymmetry: decision:cbor-delta-nesting-limit raises the nesting limit past anything a hierarchy reaches, so the profile bounds the delta's size and not its shape
an_unknown_bit_is_skippable:
  why_it_works: a CBOR item is self-delimiting, so a reader meeting a set bit it does not know skips exactly one item and stays aligned
  consequence: appending a field to a type takes the next bit and an older reader still parses the delta, which is the same schema tolerance requirement:cbor-world-codec provides through Reader.Skip
  limit: removing or reordering a field moves every bit after it, which is a protocol change and must move the version
naming:
  state_delta: this, world state between two ticks
  boundary_delta: rule:delta-consistency-model, an HTML live boundary, unrelated
  never_abbreviated_to_delta: not in one sentence with the other
related:
  - requirement:cbor-state-delta-generation
  - rule:cbor-entity-identity
  - decision:cbor-delta-nesting-limit
  - requirement:cbor-delta-codec
  - requirement:cbor-world-codec
  - requirement:cbor-protocol-version
  - rule:delta-consistency-model
```
