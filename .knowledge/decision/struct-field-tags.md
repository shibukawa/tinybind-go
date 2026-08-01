---
id: decision:struct-field-tags
type: decision
title: Config Struct Field Tags
---
Struct field tags declare defaults, help, CLI names, enum allowlists, secret disclosure, parent dependencies, and positional arg roles.

```yaml
status: accepted
option_tags:
  default:
    form: 'default:"value"'
    meaning: default value string parsed into the field type
    layer: default in rule:source-precedence for Bind fields
    shared_form: request models use the same tag per decision:default-tag-form
  help:
    form: 'help:"text"'
    meaning: human label for CLI usage/help and TOML scaffold comments
    when_absent: seeded from the field godoc comment and backfilled into source via decision:godoc-help-precedence
  opt:
    form: 'opt:"long[,short]"'
    meaning: override CLI flag names; suppress default --prefix-key
    naming: decision:cli-flag-naming
    example: 'opt:"port,p" yields --port and -p'
  enum:
    form: 'enum:"a,b,c"'
    meaning: comma-separated allowlist of accepted string values
    validation: rule:enum-value-validation
    shared_form: request models use the same tag per decision:enum-tag-form
    applies_to:
      - string scalar fields primarily
      - other scalar string-like encodings TBD if needed
  secret:
    form: 'secret:"hide|mask|show"'
    meaning: how values appear in provenance log helpers
    modes:
      hide: omit entry from log output
      mask: replace value with a fixed-width asterisk run; see rule:secret-redaction
      show: emit raw value even when the key name looks sensitive
    default_without_tag: rule:secret-redaction auto policy
    placement: a leaf field, a nested struct field, or an array-of-tables field, each covering its whole subtree
    element_fields: honored per requirement:array-of-tables-provenance
    redaction: rule:secret-redaction
  falsy:
    form: 'falsy:"off"'
    meaning: the value that means "off" for this option
    resolution: rule:falsy-value-resolution
    applies_to: string, int, and duration fields
    required_for: an int or duration named as a dependon parent
    detail: decision:falsy-tag-form
  dependon:
    form: 'dependon:"prefix.parent_key"' or 'dependon:".sibling_key"'
    meaning: hide this field from provenance output while the named parent is empty
    parent_key: one term:config-key, absolute or dot-prefixed relative; see decision:dependon-tag-form
    placement: a leaf field, a nested struct field, or an array-of-tables field, each covering its whole subtree
    not_on: an array-of-tables element field, whose key carries a runtime index
    visibility: rule:dependent-key-visibility
    scope: output only; apply, CLI flags, and validation are unaffected
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
  - Bind option fields use default, help, optional opt, optional enum, optional secret, optional dependon, optional falsy
  - SubCommand fields are CLI-only; no TOML or env mapping; may use opt and help
  - positional arg fields use arg tags on subcommand option structs only
  - help text seeds generated CLI --help and Bind TOML scaffold comments
  - help tag wins over any godoc comment; see decision:godoc-help-precedence
  - struct type godoc has no tag form and feeds the scaffold table comment only
  - enum allowlist is enforced after parse from every source that sets the field
  - default value must be in enum when both tags are present
  - secret tag affects log helpers only, not runtime stored values
  - dependon affects output visibility only; the field is still applied
  - falsy affects the resolved value and dependent visibility; a default outranks it
  - opt changes CLI surface only; overlay config_key stays prefix.field_key
  - every tag on a nested struct field either propagates or fails generation; see requirement:struct-tag-placement-totality
example:
  go: |
    type WebServerConfig struct {
      Port int `default:"8080" help:"HTTP listen port" opt:"port,p"`
      // TOML [webserver] port; CLI --port -p; no --webserver-port
      ReadTimeout time.Duration `default:"5s" help:"read timeout"`
      // CLI default --webserver-read_timeout (or normalized key form)
      // duration form per rule:duration-value-parsing
      LogLevel string `default:"info" enum:"debug,info,warn,error" help:"log level"`
      APIToken string `secret:"hide" help:"API token"`
      TLSCertPath string `dependon:"webserver.tls.enabled" help:"TLS certificate path"`
      Tracing string `enum:"off,otlp" falsy:"off" help:"tracing exporter"`
      TracingURL string `dependon:"webserver.tracing" help:"collector URL"`
    }
  default_flag_without_opt:
    - '[webserver] port -> --webserver-port'
  with_opt:
    - 'opt:"port,p" -> --port, -p only'
related:
  - requirement:struct-field-metadata
  - requirement:source-provenance-logging
  - requirement:dependent-field-visibility
  - requirement:duration-config-fields
  - decision:dependon-tag-form
  - decision:falsy-tag-form
  - rule:dependent-key-visibility
  - rule:falsy-value-resolution
  - rule:duration-value-parsing
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
