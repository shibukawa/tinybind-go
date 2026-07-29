---
id: rule:secret-redaction
type: rule
title: Secret Redaction for Logs
---
Provenance log helpers redact field values by secret tag or sensitive key-name heuristics.

```yaml
explicit_tag: 'secret:"hide|mask|show"'
modes:
  hide:
    action: omit the entry from returned log records
  mask:
    action: replace Value with asterisks
    mask_form: '*****'
    fixed_width: true
    why_not_jittered: >
      an earlier design randomized the width by +/- 2, which would break the
      byte-identical output that requirement:deterministic-config-output-order
      requires; a fixed width leaks no length either, so the jitter bought
      nothing
  show:
    action: include raw Value
auto_policy_when_tag_absent:
  - if key path contains a sensitive token (case-insensitive substring): mask
  - else: show
sensitive_key_tokens:
  - password
  - secret
  - apikey
  - api_key
  - credential
  - access_key
  - accesskey
  - token
  - dsn
  - private_key
token_rationale:
  dsn: a connection string carries its password inline, as in postgres://user:pass@host/db
  private_key: a PEM body is a credential under a key name matching no other token
heuristic_limits:
  - a substring match also masks an innocent compound such as token_bucket_size or secret_manager_endpoint
  - over-masking is the safe direction; the explicit tag is the escape hatch
implementation_state:
  - the tag-free auto policy is live in api:configbind-provenance
  - the secret tag is read from generated field metadata and outranks the auto policy
  - data:provenance-event reports whether the value shown is the mask
placement:
  - a leaf field
  - a nested struct field, which applies the mode to every field of that subtree
  - rejected on an array-of-tables element or the array field itself, which have no one stable key
scope:
  - Bind provenance / log helpers only
  - does not change values written into config structs
  - SubCommand fields may use the same helper if logged, but no TOML/env layers
priority:
  - explicit secret tag wins over auto policy
array_of_tables:
  state: implemented; an element field's secret tag reaches the generated map
  array_field_mode: a mode on the array covers every field of every element
  keying: >
    an element key carries a runtime index, so the map keys element fields by their
    path under the array key and expansion applies it at every index
  detail: requirement:array-of-tables-provenance
related:
  - requirement:array-of-tables-provenance
  - decision:struct-field-tags
  - requirement:source-provenance-logging
  - data:provenance-event
  - concept:provenance-log-helper
  - flow:config-load
```
