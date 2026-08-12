---
id: decision:falsy-tag-form
type: decision
title: falsy Tag Form
---
A falsy tag names the value that means "off" for a string, int, or duration option.

```yaml
status: accepted
form: 'falsy:"off"', 'falsy:"0s"', 'falsy:"0"'
value: one value of the field's own type; for a string, normally an enum member
motivation:
  - an enum-style switch is disabled by a named choice, not by "" or false
  - a zero threshold or timeout is a common way to say a feature is off
  - rule:dependent-key-visibility could otherwise only read "" and false as empty
applies_to:
  - Bind string fields, typically ones carrying an enum tag
  - Bind int and duration fields, where zero is the usual off value
validated_at_codegen:
  - an int falsy must parse at the field's own width
  - a duration falsy must parse as a Go duration
rejected_at_codegen:
  - falsy on a bool field, which already has false, or on a slice field
  - falsy on a struct field, which has no single value
  - falsy on an array-of-tables element field, whose key is per element, not stable
  - falsy on a subcommand field, which has no overlay
zero_valued_parents:
  status: accepted
  rule: >
    a number or duration is a deliberate setting on its own, so only an explicit
    falsy tag may reclassify its zero as off; without the tag it cannot be a
    dependon parent at all
  comparison: by parsed value, so 0, 0s, and 0ms are one duration
  kind_source: the parent's generated Scaffold entry, which already names every key's kind
  no_longer_the_only_way: >
    decision:dependon-value-condition lets a dependent name the off value inline,
    which needs no falsy tag on the parent; falsy stays the way to say it once for
    every dependent, and stays the only way to reach the fill-in half of
    rule:falsy-value-resolution
not_enforced_yet:
  - >
    membership of the falsy value in the enum allowlist; configbind now reads enum
    tags for decision:dependon-value-condition, so the check is reachable and
    simply not written yet
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
  - decision:dependon-value-condition
  - rule:falsy-value-resolution
  - rule:dependent-key-visibility
  - rule:enum-value-validation
  - requirement:dependent-field-visibility
  - system:configbind
```
