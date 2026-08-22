---
id: requirement:sized-integer-field-kinds
type: requirement
title: Sized Integer Field Kinds
---
Admit every fixed-width Go integer as a field kind, encoding and decoding it at its declared width and reporting an out-of-range value rather than truncating it, so a struct can say how wide a field is and every mode agrees.

```yaml
priority: should
status: implemented 2026-08-22
review_gate: proposed
source:
  - maintainer 2026-08-22, asking for sized integers with the CBOR codec work
  - the same blocker found 2026-08-19 while generating the TinyGo CBOR smoke, recorded in the removed cborbind decision at commit 5436485^ and deferred there as a JSON wire-format question
today:
  admitted: 'generator/plan.go fieldTypeKind maps string, int, int64, bool and float64 for a predeclared identifier, and nothing else'
  consequence: a struct holding uint32 is a generation error once anything uses it, so the type cannot be bound, encoded, decoded or documented
  softened_but_not_fixed: checkUnmappable holds the refusal until usage is assigned, so an unused struct no longer fails a whole run; a used one still does
  why_it_blocks_cbor: requirement:cbor-codec-generation array shape exists to carry narrow fields, and narrow fields are exactly what this refuses
kinds_admitted:
  signed: int8, int16, int32, int64, int
  unsigned: uint8, uint16, uint32, uint64, uint
  already_there: int and int64
  aliases: byte and rune resolve to uint8 and int32 through requirement:alias-transparent-type-analysis
  named_types: a named type over any of them takes that kind and converts at every site, unchanged from rule:named-type-field-kind
platform_width_is_admitted_not_refused:
  what: int and uint are 64 bits on a host and 32 under wasm
  position: int is already admitted everywhere, so refusing uint would be inconsistent; both are admitted and neither carries a warning
  and_no_profile_switch: per decision:cbor-shape-is-the-only-axis, a project that wants a fixed width declares one, which is the statement the struct already carries
as_built:
  planner: generator/plan.go intKindBits, isSizedIntKind and intKindBounds; isScalarKind routes through them, so fieldTypeKind admits each width by name
  named_types_needed_widening_too: resolveNamedKind had its own types.Basic switch listing five kinds, so a named type over uint16 was refused with the cannot-map diagnostic even after fieldTypeKind admitted the width; found by the fixture test, not by reading
  emitter: generator/emit.go sizedIntRead, sizedIntElemReader, sizedIntCase and sizedIntLit; a case arm per site rather than ten
  sized_int_case_is_a_switch_trick: a switch on the kind string cannot spell "any of eight" in one case, so sizedIntCase returns the kind when it is sized and a value nothing equals otherwise; documented at the definition because it reads as clever until it is explained
  runtime_added: jsonbind Parser.Uint64 and ErrIntegerRange, httpbind and fasthttpbind ParseIntBits and ParseUintBits, sqlbind Uint64, SignedN and UnsignedN
  sql_took_helpers_rather_than_bounds: emitSQLFill writes one line per field and generated SQL output imports no fmt, so the width check lives in sqlbind SignedN and UnsignedN rather than in emitted code; this is the one place the bound is not a generated literal
  a_generic_scanner_was_tried_first: Signed[T] and Unsigned[T] deriving the width from unsafe.Sizeof, dropped rather than put unsafe into a package a TinyGo target links
  cbor_needed_no_bounds: the driver has ReadInt8 through ReadUint64 answering the exact width and ErrIntegerOverflow, so the emitted call is direct; only int and uint keep a round-trip test, since a platform width has no reader
  openapi: schemaForKind gives each width an integer type with its format and its bounds; without it the default arm called a uint32 a string
  validation_bounds_are_untyped: sizedIntLit emits a bare literal, as intLit already did, which is why a named integer field compares against it with no conversion
  also_widened: emptyExpr and zeroExpr, or omitempty and omitzero would have been silently inert on a sized field
  tests: generator/sized_integer_test.go, five cases including a compiled HTTP round trip of every width through JSON and CBOR, an out-of-range member, slice element, query value and CBOR member, and the OpenAPI bounds
  compatibility_verified: the golden fixtures under internal/ regenerate unchanged, which is the byte-for-byte acceptance
  docs: docs/httpbind.md and its Japanese twin name the widths and say an out-of-range value is a 400
representation:
  chosen: the kind string stays the Go type name, as it already is for int, int64 and float64
  why: the switches dispatch on it, and the error messages quote it, so an author reads the width they declared
  the_cost_it_avoids: a family kind plus a width field would make every emitter do two lookups to say one thing
  shared_helper: one intKind(kind) returning bits, signedness and ok, so each switch gains one arm rather than ten; the roughly twelve scalar switches in generator/emit.go, cborhttp_emit.go, check.go, openapi.go and plan.go are the surface
  do_not_miss: emit.go emptyExpr and zeroExpr carry their own int, int64, float64 arm, so omitempty and omitzero on a sized field are silently inert until they are widened too
encode:
  json_signed: jsonbind.AppendInt with an int64 conversion, which is what int already does
  json_unsigned: jsonbind.AppendUint, which already exists and is unused by any emitter today
  cbor: cbor.AppendInt and cbor.AppendUint, both already in the driver
  no_new_runtime_surface_on_this_side: verified 2026-08-22 by reading jsonbind/append.go and system:tinygodriver-cbor
decode_range_is_checked_never_truncated:
  must: a value outside the declared width is an error naming the field, not a wrapped or truncated number
  why: a silent truncation is a wrong value that survives into storage, which is the failure class rule:generated-source-not-discovered and the JSON codec work both keep choosing a diagnostic over
  json:
    today: Parser has Int, Int64, Float64, Bool and String, none width-checked and none unsigned
    add: Parser.Uint64 only, and let the generator emit one bounds check per narrow field against the width's limits
    why_one_method_and_not_eight: the bounds are constants the emitter already knows, so eight parser methods would move a comparison into the runtime and grow the TinyGo surface for nothing
    error: jsonbind.FieldError naming the member, the same shape a malformed number already produces
  binder:
    add: 'httpbind.ParseIntBits(s string, bits int) (int64, error) and ParseUintBits(s string, bits int) (uint64, error), over strconv.ParseInt and ParseUint'
    error: httpbind.BindError naming the field and location, so an out-of-range query value is 400 rather than a wrapped number
    parity: each helper gets its fasthttpbind twin in the same change, per the standing pair-wise constraint of decision:fasthttpbind-runtime-package
  cbor:
    nothing_to_add: the driver already has ReadInt8 through ReadInt64 and ReadUint8 through ReadUint64, and reports ErrIntegerOverflow for a value too wide for the declared width
    which_is_why_this_lands_with_the_codec_work: the format that most needs the widths is the one already able to enforce them
openapi:
  today: schemaForKind maps int and int64 to integer and falls through to string for anything else, so a sized field would be documented as a string
  add: 'integer with format int32 for widths up to 32 bits and int64 above, plus minimum and maximum for every width narrower than the format, and minimum 0 for unsigned'
  why_the_bounds: the binder enforces a range, and a document that does not state it describes a different API than the one running
other_modes:
  firestore_query_values: generator/firestorequery_plan.go already accepts int8 through uint32, so that path is ahead of the field planner and stops disagreeing with it
  sql_scan_dynamo_item_firestore_entity: each has its own scalar mapping; widening them is separate work and is not required for this
  configbind: decision:configbind-supported-types owns its own type set and is untouched
collections_are_admitted:
  decided: 2026-08-22, by the maintainer
  what: a slice or a map whose element is a sized integer is generated, not refused
  json_mechanism: jsonbind ParseSlice and ParseMap are already generic over the element, so the only obstacle is streamElemReader handing them a method expression; for a width with no one-to-one parser method the emitter writes a closure literal instead, which reads Uint64 or Int64, checks the width's bounds and converts
  the_closure_is_where_the_range_check_lives: per element, with the bound emitted as a literal, so generated code needs no math import
  cbor_mechanism: the driver has ReadUint32 and its siblings returning the exact width already, so an element reader is a direct call and the slice case is cheaper here than in JSON
  encode_is_free: emitAppendValue already recurses into the element with the element kind, so the scalar arms cover the collection once they exist
  also_widen: emit.go supportedElemKind, which is the gate that otherwise drops the assignment silently
  binder_is_not_affected: a slice or map binds from the body only, never from a query value, so no width-aware string parse is needed for the collection case
and_this_answers_the_named_scalar_slice:
  standing_refusal: rule:named-type-field-kind collections_are_refused rejects a slice of a named scalar because the bulk decoder answers a concrete []string that Go will not assign to a slice of the named type, and it costs a conversion loop per element kind
  what_changed: the emitted closure is that conversion loop, written once as a shape rather than once per element kind, so the cost that justified the refusal is the cost this requirement is already paying
  recommendation: lift that refusal in the same change; it is the open question the rule itself records
  not_folded_in_silently: it changes a shipped refusal into generated code, so it is called out here rather than assumed
uint64_above_the_float64_range_is_allowed:
  decided: 2026-08-22, by the maintainer
  what: no upper bound beyond the type's own; a uint64 field carries its full range in a JSON document
  this_module_is_exact: AppendUint writes the decimal digits and the decoder parses the number span with ParseUint, so a round trip through tinybind loses nothing at any magnitude
  where_the_loss_actually_is: a client parsing JSON numbers as float64, which is most JavaScript; that is a property of the reader, not of the document
  why_not_refuse_it_here: the emitter encodes the value correctly, so a refusal would override a decision the struct already carries, which is the position decision:cbor-shape-is-the-only-axis takes for every other restriction
  what_an_author_who_cares_does: declares the field as a string, which is the same statement in the place that already carries it
compatibility:
  bytes: a project declaring no sized field regenerates byte for byte, since every added arm is reached only by a kind that used to be refused
  no_behaviour_change_for_int: int and int64 keep the exact emission they have, including the int64 conversion on the append path
acceptance:
  - a struct field of each admitted width binds from query, path, header, cookie and form, and round-trips through JSON
  - a JSON number above the declared width is a field error rather than a truncated value, at the root and at depth
  - an out-of-range query value is 400 naming the field
  - a CBOR body carrying a value too wide for the field reports the driver's overflow rather than binding
  - an unsigned field above the signed 64-bit range encodes and decodes exactly, which is the case an int64 pivot would lose
  - the OpenAPI document gives each width an integer type with its format and bounds
  - a named type over a sized integer converts at every site, and a generated file using one compiles
  - a slice and a map of each admitted width round-trip through JSON and through CBOR, and an out-of-range element is a field error naming the member
  - a uint64 field above 2^53 round-trips exactly through this module's own encoder and decoder
  - a project with no sized field regenerates byte for byte
  - a struct of narrow fields generates a CBOR array codec, which is requirement:cbor-codec-generation unblocked
related:
  - requirement:cbor-codec-generation
  - decision:cbor-shape-is-the-only-axis
  - requirement:cbor-http-body
  - rule:named-type-field-kind
  - requirement:alias-transparent-type-analysis
  - concept:standalone-json-codec
  - system:tinygodriver-cbor
  - api:bind
  - decision:fasthttpbind-runtime-package
  - decision:runtime-package-boundaries
  - rule:usage-directed-generation
  - decision:configbind-supported-types
  - requirement:tinygo-wasm
open_questions:
  - whether lifting rule:named-type-field-kind collections_are_refused rides in this change or follows it, which is scope rather than design
```
