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
  summary:
    outcome: propagate
    detail: decision:summary-tag-form covers the whole subtree
    why: >
      subtree propagation is what makes the tag affordable at all; 20 struct-level
      tags cover 73 of the 106 unremarkable lines in
      requirement:effective-config-brevity measured_case
    state: implemented
  falsy:
    outcome: reject
    why: falsy names one value and a struct has none
  enum:
    outcome: reject
    why: an allowlist constrains one scalar, and a struct owns none, the same reason falsy is rejected
    state: >
      implemented. The tag became readable with decision:dependon-value-condition,
      which needs the parent's allowlist; the rejection landed in the same change,
      because a newly readable tag with no placement row is exactly the silent-drop
      class this requirement closed
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
  enum: reject, implemented with decision:dependon-value-condition
  summary:
    outcome: honored, resolved by the element's stable path under the array key
    why: >
      it rates the key being printed rather than naming one to look up, so unlike
      dependon, falsy, and enum it needs no stable key of its own; this is the same
      reason secret is honored here
    state: >
      implemented, and inert: an element has no default layer, so the Place half of
      rule:summary-key-omission never holds. The resolution is kept rather than
      special-cased; see decision:summary-tag-form element_fields_inert_today
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
  - the generation run that first reads a config enum tag also rejects it on a struct and an element field
  - 'summary:"omit" on a nested struct, an array field, and an element field each reach every key they cover'
  - a summary value other than omit fails go generate
related:
  - requirement:array-of-tables-provenance
  - decision:struct-field-tags
  - decision:shared-config-struct-instances
  - decision:dependon-tag-form
  - decision:dependon-value-condition
  - decision:falsy-tag-form
  - decision:summary-tag-form
  - rule:summary-key-omission
  - rule:enum-value-validation
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
