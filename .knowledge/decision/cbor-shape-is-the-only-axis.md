---
id: decision:cbor-shape-is-the-only-axis
type: decision
title: A CBOR Codec Has No Profile, Only A Container Shape
---
Drop the profile entirely: the container shape is the one thing a codec must be told, it rides in the entry point name, and every other restriction the word profile carried is either a capability of the emitter or a fact the struct already states.

```yaml
status: proposed 2026-08-22
review_gate: proposed
asked: the maintainer wants cborbind back with the application data kinds Wire and World gone, and asks where the profile is named; the answer is that there is nothing left to name
the_word_carried_four_things:
  container_shape: an array in declaration order versus a text-keyed map; the one axis that survives
  evolution_policy: not an axis at all, a consequence of the shape
  format_restriction: refusing floats, Go maps, interfaces, time.Time, platform-width int; dissolved, see below
  application_identity: the names Wire and World, one game's protocol; removed by decision:cbor-codecs-are-application-side and staying removed
the_refusals_dissolve:
  maintainer_2026_08_22: a codec handles what the struct declares and nothing else, so a switch that refuses a declared field is doing too much
  which_is_correct_as_measured: generator/cborhttp_emit.go emitCBORAppendValue handles string, int, int64, bool, float64, a planned struct, a slice and a Go map, and fieldTypeKind refuses everything else before the CBOR pass is reached
  therefore: time.Time, an interface, a pointer to a shared value and a sized or platform-width integer are already generation errors from the mapping pass; naming them again in a profile restates a refusal that has already happened
  and_the_rest_are_policy_not_capability:
    float: the emitter encodes it correctly through cbor.AppendFloat; a project that wants no floats in its protocol declares no float64 field, which is the same statement in the place that already carries it
    go_map: refused by requirement:cbor-http-body under RequireSortedKeys, on the ground that a runtime map cannot promise the order; the emitter does sort it, through jsonbind.SortedKeys, but in Go string order rather than RFC 8949 bytewise order over the encoded key, so the refusal stands in for an implementation gap rather than for a policy
  consequence: cborbind gets no profile option, no CBORProfile struct, and no CLI flag; what a codec can encode is a property of the emitter, and what it does encode is a property of the struct
  what_is_still_refused: a field kind the emitter cannot write, reported as a generation error naming the type and the field, which is the diagnostic the mapping pass already produces
container_shape_is_the_only_thing_left:
  where: the name of the entry point and of the generated method, AppendCBORInArrayTo versus AppendCBORInMapTo, per requirement:cbor-codec-generation
  why_it_cannot_be_a_project_option: a package may hold a compact message for one peer and an evolvable snapshot for another; that is what wire and world were, minus the names
  why_the_name_and_not_a_value: generator/plan.go discoverGenericTypeArgs reads a call's resolved symbol, its generic type arguments and the type of an argument, and no pass in the generator reads an argument value; two names are two DiscoverySymbols and cost nothing, where a value-carrying spelling is new machinery that works only for constant fields
  and_the_constraint_makes_a_mistake_loud: the generated method is shape-named too, so calling the map entry point on an array-shaped type fails to compile instead of quietly producing the other shape's bytes
what_evolution_policy_meant:
  plainly: whether a decoder built from an older version of the struct can read bytes a newer version wrote
  array: members are positional and unnamed, so adding a field changes the array length and an old decoder misreads or fails; both ends must be rebuilt and deployed together
  map: members are named keys and an unknown key is skipped, so an old decoder reads a newer message and ignores what it does not know; the two ends can be deployed at different times
  therefore_not_a_setting: nothing is chosen here; picking the container has already picked this
  which_is_the_whole_user_facing_question: can both ends be updated at once, and is the per-message cost of carrying the key names worth paying
key_order:
  where: implied by the shape, not a knob
  why: an array has no keys, and a map that claims determinism has one right answer, RFC 8949 bytewise over the encoded key; offering the CTAP2 order as an option buys a second wire format nobody asked for
observation_for_the_http_mode:
  what: the same argument applies to Options.CBORHTTPProfile, whose RejectFloats refuses a field the emitter encodes correctly and whose RequireSortedKeys map refusal stands in for the Go-string-order gap above
  not_changed_here: that option shipped, is off by default, and requirement:cbor-http-body records the maintainer asking for it; this is an observation to weigh, not a removal
  if_it_were_revisited: RejectFloats would go and the map order would be fixed rather than refused, leaving the HTTP mode with no profile either
related:
  - requirement:cbor-codec-generation
  - decision:cbor-codecs-are-application-side
  - requirement:cbor-http-body
  - system:tinygodriver-cbor
  - decision:runtime-package-boundaries
  - rule:usage-directed-generation
  - rule:cbor-codec-interface-upstream
```
