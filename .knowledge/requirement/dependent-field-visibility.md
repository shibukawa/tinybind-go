---
id: requirement:dependent-field-visibility
type: requirement
title: Dependent Field Visibility
---
Fields that only matter when a parent setting is enabled declare that parent and vanish from output while it is empty.

```yaml
priority: should
intent: keep effective-config output readable by suppressing settings of a disabled feature
problem: an unused subsystem still prints its whole default block, burying the settings in use
tag: decision:dependon-tag-form
policy: rule:dependent-key-visibility
off_valued_parents:
  tag: decision:falsy-tag-form
  policy: rule:falsy-value-resolution
  purpose: let a parent whose off state is not "" or false declare which value means off
  kinds: enum-style strings, and numbers or durations whose zero disables a feature
shared_struct_types:
  relative_parent: one tag on a shared struct type resolves per prefix it is embedded at
  subtree_parent: a tag on a nested struct field covers every field below it
  conjunctive: a key with several parents needs all of them non-empty
  detail: decision:shared-config-struct-instances
surfaces:
  - provenance records returned from api:configbind-provenance only
non_goals:
  - skipping apply or leaving dependent struct fields at their zero value
  - rejecting a dependent value set while the parent is empty
  - hiding CLI flags or help entries
  - hiding scaffold lines, which must stay discoverable before any load
  - inferring a parent from key path nesting without the tag
codegen:
  - resolve every parent key and kind at generation time
  - emit the parent keys into the generated definition alongside the field
  - a list parent, a number or duration parent with no falsy tag, self-reference, or a cycle fails generation
related:
  - decision:dependon-tag-form
  - rule:dependent-key-visibility
  - decision:struct-field-tags
  - requirement:struct-field-metadata
  - requirement:source-provenance-logging
  - requirement:scaffold-generation
  - concept:provenance-log-helper
  - api:configbind-provenance
  - flow:config-load
  - flow:configbind-codegen
  - system:configbind
acceptance:
  - 'pool_size with dependon:"middleware.rdb.dsn" is absent from provenance while dsn is ""'
  - middleware.rdb.dsn itself still appears in provenance while empty
  - setting dsn makes pool_size appear again with its own winning Place
  - 'a bool parent tls.enabled=false hides every field that depends on it'
  - a parent with int value 0 and no falsy tag does not hide its dependents
  - 'a slow_threshold duration with falsy:"0s" hides its dependents at 0, 0s, and 0ms'
  - 'one dependon:".enabled" on a shared struct resolves per embedding prefix'
  - a dependon on a nested struct field hides every key of that subtree
  - a key under a dependent struct that also names its own parent needs both to be set
  - a hidden parent hides dependents of dependents
  - hidden fields are still populated in the bound struct
  - scaffold output still lists the field regardless of its parent
  - dependon naming an unregistered key fails go generate with the key in the message
  - 'a falsy:"off" field with no default resolves to "off" when nothing sets it'
  - 'a falsy:"off" parent holding "off" hides its dependents'
  - a falsy field that also has a default keeps the default
```
