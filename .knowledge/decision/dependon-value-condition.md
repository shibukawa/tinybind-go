---
id: decision:dependon-value-condition
type: decision
title: dependon Value Condition
---
A dependon tag may test the parent's value, not only its emptiness, so a variant-selecting key hides every branch it did not select.

```yaml
status: accepted
state: implemented
supersedes_limit: decision:dependon-tag-form cardinality.out_of_scope_v1
problem:
  - >
    an emptiness test cannot express "this subtree belongs to one mode": a mode
    key holds a non-empty value in every mode, so every mode's subtree stays visible
  - >
    the reader is not merely shown too much; they are shown inert settings as if
    in force, which is why requirement:effective-config-brevity treats this as
    correctness and not only volume
grammar:
  form: 'dependon:"<parent><op><values>"'
  parent: unchanged from decision:dependon-tag-form; absolute or dot-relative
  op: '"=" or "!=" ; absent means the emptiness test of rule:dependent-key-visibility'
  values: one or more values separated by comma
  parse_order:
    - locate the first "=" ; a preceding "!" makes the op "!="
    - the parent key is the text before the op, then resolved as before
    - the value list is all remaining text
  value_charset:
    may_contain: '"=" , since only the first one is the op'
    may_not_contain: '"," , which separates values, as in the enum tag'
comma_disambiguation:
  with_op: comma separates alternative values of one parent
  without_op: still rejected as a parent list, unchanged
  why: >
    one tag carries one condition, so a comma never has to mean two things at
    once; a reader decides which meaning applies by looking for the op alone
conjunction:
  unchanged: a key's conditions are its own tag plus every ancestor struct's tag, all required
  why_no_multi_condition_tag: >
    nesting already expresses AND, and the concrete cases need no second
    separator; leaving ";" unused keeps it available if one ever does
enabled_gate_comes_free:
  observation: a mode parent normally carries its own dependon on the feature switch
  effect: >
    a value condition on that mode inherits the switch transitively, per
    rule:dependent-key-visibility transitivity, so no "enabled AND mode=x" tag is needed
  example: 'auth.mode carries dependon:".enabled", so dependon:".mode=oidc_passkey" is off when auth is off'
evaluation:
  visible_when:
    '=': the parent's effective value equals one listed value
    '!=': the parent's effective value equals none of them
  emptiness_not_consulted: >
    an op states the whole test; also applying the empty/falsy rule would make
    'dependon:"x=off"' hide at the value it names
  absent_parent: compares as the empty string, so "=" hides and "!=" shows
  absent_parent_direction: >
    over-showing on an unknown parent, matching the over-masking direction
    rule:secret-redaction already takes for an untagged key
  transitivity_kept: a parent hidden by its own conditions still hides this key
comparison: rule:falsy-value-resolution comparison, i.e. by the parent's kind
parent_kinds:
  string: exact match, no folding or trimming beyond the tag's own
  bool: '"=true" or "=false"; "=false" is the useful one, showing a fallback only while a feature is off'
  int_and_duration: parsed value, so "=0", "=0s", and "=0ms" are one value
  string_slice: still rejected; a list has no single value to compare
  falsy_no_longer_required: >
    an int or duration parent needs no falsy tag once a condition names the value
    inline; see decision:falsy-tag-form
validated_at_codegen:
  - each value token non-empty
  - no duplicate value in one list
  - a bool value parses as a bool; an int value at the field's width; a duration as a Go duration
  - every value is a member of the parent's enum tag, when the parent declares one
  - the existing parent-kind, self-reference, and cycle checks, unchanged
enum_check:
  why: >
    a mistyped value hides a whole subtree silently and forever, which is the one
    failure this feature can cause and the one a reader cannot diagnose from output
  first_consumer: >
    rule:enum-value-validation is specified but still unimplemented at load; this
    check is the first thing that reads a config enum tag at all, which is what
    makes the tag load-bearing
  plumbing: an Enum member on the codegen Field, read in the config field collector next to falsy and secret
  plumbing_state: implemented
  placement_obligation: >
    reading a tag creates a placement row for it, so the same change must reject
    enum on a struct field and on an array-of-tables element field, per
    requirement:struct-tag-placement-totality; skipping it reopens the
    accepted-and-dropped class that requirement closed
  placement_state: implemented; enum is rejected on a struct, an array, and an element field
  best_effort_scope: >
    only a parent emitted in the same generation run can be checked, the same
    limit checkParentKind already accepts per requirement:modular-package-generation
  not_load_time_validation: >
    this reads the enum tag to check a sibling tag, and does not implement
    rule:enum-value-validation, which is about rejecting a value from a source
  downstream_prerequisite: >
    the two parents the concrete cases name, auth.mode and session.backend, carry
    no enum tag yet; adding one is part of adopting this
rejected_alternatives:
  separate_tag:
    spelling: 'showwhen:"auth.mode=oidc_only"'
    why_not: >
      the evaluation, the conjunction, and the transitivity are the same machinery,
      and two tags would raise what happens when a field carries both
  encode_in_the_string_wire_form:
    spelling: keep Definition.DependsOn as map[string][]string and store "auth.mode=oidc_only,x"
    why_not: >
      it moves tag parsing into every load, and generated tables exist so that
      parsing happened at generation time; see data:dependency-condition
  parent_list_in_one_tag:
    why_not: comma would mean parents in one tag and values in another
scope_unchanged:
  - provenance output only, per rule:dependent-key-visibility never_applies_to
  - scaffolds still list every key, since they render before any load
  - CLI flags, apply, and validation are untouched
future_not_now:
  - warn when a key is set while its value condition fails, a stronger mistake signal than a set child under an empty parent
  - '";" for several conditions in one tag'
  - a contains test for a slice parent
wire_form: data:dependency-condition
example:
  go: |
    type Config struct {
      Enabled bool   `default:"false"`
      Mode    string `default:"oidc_only" enum:"oidc_only,oidc_passkey,jwt_only" dependon:".enabled"`
      OIDC    OIDCConfig    `dependon:".mode=oidc_only,oidc_passkey"`
      Passkey PasskeyConfig `dependon:".mode=oidc_passkey"`
      JWT     JWTConfig     `dependon:".mode=jwt_only"`
    }
  effect: 'auth.mode=oidc_only leaves auth.oidc visible and removes auth.passkey and auth.jwt'
  session_case: 'dependon:".backend=redis" on the Redis struct, one per backend struct'
related:
  - decision:dependon-tag-form
  - decision:falsy-tag-form
  - decision:struct-field-tags
  - decision:shared-config-struct-instances
  - rule:dependent-key-visibility
  - rule:falsy-value-resolution
  - rule:enum-value-validation
  - rule:secret-redaction
  - requirement:dependent-field-visibility
  - requirement:effective-config-brevity
  - requirement:modular-package-generation
  - requirement:struct-tag-placement-totality
  - data:dependency-condition
  - api:configbind-provenance
  - term:config-key
  - system:configbind
```
