---
id: requirement:config-env-interpolation
type: requirement
title: Config File Env Interpolation
---
Expand `${NAME}` references in file-layer string values from the process environment, so secrets reach array-of-tables elements that have no env or CLI form of their own.

```yaml
priority: must
motivating_case: >
  several database connections declared as one struct slice; each element needs
  its credential injected from outside the config file
problem: >
  decision:configbind-supported-types gives array-of-tables elements no flag and
  no env var, so before this requirement an element credential could only be
  written literally in the file
applies_to: file layer of requirement:layered-config-load
syntax: rule:env-interpolation-syntax
expansion_site: decision:env-interpolation-layer
scope:
  - string values in the single resolved TOML file
  - string elements of primitive arrays
  - fields of array-of-tables elements, expanded per element
out_of_scope:
  - keys, table headers, and non-string TOML scalars
  - values arriving from the default, env, and cli layers
  - api:configbind-subcommand fields, which read no file
place_after_expansion: file_toml
rationale:
  - element count stays data owned by the file; only leaf values come from outside
  - an index-encoded form such as DATABASES_0_PASSWORD would move the element
    count into the env layer and split ownership of it between two sources
  - one mechanism covers every nesting depth, not only struct slices
  - an undefined name is an error, so a missing secret cannot silently overwrite
    a default tag value with the empty string
secret_handling:
  - expanded values follow rule:secret-redaction like any other value
  - explicit secret-tag filtering is tracked separately and covers provenance leaks
acceptance:
  - two [[db]] elements expand their own references to distinct values
  - a value mixing literal text and ${NAME} yields the concatenation
  - an undefined name fails the load, naming the variable and the term:config-key
  - a set-but-empty variable expands to the empty string without error
  - $$ yields one literal $ and starts no expansion
  - expansion reads the same environment set the env layer reads, so injected
    test environments stay authoritative
  - ${...} written in an env or cli value stays literal
  - an expanded key reports place file_toml in provenance
related:
  - rule:env-interpolation-syntax
  - decision:env-interpolation-layer
  - requirement:layered-config-load
  - requirement:source-provenance-logging
  - requirement:configbind-tinygo
  - decision:configbind-supported-types
  - decision:toml-shape-constraints
  - decision:struct-field-tags
  - rule:source-precedence
  - rule:secret-redaction
  - rule:default-tag-semantics
  - concept:config-overlay
  - flow:config-load
  - system:configbind
```
