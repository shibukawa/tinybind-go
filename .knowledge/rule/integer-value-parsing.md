---
id: rule:integer-value-parsing
type: rule
title: Integer Value Parsing
---
Integer config values parse at the declared field width; an out-of-range value is an error, never a wrap.

```yaml
requirement: requirement:sized-integer-config-fields
bit_size:
  signed: strconv.ParseInt with the field bit size
  unsigned: strconv.ParseUint with the field bit size
  int_and_uint: strconv.IntSize, so the accepted range follows the build target
accepted:
  - base-10 digits
  - optional leading minus on a signed field
rejected:
  - a value outside the field range
  - a leading minus on an unsigned field
  - hex, octal, underscore, and other Go literal forms
  - empty string when the key is present; an absent key falls back to the default layer
rationale:
  - parsing at 64-bit and assigning narrower hides an overflow as a wrapped value
  - one base-10 form keeps TOML, env, CLI, and the default tag interchangeable
validation_time:
  - default tag: generation time; an out-of-range default fails codegen
  - external sources: load time; the error names the term:config-key and the offending raw value
scope:
  - configbind integer fields only
  - includes integer fields of an array-of-tables element, which report the full dotted key
  - does not change duration parsing, which rule:duration-value-parsing owns
related:
  - requirement:sized-integer-config-fields
  - decision:configbind-supported-types
  - rule:duration-value-parsing
  - term:config-key
  - concept:config-overlay
  - flow:config-load
```
