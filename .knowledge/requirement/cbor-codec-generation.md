---
id: requirement:cbor-codec-generation
type: requirement
title: CBOR Codec Generation
---
Generate a CBOR codec for a type from an ordinary call whose name carries the container shape, so asking for a codec is using one, and using the wrong shape is a compile error.

```yaml
priority: should
status: proposed 2026-08-22; reopens the mechanism decision:cbor-codecs-are-application-side removed, without the application names and without a codec-declaration surface
review_gate: proposed
source:
  - maintainer 2026-08-22, asking for cborbind back with JSON's ergonomics, shape-named append entry points, and no Codec noun
  - decision:dynamobind-static-dispatch, whose pointer constraint is the dispatch this copies
entry_points:
  count: four generic functions in cborbind and two shape-named method pairs, no declaration verbs at all
  encode: 'AppendCBORInArrayTo[T ArrayAppender](dst []byte, v T) []byte and AppendCBORInMapTo[T MapAppender](dst []byte, v T) []byte'
  decode: 'DecodeCBORInArrayFrom[T any, PT interface{*T; ArrayDecodable}](data []byte) (T, error) and the Map twin; PT is inferred, so a call site writes one type argument'
  contracts: 'ArrayAppender is AppendCBORInArrayTo(dst []byte) []byte, ArrayDecodable is DecodeCBORInArrayFrom(data []byte) error on the pointer, and the two Map twins'
  array: a fixed-length array in declaration order; member names are not on the wire, so adding a field changes the length and both ends must be rebuilt together
  map: a text-keyed map in RFC 8949 bytewise key order; a decoder skips a key it does not know, so an old build reads a newer message and the two ends can ship separately
  the_choice_is_one_question: can both ends be updated at once, and is carrying the key names per message worth what it buys, per decision:cbor-shape-is-the-only-axis
  shape_is_in_the_name: two DiscoverySymbols per direction, which is what generator/plan.go discoverGenericTypeArgs can already read, per decision:cbor-shape-is-the-only-axis
  direction_is_which_one_is_called: rule:usage-directed-generation unchanged; a package that only appends gets no decoder, with no Encoder or Decoder verb to say so
  no_codec_noun: the removed design had six Generate verbs; a call is the ask, so there are none
no_profile:
  what: no CBORProfile struct, no project option, no CLI flag; the shape is the only thing a codec is told
  why: a codec handles what the struct declares, so a switch refusing a declared field restates a refusal the mapping pass has already made or overrides a decision the struct already carries, per decision:cbor-shape-is-the-only-axis
  a_project_wanting_no_floats: declares no float64 field, which is where that statement already lives
field_kinds_are_a_capability:
  encodable: string, int, int64, bool, float64, a planned struct, a slice of those, and a Go map, which is what generator/cborhttp_emit.go already writes
  everything_else: a generation error naming the type and the field, raised by the mapping pass before the CBOR pass is reached, which is where a sized integer, an interface and time.Time already stop
  the_gap_to_close_first: the map arm sorts through jsonbind.SortedKeys, which is Go string order rather than bytewise order over the encoded key; the map shape needs the latter, so this is an implementation fix rather than a field kind to refuse
  and_the_sized_integers: requirement:sized-integer-field-kinds, which is prerequisite work rather than part of this -- fieldTypeKind maps neither uint32 nor the other widths, so an array-shaped message cannot yet carry the narrow fields that shape exists for, and the same blocker was found on 2026-08-19 and deferred
the_shape_is_in_the_method_too:
  what: the generated method is AppendCBORInArrayTo or AppendCBORInMapTo, not only the free function
  buys_a_compile_error: the constraint is shape-specific, so calling the map entry point on an array-shaped type does not build; had both shapes shared cbor.Appender the call would compile and quietly produce the other shape's bytes
  buys_both_shapes_on_one_type: two method names do not collide, so a type may be array-shaped for one peer and map-shaped for another, which the single AppendCBORTo could not express
  keeps_the_driver_interface: a type declaring exactly one shape also gets AppendCBORTo and DecodeCBORFrom delegating to it, so it satisfies cbor.Appender and cbor.Decodable and any driver-generic consumer reaches it
  and_a_type_declaring_both_gets_neither: the delegating pair has no unambiguous target, and the ambiguity is then visible in the type rather than resolved by a coin flip
dispatch_measured:
  when: 2026-08-22, darwin/arm64 Apple M3, go test -bench -benchtime 2s, entry point and type in different packages, a ten-field struct with a string
  constrained_generic_shape_named: 5.4 ns and 0 allocs, and 6.5 ns and 0 allocs for a freshly built local; nothing escapes because no interface value is ever materialised
  direct_method_call: 3.3 ns, 0 allocs; still available and still what a hot loop may write
  unconstrained_generic_taking_a_value: 18.7 ns, 48 B, 1 alloc; any(v) boxes the struct and escape analysis cannot recover it, with or without inlining
  unconstrained_generic_taking_a_pointer: 4.2 ns and 0 allocs only when the caller's value is already addressable; 17.1 ns and 1 alloc for a fresh local, which the boxed pointer forces to the heap
  registry_lookup: 7.1 ns in a same-package probe, above the driver's whole 9.2 ns encode once cache effects are counted
  conclusion: the constrained form is the only one that is allocation-free for every caller, and it is also the one that reports a missing codec
a_missing_codec_is_a_compile_error:
  what: a type with no generated method does not satisfy the constraint, so the call site fails to build
  why_that_is_the_win: decision:dynamobind-static-dispatch chose the same trade against jsonbind, whose registry reports a missing codec at run time
  the_objection_that_does_not_hold:
    raised: that a constrained call cannot compile before generation has run, so it cannot be the trigger for its own generation
    answered: generator/load.go checkLoadedPackage accepts a package that does not type-check and refuses only an unresolved import, and its comment says why -- a package analyzed before its codec exists does not satisfy the constraint yet, by design, and refusing that would refuse every first run
    therefore: the first run discovers the call, writes the methods, and the second build succeeds; this is how the DynamoDB and Firestore modes already bootstrap
    verified: 2026-08-22, reading generator/load.go and compiling the four entry points against a hand-written stand-in for the generated methods
no_registry:
  what: the entry point calls the method through the constraint; nothing is registered and no init is emitted
  therefore: no generated init for CBOR-only output, and no reflect
what_is_generated:
  functions: appendXCBORArray and decodeXCBORArray, and the Map twins; a type reached from both shapes gets one function per shape, since the shapes disagree about the container
  methods: the shape-named pair for each declared shape, plus the delegating driver pair when exactly one shape is declared
  a_hand_written_method_wins: a type already carrying a shape method is used through it and a generated one is refused, per rule:cbor-codec-interface-upstream
same_package_only:
  rule: rule:same-package-convention unchanged; generation is per package
  a_foreign_type: the call site names a type another package declares, that package generates nothing here, and the constraint fails to be satisfied -- which reports at compile time, so the silent nothing requirement:declared-json-codec had to add a check for cannot happen through this door
no_generate_all:
  what: the CBOR usage bits stay outside UsageAll
  why: a CBOR codec is a protocol, so giving every struct in a package one publishes a wire format nobody asked for
what_stays_out:
  - the names Wire and World, and any preset standing for one application's protocol
  - the delta pass: the generated diff and apply types and the cbor identity tag that ordered an entity collection; a state diff is a game's sync strategy, not a codec
  - the protocol version negotiation that rode with them
  - a codec-declaration verb; if a type ever needs one because nothing in its package calls an entry point, requirement:declared-json-codec is the shape to copy, and it is not built until that case appears
acceptance:
  - a call to an entry point generates the codec for its type, in that direction and that shape only
  - a package appending but never decoding emits no decoder, at the root and at depth
  - a struct reached from both shapes gets one function per shape
  - a type declaring one shape satisfies cbor.Appender through the delegating method, and one declaring both satisfies it through neither
  - calling the map entry point on an array-shaped type fails to compile
  - an entry point call allocates nothing for a freshly built local, verified by an allocation test rather than by inspection
  - a first run over a package whose calls do not yet type-check discovers them and writes the methods
  - a project calling no entry point regenerates byte for byte and links no driver cbor symbol
  - the emitted codecs build and run under TinyGo for wasm and wasip1
related:
  - decision:cbor-shape-is-the-only-axis
  - requirement:sized-integer-field-kinds
  - rule:cbor-codec-interface-upstream
  - decision:cbor-codecs-are-application-side
  - decision:dynamobind-static-dispatch
  - requirement:cbor-http-body
  - requirement:declared-json-codec
  - requirement:json-codec-interface
  - concept:standalone-json-codec
  - system:tinygodriver-cbor
  - rule:usage-directed-generation
  - rule:same-package-convention
  - rule:generator-feature-disable
  - decision:runtime-package-boundaries
  - decision:reflection-free
  - requirement:tinygo-wasm
open_questions:
  - whether the delegating driver pair is emitted at all, or a type that wants cbor.Appender writes the one-line delegation itself
  - whether the decode entry point returns a value or fills a caller-supplied pointer; returning is jsonbind's shape and is what dynamobind already does with PT
  - whether InArray and InMap are the right words in the name, per decision:cbor-shape-is-the-only-axis naming
  - whether the downstream game framework migrates to these entry points or keeps its own codegen; the delta pass has no replacement here either way
```
