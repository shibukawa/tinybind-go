---
id: requirement:array-of-tables-provenance
type: requirement
title: Array of Tables Provenance
---
Provenance expands each array-of-tables element into per-element keys, applying redaction and declaration order there, so no caller re-implements the expansion.

```yaml
priority: must
status: implemented
scope: output only; apply, scaffold, and element defaults already work
former_behavior:
  emitted: one entry per array key whose Value is empty, because data:overlay-entry sets Raw only for scalars
  consequence: three configured connections rendered as one blank line
why_it_was_not_only_cosmetic:
  - the only way to render elements was for the caller to read concept:config-overlay directly
  - that hand-rolled path has no access to the generated secret map or to rule:secret-redaction
  - so element redaction was not merely missing, it was unreachable from outside the package
  - every caller wrote the same expansion, and each one re-derived ordering and masking
expansion:
  key_form: 'arrayKey[index].fieldKey, nested element structs continuing with dots'
  index_origin: position in data:overlay-entry Tables, which is file order
  entry_addressing:
    problem: a caller grouping entries into a tree must not split the key string on a bracket
    chosen: carry the array key and the index as fields beside Key
    spelling: 'ArrayKey string, Index int; empty ArrayKey means the entry is not an element'
    state: implemented
    why: matches the empty-string-means-absent convention already used for Secret and Falsy
    alternative_considered: 'Index *int, rejected as the only pointer field in data:provenance-event'
  ordering:
    within_an_element: struct field declaration order from the generated scaffold Nested list
    across_elements: index ascending
    array_position: where the array field is declared, per rule:config-output-ordering
    do_not_change: >
      concept:config-overlay Keys stays sorted; it enumerates an untyped overlay and its
      sort is that surface's determinism guarantee, not a provenance ordering source
redaction:
  requirement: an element field resolves a disclosure mode exactly as a leaf field does
  keying_constraint:
    problem: an index is a runtime value, so no generated map can hold arrayKey[0].dsn
    rule: the generated secret map keys element fields by their path under the array key
    resolution: expansion matches the element-relative path, then applies it to every index
  auto_policy: rule:secret-redaction key tokens apply to the element leaf name, so dsn masks with no tag
tag_placement_on_the_array_field:
  finding: >
    one rejection used to cover dependon, falsy, and secret with one reason, that
    elements have no stable key; the reason holds for element fields but not for the
    array field itself, which has one
  dependon:
    verdict: allow
    effect: an empty parent hides the array and every expanded element
    motivation: folding a disabled feature's settings is what the tag is for, and a connection set is the largest such block
  secret:
    verdict: allow
    effect: covers every field of every element, as it does for a nested struct subtree
    note: this is the smallest complete fix for element redaction
  falsy:
    verdict: keep rejected
    why: falsy names one value and an array has none
element_field_tags:
  enum:
    was: rejected alongside dependon and falsy for a reason only those two share
    now: honored per rule:enum-value-validation, and its rejection names the index
  secret:
    was: accepted and dropped, unlike Opt, Env, DependsOn, and Falsy which are rejected
    now: honored per requirement:struct-tag-placement-totality
  arg: rejected alongside opt and env, which it had been missing from
diagnostics:
  requirement: an apply error inside an element names the index
  was: 'configbind: middleware.rdb.connections.conn_max_lifetime: ...'
  now: 'configbind: middleware.rdb.connections[0].conn_max_lifetime: ...'
  why: file position is an element's only identifier, so an operator with five replicas searches by hand
  implementation_note: >
    the key reaches the emitted fmt.Errorf as part of a format literal, so the emitted
    key carries a [%d] verb per enclosing array and the loop variables are passed as
    arguments; a message that spells the key twice needs the arguments twice
acceptance:
  - three configured connections produce entries for every field of all three
  - 'an element field tagged secret:"mask" renders masked at every index'
  - 'a secret tag on the array field masks every field of every element'
  - an element dsn with no tag masks by key token
  - group declared before dsn is reported before dsn, not alphabetically
  - 'dependon on the array field hides the whole array when the parent is empty'
  - falsy on the array field still fails generation
  - a caller renders a tree from ArrayKey and Index without parsing the key string
  - a bad element duration names its index
  - scalar entries keep their current key form and order
related:
  - requirement:struct-tag-placement-totality
  - requirement:deterministic-config-output-order
  - requirement:source-provenance-logging
  - requirement:dependent-field-visibility
  - decision:toml-shape-constraints
  - decision:configbind-supported-types
  - decision:dependon-tag-form
  - rule:config-output-ordering
  - rule:secret-redaction
  - rule:dependent-key-visibility
  - api:configbind-provenance
  - data:provenance-event
  - data:overlay-entry
  - concept:config-overlay
  - concept:provenance-log-helper
  - flow:config-load
  - system:configbind
```
