---
id: requirement:dependent-field-visibility
type: requirement
title: Dependent Field Visibility
---
Fields that only matter under a particular parent setting declare the condition and vanish from output while it fails.

```yaml
priority: should
intent: an effective-config reader sees the settings in force and not the ones that are inert
problem: an unused subsystem still prints its whole default block, burying the settings in use
volume_and_correctness: requirement:effective-config-brevity
tag: decision:dependon-tag-form
policy: rule:dependent-key-visibility
off_valued_parents:
  tag: decision:falsy-tag-form
  policy: rule:falsy-value-resolution
  purpose: let a parent whose off state is not "" or false declare which value means off
  kinds: enum-style strings, and numbers or durations whose zero disables a feature
variant_selected_subtrees:
  tag: decision:dependon-value-condition
  purpose: >
    let a subtree belong to one value of a mode or backend key, which an emptiness
    test cannot express because such a key is non-empty in every variant
  cases: one backend struct per session.backend value, one login-method struct per auth.mode value
  safety: the values are checked against the parent's enum tag at generation time
shared_struct_types:
  relative_parent: one tag on a shared struct type resolves per prefix it is embedded at
  subtree_parent: a tag on a nested struct field covers every field below it
  conjunctive: a key with several conditions needs all of them to pass
  detail: decision:shared-config-struct-instances
surfaces:
  - provenance records returned from api:configbind-provenance only
non_goals:
  - skipping apply or leaving dependent struct fields at their zero value
  - rejecting a dependent value set while the condition fails
  - hiding CLI flags or help entries
  - hiding scaffold lines, which must stay discoverable before any load
  - inferring a parent from key path nesting without the tag
  - hiding a key merely because it sits at its default; that is the other lever of requirement:effective-config-brevity
codegen:
  - resolve every parent key, kind, and condition at generation time
  - emit the conditions into the generated definition alongside the field, per data:dependency-condition
  - a list parent, a number or duration parent with neither a falsy tag nor an inline value, self-reference, or a cycle fails generation
  - a value outside the parent's declared enum fails generation
related:
  - decision:dependon-tag-form
  - decision:dependon-value-condition
  - rule:dependent-key-visibility
  - requirement:effective-config-brevity
  - data:dependency-condition
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
  - 'dependon:".mode=oidc_only,oidc_passkey" keeps its subtree at either value and hides it at jwt_only'
  - 'dependon:".backend!=cookie" hides its subtree only at cookie'
  - a value condition on a duration parent needs no falsy tag, and matches 0, 0s, and 0ms alike
  - a value condition whose parent is off through its own dependon hides the subtree whatever the value
  - a value outside the parent's enum fails go generate with the value and the parent in the message
  - a comma in a dependon with no operator still fails go generate as a parent list
```
