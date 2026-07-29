---
id: rule:sql-dialect-syntax-rejection
type: rule
title: Dialect Difference Rejection Over Translation
---
Absorb a dialect difference only when it is purely lexical; reject a semantic one at generation time instead of rewriting it.

```yaml
priority: must
source: user design discussion 2026-07-30
applies_to: every SQL template compiled against a data:sql-dialect value
boundary:
  absorb:
    test: the two forms carry identical meaning, so substituting one for the other cannot change a result
    member: bind placeholder style, per rule:sql-placeholder-emission
    why_safe: '$1 and ? are both positional bind parameters; nothing but the token differs'
  reject:
    test: the forms differ in meaning, availability, or NULL and collation behavior
    action: generation error naming the construct and the selected dialect
    never: silently rewrite author SQL into the target engine's equivalent
rationale:
  - the contract of concept:typed-template-language is that the author writes real SQL and only the holes are typed; a translation layer makes it a query builder with unbounded surface
  - a translation that looks correct and is subtly wrong reproduces the failure shape of requirement:sql-dialect-selection in a place that is harder to find
  - 'concatenation is the clearest trap: || is string concatenation in PostgreSQL and SQLite but logical OR in MySQL unless PIPES_AS_CONCAT is set, so translating it can invert a predicate'
  - deterministic golden output survives only while emitted SQL text is the author text
rejection_candidates:
  mysql:
    - RETURNING, which MySQL does not implement
    - ON CONFLICT, whose MySQL form is ON DUPLICATE KEY UPDATE
    - dollar-quoted string literals
  sqlite:
    - dollar-quoted string literals
    - note: SQLite implements RETURNING and ON CONFLICT and accepts every common identifier quote, so it has the shortest list
  postgresql:
    - backtick-quoted identifiers
  note: candidates, not a committed set; each costs a keyword check and must not reject valid SQL
detection:
  reuse: rule:sql-top-level-keyword-scan tokenizer, which already skips literals, comments, and quoted identifiers
  depth: keyword level only
  no_full_parse: a false positive blocking valid SQL costs more than a missed construct the database itself will reject
out_of_scope:
  - operator, function, and clause translation
  - LIMIT and OFFSET syntax differences
  - collation and case-sensitivity differences
  - type storage differences, which surface in the driver rather than the SQL text
driver_layer:
  observation: generated code also depends on the database through rows.Scan target types, which no dialect option controls
  examples:
    - a datetime field needs parseTime=true in a MySQL DSN or the driver returns bytes and the scan fails
    - a decimal field scans as string on both engines
    - a url field cannot cross database/sql at all; requirement:sql-url-column-boundary converts it, and that conversion is dialect-independent
  treatment: document the DSN requirement; do not encode driver configuration in a dialect value
acceptance:
  - every non-postgresql target emits the author SQL byte for byte apart from placeholders
  - no generated file contains a rewritten operator, function, or clause
  - a rejected construct names both the construct and the dialect
  - a construct inside a string literal or comment is never rejected
```
