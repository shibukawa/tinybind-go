---
id: data:firestore-property-mapping
type: data
title: Go To Firestore Datastore Property Mapping
---
The Go type set the generator accepts and the property value each one produces; anything outside it is a generation error.

```yaml
scalar:
  string: String; the empty string is valid and is stored, and differs from an absent property
  int, int8, int16, int32, int64: Integer, which is text on the wire
  uint8, uint16, uint32: Integer; every value fits the int64 Datastore stores
  float32, float64: Double, a real JSON number
  bool: Bool
  "[]byte": Blob, base64 on the wire
time:
  time.Time: Timestamp, RFC 3339 on the wire
  precision: storage keeps microseconds, so a round trip truncates nanoseconds; the codec does not hide it and the godoc says so
  no_unixtime_option: Datastore has a real timestamp type, so the DynamoDB epoch-seconds encoding of rule:dynamo-tag-options has nothing to solve here
key:
  datastore.Key: KeyValue, a full key including partition; useful for a reference to another entity
  reading: this is the nearest thing to a foreign key, and nothing enforces it
geo:
  datastore.LatLng: GeoPoint
  no_analogue: DynamoDB has no geo type, so data:dynamodb-attribute-mapping has no row to compare
composite:
  "[]T": Array of the element mapping
  nested struct: a nested entityValue, generated recursively and carrying no key
  "*T": the pointee, or Null when nil
maps_are_rejected:
  statement: a map field of any key type is a generation error
  reason: a map would become an entityValue whose property names come from run-time data rather than from the struct, and property names coming from anywhere but a tag is the one thing requirement:firestorebind-product-goals exists to prevent
  same_call_upstream: the driver's own MarshalEntity rejects maps for the same reason, stated in its doc comment; agreeing costs nothing and disagreeing would mean one of the two paths accepting a struct the other refuses
  what_a_caller_does_instead: a nested struct, whose property names are declared, or a datastore.Value field holding an entityValue built by hand where the names really are dynamic
  changed_2026_08_03: an earlier draft mapped map[string]T to a nested entity; that was written before v1.1.5 and against a weaker version of this argument
escape_hatch:
  datastore.Value: passed through unchanged, for what this table cannot express
unsigned_wider_than_int64_is_rejected:
  kinds: uint, uint64 and uintptr are generation errors naming the field
  reason: a Datastore integer is an int64, and the driver refuses to marshal an Integer whose text does not parse as one, so nothing wider ever reaches the wire
  measured_2026_08_03: an IntString carrying math.MaxUint64 fails inside the driver's own MarshalJSON with "integerValue is not an int64"; the value cannot be sent at all, so there is no stored form to describe
  why_reject_the_field_rather_than_the_value: a field that accepted uint64 would compile, store small values for months and fail on the first large one, with the error surfacing from inside json.Marshal without naming the property
  changed_2026_08_03: an earlier draft mapped uint64 through IntString and called an over-large value a runtime encode error; that was written before the driver's marshal-side check was measured
  contrast: data:dynamodb-attribute-mapping takes every unsigned kind, because a DynamoDB N is arbitrary-precision text with 38 significant digits
  workaround: int64 where the range allows, or a string property where the value really is that wide and is never ordered on
two_number_types:
  statement: Integer and Double are separate types, not two spellings of one
  forbidden: encoding an int field as Double, or decoding a Double into an int field
  reason: Datastore orders and compares them separately, so a coerced value stops matching the filter that was written for it
  contrast: DynamoDB N is one type and data:dynamodb-attribute-mapping folds every numeric Go type into it
integer_rule:
  statement: Integer is text end to end, as DynamoDB N is
  reason: proto3 JSON writes int64 as a string
  constructors: datastore.Int for a Go integer type, datastore.IntString for text the caller already holds
  forbidden: routing an integer through float64
sets:
  none: Datastore has no set type
  effect: the stringset, numberset and binaryset options of rule:dynamo-tag-options have no counterpart; a slice is an Array, and duplicate elements are stored
  query_behaviour: an Array property matches a filter if any element matches, which is the closest thing to set membership and is a query fact rather than a codec one
indexing:
  default: every property is indexed, unlike DynamoDB where only key and index attributes are
  noindex: the tag sets ExcludeFromIndexes, which composes with any value through datastore.Unindexed
  silent_ceiling: a string over datastore.MaxIndexedStringBytes stops being indexed rather than failing, so a long text field that is never filtered on should carry noindex to say so deliberately
  arrays: excludeFromIndexes on an array applies to its elements
  cost: an indexed property costs write throughput and index storage, which is why the option exists at all
rejected:
  - a map of any key type, per maps_are_rejected above
  - a channel, function, interface or complex field
  - a nested type declaring an identifier tag, per requirement:firestorebind-generated-entity-codec
  - a slice of slices, since an Array of Arrays is not a Datastore value
decode_errors:
  shape: field level; property name, expected kind, got kind
  reuse: the jsonbind FieldError shape where it fits, as data:dynamodb-attribute-mapping does
  missing_property: absent leaves the zero value; Null decodes as the zero value or nil pointer
limits:
  named_not_copied: datastore.MaxEntityBytes, datastore.MaxNestingDepth and datastore.MaxKeyBytes, per rule:firestorebind-driver-passthrough
  entity: no generation-time check can bound it for a string or slice field, so it stays a runtime error
  nesting: static nesting is checkable at generation time against MaxNestingDepth; a recursive slice is not
related:
  - requirement:firestorebind-generated-entity-codec
  - rule:firestore-tag-options
  - system:tinygodriver-firestore
  - data:dynamodb-attribute-mapping
```
