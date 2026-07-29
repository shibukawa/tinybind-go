---
id: rule:falsy-value-resolution
type: rule
title: Falsy Value Resolution
---
A falsy choice fills in for an empty value and counts as empty for dependents.

```yaml
tag: decision:falsy-tag-form
fill_in:
  when:
    - the key has no default tag
    - and the key is absent from the overlay, or its winning value is ""
  action: store the falsy choice in concept:config-overlay before apply
  place:
    absent_key: default
    empty_value: the Place that set the empty value, since that source did decide
  effect: the bound struct field and the provenance entry both read the falsy choice
default_wins:
  - a key with a default tag never receives its falsy choice
  - reason: the default already states what an unset key means
emptiness:
  - the falsy choice joins "" and false as an empty parent in rule:dependent-key-visibility
  - the falsy field itself stays visible; only its dependents disappear
timing: after all source layers merge, before generated apply
scope:
  - Bind string fields only
  - no effect on CLI flags, help, or scaffolds
related:
  - decision:falsy-tag-form
  - rule:dependent-key-visibility
  - rule:source-precedence
  - concept:config-overlay
  - api:configbind-provenance
  - flow:config-load
```
