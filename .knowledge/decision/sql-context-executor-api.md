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
  SQLContextOnlyAPI:
    type: bool
    behavior: publish only the context-resolved surface under the source-declared name
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
context_only_mode:
  motivation: a framework publishes the source-declared name as its only executable API
  requires: standard runtime resolver or configured SQLExecutorResolver
  naming:
    public: <Component> taking context.Context plus typed component parameters
    internal: _tinybindExec<Component>, unexported and non-conflicting
    builder: Build<Component> stays exported and unchanged
  rules:
    - no exported function accepts sql.DB, sql.Tx, sqlbind.Execer, or sqlbind.Querier
    - no <Component>Context wrapper is generated; that name stays free
    - the same public function is used inside and outside a transaction
    - transaction selection stays in the Context, not in the signature
    - mode is fixed at generation time and applies to the whole package
  acceptance:
    - FindUser generates as func FindUser(ctx context.Context, id int) (User, error)
    - a call inside a transaction Context reaches sql.Tx through the resolver
    - the explicit and wrapper surfaces remain available when the mode is off
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
