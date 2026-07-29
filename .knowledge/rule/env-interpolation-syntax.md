---
id: rule:env-interpolation-syntax
type: rule
title: Env Interpolation Syntax
---
Reference, escape, and failure rules for `${NAME}` inside file-layer string values.

```yaml
reference_form: '${NAME}'
name_charset: '[A-Za-z_][A-Za-z0-9_]*'
partial_expansion: >
  allowed; one value may mix literal text with any number of references, so a
  DSN can be assembled around an injected credential
escape:
  double_dollar: '$$ yields one literal $ and starts no expansion'
  lone_dollar: 'literal when not followed by { or $'
  why_not_backslash: >
    minitoml parseBasicString rejects unknown escapes, so \$ never reaches the
    interpolation layer; exempting literal strings instead would need a quoting
    flag on the parsed value, which decision:env-interpolation-layer avoids
undefined_name: load error
empty_value: 'expands to empty string; a set-but-empty variable counts as defined'
errors:
  - undefined variable name
  - unterminated reference, ${ with no closing brace
  - empty name, ${}
  - name outside name_charset
error_content: the variable name plus the term:config-key being expanded
name_independence:
  - a reference names a raw environment variable directly
  - the generated per-field env name and the env:"-" opt-out govern the env layer
    only and do not change which names a reference can read
implementation:
  - hand-written scanner, no regexp, per requirement:configbind-tinygo
migration_hazard: >
  an existing file whose string value contains $$ changes meaning, because $$
  now collapses to a single $
out_of_scope_v1:
  - '${NAME:-fallback} default form'
  - '$NAME without braces'
  - references to other config keys
applies_to:
  - requirement:config-env-interpolation
  - decision:env-interpolation-layer
  - flow:config-load
related:
  - decision:toml-shape-constraints
  - rule:toml-shape-validation
  - rule:source-precedence
```
