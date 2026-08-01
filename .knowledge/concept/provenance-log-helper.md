---
id: concept:provenance-log-helper
type: concept
title: Provenance Log Helper
---
Helper returns an ordered slice of {Key, Value, Place} records for safe logging of effective Bind config.

```yaml
surface: api:configbind-provenance on the load result
return_type: data:provenance-event slice
behavior:
  - one entry per effective Bind key from concept:config-overlay after layered load
  - apply rule:secret-redaction before returning
  - apply rule:dependent-key-visibility before returning
  - hidden entries are absent from the slice, not blank
  - Place is the winning term:config-source from data:overlay-entry
  - Value is show or mask form only
  - order follows rule:config-output-ordering, not key sort
filters_in_order:
  - dependency visibility, so an empty parent removes its dependents
  - secret policy, so a surviving entry is still redacted or dropped
non_goals:
  - mandatory printing; caller logs the slice
  - mutating stored config values
  - returning a map or otherwise leaking map iteration order
related:
  - requirement:source-provenance-logging
  - requirement:deterministic-config-output-order
  - requirement:dependent-field-visibility
  - concept:provenance-callback
  - data:provenance-event
  - rule:secret-redaction
  - rule:dependent-key-visibility
  - rule:config-output-ordering
  - decision:struct-field-tags
  - flow:config-load
```
