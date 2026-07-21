---
id: concept:provenance-log-helper
type: concept
title: Provenance Log Helper
---
Helper returns a slice of {Key, Value, Place} records for safe logging of effective Bind config.

```yaml
api_sketch: 'func Provenance() []struct{Key, Value, Place string}'
# exact exported name TBD; may be method on load result
return_type: data:provenance-event slice
behavior:
  - one entry per effective Bind key from concept:config-overlay after layered load
  - apply rule:secret-redaction before returning
  - hide entries are absent from the slice
  - Place is the winning term:config-source from data:overlay-entry
  - Value is show or mask form only
non_goals:
  - mandatory printing; caller logs the slice
  - mutating stored config values
related:
  - requirement:source-provenance-logging
  - concept:provenance-callback
  - data:provenance-event
  - rule:secret-redaction
  - decision:struct-field-tags
  - flow:config-load
```
