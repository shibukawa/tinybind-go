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
implementation_state:
  - the tag-free auto policy is live in api:configbind-provenance
  - the explicit secret tag is not read yet; every key follows the auto policy
scope:
  - Bind provenance / log helpers only
  - does not change values written into config structs
  - SubCommand fields may use the same helper if logged, but no TOML/env layers
priority:
  - explicit secret tag wins over auto policy
related:
  - decision:struct-field-tags
  - requirement:source-provenance-logging
  - data:provenance-event
  - concept:provenance-log-helper
  - flow:config-load
```
