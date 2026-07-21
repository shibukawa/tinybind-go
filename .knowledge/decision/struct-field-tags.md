---
id: decision:struct-field-tags
type: decision
title: Config Struct Field Tags
---
Struct field tags declare defaults, help, CLI names, enum allowlists, secret disclosure, and positional arg roles.

```yaml
status: accepted
option_tags:
  default:
    form: 'default:"value"'
    meaning: default value string parsed into the field type
    layer: default in rule:source-precedence for Bind fields
  help:
    form: 'help:"text"'
    meaning: human label for CLI usage/help and TOML scaffold comments
  opt:
    form: 'opt:"long[,short]"'
    meaning: override CLI flag names; suppress default --prefix-key
    naming: decision:cli-flag-naming
    example: 'opt:"port,p" yields --port and -p'
  enum:
    form: 'enum:"a,b,c"'
    meaning: comma-separated allowlist of accepted string values
    validation: rule:enum-value-validation
    applies_to:
      - string scalar fields primarily
      - other scalar string-like encodings TBD if needed
  secret:
    form: 'secret:"hide|mask|show"'
    meaning: how values appear in provenance log helpers
    modes:
      hide: omit entry from log output
      mask: replace value with asterisks of length ~5 plus random +/- 2
      show: emit raw value
    default_without_tag: rule:secret-redaction auto policy
    redaction: rule:secret-redaction
arg_tags:
  required:
    form: 'arg:"required"'
    meaning: required positional argument for a subcommand
  optional:
    form: 'arg:"optional"'
    meaning: optional positional argument for a subcommand
  rest:
    form: 'arg:"*"'
    meaning: remaining positional arguments as array or multi-value
rules:
  - Bind option fields use default, help, optional opt, optional enum, optional secret
  - SubCommand fields are CLI-only; no TOML or env mapping; may use opt and help
  - positional arg fields use arg tags on subcommand option structs only
  - help text seeds generated CLI --help and Bind TOML scaffold comments
  - enum allowlist is enforced after parse from every source that sets the field
  - default value must be in enum when both tags are present
  - secret tag affects log helpers only, not runtime stored values
  - opt changes CLI surface only; overlay config_key stays prefix.field_key
example:
  go: |
    type WebServerConfig struct {
      Port int `default:"8080" help:"HTTP listen port" opt:"port,p"`
      // TOML [webserver] port; CLI --port -p; no --webserver-port
      ReadTimeout time.Duration `default:"5s" help:"read timeout"`
      // CLI default --webserver-read_timeout (or normalized key form)
      LogLevel string `default:"info" enum:"debug,info,warn,error" help:"log level"`
      APIToken string `secret:"hide" help:"API token"`
    }
  default_flag_without_opt:
    - '[webserver] port -> --webserver-port'
  with_opt:
    - 'opt:"port,p" -> --port, -p only'
related:
  - requirement:struct-field-metadata
  - requirement:source-provenance-logging
  - requirement:cli-subcommands
  - requirement:cli-option-codegen
  - requirement:scaffold-generation
  - decision:cli-flag-naming
  - data:cli-flag-def
  - rule:enum-value-validation
  - rule:secret-redaction
  - api:configbind-bind
  - api:configbind-subcommand
  - concept:cli-option-codegen
  - concept:provenance-log-helper
  - system:configbind
```
