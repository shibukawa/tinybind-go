---
id: requirement:fixed-length-array-fields
type: requirement
title: Fixed Length Array Fields
---
Admit a fixed-length Go array as a field kind, filling it in place from the wire: a short array leaves the tail at the zero value, and a long one is an error naming the field, so a declared length is a contract rather than a silent truncation.

```yaml
priority: should
status: implemented 2026-08-24
review_gate: proposed
source:
  - maintainer 2026-08-24, asking whether cborbind and jsonbind support only slices
  - the defect below, found the same day by generating a fixture and building it
the_defect:
  what: generator/plan.go fieldTypeKind matched *ast.ArrayType and never read Len
  but: '[]T and [N]T are the same AST node, and only Len tells them apart'
  result: a fixed-length field was planned as KindSlice, so the decoder assigned a []int to a [4]int
  failure_shape: generated source that does not compile, inside a file headed DO NOT EDIT, with no diagnostic from the generator
  encode_half_compiled: len and range read an array exactly as they read a slice, so an encode-only project shipped and only a generated decoder broke
  reach: jsonbind codec, httpbind JSON binder, cborbind codecs and the cbor-http mode, which all read the one field plan
  documentation_disagreed: docs/cborbind.md promised a generation error naming the type and the field for anything unsupported
  same_lesson_as: rule:named-type-field-kind the_defect, another AST arm that mapped a shape it could not emit
semantics_of_the_two_ends:
  decided: 2026-08-24, by the maintainer
  short: fill what arrived and leave the rest at the zero value
  why_short_is_not_an_error: the length is the Go type's statement, and a document does not have to restate it
  long: an error naming the field
  why_long_is: keeping the first N elements drops the tail, which is the silent data loss a declared length exists to prevent
  null: leaves the destination untouched, as a slice member already does
  repeated_member: decodes to the second array, not to the two overlaid, which is why the tail is zeroed rather than left alone
  encode: the full length every time, since an array has no nil form to collapse; it shares the slice arm unchanged
as_built:
  planner: 'generator/plan.go KindArray, FieldPlan.ArrayLen, IsComposite and GoType; fieldTypeKind splits on Len and the two kinds differ only past that point'
  array_len_is_source_text: types.ExprString of the length expression, so a generated type spells [seatCount]uint8 the way the source did rather than resolving the constant behind the author's back
  read_off_the_field_node: KindArray is produced only at the top level, since an array nested in a slice or a map is refused, so analyzeField re-reads the node instead of widening the fieldTypeKind return
  json_emitter: 'generator/emit.go emitStreamArray serves both the codec walk and the binder walk, which differ only in the member name'
  cbor_emitter: 'generator/cborhttp_emit.go emitCBORReadValue KindArray arm; the encode arm is shared with KindSlice'
  tag_gates: check.go isComposite, default.go and enum.go each gained the kind, so a check, default or enum tag on an array is refused the way it is on a slice
  tests: generator/fixed_array_test.go, two compiled fixtures covering the HTTP binder with cbor-http on and the standalone jsonbind and cborbind codecs
  compatibility: verified 2026-08-24 by regenerating both examples; every generated body is byte for byte what it was, since each added arm is reached only by a kind that used to be planned wrong, and only the inputs fingerprint moves, which any generator change moves
runtime_added:
  json: 'jsonbind.ParseArray(p, field, message, dst []T, read) error and jsonbind.ErrArrayTooLong'
  destination_is_a_slice_over_the_array: the emitter passes out.Field[:], so one helper serves every length and neither the helper nor the generated line carries a length of its own
  cbor_needed_nothing: the count is in the array header, so the generated code compares it against len(dest) directly
error_values:
  json: jsonbind.FieldError naming the member, with ErrArrayTooLong as its cause and the limit in the message; over HTTP the binder surfaces it as the 400 a bad member already produces
  cbor_plain: cbor.ErrLimitExceeded, which both the cborbind codecs and the standalone cbor-http decoder can name because both already import the driver
  cbor_binder: httpbind.BindError naming the field, so a too-long CBOR array is a 400 like every other bad payload member
  why_the_plain_shape_needs_its_own_value: cborPlainErrRet hands up the err in scope, and there is none at a count comparison; the overflow travels as a distinguished what rather than as a second error function
refused_rather_than_mishandled:
  named_element: 'an array of a named scalar, for the reason rule:named-type-field-kind collections_are_refused gives for a slice; the diagnostic names the array and spells the fix as [N]underlying'
  omitzero_on_an_array_of_structs: a Go array is comparable only when its element is, and a struct in this plan may hold a slice or a map, so the comparison would be a compile error in the generated file
  the_same_array_inside_an_isZero_helper: left out of the conjunction, which is the convention a foreign field already follows there
  nested_array: an array inside a slice or a map, since the field plan carries one element kind and no second level
  named_array_type: unchanged from resolveNamedKind, which admits a basic or struct underlying and nothing else
not_in_scope:
  openapi: schemaForKind documents a slice as a string today, so a fixed array is documented the same way; giving only the array a maxItems would describe a document the slice half does not
  sqlbind: emitSQLIndexDecls and emitSQLFill key on KindSlice, so an array field is not filled from a row; that mapping is its own decision
  query_binding: a slice or map binds from the body only, and an array follows it
  byte_element: 'settled the same day by requirement:byte-and-rune-field-kinds and decision:byte-slices-are-base64: [4]byte was refused when this landed, and is now a base64 blob that keeps the two-ended contract above, measured in bytes rather than elements
acceptance:
  - a struct holding [N]scalar, [N]string, [N]Struct and a constant-length array binds over HTTP, round-trips through JSON, and compiles
  - a document carrying fewer elements than the length leaves the tail at the zero value
  - a document carrying more elements than the length is a 400 naming the field, and errors.Is finds ErrArrayTooLong on the codec path
  - a null member leaves the field alone and a repeated member does not overlay
  - the same three cases hold over CBOR, through the cbor-http mode and through the cborbind codecs
  - an array of a named scalar and omitzero on an array of structs are refused with a diagnostic naming the field
  - a project with no array field regenerates byte for byte
related:
  - rule:named-type-field-kind
  - requirement:sized-integer-field-kinds
  - requirement:cbor-codec-generation
  - requirement:cbor-http-body
  - concept:standalone-json-codec
  - rule:usage-directed-generation
  - rule:generated-source-not-discovered
  - api:bind
open_questions:
  - whether the OpenAPI document should describe a slice and an array as arrays at all, which is one decision covering both kinds
```
