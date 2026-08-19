---
id: rule:cbor-entity-identity
type: rule
title: An Identified Collection Element
---
A slice element is diffable only when it declares which field identifies it, because without one nothing distinguishes a changed element from a replaced one.

```yaml
status: implemented 2026-08-19
as_built:
  tag: the identity option of a cbor field tag, parsed beside the key option
  admitted: an integer or a string, refused otherwise because an identity is compared and sorted
  one_per_type: a second is a generation error naming both
  the_identity_takes_no_mask_bit: it is carried by the collection rather than by the delta of the element, since an identity that changed is a different entity
  carried_whole_is_reported: the generated file lists every collection it could not diff, by field and element type
priority: must, for a collection that is to be diffed rather than replaced
source:
  - maintainer observation 2026-08-19, that array elements are the one place a struct diff needs an entity identifier
the_problem:
  what: two slices of the same length hold different values at index 3
  ambiguous: whether entity 3 changed, or entity 3 was removed and another inserted before it, or the slice was reordered
  why_it_matters: the first is a few bytes and the other two are a resend, and a diff that guesses wrong produces a state the receiver cannot correct
  index_is_not_identity: a position is a property of the container, and an entity that survives a removal earlier in the slice keeps its identity and loses its index
declaration:
  form: 'a cbor tag option on the identifying field, spelled `cbor:"id,identity"`'
  one_per_element_type: a second identity field is a generation error naming both
  admissible_types: an integer or a text string; the identity is compared and sorted, so it must be a scalar with a total order
  must_be_encoded: an identity field carrying `cbor:"-"` is a generation error, since the receiver cannot key what it never received
  inherited_by_every_collection_of_that_type: the identity is the element type's, not the field's, so one declaration serves every slice holding it
without_an_identity:
  behavior: the collection is replaced whole, as one value under one mask bit
  not_an_error: a short fixed slice is cheaper to resend than to diff, and refusing it would make a delta impossible for types that do not need one
  the_cost_is_silent: a large entity collection with no identity produces deltas the size of a snapshot, which looks like the feature working
  therefore: the generator reports the collections it replaced whole, so the size is a choice rather than a discovery
order_is_not_state:
  rule: for an identified collection, slice order is not part of the state; a generated apply produces elements in identity order
  why: an id-keyed delta cannot express a reordering, and encoding one would put the container's arrangement back into a message about entities
  consequence: a game that needs a meaningful order carries it in a field, not in the slice arrangement
  and_the_encoder_agrees: requirement:cbor-world-codec emits an identified collection in identity order too, so a snapshot and a delta-applied state encode to the same bytes; this is what makes the round trip checkable
  cost_accepted: a game holding entities in spawn order sees them re-ordered by a decode, which is a real behavior change and is why the rule is stated rather than assumed
determinism:
  sorted_not_ranged: identity order is a sort over a slice, never a range over a Go map, so rule:cbor-deterministic-types is unaffected
  same_on_both_sides: the sort is over the encoded identity, as requirement:cbor-world-codec orders its map keys, so two implementations agree without agreeing on a locale or a collation
related:
  - data:cbor-state-delta
  - requirement:cbor-state-delta-generation
  - requirement:cbor-world-codec
  - rule:cbor-deterministic-types
  - requirement:cbor-protocol-version
```
