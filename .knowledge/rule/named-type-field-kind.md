---
id: rule:named-type-field-kind
type: rule
title: Named Type Field Kind
---
A field whose type is a same-package named type binds and encodes as that type's underlying kind, and is refused when the underlying kind is one nothing can be mapped to.

```yaml
status: implemented 2026-08-14
source:
  - defect found 2026-08-13 while reviewing requirement:typed-server-action parameter types
  - rule:same-package-convention
the_defect:
  what: fieldTypeKind mapped every same-package identifier that was not a predeclared scalar to KindStruct, naming that identifier as the nested type
  but: analyzeStruct collects only struct type declarations, so a named scalar never entered the plan and no codec was emitted for it
  result: 'generation named appendUserIDJSON, decodeUserIDBytes and decodeUserIDJSON and defined none of them'
  failure_shape: generated source that does not compile, on an undefined identifier, inside a file headed DO NOT EDIT, with no diagnostic from the generator
  reach: any request or response model holding such a field, so it belonged to the codec generator rather than to any one feature
  same_lesson_as: rule:generated-source-not-discovered transform_was_missed, where a diagnostic also named a function the generator itself wrote
chosen:
  what: resolve the underlying type
  why: it is what encoding/json does with a named type and what an author declaring an ID type expects; a named string on the wire is a string
  available_because: this analysis path loads go/types, so the underlying type is there to read, unlike routetree which parses before the package compiles
  rejected_plan_a_codec_per_named_type: a generated appendUserIDJSON for a string is code and indirection bought for nothing
  rejected_refuse_everything: a named ID type is ordinary Go, and refusing it would take those models out of reach entirely
resolution:
  basic_underlying: the field takes that scalar kind, and the declared name is carried so generated code converts in both directions
  struct_underlying: KindStruct, which is what the old assumption got right
  anything_else: a generation error naming the field, the type and what it is underneath
  no_type_information: the old assumption stands, since an analysis without a loaded package has nothing better and the ordinary case is a struct
conversion:
  why_needed: Go has no implicit conversion between a named type and its underlying one, so a codec working in the underlying kind cannot assign to the field or read from it
  read: the field converted to its underlying kind, for an encoder consuming it
  write: the decoded value converted to the declared type, for every assigning path
  sites: the encoder, the document decoder, and every binder source — query, form, path, header and method
  untyped_constants_need_none: a default tag emits a literal, and an untyped constant assigns to a named type as it stands
collections_are_refused:
  what: a slice or map whose element is a named scalar is a generation error
  why: the bulk decoders answer a concrete []string or map[string]int, which Go will not assign to a slice or map of the named type
  cost_of_supporting: a conversion loop per element kind in every decoding path, which is roughly the size of the rest of this again
  not_a_regression: the same shape produced a call to a codec nothing defined before this, so the change is from broken output to a diagnostic
  the_diagnostic_says_the_fix: declare it as the underlying element type
compatibility:
  bytes: a project whose fields are all written as predeclared types emits identical output, since both conversions are the identity there
  evidence: every golden fixture and both page trees pass unchanged
verification:
  the_test_is_a_compile: a named scalar reaches the encoder, the decoder and five binder sources, and a site that forgets a conversion emits source that does not build, which no substring assertion would catch
  wire_form: a round trip proves a named scalar appears as its underlying kind rather than merely compiling
  both_were_checked_against_a_wrong_expectation_first: so they fail when they should
related:
  - concept:standalone-json-codec
  - rule:usage-directed-generation
  - rule:same-package-convention
  - requirement:typed-server-action
answered_2026_08_22:
  question: whether a slice or map of a named scalar is worth the conversion loops, which is the one shape this refuses rather than maps
  answer: requirement:sized-integer-field-kinds pays for the same loop as an emitted element-reader closure, written once as a shape rather than once per element kind, so the cost that justified this refusal is already being paid there
  status: proposed to lift with that change; the refusal stands until it lands
open_questions:
  - whether a named type whose underlying is a named type of another package should follow requirement:json-codec-interface instead
```
