---
id: requirement:duration-config-fields
type: requirement
title: Duration Config Fields
---
time.Duration struct fields bind from every config source through a dedicated duration field kind.

```yaml
priority: must
intent: express timeouts and intervals as time.Duration instead of unit-ambiguous ints
motivation:
  - decision:configbind-supported-types already lists duration as a v1 scalar
  - time.Duration currently degrades to the int field kind because its underlying type is int64
  - int binding rejects "5s" and silently accepts a unitless number
field_kind:
  codegen_name: FieldDuration
  scaffold_name: ScaffoldDuration
  cli_kind: string token; cliparser needs no new Kind
  go_type: time.Duration
detection:
  - match the named type time.Duration before falling through to underlying basic kinds
  - a locally defined named type whose underlying type is time.Duration is not duration in v1
value_form: rule:duration-value-parsing
behavior:
  - default tag holds a ParseDuration string, validated at generation time
  - TOML, env, and CLI layers carry the same string form into concept:config-overlay
  - generated apply parses the raw string with time.ParseDuration; no reflection
  - scaffold and provenance render duration through String()
  - array of duration is out of scope in v1
related:
  - decision:configbind-supported-types
  - rule:duration-value-parsing
  - concept:config-struct-mapping
  - concept:cli-option-codegen
  - requirement:struct-field-metadata
  - requirement:scaffold-generation
  - requirement:configbind-tinygo
  - flow:config-load
  - flow:configbind-codegen
  - system:configbind
acceptance:
  - 'field ReadTimeout time.Duration with default:"5s" loads as 5s when unset elsewhere'
  - TOML read_timeout = "1h30m" parses to 90m
  - env and CLI accept the same ParseDuration strings
  - unparsable duration returns a load error naming the config key
  - a default tag that ParseDuration rejects fails generation, not load
  - TOML scaffold emits a quoted duration string, not a bare number
  - provenance Value for a duration key is its String() form
  - generated code compiles under TinyGo without reflect
```
