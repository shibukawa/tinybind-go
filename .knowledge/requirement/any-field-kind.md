---
id: requirement:any-field-kind
type: requirement
title: An interface{} Field Is Supported Or Refused, Never Dropped
---
Give a struct field of type any or interface{} a field kind of its own, decoded through Parser.Any and encoded through AppendAny, and until that lands refuse it with a diagnostic; today it vanishes from both codecs without a word.

```yaml
priority: should
source:
  - system:odjson harness 2026-09-12: the Twitter struct's fourteen interface{} fields were omitted from encode and decode, and the output lost 17% of its bytes with no diagnostic
review_gate: proposed
status: implemented 2026-09-12; see as_built
as_built:
  kind: KindAny in generator/plan.go, from the identifier any and from an empty *ast.InterfaceType; also as a slice, array and map element, and a map[string]any without payload:"*" is now an ordinary map rather than a silent drop
  decode: Parser.Any through emitStreamAssign and emitBinderStreamAssign; an element through parserMethod
  encode: jsonbind.AppendAny; omitempty and omitzero treat a nil any as empty
  refused: error stays refused, by name; cborbind refuses an any field or element by name
  openapi: the empty schema
  test: generator/any_field_test.go, including a compiled round trip against encoding/json
  measured: the odjson Twitter fixture round-trips completely; its fourteen interface{} fields cost the decode 4349 allocations against 2211 without them, which is what any means
problem:
  where: generator/plan.go fieldTypeKind returns ok=false and no error for the identifiers any and error, and analyzeStruct then leaves the field out of the plan
  effect: a document member arrives and is skipped; a value set in Go is never written; the OpenAPI schema omits the property; nothing tells the author
  contrast: a foreign type or a rest map without payload:"*" is refused with a named error, so silence here is the odd one out
direction:
  support_it: the runtime already has both halves for the rest map, Parser.Any producing encoding/json's shapes and AppendAny writing them back, so a KindAny field is emission plumbing rather than new parsing; OpenAPI writes an empty schema, which is what any is
  refuse_error: the error type stays refused, with a diagnostic naming the field; there is no document shape for it
  map_and_slice_elements: '[]any and map[string]any as ordinary fields follow the same kind; the rest map stays the special case it is'
  keep_silent_drop_for: nothing; json:"-" is the one way a field leaves the codec, and it is written by the author
if_support_waits: fieldTypeKind returns an error for any the way it does for a foreign alias, so checkUnmappable reports it for a struct something asks to map; that is a one-line change and makes the harness case a generation failure instead of a wrong document
tinygo: Parser.Any allocates maps and slices, which is what any means; the field is the author's choice, and a codec without it stays as small as it is now
related:
  - concept:standalone-json-codec
  - api:decode-json
  - api:encode-json
```
