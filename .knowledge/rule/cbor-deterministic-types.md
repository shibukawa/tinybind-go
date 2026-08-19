---
id: rule:cbor-deterministic-types
type: rule
title: CBOR Determinism Rejections
---
Refuse at generation every type whose encoding or traversal could differ between two runs of the same input, so a float leak cannot reach production as a desync.

```yaml
status: implemented 2026-08-19
priority: must
as_built:
  where: generator/cborbind_types.go resolve, which refuses rather than skips, so reachability from a declared root is the checked set with no annotation naming it
  a_sixth_rejection_was_added: a platform-width int or uint, which the downstream requirements did not list; it is 64 bits on a host target and 32 on wasm, so the range a field accepts would depend on where the binary ran
  also_refused: an anonymous struct, which has no name to generate a codec for, and a type reaching itself, which has no fixed shape
  every_diagnostic_names_the_type_and_the_field: verified by test, including that the float message names the underlying kind rather than only the declared one
  tests: generator/cborbind_test.go TestNondeterministicTypesAreRefused, one case per class
source:
  - downstream game framework CBOR requirements 2026-08-19, its section 6
  - the framework's codegen-rejects-nondeterministic-types rule, which this implements
scope:
  what: the type set transitively reachable from a world-state root
  how_that_set_is_named: reachability, not an annotation; the framework's decision to hold world state as ordinary Go structs is what makes reachability the definition
  also: any type reached by a wire or world codec declaration, per requirement:declared-cbor-codec
rejections:
  float32_and_float64:
    why: platform variance and fused multiply-add
    behind_a_named_type: rule:named-type-field-kind admits a defined type over float64 today, and the named-scalar suite has one; under a CBOR profile it must fail
    diagnostic_must_name_the_underlying_kind: the declared kind looks innocent, so a message naming only the declared name sends the author looking in the wrong place
  map:
    why: Go randomizes iteration order, so traversal and diff output vary per run
    escape: an ordered slice, or generated traversal in sorted key order
    consequence: the map output of requirement:cbor-world-codec comes from generated ordered traversal, never from ranging a Go map
    and_a_collection_is_the_same_answer: rule:cbor-entity-identity orders an entity collection by a declared identity rather than by anything the container arranged, which is the ordered-slice half of this escape
  interface_and_pointer_to_a_shared_value:
    why: identity and aliasing are not reproducible from a snapshot
  time_Time_and_wall_clock_derived_values:
    why: not a function of the tick
dropped_rejection:
  was: a fixed-point field with no declared scale, the fifth class the downstream requirements named
  why_it_is_gone: decision:cbor-scale-lives-in-the-type puts the scale in the type rather than in a field tag, so there is no undeclared state left to reject; two scales are two types and each carries its own methods
diagnostic:
  form: a generation error naming the type and the field
  never: a runtime warning
not_disableable:
  what: this check is not a lint a project can leave failing
  and_not_a_feature: rule:generator-feature-disable lets features be turned off, and this must not be one of them for a type reached by a wire or world codec
  open: whether that rule may disable the CBOR mode as a whole, which is a different question from disabling the check inside it
why_build_time:
  the_failure_it_replaces: a desync observed in production, whose cause is one float on one field of one type, found by comparing two replays
  cost_of_the_gate: a generation error the author sees the first time they add the field
related:
  - requirement:cbor-wire-codec
  - requirement:cbor-world-codec
  - decision:cbor-scale-lives-in-the-type
  - requirement:cbor-state-delta-generation
  - rule:named-type-field-kind
  - rule:generator-feature-disable
```
