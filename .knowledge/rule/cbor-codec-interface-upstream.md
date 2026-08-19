---
id: rule:cbor-codec-interface-upstream
type: rule
title: The CBOR Codec Interface Is The Driver's
---
Recognize cbor.Appender and cbor.Decodable structurally and declare no second spelling of them, because two spellings of one contract silently skip a type that satisfies the wrong one.

```yaml
status: implemented 2026-08-19
priority: must
as_built:
  where: generator/cborbind_types.go selfCodec, isAppendCBORSignature, isDecodeCBORSignature
  structural: method sets of T and *T are scanned by name and signature, so nothing imports the driver's interfaces and no analyzed package needs to import cborbind
  both_receivers_admitted: the pointer's method set is scanned too, so a value-receiver AppendCBORTo and a pointer-receiver one both count; the encoder's parameter is addressable either way
  precedence_is_first_not_last: resolve checks selfCodec before every other case, so a type carrying the methods is never planned instead
  a_previous_run_is_not_evidence: methods declared in a file this generator wrote are excluded by position, or a codec regenerated over its own output would find the methods it emitted last time and conclude the type encodes itself
  a_declared_type_carrying_them_is_refused: naming a type that already has a hand-written codec is a generation error rather than a second codec, since either outcome would be worse than saying so
  covers_the_same_package_case: the kind is CborSelf rather than foreign, because a fixed-point type declared right here is the ordinary case after decision:cbor-scale-lives-in-the-type, not the exceptional one
  tests: generator/cborbind_test.go over the pinned reference bytes, a slice of entities holding one, and the refused declaration
source:
  - downstream game framework CBOR requirements 2026-08-19
  - requirement:json-codec-interface, which is the same shape with the opposite ownership
the_difference_from_json:
  json: this module declares the contract itself, in jsonbind/interface.go, as Appender with AppendJSONTo and Decoder with DecodeJSONFrom
  cbor: system:tinygodriver-cbor already declares it, as Appender with AppendCBORTo and Decodable with DecodeCBORFrom
  the_name_is_not_symmetric: Decodable rather than Decoder, because cbor.Decoder is the streaming reader; a mechanical port of the JSON naming would collide
  why_it_matters_now: this is the one structural difference between the two modes, and it is cheap to get wrong and expensive to undo once an interface is published
rules:
  - cborbind declares no codec interface of its own
  - the generator recognizes the driver's two interfaces structurally with go/types, so a game package need not import cborbind merely to have its types admitted
  - a generated codec satisfies them by delegation, the way requirement:dynamobind-generated-item-codec emits EncodeItem and DecodeItem onto the type
  - a type carrying AppendCBORTo is encoded through it even when the run also planned that type
  - a type carrying DecodeCBORFrom is decoded through it on the same terms
why_no_second_declaration:
  failure_shape: a type satisfies one spelling, the generator checks the other, and the field is dropped with no diagnostic
  already_paid_once: requirement:declared-json-codec a_foreign_type_is_refused fixed exactly that silent nothing for JSON, arriving through the mechanism's own front door
  worse_here: under a fixed-shape profile both ends still agree about everything they did encode, so a dropped field is a running protocol with a hole in it rather than a parse failure
method_over_plan:
  rule: unchanged from requirement:json-codec-interface precedence
  why: generating a codec for a type whose author wrote an encoder, and then using the generated one, silently produces bytes the author did not intend
  resolved_at_generation: the binding phase type-checks, so the emitted call names one path and no runtime branch exists
  what_is_opted_out_of: the field ordering, widths and scales of requirement:cbor-wire-codec; a hand-written method is not checked against them
not_reflection:
  what: a type assertion to an interface is dispatch, not field walking, and here even the assertion is resolved at generation
  therefore: decision:reflection-free is unaffected and requirement:tinygo-wasm keeps holding
a_foreign_append_cannot_be_trusted:
  fact: AppendCBORTo returns no error, so a foreign implementation can write bytes the profile refuses and nothing below reports it
  available: Profile.ValidateAppended over the region appended since a recorded length, which the driver added for this
  applies_to: a foreign field embedded in a wire-profile message, where the profile is part of the contract rather than advice
  as_built: not emitted; the generated encoder calls the foreign method and validates nothing
  why_not_yet: the cost is a second pass over bytes the codec just wrote, on the path whose whole budget is 9.2 ns, and no measurement says what that costs here
  what_stands_in_for_it_today: the generated tests and the TinyGo smoke validate the whole message under the profile after encoding it, which catches a foreign implementation that writes something the profile refuses -- at test time rather than in production
  still_open: whether to emit the per-field check always, under a build tag CI sets, or only into generated tests
  same_trade_as: requirement:json-codec-interface the_contract_is_unverifiable, which accepted the unchecked half because refusing the feature was the alternative; the difference is that here a check exists
related:
  - system:tinygodriver-cbor
  - requirement:json-codec-interface
  - requirement:declared-cbor-codec
  - requirement:cbor-composite-field-kinds
  - decision:cborbind-runtime-package
  - decision:reflection-free
```
