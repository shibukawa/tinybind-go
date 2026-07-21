---
id: data:provenance-event
type: data
title: Provenance Event
---
One log-oriented record for a resolved Bind config key after redaction policy is applied.

```yaml
fields:
  - name: Key
    type: string
    ref: term:config-key
    description: dotted or hierarchical key path
  - name: Value
    type: string
    description: display value after secret policy; omitted entirely when hide
  - name: Place
    type: string
    ref: term:config-source
    description: winning source layer (default, file_toml, env, cli)
go_shape: '{Key, Value, Place string}'
helper_return: '[]struct{Key, Value, Place string} or named type alias'
notes:
  - hide mode drops the record instead of returning empty Value
  - mask mode sets Value to asterisks with length jitter
  - show mode sets Value to string form of the effective value
used_by:
  - concept:provenance-callback
  - concept:provenance-log-helper
  - requirement:source-provenance-logging
  - flow:config-load
  - rule:secret-redaction
```
