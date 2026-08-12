---
id: rule:summary-key-omission
type: rule
title: Summary Key Omission
---
A key tagged as detail is reported as omittable while its winning source is the default layer; the entry is still returned, and the caller decides.

```yaml
trigger: key carries decision:summary-tag-form, its own or one inherited from a struct above it
omittable_when:
  - the tag is in force for this key
  - and the winning Place is default, per rule:source-precedence
not_omittable_when:
  - no tag is in force, which is every untagged key
  - the winning Place is file, env, or CLI, whatever the tag says
why_the_second_condition: >
  a value someone wrote down is what a reader opened the output to find; the tag
  rates how interesting the key is in general, and the Place says whether this
  deployment had anything to say about it
falsy_interaction:
  - a key filled in by rule:falsy-value-resolution from an absent key reports Place default, so it is omittable
  - a key filled in from a source-set empty value keeps that source's Place, so it is not
  reason: the Place already answers "did a source decide this", and nothing here re-decides it
reported_not_applied:
  - api:configbind-provenance returns the entry with data:provenance-event Omittable set
  - the library never drops an entry for this reason
  - a caller rendering a summary skips the marked entries; a caller dumping renders all of them
contrast_with_dependon:
  dependon: a fact about the configuration, so rule:dependent-key-visibility drops the entry
  summary: a judgment about one surface, which only the caller knows it is on
filter_order:
  - rule:dependent-key-visibility drops inert keys
  - rule:secret-redaction drops hidden keys and masks values
  - this marks what survives; it removes nothing
  effect: a dropped entry is never marked, because it is not there to mark
subtree_scope:
  trigger: the tag on a nested struct field
  meaning: every key of the subtree carries it
  own_tag_coexists: a leaf's own tag and an ancestor's say the same thing, so the nearest one in force wins
  no_conjunction_needed: unlike a dependon condition, two tags do not compose into a stricter rule
array_of_tables_scope:
  array_field: covers the array and every expanded element entry
  element_field: honored, resolved by the element's stable path under the array key
  same_mechanism: rule:secret-redaction element handling
  inert_today: >
    no element field can satisfy the Place half, because nothing seeds an element
    with a default; see decision:summary-tag-form element_fields_inert_today
never_applies_to:
  - the bound struct; the field is populated from the overlay as always
  - overlay contents; a marked key keeps its entry and Place
  - validation; nothing about being unremarkable is an error
  - CLI help and flag registration
  - TOML and .env scaffolds from requirement:scaffold-generation, which must stay complete
scaffold_exclusion_reason: >
  a scaffold renders before any load, so no Place exists yet, and it advertises
  what is settable; omitting the unremarkable makes it undiscoverable
related:
  - decision:summary-tag-form
  - requirement:effective-config-brevity
  - rule:dependent-key-visibility
  - rule:secret-redaction
  - rule:falsy-value-resolution
  - rule:source-precedence
  - concept:config-overlay
  - concept:provenance-log-helper
  - data:provenance-event
  - api:configbind-provenance
  - term:config-source
```
