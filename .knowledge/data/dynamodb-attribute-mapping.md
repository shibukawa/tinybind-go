---
id: data:dynamodb-attribute-mapping
type: data
title: Go To DynamoDB Attribute Mapping
---
The Go type set the generator accepts and the attribute each one produces; anything outside it is a generation error.

```yaml
scalar:
  string: S; the empty string is valid and is stored
  int, int8, int16, int32, int64: N via strconv
  uint, uint8, uint16, uint32, uint64: N via strconv
  float32, float64: N
  bool: BOOL
  "[]byte": B
time:
  default: S in RFC 3339 nano, matching the driver reflection path
  option: N seconds since epoch under the unixtime tag option
composite:
  "[]T": L; the set options select SS, NS or BS instead
  "map[string]T": M
  nested struct: M, generated recursively
  "*T": the pointee, or NULL when nil
escape_hatch:
  dynamodb.AttributeValue: passed through unchanged, for what this table cannot express
number_rule:
  statement: N is text end to end
  forbidden: routing a number through float64
  reason: DynamoDB numbers carry 38 significant digits
  constructors: strconv output or dynamodb.NString; dynamodb.N is acceptable only where the Go type is already the source of truth
rejected:
  - a map with a non-string key
  - a channel, function, interface or complex field
  - a set option on a non-slice field, or on a slice whose element does not match the set kind
decode_errors:
  shape: field level; attribute name, expected kind, got kind
  reuse: the jsonbind FieldError shape where it fits
  missing_attribute: absent attribute leaves the zero value; NULL decodes as the zero value or nil pointer
related:
  - requirement:dynamobind-generated-item-codec
  - rule:dynamo-tag-options
  - system:tinygodriver-dynamodb
```
