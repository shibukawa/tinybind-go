---
id: rule:duration-value-parsing
type: rule
title: Duration Value Parsing
---
Duration config values accept only Go ParseDuration strings; bare numbers are an error in every source.

```yaml
accepted:
  - time.ParseDuration syntax, e.g. 5s, 1h30m, 250ms, 0s
  - optional leading minus where the field semantics allow it
rejected:
  - bare integer such as 5, in TOML, env, CLI, and default tag
  - TOML integer or float literal for a duration key
  - empty string when the key is present; absent key falls back to the default layer
rationale:
  - a unitless number cannot say seconds or nanoseconds without hidden convention
  - one accepted form keeps TOML, env, CLI, and the default tag interchangeable
rendering:
  - scaffold and provenance use time.Duration.String()
  - TOML scaffold quotes the value because the key type is a string
validation_time:
  - default tag: generation time; a bad default fails codegen
  - external sources: load time; error names the term:config-key and the offending raw value
scope:
  - configbind duration fields only
  - does not change how other scalars parse
related:
  - requirement:duration-config-fields
  - decision:configbind-supported-types
  - decision:struct-field-tags
  - term:config-key
  - concept:config-overlay
  - flow:config-load
```
