---
id: requirement:sql-read-only-executor
type: requirement
title: Read-Only SQL Executor Rejection
---
A Context executor tagged read-only rejects write statements before the statement reaches the database.

```yaml
source:
  - requirement:sql-generated-api-layers
  - user design discussion 2026-07-29
motivation:
  - an Aurora-style deployment holds separate reader and writer connections
  - a reader instance rejects a write only when actually connected to a replica, after a network round trip
  - the framework stores one executor type for both roles, so the tag cannot live in the Go type
  - sql.Tx erases read-only intent; sql.TxOptions.ReadOnly is invisible to the type system
runtime:
  setter: WithSQLExecutor(ctx, executor, opts ...ExecutorOption)
  option: AsReadOnly() marks the stored executor read-only
  storage: executor and flag are stored as one context entry under the existing private key
  read_resolver: SQLExecutorFromContext, signature and behavior unchanged
  write_resolver: WriteExecutorFromContext(ctx, statementName) returns ErrReadOnlyExecutor when the flag is set
  error: ErrReadOnlyExecutor names the rejected statement
generation:
  access_mode: rule:sql-statement-access-mode
  emission: the Context wrapper of a write statement calls the write resolver; a read statement keeps the existing resolver
  cost: one resolver-name switch in the emitter; no generated guard, no runtime SQL inspection
scope:
  covered:
    - <Component>Context wrappers
    - context_only_mode public functions of decision:sql-context-executor-api
  uncovered:
    - the explicit executor API, whose caller chooses the handle and owns that choice
    - a configured SQLExecutorResolver, whose contract cannot carry the flag
custom_resolver: read-only checking is disabled while SQLExecutorResolver is configured; extending that contract is deferred
non_goals:
  - automatic reader and writer routing
  - a security boundary; a read statement calling a writing function stays undetectable
  - detecting a write that the database itself would accept
acceptance:
  - WithSQLExecutor(ctx, db) keeps every existing call site compiling and behaving unchanged
  - a sql.exec Context wrapper under AsReadOnly returns ErrReadOnlyExecutor without building SQL
  - the returned error names the statement
  - a sql.many write statement yields ErrReadOnlyExecutor once and stops
  - a read statement under AsReadOnly executes normally
  - no generated file contains a read-only guard or its error string
```
