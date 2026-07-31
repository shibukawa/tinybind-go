---
id: decision:dynamobind-static-dispatch
type: decision
title: dynamobind Dispatch By Pointer Constraint
---
Dispatch to generated item codecs through a pointer type constraint, with no registry and no reflect lookup.

```yaml
status: accepted
problem: jsonbind resolves decoders by reflect.TypeFor[T] through a registry, so a missing registration fails at run time
rule:
  - the decode side takes "PT interface { *T; ItemDecoder }" and calls PT(&out).DecodeItem
  - the encode side constrains T to ItemEncoder directly
  - a type without generated code fails to compile
  - no registry, no init-time registration, no reflect
interfaces:
  ItemEncoder: "EncodeItem() dynamodb.Item"; value receiver
  ItemDecoder: "DecodeItem(dynamodb.Item) error"; pointer receiver
  Keyer: "ItemKey() dynamodb.Key"; emitted only when key tags are present
inference:
  fact: PT is inferred from T, so the call site writes one type argument
  example: "dynamobind.Load[Reading](ctx, client, \"readings\", in.ItemKey())"
measured_tinygo_0_41_1:
  compiles_and_links: true
  size_over_hand_written_map: +1,392 bytes
  size_of_reflection_marshaler: +23,696 bytes
  reflect_symbols: 87, unchanged either way
consequence:
  - no generated init function is needed for DynamoDB-only output
  - rule:usage-directed-generation still decides which methods are emitted, but registration has nothing to add
related:
  - decision:reflection-free
  - decision:dynamobind-runtime-package
  - api:dynamobind-operations
  - requirement:dynamobind-generated-item-codec
  - requirement:dynamobind-verification
```
