---
id: api:configbind-provenance
type: api
title: LoadResult Provenance
---
LoadResult exposes the effective config as an ordered, redacted, dependency-filtered record slice.

```yaml
signature: 'func (r *LoadResult) Provenance() []ProvenanceEntry'
record: data:provenance-event
entry_shape: 'ProvenanceEntry{Key string, Value string, Place Place, Masked bool, ArrayKey string, Index int}'
coverage:
  - only keys present in the overlay; a field with no default and no source is absent
  - >
    an array of tables expands in place into one entry per element field, keyed
    key[index].field, per requirement:array-of-tables-provenance; the bare array
    key is not reported, because it carries no value of its own
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
caller_burden_to_avoid: >
  a caller that reaches past this API into concept:config-overlay to render what the
  API omits also leaves rule:secret-redaction behind, because the mask policy and the
  generated secret map are both package-internal
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
