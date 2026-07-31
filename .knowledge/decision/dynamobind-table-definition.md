---
id: decision:dynamobind-table-definition
type: decision
title: Emit The Table Definition From The Same Tags
---
Emit a table definition constructor beside the key builder, so the key names in the schema, the codec, and the request come from one declaration.

```yaml
status: accepted
decided: user 2026-07-31; emit it
form: "func ReadingTable(name string) dynamodb.TableDefinition"
content:
  Name: the argument
  PartitionKey: "dynamodb.KeyAttribute{Name: <partitionkey attribute>, Type: <S, N or B>}"
  SortKey: "a *dynamodb.KeyAttribute when a sortkey tag exists, otherwise nil"
  BillingMode: left zero; the driver defaults to PayPerRequest
value: this is what closes the drift described in requirement:dynamobind-product-goals; without it the key name is still a string repeated in three places
against:
  - table creation is arguably CloudFormation or Terraform territory
  - the driver CreateTable is dropped by the linker when unused; measured 22,320 bytes, present only when called
for:
  - tests need to create tables
  - key names stay single-source even for a program that never calls CreateTable
  - the constructor itself links to nothing unless it is called
emission:
  default: on, wherever a partitionkey tag exists
  suppression: rule:generator-feature-disable, as a named feature
  cost_when_unused: the constructor is a leaf function; nothing links CreateTable unless the program calls it
deferred: GlobalIndexes and LocalIndexes, until the primary key path is proven
related:
  - requirement:dynamobind-generated-item-codec
  - rule:dynamo-tag-options
  - requirement:dynamobind-product-goals
  - system:tinygodriver-dynamodb
  - rule:generator-feature-disable
```
