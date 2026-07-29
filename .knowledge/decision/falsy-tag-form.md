---
id: decision:falsy-tag-form
type: decision
title: falsy Tag Form
---
A falsy tag names the enum choice that means "off" for a string option.

```yaml
status: accepted
form: 'falsy:"off"'
value: one choice of the field's own value set, normally an enum member
motivation:
  - an enum-style switch is disabled by a named choice, not by "" or false
  - rule:dependent-key-visibility could otherwise only read "" and false as empty
applies_to:
  - Bind string fields, typically ones carrying an enum tag
rejected_at_codegen:
  - falsy on a bool, int, duration, or slice field
  - falsy on a subcommand field, which has no overlay
not_enforced_yet:
  - membership of the value in the enum allowlist; configbind does not read enum
    tags yet, so a mismatch is not caught
two_effects: rule:falsy-value-resolution
example:
  go: |
    type WebServerConfig struct {
      Tracing    string `enum:"off,otlp,jaeger" falsy:"off" help:"tracing exporter"`
      TracingURL string `dependon:"webserver.tracing" help:"tracing collector URL"`
    }
  effect: unset tracing resolves to "off", and "off" hides webserver.tracing_url
related:
  - decision:struct-field-tags
  - decision:dependon-tag-form
  - rule:falsy-value-resolution
  - rule:dependent-key-visibility
  - rule:enum-value-validation
  - requirement:dependent-field-visibility
  - system:configbind
```
