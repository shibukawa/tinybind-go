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
  - name: Masked
    type: bool
    description: >
      Value is the redaction placeholder rather than the configured value, so a
      caller re-rendering these records never compares against the mask text
go_shape: '{Key, Value, Place string, Masked bool}'
notes:
  - hide mode drops the record instead of returning empty Value
  - mask mode sets Value to a fixed-width asterisk run per rule:secret-redaction, never length-jittered
  - show mode sets Value to string form of the effective value
  - rule:dependent-key-visibility can drop the record before redaction runs
  - duration Value is the time.Duration String() form
  - slice position follows rule:config-output-ordering
used_by:
  - concept:provenance-callback
  - concept:provenance-log-helper
  - requirement:source-provenance-logging
  - flow:config-load
  - rule:secret-redaction
```
