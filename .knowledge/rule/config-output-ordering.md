---
id: rule:config-output-ordering
type: rule
title: Config Output Ordering
---
Config output follows declaration order, and no public API leaks Go map iteration order.

```yaml
ordering_authority:
  - generated definitions, whose known keys already sit in struct declaration order
  - never the overlay map, the definitions map, or any source parser map
provenance_order:
  outer: Bind registration order, i.e. the order of api:configbind-bind calls
  inner: struct field declaration order, nested structs expanded in place
  array_of_tables:
    elements: index ascending at the array field's declared position
    within_an_element: declaration order, from the generated scaffold Nested list
    detail: requirement:array-of-tables-provenance
    not_from: >
      concept:config-overlay Keys, whose sort is that surface's own determinism
      guarantee; reading element order from it reintroduces a lexicographic list
      inside an otherwise declaration-ordered output
  unknown_keys:
    definition: overlay keys owned by no registered definition, e.g. stray TOML keys
    position: after every known key, sorted lexicographically
toml_scaffold_order:
  outer: prefix then Go type name, lexicographic and unchanged
  inner: struct field declaration order, replacing the previous key sort
  reason: >
    requirement:scaffold-generation forbids depending on package init order, and
    generated Register calls run in init order, so only the inner order changes
env_scaffold_order:
  rule: global environment variable name sort, unchanged
  reason: env output has no table grouping to hang declaration order on
api_shape:
  - return an ordered slice, or an already ordered iter.Seq2, never a map
  - a map may remain the internal store only if an ordered key list accompanies it
  - Overlay keeps its map plus an ordered All iter.Seq2 surface
  - generated Definition maps are registration input, not output; KnownKeys is
    their order, and load walks defaults through it instead of ranging the map
determinism:
  - identical registrations and identical inputs produce byte-identical output
  - order never depends on Go map iteration or hash seed
related:
  - requirement:deterministic-config-output-order
  - requirement:array-of-tables-provenance
  - concept:provenance-log-helper
  - concept:config-overlay
  - data:overlay-entry
  - requirement:scaffold-generation
  - api:configbind-provenance
  - api:configbind-bind
  - flow:config-load
```
