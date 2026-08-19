---
id: requirement:declared-cbor-codec
type: requirement
title: Declared CBOR Codec
---
Let a declaration request CBOR codec generation for a type and name the profile it is generated for, since a message type crosses to the session framework with no generic call at the crossing.

```yaml
priority: should
status: implemented 2026-08-19
as_built:
  annotations: GenerateWireCodec, GenerateWireEncoder, GenerateWireDecoder, and the three World twins, each generic over T and returning a zero-size Declaration
  operations: four, one per profile and direction, so a codec form registers two patterns against its one target and each is gated on its own feature
  usage_bits: UsageCBORWireEncode, UsageCBORWireDecode, UsageCBORWorldEncode, UsageCBORWorldDecode, outside UsageAll and with no generate-all rule at all
  no_generate_all_on_purpose: a CBOR codec is a protocol, so giving every struct in a package one would publish a wire format nobody declared; the item and entity codecs each have a tag-driven generate-all rule and this has none
  patterns: canonicalCBORCalls, given its own arm in callPatterns like firestorebind's, since cborbind shares no entry name with the canonical set
  features: FeatureCBORWireCodec and FeatureCBORWorldCodec
  no_new_discovery: discoverGenericTypeArgs already walks the whole file, so the package-level var initializer was in front of it, exactly as the JSON declaration found
  profile_pinned: the generated decoder names cborWireProfile or cborWorldProfile, both package-level vars in the emitted file, rather than taking a profile from the caller
  a_type_declared_for_both_profiles_is_refused: one type cannot publish two AppendCBORTo methods, so it is a generation error rather than a file that does not compile
  nested_types_are_keyed_by_profile: a struct reached from a wire root and from a world root gets two functions, appendXCBORWire and appendXCBORWorld, since the profiles disagree about the container it encodes as
  directions_reach_nested_types:
    found: 2026-08-19, while building; the first emitter gave every nested type both halves whatever the root asked for
    why_it_mattered: an encode-only declaration on a wasm client would still have carried a decoder for every struct below the root, which is the code size the direction narrowing exists to avoid
    fix: the direction set rides down the collector, and a nested type reached from two roots accumulates rather than the first one winning
    and_the_profile_var_followed: only a published decoder reads cborWireProfile, so an encode-only package now declares neither var
  tests: generator/cborbind_test.go
source:
  - downstream game framework CBOR requirements 2026-08-19, its section 4.1
  - requirement:declared-json-codec, which is the same trigger for the same reason
why_not_call_driven:
  today: rule:usage-directed-generation emits a mapping path only when its configured generic call is present
  but: a game hands its message types to the session framework, which encodes them generically, so no cborbind.Encode call appears in the game's own source
  precedent: this is exactly the case requirement:declared-json-codec names, a type crossing a boundary with no generic call at the crossing
  no_new_discovery_needed: discoverGenericTypeArgs already walks the whole file, so a package-level var initializer is in front of it, as the JSON declaration found
declaration:
  spelling: 'var _ = cborbind.GenerateWireCodec[PlayerInput]() and var _ = cborbind.GenerateWorldCodec[WorldState](), beside the type'
  shape: generic over the type, returning a zero-size declaration so the call can be written as a package-level var, per decision:typed-action-declaration
  init_cost: the call runs at init and does nothing, the footprint that shape already accepts
the_profile_is_part_of_the_declaration:
  why: the profile is part of the contract and the two ends must not disagree about it, so it cannot be a generator flag or a per-run default
  consequence: one type may carry both codecs, and the two are separate declarations rather than one with an option
a_delta_is_declared_beside_the_codec:
  form: 'var _ = cborbind.GenerateWorldDelta[WorldState]()'
  implies: the world codec, since a delta is diffed from values that must also be encodable
  where_specified: requirement:cbor-delta-codec
  not_built_yet: yes
direction_narrowing:
  forms: GenerateWireEncoder, GenerateWireDecoder, and the world equivalents
  why_it_is_named: emitting an unused direction is code size on a wasm client, which is the same reasoning requirement:declared-json-codec directions records
  per_direction_patterns: each direction registers its own pattern against the one target, so rule:generator-feature-disable can remove one and leave the other standing; the JSON declaration found that a single both-directions operation cannot be half-removed
same_package_only:
  rule: rule:same-package-convention applies unchanged; the declaration lives in the package that declares the type
  foreign_type: a generation error naming the type and the package, not silence, per requirement:declared-json-codec a_foreign_type_is_refused
the_generated_code_pins_its_profile:
  what: a wire-profile codec is not usable to read a world-profile message
  why: the profiles differ in what they admit, so a codec that accepted either would be enforcing neither
  how: the emitted decoder names the profile's reader options rather than taking them from the caller
methods_are_the_driver_s_interfaces:
  what: a declared codec satisfies cbor.Appender and cbor.Decodable by delegation
  where_stated: rule:cbor-codec-interface-upstream
  precedence: a type already carrying a hand-written AppendCBORTo is encoded through it even when a declaration also named it
acceptance:
  - a type no call site names gets a codec because a declaration asked for it
  - a declaration naming one direction emits that direction only
  - a declaration naming a type of another package fails generation, naming that package
  - a wire codec refuses a world-profile message
  - a project declaring no CBOR codec regenerates byte for byte
related:
  - requirement:declared-json-codec
  - requirement:cbor-wire-codec
  - requirement:cbor-world-codec
  - rule:cbor-codec-interface-upstream
  - rule:usage-directed-generation
  - rule:same-package-convention
  - decision:cborbind-runtime-package
  - decision:typed-action-declaration
open_questions:
  - whether one package name should carry both profiles, per decision:cborbind-runtime-package name_question
```
