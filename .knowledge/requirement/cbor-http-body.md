---
id: requirement:cbor-http-body
type: requirement
title: CBOR HTTP Body Negotiation
---
Let a route read and write application/cbor bodies through declared codecs, off by default and reaching zero emitted bytes for a project that never asks, since the TinyGo targets this module serves cannot pay for a second body format they do not use.

```yaml
priority: should
status: implemented 2026-08-20, same day the driver bump landed
source:
  - maintainer request 2026-08-20, naming the TinyGo size concern explicitly
decided_2026_08_20:
  opt_in: a generator option, project-wide and off by default; no per-route or per-type spelling, so every route accepts CBOR when it is on and none does when it is off
  why_uniform: the maintainer refused a surface where one route accepts a media type and its neighbor does not; the accepted media types are a property of the service, not of a handler
  profile: an independent http profile key, appendXCBORHTTP and decodeXCBORHTTP, and the profile itself is a generator option rather than a fixed literal, at the maintainer's ask
  game_profiles_are_out_of_scope: the wire and world declarations serve the session framework and HTTP names neither; a type used by both worlds carries both keyings
as_built:
  option: Options.EnableCBORHTTP, with Options.CBORHTTPProfile beside it; CLI -cbor-http, -cbor-http-reject-floats, -cbor-http-sorted-keys
  profile_type: CBORHTTPProfile is the generator's own struct, RejectFloats and RequireSortedKeys, because the generator never imports the driver; the restrictions become code in the emitted file rather than a runtime Profile value
  profile_zero_value: floats are ordinary and members come out in struct field order, the JSON encoder's own order; RequireSortedKeys switches to RFC 8949 bytewise order of the encoded key, computed at generation by cborEncodedTextKey
  hashing: both fields ride the generation fingerprint, so a profile change forces regeneration, which is what keeps the two ends agreeing
  codecs: generator/cborhttp_emit.go emits append<T>CBORHTTP and decode<T>CBORHTTP from the same TypePlan the JSON codecs use, so both formats agree about wire names, payload membership and nesting; a nested struct joins its parent's Reader walk with no second scan
  binder: an inline walk over the body map fills payload fields and their presence flags before the JSON and form arms run and find no body of their kind; query still overrides a body member for an input field because the query arms run later and overwrite
  writer: an AcceptsCBOR arm ahead of the JSON path; only an explicit application/cbor Accept entry counts, so a browser's */* never flips the format
  runtime: bindcore owns the shared media-type check, Accept parsing and the one process-wide limit; httpbind and fasthttpbind each publish IsCBORRequest, AcceptsCBOR, ReadCBORBody, WriteCBORBytes and SetMaxCBORBodyBytes, pair-wise as required, and none of them names a driver type
  read_limits: every generated DecoderOptions defers to the body length ReadCBORBody already capped, so the deployment tunes one number
  refusals: a payload rest map, a foreign-codec field, an uploaded file in a response type, a float64 under RejectFloats, and a runtime map under RequireSortedKeys are generation errors naming the field
  emitted_all_members: omitempty and omitzero are JSON member semantics and do not drop CBOR members, which keeps the map header count a generation-time constant
  tests: generator/cborhttp_test.go, including a full HTTP round trip of the generated code in both formats; runtime twins in cbor_test.go, fasthttpbind/cbor_test.go, internal/bindcore/cbor_test.go
  v1_stays_json: WriteStatus, streaming, server actions and the OpenAPI document; each is a later decision, not an accident
trigger:
  what: term:payload decodes by Content-Type and knows json, form and multipart; api:write answers application/json only
  want: a request carrying application/cbor binds, and a response is CBOR when the client asked for it
size_is_the_shaping_constraint:
  the_switch_is_the_generator_option: EnableCBORHTTP off emits today's bytes exactly, so a project that never asks pays nothing; on, the negotiation arms and http-keyed codecs are emitted for the types routes actually use, per rule:usage-directed-generation
  linker: rule:transport-dead-code-elimination; no registry and no init, so nothing becomes reachable that the option did not emit, and the threat named there, a registering init, is exactly what this must not add
  consequence: the ON/OFF the request asks for is a generation-time option, not a runtime flag; a runtime flag would link both paths and toggle one
runtime_stays_driver_free:
  rule: no runtime package imports the driver, per decision:runtime-package-boundaries; the codec calls live in generated code, which is the one place the driver is named
  therefore: httpbind gains only helpers that never name a driver type; the codec call sites live in generated code, which already imports the driver through its own emitted methods
  surface:
    IsCBORRequest: media-type compare on Content-Type, string only
    WriteCBORBytes: the WriteJSONBytes twin, status plus finished bytes plus the application/cbor header
    AcceptsCBOR: Accept header check choosing the response encoding
  rejected: an any(value).(cbor.Appender) arm in api:write beside the jsonbind.Appender one; it puts a driver import and a live branch into every httpbind build, which is the cost the request exists to avoid
  parity: each helper gets its fasthttpbind twin in the same change, per the standing pair-wise constraint
binding_shape:
  v1: the payload field set decodes as one CBOR map through the type's generated http-profile decoder; query, path, header and cookie fields bind as today
  not_v1:
    - a generic CBOR object mirroring jsonbind.Object; per-field body binding from a generic container is the allocating surface this mode has no need for yet
    - rule:payload-rest-map for a CBOR body; refused at generation when a route opts in, not silently empty
  read_limit: since driver v1.2.7 every resource limit is a DecoderOptions the deployment passes beside the profile, so the configured HTTP body limit maps to MaxInputBytes at the emitted read site rather than fighting a profile constant
negotiation:
  request: application/cbor and *+cbor bind through the CBOR path; with the option off a CBOR body is an unknown media type exactly as today — the binder reads nothing and required checks answer 400, since no 415 path exists for any unknown body and inventing one for CBOR alone would be the inconsistency
  response: an explicit Accept application/cbor selects CBOR; the default stays application/json, and a wildcard is not an ask
  errors: problem responses stay bindcore.ProblemContentType in v1; application/problem+cbor is size for a document a failing client rarely parses
acceptance:
  - a project leaving the option off regenerates byte for byte, and its binary carries no driver cbor symbol
  - with the option on, a route round-trips a CBOR body and still serves JSON to a JSON client; verified over httptest in generator/cborhttp_test.go
  - a malformed or truncated CBOR body is 400, an oversize one 413, and an unknown member is skipped
  - turning the option off removes the negotiation arms and the helper references from generated output
not_yet_verified:
  - the TinyGo wasm link of an option-on project; the codecs call the driver layer that was linking under TinyGo before decision:cbor-codecs-are-application-side removed that smoke fixture, so the risk is low, and a fresh fixture is where it would be verified
related:
  - decision:cbor-codecs-are-application-side
  - system:tinygodriver-cbor
  - rule:generator-feature-disable
  - rule:transport-dead-code-elimination
  - decision:runtime-package-boundaries
  - term:payload
  - api:write
profile_question_resolved_by_driver_v1_2_7:
  was: whether HTTP uses World with floats reopened or the driver grows a third profile, since the shipped World refused floats
  now: a Profile is a format-restriction struct literal any consumer names for itself, limits live in DecoderOptions, and floats are ordinary unless refused, per system:tinygodriver-cbor profiles_reshaped_v1_2_7
  therefore: this mode names its own subset without any driver change, and the float objection is gone
  and_then_the_literal_dissolved: as built there is no runtime Profile value at all; every restriction is either code the emitter writes (member order, definite-length walks) or a DecoderOptions field (float refusal), so the profile lives in Options.CBORHTTPProfile and the generated shapes rather than in a var
  sequencing: the bump was prior work and landed first, per system:tinygodriver-cbor migration_debt_at_the_v1_2_7_bump
open_questions:
  - whether this closes rule:generator-feature-disable's open question on disabling the CBOR mode as a whole, by making each CBOR feature independently removable
```
