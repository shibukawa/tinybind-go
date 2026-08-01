---
id: requirement:deterministic-config-output-order
type: requirement
title: Deterministic Config Output Order
---
Effective-config output lists bindings in registration order and fields in declaration order, through ordered APIs only.

```yaml
priority: must
intent: make logged config readable and diffable instead of alphabetically shuffled
problems:
  - provenance and scaffold read as a flat lexicographic list unrelated to the structs
  - map-returning APIs let Go map iteration order reach output
  - registered definitions live in a map, so nothing preserves Bind call order
policy: rule:config-output-ordering
api_changes:
  - provenance returns an ordered slice via api:configbind-provenance
  - concept:config-overlay gains ordered iteration; callers stop ranging a map
  - generated definitions retain declaration order for keys, defaults, and scaffold fields
  - registration keeps Bind call order beside the type-and-prefix lookup map
non_goals:
  - changing rule:source-precedence or which value wins
  - reordering .env scaffold away from environment variable name sort
  - stable ordering across different Bind call sites in the application
related:
  - rule:config-output-ordering
  - requirement:source-provenance-logging
  - requirement:scaffold-generation
  - concept:provenance-log-helper
  - concept:config-overlay
  - data:overlay-entry
  - api:configbind-provenance
  - api:configbind-bind
  - decision:configbind-runtime-architecture
  - flow:config-load
  - system:configbind
acceptance:
  - two Bind calls emit their keys in call order, not prefix alphabetical order
  - a struct declaring port before host logs port before host
  - a nested struct expands in place at its declaring field position
  - repeated loads of identical input produce identical output order
  - no exported configbind API returns a bare map for output
  - an overlay key owned by no definition sorts after all known keys
  - TOML scaffold lists fields of one table in declaration order
  - .env scaffold stays sorted by environment variable name
```
