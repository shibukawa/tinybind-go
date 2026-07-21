---
id: rule:source-precedence
type: rule
title: Config Source Precedence
---
For each config key, the last present value among ordered sources wins.

```yaml
order_low_to_high:
  - default
  - file_toml
  - env
  - cli
semantics:
  - absent key in a layer does not clear prior value
  - present key always overrides prior layer
  - provenance records the winning term:config-source
applies_to:
  - flow:config-load
  - concept:layered-config-sources
  - requirement:layered-config-load
related:
  - concept:provenance-callback
  - data:provenance-event
```
