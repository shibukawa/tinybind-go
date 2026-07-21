---
id: requirement:source-provenance-logging
type: requirement
title: Source Provenance Logging
---
Log helpers report effective Bind keys as {Key, Value, Place} with secret redaction.

```yaml
priority: must
intent: observability for debugging effective configuration without leaking secrets
helper: concept:provenance-log-helper
source_table: concept:config-overlay
payload: data:provenance-event
record_fields:
  - Key: config key path
  - Value: display value after redaction
  - Place: winning source layer
secret_policy: rule:secret-redaction
secret_tag: 'secret:"hide|mask|show"'
auto_mask_key_tokens:
  - password
  - secret
  - apikey
  - api_key
  - credential
  - access_key
  - accesskey
mask_form:
  base: '*****'
  length_jitter: random +/- 2
defaults:
  - no secret tag and non-sensitive key name -> show
  - no secret tag and sensitive key name -> mask
  - secret:"hide" -> omit from helper output
  - secret:"mask" -> asterisks with jitter
  - secret:"show" -> raw value
non_goals:
  - mandatory stdout logging; caller prints or logs the returned slice
  - changing in-memory config values
related:
  - concept:provenance-callback
  - concept:provenance-log-helper
  - data:provenance-event
  - decision:struct-field-tags
  - rule:secret-redaction
  - term:config-key
  - term:config-source
  - rule:source-precedence
  - flow:config-load
acceptance:
  - helper returns []{Key, Value, Place}
  - Place distinguishes default vs file vs env vs CLI
  - secret:"hide" omits the key from the slice
  - secret:"mask" returns asterisk Value with length jitter around 5
  - keys containing password/secret/apikey-like tokens auto-mask without tag
  - other keys default to show
  - explicit secret tag overrides auto policy
```
