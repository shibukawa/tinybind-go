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
    empty_for: an array-of-tables entry, whose values live in tables instead
  - name: is_tables
    type: bool
    description: the entry holds [[key]] elements rather than a value
  - name: tables
    type: overlay_array
    description: one nested overlay per element, in file order
    consumers: apply, scaffold, and provenance expansion per requirement:array-of-tables-provenance
  - name: place
    type: enum
    ref: term:config-source
    description: winning source layer
operations:
  - Set(key, raw, place): overwrite prior entry for key
ordering:
  - entries carry no order of their own
  - output order comes from generated definitions via rule:config-output-ordering
used_by:
  - requirement:array-of-tables-provenance
  - concept:config-overlay
  - decision:configbind-runtime-architecture
  - flow:config-load
  - concept:provenance-log-helper
```
