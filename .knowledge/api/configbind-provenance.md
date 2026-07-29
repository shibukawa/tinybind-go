---
id: api:configbind-provenance
type: api
title: LoadResult Provenance
---
LoadResult exposes the effective config as an ordered, redacted, dependency-filtered record slice.

```yaml
signature: 'func (r *LoadResult) Provenance() []ProvenanceEntry'
record: data:provenance-event
entry_shape: 'ProvenanceEntry{Key string, Value string, Place Place, Masked bool}'
coverage:
  - only keys present in the overlay; a field with no default and no source is absent
behavior:
  - one record per effective term:config-key from concept:config-overlay
  - Place is the winning term:config-source
  - order follows rule:config-output-ordering
  - rule:secret-redaction decides Value and Masked, and drops hide entries
  - rule:dependent-key-visibility drops entries under an empty parent
  - multi-value keys render as their joined raw form
  - the process-only config path key never appears
non_goals:
  - printing or logging; the caller formats the slice
  - mutating the overlay or the bound structs
  - exposing hidden or redacted raw values through another accessor
callers: concept:provenance-callback
related:
  - concept:provenance-log-helper
  - requirement:source-provenance-logging
  - requirement:deterministic-config-output-order
  - requirement:dependent-field-visibility
  - data:provenance-event
  - rule:config-output-ordering
  - rule:dependent-key-visibility
  - rule:secret-redaction
  - flow:config-load
  - system:configbind
```
