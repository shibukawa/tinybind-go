---
id: requirement:json-codec-interface
type: requirement
title: JSON Codec Interface
---
Publish an append-style codec interface that generated codecs satisfy, so a type this module never analyzed is encodable by contract rather than by analysis.

```yaml
priority: should
status: implemented 2026-08-13, both halves; a type carrying the interfaces is now published, and read back at a root and at depth
as_built:
  interfaces: jsonbind/interface.go declares Appender with AppendJSONTo and Decoder with DecodeJSONFrom
  no_error_on_append:
    what: AppendJSONTo returns only the extended slice
    why: the append path carries no error anywhere below this point, so adding one would restructure every emitted encoder rather than the interface alone
    consequence: an implementation must append valid JSON for every value of its type, which the doc comment states
  appendany: jsonbind/append.go gained the Appender arm after the concrete cases and before the null default
  emission: generator/emit.go emitCodecMethods, guarded on the direction bits an annotation sets
  method_direction_is_its_own_bit:
    found: 2026-08-13, while building
    what: guarding emission on the type's overall usage published both methods for a type that asked for one, because GenerateAll gives every type every codec
    fix: UsageAppendMethod and UsageDecodeMethod, set only by the annotation that names that direction
  tests: jsonbind/interface_test.go over both interfaces and five AppendAny cases, generator/declared_codec_test.go over the emitted method set and a compile of the generated code against the interfaces
consuming_side:
  root_runtime: jsonbind EncodeJSON, DecodeJSONBytes, and the reader entries try the interface before the registry, and httpbind Write does the same before its writer lookup; WriteStatus inherits it by already going through EncodeJSON
  interface_first: as precedence.method_over_plan requires; for a declared codec the two routes are the same code, since the emitted method delegates to the emitted function
  reader_checks_before_reading: the decoder existence check now covers both routes, so a type decodable by neither still fails without consuming the reader
  foreign_field:
    was: a field whose type is qualified fell through fieldTypeKind and was dropped from the plan without a word, because analysis is per package and such a name resolves to nothing it can walk
    now: KindForeign, admitted when the type carries either method, checked structurally with go/types rather than against the declared interfaces so the analyzed file need not import jsonbind
    one_half_is_enough:
      rule: a foreign field is admitted when its type carries either method, and refused only when its parent turns out to need the half it lacks
      the_constraint_that_shapes_it: analyzeStruct runs while the file is being walked and usage is assigned after every type is collected, so at the moment a field is admitted the parent's direction is not yet known
      therefore: admission takes what the type has and records it, and the requirement is a separate check where usage is settled
      where_checked: emit checkForeignFieldDirections, over the types about to be emitted
      what_needs_which: a parent carrying write or encode usage needs AppendJSONTo; a parent carrying decode or bind usage needs DecodeJSONFrom, since the binder reads a foreign field through the same decoder call
      first_shape_was_both: requiring both halves at admission, which was decidable at the right moment but refused a one-directional type in a package that only ever needed that direction; corrected 2026-08-13 at the maintainer's ask
      what_the_check_prevents: the decoder writes the DecodeJSONFrom call unconditionally for a foreign field, so without it a type carrying only the append method produces a generated file that does not compile
      why_a_diagnostic_matters_here: that compile error lands inside a DO NOT EDIT file and names a call the generator wrote, which is the failure rule:generated-source-not-discovered transform_was_missed already recorded once
      generate_all_interacts: GenerateAll gives every type every codec usage, so under it a one-directional foreign field is always reported; that is the honest answer, since asking for every codec for a type that cannot carry one is a real conflict rather than something to skip silently
    encode: an AppendJSONTo call on the field into the same dst, which composes at any depth because it appends into the destination the parent is already building
    decode: the raw sub-slice the nested-struct case already lifts out, so the second scan the decode half costs at depth is one this path was paying anyway
    streaming: the parser walk cannot be joined by a method taking a complete document, so it captures through Parser.RawValue, which is where that second scan is actually added
    is_composite: a foreign field counts, so the binder reads it from the body and never from the query
  omitempty_refused:
    what: omitempty and omitzero on a foreign field are a generation error naming the field, the option, and the type
    why: both ask this module whether the value is empty, and a type carrying its own codec is one whose shape it never read
    not_treated_as_never_empty: silently writing a member the author asked to omit is the failure they would find last
  tests: jsonbind and root package tests over each entry point in both directions, generator/foreign_field_test.go over the emitted calls, a compiled cross-package round trip, the refused option, a foreign type carrying no method staying dropped, each one-directional type accepted by a parent needing only that direction, and each reported by a parent needing both
not_built_yet:
  collections: a slice or map whose element is a foreign type is still dropped, since fieldTypeKind admits only scalars and planned structs as elements
  collections_were_not_fixed_by_the_cbor_mode: the removed CBOR codec pass admitted them through its own go/types collector rather than through fieldTypeKind, so nothing here changed and this gap is still open
source:
  - maintainer proposal 2026-08-13
  - requirement:typed-server-action the_result_type_must_be_declared_in_the_route_package
  - concept:standalone-json-codec type_path.fallback
review_gate: proposed
today:
  no_interface: jsonbind exports buffer helpers, read limits, and the generic pair, and no interface type at all
  free_functions: the generator emits appendUserJSON as a package function, so a generated type satisfies nothing and advertises nothing
  registry_only: rule:usage-directed-generation and the sync.Map of registry.go are the whole dispatch story, and both are keyed on a type this run planned
  consequence: encodability is a property of having been analyzed, which no type outside the analyzed package can acquire
model:
  decided: 2026-08-13, by the maintainer; the module declares its own interface rather than adopting a standard one, and prefers it over any other
  contract: a method appending the type's JSON to a destination slice and returning the extended slice
  shape: 'the emitted appendUserJSON already has that signature, so the interface is the method form of what generation writes today'
  name: AppendJSONTo, settled 2026-08-13
  name_history: MarshalToWriter was the first sketch and was dropped because a name saying Writer for a method taking and returning a byte slice reads as io.Writer to everyone who meets it before the doc comment
  name_agrees_with_the_shape: the verb is what the method does and the suffix says where it goes, which is the vocabulary jsonbind AppendString and AppendInt already use
  satisfied_by: a generated codec, and equally a hand-written method in a package this module never sees
  used_by: any emitter or runtime holding a value it cannot plan, per the call sites below
why_append_and_not_marshaljson:
  v1_cost: 'encoding/json Marshaler returns []byte, so every value allocates and the buffer pooling of jsonbind GetBuffer is undone at each nested field'
  v2_shape: encoding/json/v2 writes into an encoder rather than returning bytes, which is the same intent as appending into a destination
  alignment_exists_already: concept:standalone-json-codec encoding already follows json/v2 for nil slices, omitempty, and omitzero, so following it here is consistent rather than new
  own_interface_anyway: an append-into-a-slice method is what this module already generates and what its buffer pooling is built around, so declaring it costs nothing and depends on nothing
standard_interfaces_wait:
  decided: 2026-08-13, by the maintainer
  rule: recognize no standard JSON interface until encoding/json/v2 ships without the experiment, targeted at Go 1.27
  why: an interface behind a GOEXPERIMENT is not a stable thing to write into a public API, and its exact spelling can still move
  what_is_deferred_with_it: whether a standard method is recognized at all, and which of the two v2 forms
  v1_and_v2_are_not_two_questions:
    reported: maintainer 2026-08-13, as the plan rather than as shipped fact
    plan: v1 MarshalJSON is to be declared as a type alias of the v2 one, so the two spellings are one interface
    consequence: the deferred decision is not v1 or v2 or both; it is whether to recognize the allocating byte-returning form, the encoder-writing form, or each
    worth_rechecking: at the release, since a plan is what this is
  cost_of_waiting: a type carrying only a standard method is not encodable in the meantime, which the declaration of requirement:declared-json-codec answers for any package whose source is available
  not_blocked_by_it: this requirement ships on its own interface and needs nothing from that release
precedence:
  order: this module's own interface first, then a codec the run planned, then whatever standard interfaces are recognized once they arrive
  own_first: this is the maintainer's "prefer it", and it is also what keeps the answer stable when a standard form is added later
  method_over_plan:
    rule: a type carrying the method is encoded through it even when the run also planned that type
    why: generating a codec for a type whose author wrote an encoder, and then using the generated one, silently produces bytes the author did not intend
    precedent: encoding/json resolves the same conflict the same way, letting MarshalJSON win over field walking
    consequence: the tag semantics of concept:standalone-json-codec are opted out of by writing the method, which is the trade below
  no_runtime_branch: the binding phase type-checks, so this order is resolved at generation and the emitted call names one path
not_reflection:
  what: a type assertion to an interface is dispatch, not field walking
  therefore: decision:reflection-free is unaffected and a TinyGo target keeps working
  contrast: this is exactly why the interface can do what analysis cannot; the type carries its own encoder rather than the generator deriving one
what_it_unlocks:
  cross_package_result: requirement:typed-server-action can encode a result type declared anywhere, provided that type satisfies the interface, with no cross-package type planning
  who_generates_it: the declaring package generates its own codec, per requirement:declared-json-codec, so the two proposals compose into one answer
  decided_at_generation: the binding phase type-checks and therefore knows whether a type satisfies the interface, so the wrapper is emitted against the method or the planned function with no runtime branch
  the_open_fallback: concept:standalone-json-codec type_path.fallback has carried the unregistered-T question unanswered; this is the answer, and an unregistered T that satisfies nothing stays the error it is today
appendany_encodes_a_user_type_as_null:
  found: 2026-08-13
  what: jsonbind AppendAny is a type switch over scalars and the shapes the parser yields, whose default arm appends null rather than failing
  effect: a user type reaching a rest map is silently written as null, which is a wrong document rather than a reported one
  fix_is_free_here: one interface case in that switch, ahead of the default, encodes any type carrying the method
  worth_stating: this is a live defect the interface fixes on the way past, not a motivation for it
decode_side:
  mirror: the same treatment for the decoding half, taking a complete byte slice and filling the receiver
  receiver: a decoder needs a settable value, so the method is on the pointer and the interface is satisfied by *User rather than User
  scope: worth deciding together, because a result type that encodes and an argument type that decodes are the two halves requirement:typed-server-action needs
  name:
    settled: DecodeJSONFrom, 2026-08-13
    pairs_with: AppendJSONTo, as a To and From pair keeping the format in both names
    dropped: DecodeFromBytes, which had real precedent in the DecodeJSONBytes and decodeUserBytes spellings but named the source where its counterpart named the format
    bytes_stays_available: if a reader-taking or parser-taking interface is ever added beside this one, the suffix is free to distinguish them, which is the distinction DecodeJSON and DecodeJSONBytes already draw for the functions
  the_two_halves_are_not_symmetric:
    found: 2026-08-13, reading generator/emit.go
    encode: 'appendUserJSON(dst []byte, v User) []byte maps to the method with no change of shape, so emitting it is a one-line delegation'
    decode: 'decodeUserBytes(data []byte) (User, error) returns a value, while the method must fill a receiver and return only an error, so emitting it converts the shape rather than forwarding it'
    still_small: the conversion is one assignment, but it means the decode method is not simply the generated function under another name
  depth_is_where_it_really_differs:
    decided: 2026-08-13; encoding reaches any depth unconditionally, decoding reaches depth on a condition
    encode: appending into the shared destination composes at any depth, so a nested field of an interface-satisfying type needs nothing special
    decode_root: always, since the whole document is the byte slice the method takes
    decode_nested: 'the nested path walks a jsonbind.Parser, and a method taking a complete byte slice joins that walk only by being handed a sub-document'
    the_condition: the parser captures the raw sub-slice for that field, as AppendRaw and the rest-field path already deal in, and the region is scanned twice
    paid_per_field_not_per_document:
      why: the binding phase type-checks, so it knows at generation which field types satisfy the interface and emits the capturing path for those fields alone
      consequence: a project using the interface nowhere, or only at a root, pays nothing, which is the same usage-directed shape rule:usage-directed-generation applies everywhere else
      not_a_runtime_fallback: no type switch and no probing; the emitted decoder names one path per field
    sequencing_note: this weakens the earlier case for shipping the encode half first, since the decode condition is a known emission path rather than an unbounded cost
the_generator_emits_it_only_when_declared:
  decided: 2026-08-13, by the maintainer
  rule: a codec reached through a discovered call gets the free functions it gets today; the method appears only for a type the declaration of requirement:declared-json-codec names
  what: generated codecs gain the method beside the function they already emit, for declared types alone
  gain: a generated type is interoperable outward, so another module holding it can encode it without regenerating anything
  why_the_declaration_is_the_right_switch:
    it_is_the_same_statement: declaring a codec for a type nothing in this package calls is already saying the type is used from somewhere this analysis cannot see, which is exactly when the method is what makes it reachable
    not_a_global_setting: a project-wide flag would put a method on every planned type to serve the few that cross a boundary
    directions_carry_over: a declaration naming encode only emits AppendJSONTo only, so the method set follows the declared directions rather than being all or nothing
  compatibility: a project declaring nothing emits the bytes it emits today, so code size on a TinyGo target is unchanged by this requirement existing
  rule_still_relevant: rule:transport-dead-code-elimination, since a declared method that a given binary never calls is what a linker has to drop
the_contract_is_unverifiable:
  what: a hand-written method is not checked against the tag semantics of concept:standalone-json-codec, so omitempty, omitzero, sorted map keys, and the wire-name derivation become the implementer's responsibility
  consequence: two types can be encoded by this module and disagree about what a zero value means on the wire
  same_trade_as: requirement:template-client-handlers the_unverifiable_dependency, where the module accepts a fact it cannot check because the alternative is refusing the feature
  mitigation: the interface is an escape hatch rather than the ordinary path, and documentation says which semantics an implementer is opting out of
constraints:
  - no reflection anywhere on the path
  - jsonbind stays a transport-neutral leaf, per decision:runtime-package-boundaries
  - a type carrying neither a plan nor the method is rejected exactly as it is today
acceptance:
  - a type in another package carrying the method encodes through the same call a planned type does
  - a declared codec satisfies the interface with no hand-written code
  - a codec reached only through a discovered call emits no method, and that project regenerates byte for byte
  - a type that is both planned and carries the method is encoded through the method
  - a type carrying only a standard library JSON method is rejected today, and that rejection is what the deferred decision revisits
  - a user type inside a rest map encodes as its own document rather than as null
  - a nested field whose type carries the encode method encodes at that depth
  - a nested field whose type carries the decode method decodes at that depth, through a captured sub-document
  - a project whose interface-satisfying types appear only at a root emits no capturing path
  - a TinyGo build of a project using the interface links and runs
  - a type carrying neither is still an error, with the message it has today
related:
  - concept:standalone-json-codec
  - requirement:declared-json-codec
  - requirement:typed-server-action
  - decision:reflection-free
  - rule:usage-directed-generation
  - api:encode-json
  - api:decode-json
open_questions:
  - whether the two interfaces are named types on the jsonbind surface or inline anonymous constraints at each use, given that the method names are now settled and a named type is what a doc comment can point at
```
