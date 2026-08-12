---
id: data:dependency-condition
type: data
title: Dependency Condition
---
One resolved visibility condition in a generated Definition: a parent key, an optional operator, and its value set.

```yaml
shape: 'Dependency{Key string, Op string, Values []string}'
fields:
  Key: absolute term:config-key of the parent, already resolved from any dot-relative tag
  Op: '"" for the emptiness test, "=" for membership, "!=" for exclusion'
  Values: the tag's value list, empty when Op is empty
wire_form:
  form: 'Definition.DependsOn map[string][]Dependency'
  was: 'map[string][]string'
  compatibility: >
    a breaking change to a public field and to the generated file shape; every
    committed generated file regenerated in the same change
  emitted_spelling:
    emptiness: '{Key: "auth.enabled"}'
    value: '{Key: "auth.mode", Op: "=", Values: []string{"oidc_only"}}'
why_not_a_string:
  - tag parsing belongs to generation time; the generated tables exist so that load does none
  - a re-parsed string would repeat the work once per key per process start
element_order: declaration order, own tag last, so a diagnostic names the ancestor condition first
invariants:
  - Op empty implies Values empty
  - Op non-empty implies at least one non-empty value and no duplicates
  - Key never equals the dependent key
produced_by: flow:configbind-codegen
consumed_by: rule:dependent-key-visibility, through the provenance filter
related:
  - decision:dependon-value-condition
  - decision:dependon-tag-form
  - rule:dependent-key-visibility
  - api:configbind-provenance
  - flow:configbind-codegen
  - term:config-key
  - system:configbind
```
