---
id: decision:dependon-tag-form
type: decision
title: dependon Tag Form
---
A dependon tag names one parent config key whose emptiness hides the field from generated output.

```yaml
status: accepted
value: one term:config-key, absolute by default
value_test: >
  the tag may also test the parent's value rather than its emptiness; the parent
  half of the tag is unchanged, so see decision:dependon-value-condition
absolute_form:
  spelling: 'dependon:"middleware.rdb.dsn"'
  why_default:
    - a dependency often crosses Bind prefixes, which only an absolute key can express
    - the overlay key space is already flat and absolute
relative_form:
  spelling: 'dependon:".enabled"'
  resolves_against: the absolute key of the struct the tag is written in
  motivation: decision:shared-config-struct-instances
  effect: one tag on a shared struct type names a sibling under every prefix it is embedded at
  state: implemented
cardinality:
  rule: one parent per tag; a key answers to one tag per struct level above it
  still_out_of_scope: comma-separated parent lists and boolean combinations
  comma_now_means: >
    alternative values of one parent, and only after an operator; a comma with no
    operator is still a rejected parent list, per decision:dependon-value-condition
parent_constraints:
  - parent must be a known key of some registered definition
  - parent kind must be string or bool, or an int or duration that declares its own falsy
  - a list parent is rejected; an empty list is a setting, not an absent one
  - self-reference and dependency cycles are rejected
  - a parent may itself carry dependon; visibility is transitive
  - a key may hold several parents once a struct above it declares one; all must be non-empty
  - >
    the falsy prerequisite for a number or duration parent lifts when the tag
    names the off value inline; see decision:dependon-value-condition
placement:
  - Bind option struct fields
  - nested struct fields, using the nested field's own absolute key
  - a struct field marks the whole subtree when placed on the parent struct field
subtree_placement:
  semantics: rule:dependent-key-visibility subtree_scope
  state: implemented; the parent is carried to every leaf of the subtree
  falsy_on_a_struct: rejected, since falsy names one value and a struct has none
array_of_tables_placement:
  element_field: rejected; an element key carries a runtime index, so nothing can name it
  array_field:
    verdict: allowed
    effect: an empty parent hides the array and all its elements
    correction: >
      the earlier blanket rejection gave one reason, that elements have no stable
      key, for both placements; the array field does have one
    detail: requirement:array-of-tables-provenance
effect: rule:dependent-key-visibility
scope_limits:
  - presentation only; see rule:dependent-key-visibility for what it never changes
validation_time:
  when: generation time; a rejected tag fails go generate, never load
  visible_scope: >
    only keys emitted in the same generation run can be checked, because
    requirement:modular-package-generation generates each package separately
  rejected_at_codegen:
    - dependon or falsy on an array-of-tables element field, which has no stable key
    - falsy on the array field itself, which has no single value
    - falsy on a struct field
    - a leaf-only tag on a struct field, per requirement:struct-tag-placement-totality
    - a parent in this run that is a list, or a number or duration with no falsy tag
    - self-reference
    - comma-separated parent list, i.e. a comma with no operator before it
    - cycle among parents in this run
    - a value condition's own rejections, per decision:dependon-value-condition
    - dependon on a subcommand field, which has no overlay
  passes_through:
    - parent bound in another package; unresolvable here
  runtime_consequence: >
    a typo'd or foreign parent that never reaches the overlay reads as empty and
    hides its dependents; see rule:dependent-key-visibility
example:
  go: |
    type MiddlewareConfig struct {
      RDB RDBConfig
    }
    type RDBConfig struct {
      DSN      string `help:"database DSN"`
      PoolSize int    `default:"10" dependon:"middleware.rdb.dsn" help:"connection pool size"`
    }
  effect: empty middleware.rdb.dsn hides middleware.rdb.pool_size; the dsn line itself stays
related:
  - decision:struct-field-tags
  - decision:shared-config-struct-instances
  - decision:dependon-value-condition
  - data:dependency-condition
  - rule:dependent-key-visibility
  - requirement:dependent-field-visibility
  - term:config-key
  - concept:provenance-log-helper
  - api:configbind-provenance
  - system:configbind
```
