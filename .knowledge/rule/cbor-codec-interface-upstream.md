---
id: rule:cbor-codec-interface-upstream
type: rule
title: The CBOR Codec Interface Is The Driver's
---
Recognize cbor.Appender and cbor.Decodable structurally and declare no second spelling of them, because two spellings of one contract silently skip a type that satisfies the wrong one.

```yaml
priority: must
status: was implemented 2026-08-19 and removed with its pass; returns unchanged with requirement:cbor-codec-generation, since it names no application
the_difference_from_json:
  json: this module declares the contract itself, jsonbind Appender with AppendJSONTo and Decoder with DecodeJSONFrom, per requirement:json-codec-interface
  cbor: system:tinygodriver-cbor already declares it, Appender with AppendCBORTo and Decodable with DecodeCBORFrom
  the_name_is_not_symmetric: Decodable rather than Decoder, because cbor.Decoder is the streaming reader; a mechanical port of the JSON naming would collide
  why_it_matters_early: this is the one structural difference between the two modes, cheap to get wrong and expensive to undo once an interface is published
shape_named_methods_are_not_a_second_spelling:
  what: requirement:cbor-codec-generation emits AppendCBORInArrayTo and AppendCBORInMapTo, and cborbind declares the constraints naming them
  why_that_is_allowed: they name a container shape this module chose, which the driver has no opinion about; they are not another way to spell AppendCBORTo
  and_the_driver_pair_is_still_emitted: a type declaring exactly one shape gets AppendCBORTo and DecodeCBORFrom delegating to that shape, so the contract below is satisfied without a second spelling of it
  a_type_declaring_both_shapes: gets no delegating pair, since there is no unambiguous target; it satisfies the driver interfaces through neither, which is the honest answer
rules:
  - cborbind declares no spelling of cbor.Appender or cbor.Decodable
  - the generator recognizes the driver's two interfaces structurally with go/types, so an analyzed package need not import cborbind merely to have its types admitted
  - a generated codec satisfies them by delegation, as requirement:dynamobind-generated-item-codec emits EncodeItem and DecodeItem onto the type
  - a type carrying AppendCBORTo is encoded through it even when the run also planned that type, and the same for DecodeCBORFrom
as_built_when_it_existed:
  structural: method sets of T and of *T scanned by name and signature
  both_receivers_admitted: a value-receiver AppendCBORTo and a pointer-receiver one both count; the encoder's parameter is addressable either way
  precedence_is_first_not_last: the self-codec case is checked before every other, so a type carrying the methods is never planned instead
  a_previous_run_is_not_evidence: methods declared in a file this generator wrote are excluded by position, or a codec regenerated over its own output would find what it emitted last time and conclude the type encodes itself
  a_declared_type_carrying_them_is_refused: a generation error rather than a second codec
why_no_second_declaration:
  failure_shape: a type satisfies one spelling, the generator checks the other, and the field is dropped with no diagnostic
  already_paid_once: requirement:declared-json-codec a_foreign_type_is_refused fixed exactly that silent nothing for JSON
  worse_here: under a fixed-length array shape both ends still agree about everything they did encode, so a dropped field is a running protocol with a hole in it rather than a parse failure
not_reflection:
  what: a type assertion to an interface is dispatch, not field walking, and here even the assertion is resolved at generation
  therefore: decision:reflection-free is unaffected and requirement:tinygo-wasm keeps holding
a_foreign_append_cannot_be_trusted:
  fact: AppendCBORTo returns no error, so a foreign implementation can write bytes a restriction refuses and nothing below reports it
  available: Profile.ValidateAppended over the region appended since a recorded length, which the driver added for this
  was_not_emitted: the cost is a second pass over bytes the codec just wrote, on the path whose whole budget is 9.2 ns, and no measurement says what that costs
  what_stood_in_for_it: generated tests validating the whole message after encoding it, which catches a foreign implementation at test time rather than in production
  still_open: whether to emit the per-field check always, under a build tag CI sets, or only into generated tests
related:
  - system:tinygodriver-cbor
  - requirement:cbor-codec-generation
  - requirement:json-codec-interface
  - requirement:declared-json-codec
  - requirement:dynamobind-generated-item-codec
  - decision:reflection-free
  - requirement:tinygo-wasm
```
