---
id: api:config-scaffold-output
type: api
title: Config Scaffold Output API
---
Public configbind functions render all registered data:config-scaffold-fragment values without owning a CLI or file path.

```yaml
signatures:
  - func ScaffoldTOML() (string, error)
  - func ScaffoldEnv() (string, error)
  - func WriteScaffoldTOML(w io.Writer) error
  - func WriteScaffoldEnv(w io.Writer) error
errors:
  - invalid fragment metadata
  - duplicate config key
  - duplicate environment name
  - conflicting fragment identity
requirement: requirement:scaffold-generation
```
