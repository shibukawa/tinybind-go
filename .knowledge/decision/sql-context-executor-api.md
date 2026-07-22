---
id: decision:sql-context-executor-api
type: decision
title: Optional Context SQL Executor API
---
Keep explicit executor parameters as the stable generated API and optionally add Context-resolved wrappers for web-framework transaction propagation.

```yaml
source:
  - requirement:sql-generated-api-layers
  - user design discussion 2026-07-22
default:
  context_api: disabled
  explicit_api: always generated
generation_options:
  SQLContextAPI:
    type: bool
    behavior: generate <Component>Context wrappers without executor parameters
  SQLExecutorResolver:
    type: optional SymbolPattern
    behavior: select a framework resolver; implies SQLContextAPI
resolver_contract:
  signature: func(context.Context) (SQLExecutor, error)
  SQLExecutor: ExecContext-compatible and QueryContext-compatible
  accepted_values: [sql.DB, sql.Conn, sql.Tx]
standard_runtime:
  setter: WithSQLExecutor(context.Context, SQLExecutor) context.Context
  resolver: SQLExecutorFromContext(context.Context) (SQLExecutor, error)
  key: private typed key
wrapper_behavior:
  sql.exec: resolve then delegate to explicit API
  sql.one<T>: resolve then delegate to explicit API
  sql.optional<T>: resolve then delegate to explicit API
  sql.many<T>: resolve inside iter.Seq2 execution; yield resolver error once
naming:
  explicit: <Component>
  context: <Component>Context
framework_flow:
  - transaction middleware derives a Context containing sql.Tx
  - handler calls <Component>Context with that Context
  - generated wrapper resolves sql.Tx and delegates to <Component>
constraints:
  - context mode never replaces or changes the explicit API signature
  - missing or incompatible executors produce errors, not panics
  - resolver configuration is fixed at generation time
  - generated wrappers do not begin, commit, or roll back transactions
  - transaction Context must not outlive its transaction callback
```
