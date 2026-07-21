---
id: concept:scaffold-templates
type: concept
title: Scaffold Templates
---
Plain-text TOML and .env templates embedded in generated code and printed by scaffold subcommands.

```yaml
formats:
  - toml
  - env
delivery:
  - embedded plain text in generated Go
  - generated CLI subcommands print to stdout
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
pipeline: flow:configbind-codegen
related:
  - requirement:scaffold-generation
  - requirement:struct-field-metadata
  - system:configbind
  - concept:config-struct-mapping
  - decision:cli-flag-naming
```
