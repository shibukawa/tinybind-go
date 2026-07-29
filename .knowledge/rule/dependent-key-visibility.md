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
  - parent declares decision:falsy-tag-form and holds that choice
visible_when:
  - parent has any other effective value, including a non-empty default
hidden_when_also:
  - parent key is absent from the overlay, i.e. no source and no default set it
  - a parent named by a tag but registered nowhere is indistinguishable from this
not_emptiness:
  - int 0
  - empty string slice
  - zero duration
  reason: those are meaningful settings, not an unconfigured parent
effective_value: post-merge winning value from concept:config-overlay, not the raw default tag
parent_itself:
  - always emitted, even when empty; the empty parent is the reason the children vanished
transitivity:
  - a hidden parent hides its own dependents
  - resolve in dependency order; decision:dependon-tag-form forbids cycles
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
