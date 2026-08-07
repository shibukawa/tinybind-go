---
id: requirement:sql-driver-agnostic-rows
type: requirement
title: Driver-Agnostic SQL Rows Interface
---
A non-database/sql backend such as pgxpool serves generated row-returning statements through the sqlbind.Rows cursor interface, while *sql.DB, *sql.Conn, and *sql.Tx keep satisfying the unchanged executor interfaces.

```yaml
source:
  - downstream Popcorn Wave request 2026-08-07
problem:
  - sql.Result is an interface, so Execer already accepts custom backends
  - "*sql.Rows is a struct with unexported fields; only database/sql can construct it"
  - Querier returning *sql.Rows made row-returning statements unsatisfiable outside database/sql
rejected_downstream_proposal:
  change: Querier.QueryContext returns a Rows interface
  reason: Go interface satisfaction is exact with no return covariance, so *sql.DB would stop satisfying Querier, breaking every std call site, WithSQLExecutor, and the compile-time assertions in sqlbind/context_test.go
design:
  pattern: optional-interface upgrade, as io.Copy prefers io.ReaderFrom
  Rows:
    methods: [Next, Scan, Err, Close, Columns]
    satisfied_by: "*sql.Rows unchanged"
    columns_rationale: downstream listed four methods, but ForEach reads column names for Row maps
  Querier: unchanged; QueryContext still returns *sql.Rows so std handles satisfy it as before
  RowsQuerier: "QueryRows(ctx, query, args...) (Rows, error); the driver-agnostic surface a custom backend adds"
  Query: "sqlbind.Query(ctx, db Querier, query, args...) (Rows, error); prefers RowsQuerier, falls back to QueryContext; the single dispatch point"
  UnimplementedQuerier: embeddable stub whose QueryContext errors; lets a backend that cannot construct *sql.Rows satisfy Querier
  generated_code: one/optional/many bodies call sqlbind.Query instead of db.QueryContext; still one body per statement, no per-backend branching
signature_changes:
  ForEach: takes Rows
  ScanRows: takes Rows
  RegisterScanRows: registers func(Rows) ([]T, error)
  generated_tree_scanner: scan<T>Rows(rows sqlbind.Rows)
compatibility:
  std_handles: satisfy Querier and SQLExecutor unchanged; no call-site edits
  pre_seam_generated_code: still compiles and runs on std backends; against a RowsQuerier-only executor its direct QueryContext hits the stub's explanatory error, so gaining pgx support requires one regeneration
  committed_fixtures: none reference *sql.Rows or RegisterScanRows; no regeneration in this module
  source_break: only code annotating results as *sql.Rows in ForEach/ScanRows callbacks or registering scanners
adapter_notes:
  pgx_close: pgx.Rows.Close returns nothing; the adapter wraps it to return error
  pgx_columns: adapter derives Columns from FieldDescriptions
  pgx_exec: adapter converts pgconn.CommandTag into sql.Result for Execer
non_goals:
  - a second executor parameter type in generated signatures; the parameter stays sqlbind.Querier
  - shipping a pgx adapter in this module; downstream owns it
acceptance:
  - an executor embedding UnimplementedQuerier and implementing QueryRows executes generated one/optional/many statements and closes its cursor
  - sqlbind.Query dispatches to QueryRows when present
  - std compile-time assertions and the full test suite pass without regenerating committed fixtures
```
