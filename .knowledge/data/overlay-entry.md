---
id: data:overlay-entry
type: data
title: Overlay Entry
---
One winning value in the config overlay before type-specific apply.

```yaml
fields:
  - name: key
    type: string
    ref: term:config-key
  - name: raw
    type: string_or_primitive_array
    description: source-normalized raw value before typed parse into struct field
  - name: place
    type: enum
    ref: term:config-source
    description: winning source layer
operations:
  - Set(key, raw, place): overwrite prior entry for key
used_by:
  - concept:config-overlay
  - decision:configbind-runtime-architecture
  - flow:config-load
  - concept:provenance-log-helper
```
