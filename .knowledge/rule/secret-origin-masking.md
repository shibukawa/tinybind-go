---
id: rule:secret-origin-masking
type: rule
title: Secret Origin Masking
---
A value a secret source supplied is masked in provenance because of where it came from, and the secrecy follows the value into any TOML string that expanded it.

```yaml
origin_secret_sources:
  - EnvSecretFiles entries
  - EnvSecretDirs entries
not_origin_secret:
  - EnvFiles entries
  - Environ, even for a name a secret source also set; the process value is the one shown and it was not read from a secret store
mode_resolution:
  order_high_to_low:
    - 'secret:"hide" tag: drop the entry'
    - origin secret: mask
    - 'secret:"mask" or secret:"show" tag'
    - key-token auto policy of rule:secret-redaction
  show_tag_loses_to_origin:
    why: >
      the tag is the author's statement about the field at generation time; the
      operator mounting the value from a secret store is a statement about this
      deployment, and over-masking is the safe direction rule:secret-redaction
      already chose
    escape: pass the file through EnvFiles instead of EnvSecretFiles
  empty_value: an empty secret-origin value renders empty and unmasked, matching displayValue today
taint_through_interpolation:
  rule: a file_toml string whose ${NAME} references resolved at least one secret-origin name is masked
  why: >
    dsn = "postgres://app:${DB_PASSWORD}@host/db" carries the secret in the
    expanded value; the dsn token catches this one, but webhook = "https://h/${HOOK_TOKEN}"
    under a key with no token would print it
  granularity: the whole string; a partial mask would need the expansion to remember spans
  primitive_arrays: per element; one tainted element masks the whole entry, since provenance renders the joined form
  does_not_taint: a ${NAME} that resolved from EnvFiles or Environ
carrier:
  - the composed environment records, per name, whether the winning supplier was a secret source
  - data:overlay-entry carries Secret to Provenance, so Provenance needs no name table and no second pass; a later Set clears it
  - struct apply never sees the flag; the struct holds the real value
provenance_output:
  Masked: true
  Value: the fixed-width mask of rule:secret-redaction
  Place: unchanged, PlaceEnvFile + path or file_toml
applies_to:
  - requirement:secret-env-sources
  - api:configbind-provenance
  - flow:config-load
related:
  - rule:secret-redaction
  - rule:env-interpolation-syntax
  - decision:env-interpolation-layer
  - data:provenance-event
  - concept:provenance-log-helper
```
