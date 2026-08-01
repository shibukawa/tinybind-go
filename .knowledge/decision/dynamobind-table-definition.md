---
id: decision:dynamobind-table-definition
type: decision
title: Emit The Table Definition From The Same Tags
---
Emit a table definition constructor beside the key builder, so the key names in the schema, the codec, and the request come from one declaration.

```yaml
status: accepted
form: "func ReadingTable(name string) dynamodb.TableDefinition"
content:
  Name: the argument
  PartitionKey: "dynamodb.KeyAttribute{Name: <partitionkey attribute>, Type: <S, N or B>}"
  SortKey: "a *dynamodb.KeyAttribute when a sortkey tag exists, otherwise nil"
  BillingMode: left zero; the driver defaults to PayPerRequest
why:
  - it is what makes a key name single-source, per requirement:dynamobind-product-goals
  - tests need to create tables
  - the constructor links to nothing unless it is called; the driver CreateTable is about 22 KB and is dropped when unused
emission:
  default: on, wherever a partitionkey tag exists
  suppression: rule:generator-feature-disable, as a named feature
  cost_when_unused: the constructor is a leaf function; nothing links CreateTable unless the program calls it
deferred: GlobalIndexes and LocalIndexes, until the primary key path is proven
one_table_per_type: assumed here, and it is one of the only two places that assumption lives, per decision:dynamo-single-table-scope
ttl: requirement:dynamo-ttl-attribute would add a TTL specification to this definition, once the driver can apply one
related:
  - requirement:dynamobind-generated-item-codec
  - rule:dynamo-tag-options
  - requirement:dynamobind-product-goals
  - system:tinygodriver-dynamodb
  - rule:generator-feature-disable
```
