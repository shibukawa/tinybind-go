---
id: concept:layered-config-sources
type: concept
title: Layered Config Sources
---
Ordered sources feed a shared overlay; later sources override earlier ones per key.

```yaml
sources:
  - default
  - file_toml
  - env
  - cli
merge_model: key-wise overlay Set; not whole-tree replace
intermediate: concept:config-overlay
parsers: concept:reusable-source-parsers
file_format: decision:toml-config-format
precedence: rule:source-precedence
architecture: decision:configbind-runtime-architecture
related:
  - requirement:layered-config-load
  - flow:config-load
  - term:config-source
  - concept:provenance-callback
  - concept:provenance-log-helper
```
