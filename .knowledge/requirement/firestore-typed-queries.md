---
id: requirement:firestore-typed-queries
type: requirement
title: Typed Firestore Datastore Queries
---
Generate one named function per declared access pattern, so a query names no kind, no property and no operator at the call site.

```yaml
status: implemented
proposed: 2026-08-03
implemented: 2026-08-04
stage: 2 of requirement:firestorebind-product-goals
built:
  grammar: templates/firestorebind/query.go, beside the HTML, SQL and DynamoDB template packages
  formatter: templates/firestorebind/print.go, reached through templatefmt as the other three are
  checks: generator/firestorequery_plan.go
  emitter: generator/firestorequery_emit.go
  wiring: generator/firestorequery_generate.go, writing firestorequery_gen.go
  fixture: internal/firestorefixture/readings.tb.firestore
problem:
  now: "firestorebind.Query[Reading](ctx, datastore.NewQuery(\"Reading\").Filter(\"sensor\", datastore.Equal, datastore.String(id)).Order(\"at\"))"
  strings: the kind, the property name and the order property are three unrelated strings, and a tag rename breaks none of them at compile time
  quieter_than_dynamodb: a renamed property is not a ValidationException here; it is a filter matching nothing, so an empty result is the whole failure signal
  values_too: the caller builds a datastore.Value by hand, so the filter's type has to agree with the codec's by inspection
declaration:
  file: a template source discovered beside the package, as the .tb.sql, .tb.html and .tb.dynamo of requirement:configurable-template-file-patterns are
  pattern: "*.tb.firestore"
  outer_structure: reused from .tb.sql and .tb.dynamo - export statement, a typed parameter list, a result type after a colon, a braced body
  example: "export statement ReadingsSince(sensor: string, from: time.Time): firestore.many<Reading> { where sensor == {sensor} and at > {from}; order at }"
  export_keyword: must agree with the name's own casing, since Go decides visibility by the name; either without the other is an error rather than a silent rename
  parameters: named in the caller's vocabulary and bound to properties where the condition names them, so the two namespaces stay separate
no_kind_clause:
  fact: the result type names the bound Go type, and decision:firestore-key-identity fixes its kind, so the kind is already known
  contrast: requirement:dynamo-typed-queries requires a table clause in every body, because a type is not one table there
  effect: the declaration body holds only the filter, the order and the ancestor; everything else comes from the type
  consistency_with_item_operations: an item operation names no kind either, so the two agree rather than differing the way the DynamoDB pair does
result_type_slot:
  chooses: the request shape, since a Datastore query always returns a batch
  batch: one request, returning Page[T]
  many: the iterator over every batch
  count: an int64 through the aggregation query, with no entities decoded
  keys: a keys-only query, returning KeyPage
  keys_returns_a_page_not_a_slice:
    changed_2026_08_04: an earlier draft said []datastore.Key
    why: a flat slice would have to page internally to be complete, which hides the request count the shape exists to keep visible; KeyPage carries the cursor and the reason the batch ended, exactly as Page does
    runtime: firestorebind.QueryKeysPage, added for this shape
  reason: rule:firestorebind-driver-passthrough keeps the request count visible, so the author picks rather than a default
body_clauses:
  where: property filters composed with and, using ==, !=, <, <=, >, >=, in and not in
  and_only: Datastore composes filters with AND on the wire and offers no OR, so a declaration with or is a parse error naming the wire limitation rather than a silent expansion into several queries
  ancestor: "ancestor {parent}", which is HAS_ANCESTOR on the key path rather than a property filter
  order: one or more properties, each ascending or descending
  limit and offset: constants or parameters; an offset reads and bills the entities it steps over, which the generated godoc says
  index: optional; names the composite index this access pattern needs, emitted as a datastore.Index value rather than derived, per decision:firestore-no-schema-artifact
  project and distinct: deferred until the primary path is proven, since a projection returns a partial entity the codec cannot fill
scope:
  in: kind, property filters, ancestor, order, limit, offset, cursor paging, read consistency
  out: OR, projections, distinctOn, GQL, aggregations beyond count
  reason: what is left out is either absent from the wire or returns something other than a whole entity of the declared type
type_checking:
  property_names: checked against the declared type's rule:firestore-tag-options tags, so a rename is a generation error
  parameter_types: checked against the field's own Go type, and encoded through the same value constructors data:firestore-property-mapping uses for the codec
  two_number_types: a float parameter against an integer property is a generation error rather than a coercion, since a coerced filter matches nothing at run time
  unindexed_properties: filtering or ordering on a field tagged noindex is a generation error, because an excluded property is not in any index and the query can never match
  string_length: nothing can check the 1500-byte indexing ceiling at generation time; it stays a documented runtime surprise
generated:
  one_function_per_declaration: named by the declaration, returning the batch, iterator, count or key form its result type selects
  signature: context, the declared parameters, then variadic driver read options, and nothing else
  query_value: built per call, since datastore.Query is a builder whose methods clone; only the kind is a package-level constant
  kind_constant: one per statement, taken from the type's own Kind rather than from the declaration, so the two cannot disagree
  no_builder: the function embeds its query directly; no per-type query builder is generated
  transaction_form: <Name>Tx beside <Name>, for the batch, count and keys shapes; the iterator gets none, per decision:firestore-transaction-scope
counts_as_usage:
  what: a declaration is a use of its result type, feeding the decoder into the codec pass
  why: the generated function instantiates firestorebind.Query with that type, which does not compile without it
  effect: a package whose only Datastore use is a declaration still gets a codec, per rule:usage-directed-generation
what_typing_still_cannot_promise:
  index: a query combining an equality filter with an inequality or an order on another property needs a composite index, so it compiles and fails on first run with FAILED_PRECONDITION
  no_derivation: the generator does not work out which index a declaration needs, per decision:firestore-no-schema-artifact; the upstream driver declined the same derivation in v1.1.5 and the argument holds here
  what_the_author_can_do: an index clause emits a datastore.Index value a deploy step can marshal, exported when the statement is, so the index is declared rather than inferred
  a_hint_that_names_nothing:
    what: a declaration whose shape commonly needs a composite index, and that declares none, gets a godoc line saying it may need one
    why_this_is_not_the_declined_derivation: it names no index and claims no certainty, so an author cannot act on it wrongly; the failure mode upstream warned about is a named index that does not fix the query
    where: the generated godoc, not a build diagnostic, so it is read by someone already looking at the function
  duty: the guide says a declaration is not a deployment
  comparison: requirement:dynamo-typed-queries could lean on decision:dynamobind-table-definition for the schema half; here the schema half is opt-in and the author owns it
depends_on:
  - requirement:firestorebind-generated-entity-codec, for the property names and value encoders
  - decision:firestore-key-identity, for the kind and the ancestor path
related:
  - api:firestorebind-operations
  - rule:firestore-tag-options
  - data:firestore-property-mapping
  - concept:typed-template-language
  - requirement:dynamo-typed-queries
  - requirement:configurable-template-file-patterns
  - decision:firestore-no-schema-artifact
```
