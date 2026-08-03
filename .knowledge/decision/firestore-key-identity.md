---
id: decision:firestore-key-identity
type: decision
title: The Key Is A Path, Not Two Attributes
---
A generated key builder produces a datastore.Key path from a kind fixed by the type and one identifier field, with ancestors declared by a parent field rather than inferred.

```yaml
status: proposed
proposed: 2026-08-03
why_this_is_its_own_decision: the DynamoDB key model is two attributes that are also item attributes, and every difference in requirement:firestorebind-generated-entity-codec traces back to this one
kind:
  default: the Go type name, unqualified
  override: "firestore:\"-,kind=Reading\"" on a blank field, or a generator option; open, decided when the codec is built
  why_the_type_name_is_the_default:
    a_kind_is_intrinsic: an entity of kind Reading is a Reading wherever it is stored, unlike a table name, which decision:dynamo-single-table-scope calls a deployment fact
    consequence: requirement:firestore-typed-queries needs no table clause, and an item operation needs no kind argument, so both signatures lose a string the DynamoDB ones carry
  effect_on_scope: there is no single-table question here; one type is one kind by construction, so decision:dynamo-single-table-scope has nothing to decline
identifier_field:
  name_key: "firestore:\"-,name\"" on a string field, producing datastore.NameKey(kind, v)
  id_key: "firestore:\"-,id\"" on an int64 field, producing datastore.IDKey(kind, v)
  exactly_one: a type declaring both, or two of either, is a generation error
  the_name_is_a_dash: the identifier is not a property, so it has no property name; writing one is a generation error rather than a silent duplicate
  zero_value: an int64 id of zero and an empty string name are both an incomplete key, which is legal on insert and an error on Get, Update or Delete
key_is_lifted_out:
  rule: an identifier field and a parent field are absent from Entity.Properties
  reason: Datastore stores the key beside the properties, so writing them as properties too stores identity twice and lets the copies drift
  contrast: a DynamoDB partition key is an item attribute, so requirement:dynamobind-generated-item-codec has no lift step at all
  decode: DecodeEntity fills the identifier and parent fields from Entity.Key, so a decoded value is complete without a second read
  filtering_on_it: a query cannot filter a property that is not stored; filtering on identity uses the key or an ancestor, which requirement:firestore-typed-queries covers
  opt_out: "firestore:\"sensor,name\"" with a real name stores it as a property as well; allowed, because a caller who wants to filter on it may need the duplicate, and it is then the caller's drift
ancestors:
  declaration: "firestore:\"-,parent\"" on a datastore.Key field, or on a struct field of another bound type
  generated: ItemKey walks the parent chain and returns the full path
  cycle: a parent chain that reaches its own type is a generation error, since the path length would be unbounded
  no_inference: a nested struct is not an ancestor unless it is tagged as one; ancestry changes identity and query semantics, so it is declared rather than guessed
  why_it_matters: an ancestor query is the only strongly consistent multi-entity read Datastore offers, so the parent has to be reachable from the type
incomplete_keys:
  insert: an entity with an incomplete key is legal, and the server allocates; Put and Insert return the completed Key
  writing_it_back: "Store returns the Key, and the caller assigns it, rather than the runtime mutating a value it was given by copy"
  open: whether a StoreReturning form filling the identifier field in place is worth a second signature; the same question requirement:dynamo-optimistic-locking left open, and it is answered once for both
  AllocateIDs: exposed as a typed helper only if a caller needs a key before the write; not in the first cut
namespaces:
  not_a_tag: a namespace is a deployment or tenancy fact, not a property of the type
  where: the Context, per decision:firestore-context-client-api
  effect: a generated key carries no namespace, and the client stamps one at encode time, so a bound type is portable across tenants
generated_surface:
  - "func (r Reading) Kind() string"
  - "func (r Reading) EntityKey() datastore.Key"; only when an identifier tag is present
  key_builder_is_not_usage_directed: emitted wherever an identifier tag exists, for the reason requirement:dynamobind-generated-item-codec gives; a method is not a call the generator can discover
relation_to_the_driver_mapper:
  fact: system:tinygodriver-firestore MarshalEntity marks the key field with a "__key__" tag name on a Key or *Key field
  difference: firestorebind declares identity on the fields that hold it, so name, id and parent describe a key the type already has rather than requiring a datastore.Key field beside it
  why: a Go struct whose identity is a string id should not have to carry a driver type to say so, and the generated EntityKey builds the path from what is there
  interop: a field tagged "__key__" through the datastore tag is caught by the two-path check in rule:firestore-tag-options rather than read
limits_worth_checking:
  key_size: datastore.MaxKeyBytes, which a long name plus a deep ancestor path can reach; a generation-time check can only bound the fixed parts, so this is a documented runtime failure
  path_depth: no published cap, unlike datastore.MaxNestingDepth on nested entity values
related:
  - requirement:firestorebind-generated-entity-codec
  - rule:firestore-tag-options
  - requirement:firestore-typed-queries
  - decision:firestore-context-client-api
  - system:tinygodriver-firestore
  - decision:dynamo-single-table-scope
  - decision:firestore-no-schema-artifact
```
