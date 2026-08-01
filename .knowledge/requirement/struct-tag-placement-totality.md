---
id: requirement:struct-tag-placement-totality
type: requirement
title: Struct Tag Placement Totality
---
Every tag written on a nested struct field resolves to one defined outcome, propagate or generation error, and none is accepted and dropped.

```yaml
priority: must
problem:
  observed: a default tag on a nested struct field is accepted, generates nothing, and reports nothing
  class: same silent-drop defect decision:dependon-tag-form already closed for falsy on a struct
  why_it_hurts:
    - an author reading decision:struct-field-tags sees the tag accepted by the generator and assumes it lands
    - a shared struct embedded at several prefixes is exactly where per-instance defaults are attempted first
    - the miss surfaces as a wrong runtime value, not as a failed go generate
tag_outcomes_on_a_struct_field:
  dependon:
    outcome: propagate
    detail: decision:dependon-tag-form subtree_placement
  secret:
    outcome: propagate
    detail: rule:secret-redaction covers the whole subtree
  falsy:
    outcome: reject
    why: falsy names one value and a struct has none
  default:
    outcome: reject
    why: decision:shared-config-struct-instances rejected child defaults in a struct field tag; the rejection has to be enforced, not assumed
    state: implemented
  opt:
    outcome: reject
    why: a CLI flag addresses one leaf key, and a struct owns no scalar to parse into
  env:
    outcome: reject
    why: an environment variable carries one scalar, so a subtree has no env form
  arg:
    outcome: reject
    why: a positional argument is a subcommand leaf role per decision:struct-field-tags arg_tags
  help:
    outcome: accept
    why: >
      decision:godoc-help-precedence backfills help tags into source from godoc,
      and it does so for struct fields too, so rejecting help here would fail the
      very generation run that wrote the tag
    consequence: help is the one tag whose placement is total by acceptance rather than by rejection
    still_open: rendering it as the nested table's scaffold comment, which decision:struct-field-tags currently reserves for struct godoc
tag_outcomes_on_an_array_of_tables_element_field:
  opt: reject, current
  env: reject, current
  dependon: reject, current
  falsy: reject, current
  secret:
    outcome: honored, resolved by the element's stable path under the array key
    was: accepted and dropped, the one hole in an otherwise total row
    state: implemented per requirement:array-of-tables-provenance
  arg: reject, current
totality_rule:
  statement: a tag known to the generator has a row for every placement, leaf, struct, array field, and element field
  enforcement: adding a tag without a row fails the generator test suite, not review
  why: the defect is not any one tag; it is the absence of a total mapping
  evidence: >
    the two holes found so far, default on a struct field and secret on an element
    field, sit in different check functions and were each written as complete
  placement_checks:
    - checkStructFieldTags covers nested struct and array fields
    - checkElementFields covers array-of-tables element fields
    - a placement error is worded by these, not by the collectors, so the message names where the tag sits
status: implemented
validation_time: generation, per decision:dependon-tag-form validation_time
acceptance:
  - 'default:"x" on a nested struct field fails go generate and names the pointer-write mechanism'
  - 'opt, env, and arg on a nested struct field each fail go generate with the reason'
  - dependon and secret on a nested struct field still reach every leaf of the subtree
  - falsy on a nested struct field still fails, unchanged
  - an array-of-tables field keeps its existing rejection for all of these
  - no tag reaches generated output through a path that neither propagates nor rejects it
related:
  - requirement:array-of-tables-provenance
  - decision:struct-field-tags
  - decision:shared-config-struct-instances
  - decision:dependon-tag-form
  - decision:falsy-tag-form
  - decision:default-tag-form
  - requirement:struct-field-metadata
  - requirement:analysis-diagnostics
  - requirement:scaffold-generation
  - rule:secret-redaction
  - rule:dependent-key-visibility
  - data:config-scaffold-fragment
  - flow:configbind-codegen
  - system:configbind
```
