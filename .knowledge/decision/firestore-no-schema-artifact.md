---
id: decision:firestore-no-schema-artifact
type: decision
title: There Is No Table Definition To Emit
---
Kinds are implicit and composite indexes are an admin concern, so nothing is generated in place of decision:dynamobind-table-definition; the gap is recorded rather than filled with a YAML file.

```yaml
status: proposed
proposed: 2026-08-03
what_disappears:
  table_definition: no CreateTable exists, and a kind comes into being on first write, so there is no schema call to feed
  billing_mode_and_capacity: Datastore has no provisioned throughput, so the fields decision:dynamobind-table-definition leaves zero have no counterpart
  key_attribute_types: a key is a path of kind plus id or name, not a typed attribute, so nothing declares S, N or B
  effect_on_tests: a DynamoDB test creates a table first; a Datastore test writes an entity, which is one fewer generated artifact to keep in step
what_remains_undeclared:
  composite_indexes:
    when_needed: a query combining an equality filter with an inequality or an order on another property, or one ordering on two properties
    where_declared: index.yaml applied with gcloud, or the admin API; both outside the driver's own client scope, per system:tinygodriver-firestore
    failure_mode: FAILED_PRECONDITION at run time, with a message naming the index the query wanted
    why_this_is_worse_than_it_sounds: the generator can see the query in a declaration and still cannot verify the index exists, so requirement:firestore-typed-queries makes a query typed without making it runnable
what_the_driver_now_offers:
  landed: v1.1.5, per system:tinygodriver-firestore
  types: datastore.Index, datastore.IndexProperty and MarshalIndexYAML
  effect: the descriptor this decision was waiting for exists, so the question is no longer what to emit into but whether the needed index can be worked out at all
  what_did_not_land_and_why: the driver declined RequiredIndex(*Query), holding that the rule for when a composite index is required is subtle and that a quietly wrong derivation is worse than none, since it names an index that does not fix the query
chosen:
  emit_a_descriptor_when_the_author_declares_one:
    what: an index clause in a .tb.firestore declaration produces a datastore.Index value in the generated file, and a package-level slice a migration step can marshal
    why_this_shape: it stays Go, the compiler checks it, it composes with a framework's own deploy step, and it does not claim to own a project-wide file
    the_author_declares_it: the index comes from a clause the author wrote, not from a rule this module inferred, so the upstream objection does not apply; a declaration is a statement of intent, and a derivation is a guess
  no_derivation:
    what: the generator does not work out, from the filters and orders alone, which queries need a composite index
    why: the same argument the driver gave, and it is stronger here, because a wrong diagnostic in a build log reads as authoritative and an author who adds the named index still has a broken query
    revisit_condition: a derivation worth shipping needs the rule stated precisely enough to test against a real project's index.yaml; until someone has done that, silence is more honest than a guess
    correction: an earlier draft of this decision promised the needed index as a generation diagnostic; that is exactly the derivation being declined, and requirement:firestore-typed-queries no longer claims it
declined:
  generate_index_yaml_directly:
    what: write index.yaml beside the generated code
    against:
      not_a_go_artifact: every other generator output is Go that the compiler checks; a YAML file is checked by gcloud at deploy time and by nothing here
      ownership: index.yaml is project-wide and hand-edited, so generating it means owning or merging a file this module did not write
      incomplete_by_construction: a query written against the escape hatch rather than a declaration contributes no entry, so the generated file would be authoritative and wrong
    what_replaces_it: MarshalIndexYAML over the emitted descriptors, called by whoever owns the file, which puts the merge where the ownership is
documentation_duty:
  what: the guide must say that a declared query can compile and fail on its first run for want of an index, and that nothing in the toolchain will warn first
  why: the DynamoDB reader arrives expecting a generated table definition to have covered the schema, and here there is none to cover it; the index clause is opt-in, so an author who writes no clause gets no protection and should know that
  where: beside requirement:firestore-typed-queries, not buried in a limitations list
what_this_makes_easier:
  no_one_table_per_type_assumption: decision:dynamo-single-table-scope exists because a table definition and a typed decode both assumed one type owned one table; with no definition emitted, only the decode assumes anything, and a kind is per-type anyway
  no_migration_ownership: nothing generated here claims to describe a deployed state, so nothing can drift from one
related:
  - decision:dynamobind-table-definition
  - requirement:firestore-typed-queries
  - requirement:firestorebind-generated-entity-codec
  - system:tinygodriver-firestore
  - decision:dynamo-single-table-scope
  - rule:firestore-tag-options
```
