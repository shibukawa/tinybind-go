---
id: rule:enum-value-validation
type: rule
title: Enum Value Validation
---
Fields with enum tags accept only the listed values from any source that sets the field.

```yaml
tag: 'enum:"a,b,c"'
scope: config structs; request models share the tag but not the default-must-be-listed rule, per rule:enum-tag-semantics
parse:
  - split on comma
  - trim spaces around each token, as request models do per rule:enum-tag-semantics
  - empty tokens rejected at generation
enforcement:
  - after value is chosen from default, TOML, env, or CLI
  - >
    state: implemented. The check is emitted into the generated apply function, so
    one test covers every source at once rather than each source filtering on its
    way in, and a value the tag does not list fails the load
  - value not in allowlist is a load/parse error
  - applies to Bind fields and SubCommand CLI fields when tagged
  - >
    for []string, each element must match. An allowlist on a list is the vocabulary
    its elements are drawn from; the whole-field reading would mean a field holding
    one of several fixed lists, which is a mode key and is written as a scalar. It
    also degrades better: an unlisted element names itself, where a whole-field
    match can only report that the list is wrong
  - >
    kinds: string, int, duration, and []string. An int or duration choice is matched
    on the parsed value, so 1m and 60s are one choice; bool is rejected, because
    true and false are already the only values it holds
  - >
    element fields of an array of tables are covered, and their rejection names the
    index, per requirement:array-of-tables-provenance diagnostics
generation_time_checks:
  - a default outside its own enum, which this rule requires to be listed
  - >
    a falsy outside its own enum, as in enum:"off,otlp,jaeger" falsy:"off" mistyped.
    The typo disables the emptiness test decision:dependon-value-condition rides on
    with no runtime symptom at all
  - an empty choice, a repeated choice, and a choice unparsable at the field's kind
cli_and_scaffold:
  - >
    state: implemented. The choices reach data:cli-flag-def and the scaffold field,
    so they are printed where the value is chosen rather than only where it is
    rejected; this is most of the practical value and it is far cheaper than the
    load-time check
  - usage/help lists the allowed values beside the help text
  - scaffold comments list them on a line of their own under the help lines
  - >
    a line of its own rather than a suffix, so a multi-line help comment keeps the
    choices at a fixed position
related:
  - decision:struct-field-tags
  - decision:enum-tag-form
  - decision:dependon-value-condition
  - rule:enum-tag-semantics
  - requirement:struct-field-metadata
  - requirement:struct-tag-placement-totality
  - requirement:array-of-tables-provenance
  - requirement:scaffold-generation
  - flow:config-load
  - requirement:cli-option-codegen
  - data:cli-flag-def
  - system:configbind
```
