---
id: data:sql-dialect
type: data
title: SQL Dialect Selection Value
---
One named dialect value carries every generation-time database difference, so a target engine is chosen once instead of per facet.

```yaml
status: required
selected_by: requirement:sql-dialect-selection
motivation: placeholder style was a standalone option, so a MySQL target had no single name to select and no place to hang later syntax differences
identity:
  name: Dialect
  form: string constant validated at generation time
  invalid_value: generation error naming the unknown dialect
selectable:
  postgresql:
    placeholder: dollar_numbered
    semantic_target: decision:postgresql-first-template-sql
    coverage: full portable_v1 subset
  mysql:
    placeholder: question
    coverage: placeholder emission only
    caveat: rule:sql-dialect-syntax-rejection checks are deferred, so a template using PostgreSQL-only syntax still compiles and fails at the database
  sqlite:
    placeholder: question
    note: SQLite reads several placeholder spellings; question is the positional one, matching how arguments are bound
    coverage: placeholder emission only
    caveat: same deferred checks as mysql; SQLite shares RETURNING and ON CONFLICT with postgresql, so portable CRUD often survives untranslated, but nothing verifies that it did
emitted_attributes:
  scope: rule:sql-dialect-syntax-rejection permits exactly one, because only a lexical difference may be absorbed
  placeholder_style:
    values: rule:sql-placeholder-emission initial_styles
    consumer: sqlbind.NewBuilder style argument
    source: one dialect-to-style table that also defines the accepted set, so a dialect cannot be added without choosing a style
    unmapped: emits uncompilable code rather than falling back to another engine's style
validation_attributes:
  purpose: describe what the selected dialect rejects, not what generation rewrites; a later difference extends this value instead of adding a parallel option
  status: reserved; no check is implemented yet
  identifier_quote: double quote for postgresql; backtick for mysql; sqlite accepts double quote, backtick, and brackets
  returning: supported by postgresql and sqlite; absent from mysql
  upsert: ON CONFLICT for postgresql and sqlite; ON DUPLICATE KEY UPDATE for mysql
  literal_quote: dollar-quoted strings are PostgreSQL only
  storage: sqlite has no date type and dynamic column affinity, so a datetime scan depends on the driver and the declared type; a driver concern, not a dialect attribute
non_goals:
  - lowering author SQL into an engine-specific equivalent
  - driver configuration such as the MySQL parseTime DSN parameter, which is documentation rather than a dialect attribute
  - runtime dialect detection from a driver name or a database handle
  - a dialect argument in any generated function signature; decision:sql-dialect-generation-time future_escape_hatch keeps signatures fixed if runtime selection ever lands
related:
  - decision:sql-dialect-generation-time
  - rule:sql-dialect-syntax-rejection
  - rule:sql-placeholder-emission
  - data:generator-options
```
