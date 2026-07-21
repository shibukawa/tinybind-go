---
id: concept:config-overlay
type: concept
title: Config Overlay
---
Key-wise merge table holding the winning raw value and source Place for each Bind config key.

```yaml
key: term:config-key flat path under Bind prefix
entry: data:overlay-entry
merge: rule:source-precedence later Set wins
sources_into_overlay:
  - default from generated defaults
  - file_toml from reusable TOML parser
  - env from env map filtered by known keys
  - cli from generated flag parse via generic CLI map machinery
not_in_overlay:
  - SubCommand-only fields and positionals
outputs:
  - input to generated struct apply
  - input to concept:provenance-log-helper
related:
  - decision:configbind-runtime-architecture
  - concept:layered-config-sources
  - concept:reusable-source-parsers
  - data:overlay-entry
  - data:provenance-event
  - flow:config-load
```
