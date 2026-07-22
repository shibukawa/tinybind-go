---
id: data:config-scaffold-fragment
type: data
title: Config Scaffold Fragment
---
Generated metadata for one discovered Bind type-and-prefix registration; runtime aggregation renders final TOML and .env scaffolds.

```yaml
identity:
  - Go package path
  - Bind type identity
  - Bind prefix
contains:
  - stable field keys and kinds
  - default values
  - help comments
  - CLI option names
  - environment overrides and disable markers
excludes:
  - final whole-application TOML text
  - final whole-application .env text
  - api:configbind-subcommand fields
```
