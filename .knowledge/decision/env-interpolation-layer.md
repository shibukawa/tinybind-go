---
id: decision:env-interpolation-layer
type: decision
title: Env Interpolation Layer
---
configbind expands `${NAME}` while merging the parsed document into the overlay; minitoml stays a plain TOML parser.

```yaml
status: accepted
site: configbind file-layer merge, after parse and before overlay Set
alternatives_rejected:
  - id: expand-in-parser
    why_not: >
      interpolation is a configbind policy, not TOML syntax; minitoml stays
      reusable per concept:reusable-source-parsers
  - id: indexed-env-vars
    why_not: >
      DATABASES_0_PASSWORD moves the element count into the env layer and
      contradicts the rule that repeated settings are file-owned data
  - id: expand-at-apply
    why_not: >
      generated apply code would carry string scanning into every typed field,
      against decision:configbind-codegen-no-reflect minimalism
consequences:
  - one string choke point covers every value, because the merge already coerces
    scalars to string before writing concept:config-overlay
  - primitive arrays expand per element; array-of-tables elements ride the
    existing per-element recursion with no extra traversal
  - expanded values enter the overlay as place file_toml, so rule:source-precedence
    is unchanged and provenance keeps reporting the file as the winning layer
  - the env and cli layers never see ${...}, so no expansion loop exists
  - the environment set comes from the load options, not a direct process read,
    so file interpolation and the env layer cannot disagree
related:
  - requirement:config-env-interpolation
  - rule:env-interpolation-syntax
  - requirement:layered-config-load
  - concept:reusable-source-parsers
  - concept:config-overlay
  - decision:configbind-runtime-architecture
  - decision:configbind-codegen-no-reflect
  - decision:toml-config-format
  - flow:config-load
  - system:configbind
```
