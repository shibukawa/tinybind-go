---
id: concept:provenance-callback
type: concept
title: Provenance Callback
---
Optional user callback or log helper consumption of redacted provenance records after load.

```yaml
purpose:
  - log which source won
  - mark defaults
  - audit effective runtime config safely
preferred_surface: concept:provenance-log-helper returning data:provenance-event slice
event: data:provenance-event
redaction: rule:secret-redaction
invoked_by: flow:config-load
related:
  - requirement:source-provenance-logging
  - term:config-key
  - term:config-source
  - rule:source-precedence
  - decision:struct-field-tags
```
