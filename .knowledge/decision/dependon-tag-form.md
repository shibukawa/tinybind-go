---
id: decision:dependon-tag-form
type: decision
title: dependon Tag Form
---
A dependon tag names one parent config key whose emptiness hides the field from generated output.

```yaml
status: accepted
form: 'dependon:"middleware.rdb.dsn"'
value: exactly one absolute term:config-key, including the Bind prefix
why_absolute:
  - a dependency often crosses Bind prefixes, so a prefix-relative form cannot express it
  - the overlay key space is already flat and absolute
cardinality:
  v1: one parent per field
  out_of_scope_v1: comma-separated parent lists and boolean combinations
parent_constraints:
  - parent must be a known key of some registered definition
  - parent kind must be string or bool; other kinds have no defined empty form
  - self-reference and dependency cycles are rejected
  - a parent may itself carry dependon; visibility is transitive
placement:
  - Bind option struct fields
  - nested struct fields, using the nested field's own absolute key
  - a struct field marks the whole subtree when placed on the parent struct field
effect: rule:dependent-key-visibility
scope_limits:
  - presentation only; see rule:dependent-key-visibility for what it never changes
validation_time:
  when: generation time; a rejected tag fails go generate, never load
  visible_scope: >
    only keys emitted in the same generation run can be checked, because
    requirement:modular-package-generation generates each package separately
  rejected_at_codegen:
    - parent in this run whose kind is neither string nor bool
    - self-reference
    - comma-separated parent list
    - cycle among parents in this run
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
  - rule:dependent-key-visibility
  - requirement:dependent-field-visibility
  - term:config-key
  - concept:provenance-log-helper
  - api:configbind-provenance
  - system:configbind
```
