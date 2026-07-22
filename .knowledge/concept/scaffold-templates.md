---
id: concept:scaffold-templates
type: concept
title: Scaffold Templates
---
Plain-text TOML and .env templates rendered from registered per-Bind type-and-prefix generated fragments.

```yaml
formats:
  - toml
  - env
delivery:
  - generated data:config-scaffold-fragment registration per Bind type and prefix
  - api:config-scaffold-output
  - application owns any command, stdout, or file-writing behavior
sources:
  - api:configbind-bind prefixes and types only
  - decision:struct-field-tags default help enum
  - data:cli-flag-def help for comments
  - decision:toml-shape-constraints
excluded:
  - api:configbind-subcommand
content:
  - keys for each Bind option field under prefix tables
  - comments from help tags
  - example values from default tags
  - TOML keys stay field keys; not renamed by opt CLI aliases
  - environment names follow runtime opt and env overrides; env:"-" is omitted
pipeline: flow:configbind-codegen
related:
  - requirement:modular-package-generation
  - requirement:scaffold-generation
  - requirement:struct-field-metadata
  - system:configbind
  - concept:config-struct-mapping
  - decision:cli-flag-naming
```
