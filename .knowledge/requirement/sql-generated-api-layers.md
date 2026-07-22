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
executor_interfaces:
  sql.exec: ExecContext-compatible; accepts sql.DB, sql.Conn, and sql.Tx
  row_outputs: QueryContext-compatible; accepts sql.DB, sql.Conn, and sql.Tx
execution:
  sql.exec: ExecContext; return affected-row-capable result
  sql.one<T>: QueryContext; reject zero or multiple rows
  sql.optional<T>: QueryContext; accept zero or one and reject multiple rows
  sql.many<T>: QueryContext; lazily scan as iter.Seq2<T, error>; close rows on completion or early stop
query_row_rule: QueryRowContext is insufficient for multiple-row detection; use only when at-most-one is statically proven and the contract remains enforced
benefits:
  - low-level deterministic tests without a database
  - SQL logging, middleware, and custom execution
  - one generated public convenience API for normal database/sql use
  - optional web-framework transaction propagation through Context without removing explicit dependency injection
```
