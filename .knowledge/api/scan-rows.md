---
id: api:scan-rows
type: api
title: sqlbind.ScanRows
---
Map flat joined database/sql rows into grouped typed object trees through generated scanners.

```yaml
signature: "func ScanRows[T any](rows sqlbind.Rows) ([]T, error)"
input: column names and row values from any requirement:sql-driver-agnostic-rows source
output: roots in first-seen key order with nested slice children
dispatch: generated scanner registry for T
mapping:
  scalar_column: db tag; default snake_case field name
  grouping: rule:sql-group-key
errors:
  - nil rows
  - missing root or child group key column
  - NULL root key
  - unsupported SQL-to-Go conversion
  - missing generated scanner
runtime: no application field reflection
package: github.com/shibukawa/tinybind-go/sqlbind
related:
  - rule:sql-group-key
  - rule:usage-directed-generation
  - concept:code-generation
  - decision:reflection-free
```
