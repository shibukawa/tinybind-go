---
id: requirement:byte-and-rune-field-kinds
type: requirement
title: Byte And Rune Field Kinds
---
Admit byte and rune as field types, mapping them to the widths they are alternative names for, so a field declared the way Go declares one is not refused for its spelling.

```yaml
priority: should
status: implemented 2026-08-24
review_gate: proposed
source:
  - found 2026-08-24 while checking what requirement:fixed-length-array-fields did to a [4]byte field
  - the wrong claim in requirement:sized-integer-field-kinds, corrected by this
the_defect:
  what: fieldTypeKind dispatches on the identifier as written, and neither byte nor rune is one of the names it matched
  so: the field fell through to the same-package named-type lookup, matched no *types.Named, and was refused
  the_claim_it_breaks: 'requirement:sized-integer-field-kinds recorded "byte and rune resolve to uint8 and int32 through requirement:alias-transparent-type-analysis", which was never true'
  why_the_claim_was_wrong: that requirement unwraps *types.Alias, the node a user-declared alias produces since Go 1.24; byte and rune are predeclared alternative names for one *types.Basic and produce no such node, so nothing upstream ever rewrote them
  the_tell_nobody_noticed: 'a named type over byte was accepted -- resolveNamedKind reads Underlying() and gets uint8 -- so the workaround was to add a type declaration, which is backwards'
  shapes_refused: 'byte, rune, []byte, [N]byte, []rune, map[string]byte; []uint8 and []int32 were accepted the whole time, which is the same type in every case'
as_built:
  planner: generator/plan.go predeclaredAliasKind, called from the identifier arm before the scalar test
  no_conversion_follows: the Go spec makes byte and uint8 one type and rune and int32 one type, so the resolved name describes the field exactly and generated code assigns across it without a cast
  which_is_why_named_stays_empty: nothing about the field is a named type, unlike rule:named-type-field-kind where the declared name has to be carried
  reach: 'one arm covers every shape at once, since the collections resolve their element through the same call'
  tests: generator/byte_field_test.go
also_fixed_the_diagnostic:
  what: 'resolveNamedKind returned a zero NamedKind for anything that was not a *types.Named, so the refusal read "type byte is  underneath" with a hole where the underlying type belongs'
  survives_this: the hole was reachable for any unmappable predeclared type, complex128 and uintptr among them, so it is closed rather than merely stepped around
  now: Underlying is filled from the type itself, and the message drops the underneath clause when it would only repeat the name
  why_it_matters_here: a diagnostic is what a refusal exists to be, and one with a gap in it reads as a generator bug rather than as a fact about the field
rune_needed_no_wire_decision:
  what: 'a rune field is a number and []rune is an array of numbers'
  agrees_with: encoding/json, which writes an int32 slice exactly that way
  so: only the byte sequence needed a choice, and decision:byte-slices-are-base64 is it
not_in_scope:
  bare_byte_stays_a_number: 'a byte field on its own is a number, as encoding/json writes one; only a sequence of them is a blob'
  map_of_byte: 'map[string]byte stays a map of numbers, since its values are single bytes rather than a sequence'
  slice_of_blobs: '[][]byte is dropped from the plan without a word, the same silence [][]string already gets; the field plan carries one element kind and no second level, and breaking that silence is its own change'
  form_binding: 'a multipart or urlencoded form field does not carry a byte sequence; every other composite is skipped there too, and admitting one is its own change
acceptance:
  - a field declared byte or rune binds, encodes and decodes as uint8 or int32
  - '[]rune round-trips as an array of numbers, byte-identical to encoding/json'
  - an unmappable predeclared type is refused with a message that names it and has no blank in it
  - an unmappable named type still says what it is underneath
  - a byte field bound from a query, path, header or cookie decodes as base64, per decision:byte-slices-are-base64
  - a project with no byte or rune field regenerates byte for byte
related:
  - decision:byte-slices-are-base64
  - requirement:sized-integer-field-kinds
  - requirement:alias-transparent-type-analysis
  - requirement:fixed-length-array-fields
  - rule:named-type-field-kind
  - concept:standalone-json-codec
```
