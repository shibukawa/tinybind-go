---
id: decision:cbor-scale-lives-in-the-type
type: decision
title: A Fixed Point Scale Is Carried By Its Type
---
Two scales are two types, each with its own AppendCBORTo, so the generator needs no scale tag, no conversion arithmetic and no fixed-point dependency at all.

```yaml
status: accepted 2026-08-19, by the maintainer; built the same day
as_built:
  nothing_was_needed: the generator has no scale option, no conversion, and no fixed-point import, which is what this decision predicted
  the_mechanism_is_the_interface: a scaled field resolves to CborSelf and is encoded through the type's own AppendCBORTo, so rule:cbor-codec-interface-upstream carries it with nothing added
  verified: the reference codec in generator/cborbind_test.go and testdata/cmd/tinygo-cbor-smoke both hold a Fixed1024 declared exactly this way, and both produce the driver's pinned bytes
  a_second_scale_costs: one defined type and two short methods
supersedes: the scale-tag proposal of the downstream requirements section 5, which this closes rather than defers
the_problem_it_answers:
  stated: the framework puts the scale in the schema per field, but cbor.Appender is a method on a type
  therefore_assumed: a position at 1/1024 and a velocity at 1/65536 are both the same fixed-point type, so one method cannot serve both, and the generator would have to emit the conversion at the field site
  what_that_would_have_cost: a cbor tag option, its parsing and its errors; a rule making a missing scale a generation error; a fifth determinism rejection class; and an emitted conversion the generator would have to prove float-free
the_decision:
  what: declare a distinct defined type per scale and give each its own AppendCBORTo and DecodeCBORFrom
  shape: 'type PosF64 fixed64 and type VelF64 fixed64, each with the two methods, each converting at whatever scale it means'
  consequence: the scale is no longer per field; it is per type, which is where the interface already puts it
  therefore: rule:cbor-codec-interface-upstream covers fixed-point fields with nothing added, and requirement:cbor-composite-field-kinds is what carries them inside slices
what_this_removes:
  - the cbor scale tag option, and the tag-option machinery it would have needed
  - a generation error for a fixed-point field with no declared scale
  - the fifth rejection class of rule:cbor-deterministic-types
  - any emitted conversion arithmetic, and with it the requirement that it be integer-only; the conversion is the author's method, on the author's type
  - any question of which fixed-point library is in use; cborbind names none and imports none
what_it_keeps:
  scale_is_still_protocol: a changed scale is a changed method on a changed type, and requirement:cbor-protocol-version must still move when the bytes move
  the_type_name_is_the_evidence: two fields of one declared type carry one scale by construction, which is a stronger guarantee than a tag the generator reads and the author maintains
  no_float_on_the_wire: the driver's profiles refuse floats outright, and rule:cbor-deterministic-types refuses a float field, so a conversion written with a float multiply cannot reach the wire as one even though this module no longer writes the conversion
the_cost_accepted:
  what: a game declaring many scales declares many types, each with two short methods
  why_that_is_right: those methods are where the scale is documented and tested, and a tag would have put the same fact somewhere the type does not see it
  boilerplate_is_not_this_module_s_to_remove: generating the per-scale types is a separate idea, and nothing needs it yet
related:
  - rule:cbor-codec-interface-upstream
  - requirement:cbor-composite-field-kinds
  - rule:cbor-deterministic-types
  - requirement:cbor-protocol-version
  - requirement:cbor-wire-codec
  - decision:cborbind-runtime-package
```
