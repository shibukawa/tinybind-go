---
id: rule:dynamo-tag-options
type: rule
title: dynamo Tag Options And Generation Checks
---
The dynamo tag names the attribute and its options; an option the generator does not know is a generation error, never a silently ignored string.

```yaml
status: accepted
decided: user 2026-07-31
tag:
  spelling: dynamo
  reason: it matches the house style, where a tag is short and names its purpose, as with check, db, opt, enum, query, payload and groupkey
  rejected: dynamodbav, the aws-sdk-go-v2 and driver reflection spelling; it would have let one struct work with or without generation, at the cost of a tag named after a library rather than a purpose
form: "`dynamo:\"<name>[,<option>...]\"`"
name:
  empty: the field name is used
  "-": the field is skipped
options:
  partitionkey: the field is the table partition key
  sortkey: the field is the table sort key
  omitempty: the attribute is omitted when the field is the zero value
  stringset: encode a slice as SS
  numberset: encode a slice as NS
  binaryset: encode a slice as BS
  unixtime: encode a time.Time as N seconds since epoch
unknown_option:
  behavior: generation error naming the field and the option
  contrast: the driver reflection path reads an unknown option as nothing and silently encodes an L; a generator can fail loudly instead
driver_tag_interop:
  fact: system:tinygodriver-dynamodb MarshalItem and UnmarshalItem read dynamodbav and nothing else
  consequence: a dynamo-tagged struct passed to the driver reflection path falls back to Go field names, so the two paths disagree on every renamed attribute
  check: a field carrying dynamodbav but no dynamo is a generation error naming both spellings, because it is almost always an unported SDK struct
  not_a_goal: one struct behaving identically through both paths; the generated codec is the intended path
generation_checks:
  - unsupported field type, per data:dynamodb-attribute-mapping
  - duplicate attribute name within one type
  - more than one partitionkey
  - more than one sortkey
  - sortkey without partitionkey
  - a key field whose attribute is not S, N or B; DynamoDB permits nothing else
  - a generated method name colliding with a method the type already declares
  - dynamodbav present where dynamo is absent
message: every failure names the struct, the field, and what was expected
unexported_fields: skipped without error, as in the driver reflection path
deferred:
  secondary_index: "dynamo:\"city,gsi=by-city,partitionkey\"" extends the same mechanism; defer until the primary key path is proven
related:
  - requirement:dynamobind-generated-item-codec
  - data:dynamodb-attribute-mapping
  - decision:dynamobind-table-definition
  - system:tinygodriver-dynamodb
  - decision:struct-field-tags
  - requirement:analysis-diagnostics
```
