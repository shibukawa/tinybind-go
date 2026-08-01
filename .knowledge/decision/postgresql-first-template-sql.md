---
id: decision:postgresql-first-template-sql
type: decision
title: PostgreSQL-First Template SQL
---
Use PostgreSQL as the first SQL semantic target while keeping the initial AST and feature subset portable.

```yaml
source:
  - concept:typed-template-language
  - user design discussion 2026-07-20
  - user design discussion 2026-07-29
scope: first semantic target, not an implicit default; requirement:sql-dialect-selection makes every run name its data:sql-dialect value
postgresql:
  placeholder: dollar_numbered from rule:sql-placeholder-emission
  coverage: the portable_v1 subset below
rationale:
  - strict and rich database types align with static template types
  - schema and result validation can be stronger
  - PostgreSQL supports the planned returning and structured mutation workflows
portable_v1:
  - SELECT, INSERT, UPDATE, and DELETE
  - joins, where, order by, limit, and offset
  - basic returning
  - bound values and expanded IN placeholders
mysql:
  status: selectable for placeholder emission only
  placeholder: question
  pending: rule:sql-dialect-syntax-rejection checks for RETURNING, ON CONFLICT, dollar-quoted literals, and identifier quoting
  not_planned: rewriting any of those into their MySQL equivalents
sqlite:
  status: selectable for placeholder emission only
  placeholder: question, the positional form among the several spellings SQLite reads
  pending: the same rule:sql-dialect-syntax-rejection checks as mysql
  closest_to_postgresql: RETURNING and ON CONFLICT are shared, so portable CRUD often survives untranslated, but nothing verifies it and the tested package is not the shipped one
  earlier_prerequisites_dissolved:
    reason: the list below assumed generation would lower types and syntax per engine; rule:sql-dialect-syntax-rejection ruled that out, leaving one placeholder table entry
    was:
      - dynamic-affinity and STRICT-table schema handling
      - explicit date, time, datetime, decimal, and boolean storage mappings
      - RETURNING capability restrictions
    now:
      - storage and affinity are driver and schema concerns, documented rather than generated
      - SQLite has supported RETURNING since 3.35, so no restriction is needed
      - a datetime scan depends on the driver and the declared type, like the MySQL parseTime DSN parameter
future_postgresql:
  optional_lowering:
    - array parameters and ANY
    - native JSON and JSONB
    - richer returning and PostgreSQL-specific types
constraint: dialect-specific syntax requires capability validation and must not silently change portable semantics
```
