---
id: data:sql-statement
type: data
title: Generated SQL Statement
---
Transport-neutral low-level result of a generated SQL component before database execution.

```yaml
source: concept:typed-template-language
go_type: sqlbind.Statement from decision:generated-runtime-in-module; never redeclared per generated package
go_shape:
  SQL: string
  Args: '[]any'
properties:
  - SQL contains only generator-owned bind placeholders
  - Args follow placeholder emission order
  - no database handle, rows, or dialect selection
construction_errors:
  - an empty expanded value list
  - other genuinely data-dependent structural validation failures
not_construction_errors:
  - empty mutation WHERE and empty SET are generation errors under rule:sql-static-mutation-safety
```
