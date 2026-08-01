---
id: data:config-scaffold-fragment
type: data
title: Config Scaffold Fragment
---
The Scaffold field of one generated configbind Definition; runtime aggregation renders final TOML and .env scaffolds.

```yaml
identity:
  - Go package path
  - Bind type identity
  - Bind prefix
contains:
  - stable field keys and kinds
  - default values
  - help comments
  - Doc text from the struct godoc comment, rendered above the TOML table header
  - CLI option names
  - environment overrides and disable markers
registration: func Register[T any](definition Definition)
excludes:
  - final whole-application TOML text
  - final whole-application .env text
  - api:configbind-subcommand fields
```
