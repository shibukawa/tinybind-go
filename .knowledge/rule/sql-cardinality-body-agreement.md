---
id: rule:sql-cardinality-body-agreement
type: rule
title: Declared SQL Output Matches the Statement Body
---
A declared sql.* output that provably disagrees with its statement body is a generation error, not a runtime surprise.

```yaml
priority: must
detection: rule:sql-top-level-keyword-scan
verb_agreement:
  sql.exec: top-level verb INSERT, UPDATE, or DELETE, with no top-level RETURNING
  sql.one: top-level SELECT, VALUES, WITH tail SELECT, or a mutation with a top-level RETURNING
  sql.optional: same as sql.one
  sql.many: same as sql.one
  sql.relation: SELECT, VALUES, or WITH tail SELECT only
  sql.predicate: no top-level statement verb; the body is a predicate fragment
errors:
  - sql.exec on a row-producing body; declare sql.one, sql.optional, or sql.many
  - sql.exec on a mutation with RETURNING, whose rows would be silently discarded
  - sql.one, sql.optional, or sql.many on a mutation without RETURNING, which yields no columns to scan
  - sql.relation on a mutation, which cannot appear in a subquery position
  - sql.predicate whose body opens with a statement verb
multiplicity:
  - a static LIMIT literal greater than one with sql.one or sql.optional is an error
  - a static LIMIT 1 with sql.many is an error; declare sql.optional
  - an unprovable row count keeps the requirement:sql-generated-api-layers runtime contract, because the row count is a property of the data, not of the SQL text
column_agreement:
  - the existing result shape check of requirement:sql-template-v1 stays
  - it moves onto rule:sql-top-level-keyword-scan so a subquery select list is never read as the outer one
  - column count and known column names must match the declared record fields in order
diagnostic:
  position: the statement declaration
  content: the declared output, the detected body form, and the expected outputs for that form
  format: requirement:analysis-diagnostics
acceptance:
  - 'select ... declared sql.exec generates a diagnostic'
  - 'insert ... returning id declared sql.exec generates a diagnostic'
  - 'delete from t where id = {id} declared sql.one generates a diagnostic'
  - 'delete from t where id = {id} returning id declared sql.one generates'
  - 'update ... declared sql.relation generates a diagnostic'
  - 'select ... limit 10 declared sql.one generates a diagnostic'
  - 'select ... limit 1 declared sql.many generates a diagnostic'
related:
  - requirement:sql-template-v1
  - requirement:sql-generated-api-layers
  - decision:template-declaration-kinds
  - requirement:sql-relation-composition
```
