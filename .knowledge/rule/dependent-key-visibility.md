---
id: rule:dependent-key-visibility
type: rule
title: Dependent Key Visibility
---
A field with dependon disappears from generated output when its parent value is the empty string or false; the parent stays.

```yaml
trigger: field carries decision:dependon-tag-form
hidden_when:
  - parent kind is string and the effective value is ""
  - parent kind is bool and the effective value is false
  - parent declares decision:falsy-tag-form and holds that value
  - any one of a key's parents is hidden or empty; the set is conjunctive
visible_when:
  - parent has any other effective value, including a non-empty default
hidden_when_also:
  - parent key is absent from the overlay, i.e. no source and no default set it
  - a parent named by a tag but registered nowhere is indistinguishable from this
not_emptiness:
  - int 0 with no falsy tag
  - zero duration with no falsy tag
  - empty string slice
  reason: those are meaningful settings, not an unconfigured parent
  falsy_override: >
    an int or duration parent that declares falsy is compared against that
    value, so a declared zero does disable its dependents
value_comparison:
  - a falsy match is compared in the terms of the parent's kind, not as raw text
  - duration: 0, 0s, and 0ms are one value
  - int: leading zeros and equal magnitudes are one value
  - kind source: the generated Scaffold entry of the parent's own definition
effective_value: post-merge winning value from concept:config-overlay, not the raw default tag
parent_itself:
  - always emitted, even when empty; the empty parent is the reason the children vanished
transitivity:
  - a hidden parent hides its own dependents
  - resolve in dependency order; decision:dependon-tag-form forbids cycles
subtree_scope:
  trigger: dependon placed on a nested struct field, per decision:dependon-tag-form
  meaning: every leaf key under that struct carries the declared parent
  own_tag_coexists: a leaf keeps its own parent and gains the ancestor parent
  hidden_when: any declared parent of a leaf is empty
  model: a leaf holds a set of parents, not one, once an ancestor declares a parent
  state: implemented
  wire_form: 'generated Definition.DependsOn is map[string][]string'
  still_rejected: falsy on a struct field, which names one value a struct does not have
array_of_tables_scope:
  trigger: dependon placed on an array-of-tables field
  meaning: an empty parent hides the array key and every expanded element entry
  why_the_array_field_qualifies: it owns one stable key, unlike its elements
  element_fields: cannot declare a parent, and cannot be named as one
  detail: requirement:array-of-tables-provenance
  state: implemented
applies_to:
  - provenance records from concept:provenance-log-helper only
never_applies_to:
  - generated apply; the struct field is still written from the overlay
  - overlay contents; hidden keys keep their entry and Place
  - validation; a set child under an empty parent is not an error in v1
  - CLI help and flag registration; hidden fields keep their flags
  - TOML and .env scaffolds from requirement:scaffold-generation
scaffold_exclusion_reason:
  - scaffolds render before any load, so no effective parent value exists
  - a scaffold advertises available settings; hiding one makes it undiscoverable
interaction_with_secret:
  - independent of rule:secret-redaction
  - a record dropped by either policy is dropped
related:
  - decision:dependon-tag-form
  - decision:falsy-tag-form
  - rule:falsy-value-resolution
  - requirement:dependent-field-visibility
  - concept:provenance-log-helper
  - concept:config-overlay
  - data:provenance-event
  - rule:secret-redaction
  - flow:config-load
```
