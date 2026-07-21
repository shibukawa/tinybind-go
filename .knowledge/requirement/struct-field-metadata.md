---
id: requirement:struct-field-metadata
type: requirement
title: Struct Field Defaults and Help
---
Defaults, help, opt CLI names, enum allowlists, and secret tags live on struct fields as the single metadata source.

```yaml
priority: must
tags: decision:struct-field-tags
behavior:
  - default tag seeds the default layer before TOML, env, and CLI
  - help tag provides CLI help labels and TOML scaffold comments
  - opt tag overrides CLI flag names via decision:cli-flag-naming
  - enum tag restricts accepted values via rule:enum-value-validation
  - secret tag controls provenance log disclosure via rule:secret-redaction
  - generator extracts data:cli-flag-def from fields at generate time
  - missing default means Go zero value unless type-specific policy TBD
acceptance:
  - field with default:":8080" is used when no external source sets the key
  - help text appears in generated CLI usage
  - help text appears as comments in Bind TOML scaffold stdout
  - opt:"port,p" yields --port and -p without --prefix-port
  - enum:"debug,info,warn,error" rejects values outside the list
  - default plus enum requires default to be a listed value
  - secret:"hide|mask|show" affects log helper output only
  - SubCommand tags affect CLI only, not TOML or env scaffolds
related:
  - decision:struct-field-tags
  - decision:cli-flag-naming
  - data:cli-flag-def
  - rule:enum-value-validation
  - rule:secret-redaction
  - requirement:layered-config-load
  - requirement:scaffold-generation
  - requirement:cli-option-codegen
  - requirement:source-provenance-logging
  - flow:config-load
  - system:configbind
```
