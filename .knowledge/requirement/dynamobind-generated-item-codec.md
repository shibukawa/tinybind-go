---
id: requirement:dynamobind-generated-item-codec
type: requirement
title: Generated DynamoDB Item Codec
---
For each tagged type, emit an item encoder, an item decoder, and a key builder into the user package, with compile-time assertions that they satisfy the runtime interfaces.

```yaml
status: required
implemented: 2026-07-31
input:
  directive: "go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir ."
  declaration: a Go struct whose fields carry rule:dynamo-tag-options tags
output_per_type:
  - "func (r Reading) EncodeItem() dynamodb.Item"
  - "func (r *Reading) DecodeItem(item dynamodb.Item) error"
  - "func (r Reading) ItemKey() dynamodb.Key"; only when key tags are present
  - "func ReadingTable(name string) dynamodb.TableDefinition"; per decision:dynamobind-table-definition
assertions:
  - "var _ dynamobind.ItemEncoder = Reading{}"
  - "var _ dynamobind.ItemDecoder = (*Reading)(nil)"
  reason: a stale generated file becomes a build failure instead of a silently wrong codec
mapping: data:dynamodb-attribute-mapping
validation: rule:dynamo-tag-options
usage_direction:
  policy: rule:usage-directed-generation
  Store or StoreAll call: EncodeItem plus nested encoders
  Load, LoadAll, Query, QueryPage or Scan call: DecodeItem plus nested decoders
  StoreReturning call: EncodeItem and DecodeItem
  RemoveReturning call: ItemKey and DecodeItem
  registration: none; decision:dynamobind-static-dispatch removes the registry, so no init entry is emitted
key_builder_is_not_usage_directed:
  rule: a bound type declaring a partitionkey gets ItemKey and its table definition, whether or not a discovered call needs them
  reason: the documented read is "Load(ctx, c, table, v.ItemKey())", and using a method is not a call the generator can discover; waiting for a discoverable use would mean the method never existed to be called
  cost: three lines, dropped by the linker when nothing calls them
discovery:
  read_side: the type argument is explicit, since T appears only in the result, so the AST carries it even before any codec exists
  write_side: the type comes from the value argument, not the type parameter
  why: Store, Remove and the rest are constrained by ItemEncoder and Keyer, which the generated code supplies; before the first run the call does not type-check and go/types records no instantiation, while the argument's own type resolves regardless
  consequence: a first generation on a clean checkout finds every call
nested_types:
  - a struct field reached from a tagged type inherits only the operations its parent needs
  - a nested type needs no tag of its own to be reachable, but its fields follow the same tag rules
runtime_ownership:
  - the generated file declares no helper type; shared code lives in dynamobind, per decision:generated-runtime-in-module
  - the generated file imports the driver for dynamodb.Item, dynamodb.Key and the attribute constructors
determinism: same input yields byte-identical output, as with every other generator mode
related:
  - requirement:dynamobind-product-goals
  - data:dynamodb-attribute-mapping
  - rule:dynamo-tag-options
  - decision:dynamobind-static-dispatch
  - decision:dynamobind-table-definition
  - decision:generated-runtime-in-module
  - rule:usage-directed-generation
  - concept:code-generation
```
