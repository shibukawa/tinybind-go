---
id: requirement:declared-json-codec
type: requirement
title: Declared JSON Codec
---
Let a declaration request codec generation for a type, so a codec is reachable without a discovered call site to derive it from.

```yaml
priority: should
status: implemented 2026-08-13
as_built:
  annotations: jsonbind GenerateCodec, GenerateEncoder, and GenerateDecoder, each generic over the type and returning a zero-size Declaration so the call can be written as a package-level var
  spelling: 'var _ = jsonbind.GenerateCodec[User]() beside the type'
  no_new_discovery:
    why: discoverGenericTypeArgs walks the whole file with ast.Inspect, so a package-level var initializer is already in front of it
    result: the annotation is an ordinary configured call with a generic type source, and the only new code is the operations and their usage mapping
  operations: OperationJSONEncoderDeclare and OperationJSONDecoderDeclare, each mapping to its codec usage plus the matching method bit of requirement:json-codec-interface
  one_operation_carries_both_meanings: the annotation asks for a codec and for it to be published, and usageForCallOperation already returns a combined mask for OperationItemEncodeDecode, so this needed no second call site
  one_operation_per_direction:
    found: 2026-08-13, by TestDisabledPatternCannotBeReenabledByGenerateAll failing
    what: a single both-directions operation contributed its decode usage to the enabled set even with FeatureDecodeJSON disabled, so GenerateAll reenabled a disabled feature
    why: a pattern is the unit featureDisabledForCall removes, and an operation carrying two directions cannot be half-removed
    fix: GenerateCodec registers both patterns against its one target, so each direction is gated on its own feature and disabling one leaves the other standing
    guard_widened: composeOnOneTarget admitted only the socket pair; the codec pair is named there the same way rather than relaxing the guard to any two operations
    roles_reused: the patterns carry the existing encode and decode roles rather than a new one, which primaryTypeSource would not have recognized
  registered_per_runtime_path: canonicalRuntimeCalls is applied for every runtime package, so the names are registered against each and matched by resolved symbol identity, which is how DecodeJSON and EncodeJSON already work
  tests: generator/declared_codec_test.go over each direction, over a discovered call publishing nothing, over a type beside an annotated one, and over one direction disabled
  init_cost_accepted: the call runs at init and does nothing, which is the runtime footprint decision:typed-action-declaration records for the same declaration shape
source:
  - maintainer proposal 2026-08-13
  - requirement:typed-server-action the_result_type_still_needs_a_usage
  - decision:typed-action-declaration
review_gate: proposed
today:
  call_driven: rule:usage-directed-generation emits each mapping path only when its configured generic call is present, so api:encode-json and api:decode-json call sites are the whole trigger
  consequence: a type nothing calls gets nothing, and a type whose only call the generator itself writes gets nothing either
  workaround_authors_reach_for: a call site written to be found rather than to be run, which is a fake usage in real source
model:
  authored: 'a package-level declaration naming the type and the directions wanted, in the shape decision:typed-action-declaration sets for a server action'
  spelling: 'ours; a generic call such as jsonbind.Codec[User]() reads naturally and needs no argument value'
  effect: the type enters the plan with the named usage, exactly as a discovered call would have put it there
  and_advertises_it: the declaration is also what emits the methods of requirement:json-codec-interface, decided 2026-08-13, so declaring is one statement rather than two
  why_one_statement: declaring a codec for a type this package's own calls do not reach is already saying the type is used from somewhere the analysis cannot see, which is the case the method exists for
precedent:
  exists: rule:usage-directed-generation item_key_exception already lets a declaration rather than a call add usage, where a partitionkey tag gets ItemKey and its table definition with no discovered call
  difference: that one is a tag on a field and implies one operation; this names the type and says which directions
  generalization: both are the same statement, that usage is a fact an author may assert rather than only one the analyzer may infer
directions:
  encode_only: a type only ever written
  decode_only: a type only ever read
  both: the ordinary case
  why_it_is_named: emitting both when one is wanted is code size on a TinyGo target, which is the whole reason rule:usage-directed-generation is usage-directed
what_it_unlocks:
  typed_action: requirement:typed-server-action needs its result type planned and its argument struct decoded, and this is that mechanism rather than a second one built for it
  domain_package_owns_its_codec: a package declaring its own codec and satisfying requirement:json-codec-interface is encodable by every consumer, which is how a shared domain type stops being the analyzer's problem
  boundaries_the_analyzer_cannot_see: a WebSocket message, a queue payload, or a value crossing any seam with no generic call at the crossing
  the_composition: this requirement makes the codec exist and requirement:json-codec-interface makes it reachable from outside; neither is much use alone for a cross-package type
relation_to_the_registry:
  unchanged: a declared codec registers the same public dispatch entry a discovered one does, so api:encode-json keeps working for it
  and_also: the emitted function is nameable by generated code directly, which is what requirement:typed-server-action uses instead of the registry
constraints:
  - the declaration adds usage and nothing else; it does not change what a codec does or how a tag reads
  - a type declared in another package is still not planned here, per rule:same-package-convention; the declaration belongs in the package that declares the type
  - a declaration for a type already reached by a discovered call emits no duplicate codec, but it is not a no-op: the method emission of requirement:json-codec-interface is exactly what it adds, which is how a type that is both used locally and read from another package gets both
acceptance:
  - a type no call site names gets a codec because a declaration asked for it
  - a declaration naming one direction emits that direction only
  - a declared and a discovered usage for one type emit one codec, and the methods
  - a type reached only by a discovered call gains no method
  - a declaration naming a type of another package fails generation, naming the package that should carry it
  - a project declaring nothing regenerates byte for byte
related:
  - rule:usage-directed-generation
  - requirement:json-codec-interface
  - requirement:typed-server-action
  - decision:typed-action-declaration
  - concept:standalone-json-codec
  - rule:same-package-convention
open_questions:
  - whether one declaration form covers the other generated paths, the SQL scanner and the item codecs, or stays JSON-only
```
