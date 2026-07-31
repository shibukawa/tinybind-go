---
id: decision:shared-config-struct-instances
type: decision
title: Shared Config Struct Instances
---
One struct type embedded at several prefixes keeps one tag set; per-instance defaults are written through the bound pointer, not through tags.

```yaml
status: accepted
problem: >
  a reusable option struct such as an endpoint block is embedded under several
  prefixes, but its default values and its decision:dependon-tag-form parent
  differ per instance, while a tag belongs to the type
options_considered:
  split_the_type:
    verdict: not required
    why: duplicates structurally identical types and breaks a published API
  child_defaults_in_the_struct_field_tag:
    verdict: rejected
    why: encodes a key/value mini-language inside one tag and escapes Go type checking
    enforcement: >
      the rejection has to be a generation error; a default tag on a nested
      struct field is currently accepted and dropped without a report, which
      reads as acceptance of this option. See requirement:struct-tag-placement-totality
  relative_dependon:
    verdict: adopted
    form: 'dependon:".enabled", resolved against the containing struct prefix'
    detail: decision:dependon-tag-form relative_form
    note: covers the parent problem only; per-instance defaults remain
per_instance_defaults:
  mechanism: assign the field through the pointer api:configbind-bind returns, before Load
  why_it_works:
    - generated apply mutates the existing value and never resets it
    - a field with no default tag is assigned only when a source sets its key
  limits:
    - invisible to api:configbind-provenance and to requirement:scaffold-generation output, because no overlay entry is created
    - a default tag on the shared type outranks it for every instance
    - the write has to land between Bind and Load
  status: documented workaround, not a designed API
deferred_work:
  - per-instance default registration that also feeds scaffold and help output
related:
  - requirement:struct-tag-placement-totality
  - decision:dependon-tag-form
  - decision:struct-field-tags
  - requirement:dependent-field-visibility
  - requirement:scaffold-generation
  - rule:dependent-key-visibility
  - api:configbind-bind
  - api:configbind-provenance
  - concept:config-struct-mapping
  - flow:config-load
  - system:configbind
```
