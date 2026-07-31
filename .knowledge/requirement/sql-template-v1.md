---
id: requirement:sql-template-v1
type: requirement
title: SQL Template V1
---
Generate parameterized SQL with typed result contracts and safe structured dynamic clauses.

```yaml
source: concept:typed-template-language
parser: decision:template-parser-delegation
ast:
  type_ids: [sql:statement, sql:clause, sql:parameter]
  embedded_nodes: [template:expression, template:if, template:for]
  contexts: [sql:value, sql:predicate, sql:assignment, sql:order-item]
outputs:
  sql.exec: no row result; expose affected count when supported
  sql.one<T>: exactly one row; reject zero or multiple
  sql.optional<T>: zero or one row; reject multiple
  sql.many<T>: zero or more rows
  sql.predicate: reusable predicate list
  sql.relation<T>: private typed subquery relation from requirement:sql-relation-composition
declaration: lowercase statement keyword with PascalCase name from decision:template-declaration-kinds
naming: rule:template-name-casing
values: ordinary inserted expressions follow rule:sql-placeholder-emission
statement: data:sql-statement
generated_api: requirement:sql-generated-api-layers
dialect:
  initial: decision:postgresql-first-template-sql
  selection: decision:sql-dialect-generation-time
structured_lists:
  where: AND children by default; explicit and/or groups; omit when empty for SELECT
  joins: conditional; cannot vary result shape
  set: manage commas; require a statically provable non-empty item set per rule:sql-static-mutation-safety
  order_by: static branches or enums; manage commas and empty clause
  insert: paired field-value assignments; no bulk insert
  returning: static item shape
relation_composition: requirement:sql-relation-composition
result_validation:
  - validate column count, names or aliases, types, optionality, and join nullability where provable
  - keep declared public cardinality when analysis is inconclusive
  - enforce unproven one and optional cardinality at runtime
  - reject a provable disagreement between the declared output and the body per rule:sql-cardinality-body-agreement
static_analysis: rule:sql-top-level-keyword-scan
mutation_safety:
  - UPDATE and DELETE need a provably non-empty WHERE, decided at generation time by rule:sql-static-mutation-safety
  - full-table mutation needs a future explicit opt-in
forbidden:
  - value interpolation into SQL text
  - a generated runtime check for a condition decidable at generation time
  - manually authored bind-placeholder tokens in executable SQL text; only value expressions generate them
  - arbitrary dynamic identifiers, operators, keywords, or sort directions
  - runtime-conditional select or returning columns
  - general loops in SQL clauses
```
