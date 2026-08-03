---
id: rule:firestore-tag-options
type: rule
title: firestore Tag Options And Generation Checks
---
The firestore tag names the property and its options; an option the generator does not know is a generation error, never a silently ignored string.

```yaml
status: implemented
proposed: 2026-08-03
implemented: 2026-08-04; the checks below are generator/firestorebind.go and generator/firestorebind_types.go, covered by generator/firestorebind_test.go
tag:
  spelling: firestore
  reason: it matches the package name of decision:firestorebind-runtime-package, and the house style where a tag is short and names its purpose, as with dynamo, check, db, opt, enum, query, payload and groupkey
  not_datastore: datastore is taken; v1.1.5 gave the driver its own MarshalEntity reading that spelling, so the two words name two different mappings and must stay distinct
driver_tag_interop:
  fact: system:tinygodriver-firestore MarshalEntity and UnmarshalEntity read datastore and nothing else
  consequence: a firestore-tagged struct passed to the driver mapper falls back to Go field names, so the two paths disagree on every renamed property
  check: a field carrying datastore but no firestore is a generation error naming both spellings, because it is almost always an unported official-client struct
  asked_for_upstream: the driver's own doc comment on the tag says it is authoritative for that path only, and tells a generator to treat this exact case as an error rather than as agreement
  not_a_goal: one struct behaving identically through both paths; the generated codec is the intended path
  same_shape_as: rule:dynamo-tag-options, whose dynamodbav check exists for the same reason
  __key__: the driver mapper marks the key field with a "__key__" tag name on a Key field; firestorebind uses name, id and parent on the fields themselves, per decision:firestore-key-identity, so a field carrying "__key__" is caught by the check above rather than read
form: "`firestore:\"<name>[,<option>...]\"`"
name:
  empty: the field name is used
  "-": the field carries no property; legal only with an identity option, and otherwise it means the field is skipped
options:
  name: the field supplies the key's string name, per decision:firestore-key-identity
  id: the field supplies the key's int64 id
  parent: the field supplies the ancestor path
  version: the field receives Entity.Version on decode and feeds a conditional write, per decision:firestore-transaction-scope
  noindex: the property is excluded from indexes
  omitempty: the property is absent from the map when the field is the zero value
identity_options:
  exactly_one_of: name or id, and at most one field carrying it
  property_name: normally "-", since the identifier is not stored as a property; a real name stores it twice on purpose, and the duplicate is then the caller's to keep in step
  parent: at most one, on a datastore.Key field or on another bound type
kind:
  default: the Go type name
  override: none is implemented; a kind= option or a generator option remains the shape if one is ever wanted, per decision:firestore-key-identity
  no_per_statement_form: unlike the table clause of requirement:dynamo-typed-queries, since a kind belongs to the type
unknown_option:
  behavior: generation error naming the field and the option
  contrast: the driver mapper reads a different tag entirely, so a wrong option here is not softened by anything; it would otherwise be a property that simply never appears
generation_checks:
  - unsupported field type, per data:firestore-property-mapping
  - duplicate property name within one type
  - more than one name, id, parent or version field
  - name on a non-string field, or id on a non-integer field
  - version on a non-int64 field
  - parent on a field that is neither datastore.Key nor a bound type
  - a parent chain reaching its own type
  - an identity option on a nested type, which has no key
  - noindex on a field that is not stored, which is a contradiction rather than a no-op
  - a generated method name colliding with a method the type already declares
  - a set option borrowed from rule:dynamo-tag-options, which names an encoding Datastore does not have
  - datastore present where firestore is absent
  - a uint, uint64 or uintptr field, which exceeds the int64 a Datastore integer holds, per data:firestore-property-mapping
  - a map field of any key type
message: every failure names the struct, the field, and what was expected
unexported_fields: skipped without error, as in every other mode
what_has_no_counterpart_here:
  partitionkey_and_sortkey: replaced by the key path of decision:firestore-key-identity
  stringset_numberset_binaryset: Datastore has no set type
  unixtime: Datastore has a real timestamp type
  ttl: confirmed upstream 2026-08-03 and recorded in system:tinygodriver-firestore; TTL is not expressible on this wire, but a field-level policy applied with gcloud over an ordinary timestamp property, so requirement:dynamo-ttl-attribute has nothing to mirror and no tag is needed to produce what a policy consumes
  secondary_index: single-property indexes are automatic and composite ones are declared out of band, per decision:firestore-no-schema-artifact
related:
  - requirement:firestorebind-generated-entity-codec
  - data:firestore-property-mapping
  - decision:firestore-key-identity
  - system:tinygodriver-firestore
  - decision:struct-field-tags
  - rule:dynamo-tag-options
  - requirement:analysis-diagnostics
```
