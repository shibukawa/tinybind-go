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
enum_parents:
  tag: decision:falsy-tag-form
  policy: rule:falsy-value-resolution
  purpose: let an enum-style parent declare which choice means "off"
surfaces:
  - provenance records returned from api:configbind-provenance only
non_goals:
  - skipping apply or leaving dependent struct fields at their zero value
  - rejecting a dependent value set while the parent is empty
  - hiding CLI flags or help entries
  - hiding scaffold lines, which must stay discoverable before any load
  - inferring a parent from key path nesting without the tag
codegen:
  - resolve the parent key and kind at generation time
  - emit the parent key into the generated definition alongside the field
  - unknown parent, non string or bool parent, self-reference, or cycle fails generation
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
  - a parent with int value 0 does not hide its dependents
  - a hidden parent hides dependents of dependents
  - hidden fields are still populated in the bound struct
  - scaffold output still lists the field regardless of its parent
  - dependon naming an unregistered key fails go generate with the key in the message
  - 'a falsy:"off" field with no default resolves to "off" when nothing sets it'
  - 'a falsy:"off" parent holding "off" hides its dependents'
  - a falsy field that also has a default keeps the default
```
