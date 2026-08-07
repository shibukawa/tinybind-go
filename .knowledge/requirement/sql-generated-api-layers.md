---
id: requirement:sql-generated-api-layers
type: requirement
title: Generated SQL API Layers
---
Generate a reusable statement builder and a database/sql execution wrapper for every exported executable SQL component.

```yaml
source: concept:typed-template-language
low_level:
  name: Build<Component>
  inputs: typed component parameters
  output: data:sql-statement plus error
  behavior: build SQL and Args without database access
  builder: sqlbind.Builder; the generated file declares no builder type or helper
high_level:
  name: <Component>
  inputs: context.Context, minimal executor interface, typed component parameters
  behavior: call low-level builder, execute, scan, and enforce declared result contract
context_adapter:
  decision: decision:sql-context-executor-api
  default: disabled
  name: <Component>Context
  inputs: context.Context, typed component parameters
  behavior: resolve executor from Context and delegate to <Component>
context_only_surface:
  decision: decision:sql-context-executor-api context_only_mode
  default: disabled
  effect: <Component> becomes the context-resolved public function and the executor-taking function becomes unexported
executor_interfaces:
  source: declared once in the module per decision:generated-runtime-in-module; never emitted into a generated file
  sql.exec: sqlbind.Execer, ExecContext-compatible; accepts sql.DB, sql.Conn, and sql.Tx
  row_outputs: sqlbind.Querier, QueryContext-compatible; accepts sql.DB, sql.Conn, and sql.Tx, and custom backends through the RowsQuerier upgrade of requirement:sql-driver-agnostic-rows
execution:
  sql.exec: ExecContext; return affected-row-capable result
  sql.one<T>: sqlbind.Query per requirement:sql-driver-agnostic-rows; reject zero or multiple rows
  sql.optional<T>: sqlbind.Query; accept zero or one and reject multiple rows
  sql.many<T>: sqlbind.Query; lazily scan as iter.Seq2<T, error>; close rows on completion or early stop
query_row_rule: QueryRowContext is insufficient for multiple-row detection; use only when at-most-one is statically proven and the contract remains enforced
benefits:
  - low-level deterministic tests without a database
  - SQL logging, middleware, and custom execution
  - one generated public convenience API for normal database/sql use
  - one canonical Statement and executor type shared by every generated package
  - optional web-framework transaction propagation through Context without removing explicit dependency injection
```
