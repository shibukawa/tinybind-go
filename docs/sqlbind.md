# sqlbind User Guide

tinybind-go approaches SQL from two directions, and they answer different questions:

1. Typed SQL templates turn `.tb.sql` files into parameterized builders and `database/sql` execution functions
2. Row grouping turns flat JOIN rows into object trees through `sqlbind.ScanRows[T]`

Neither workflow reflects over application struct fields at runtime. Both generate type-specific code ahead of time.

## What SQL templates automate

- Discovering `.tb.sql` files
- Turning value expressions into dialect-appropriate placeholders and `Args`
- Generating `database/sql` APIs based on result cardinality
- Checking SELECT/RETURNING column count and names against the result type
- Scanning query results
- Enforcing optional and exactly-one row counts
- Streaming many-row results as an iterator
- Maintaining placeholder order across conditional SQL
- Expanding slices into placeholder lists
- Composing predicates and typed subqueries
- Rejecting UPDATE and DELETE without a safe WHERE clause

## What you provide

1. `.tb.sql` files directly inside a Go package directory
2. Parameters, result types, and statements in the SQL template
3. An executor such as `*sql.DB`, `*sql.Conn`, or `*sql.Tx`
4. Transaction boundaries, connection configuration, migrations, and schema management
5. A code-generation command

SQL templates do not run migrations or create database tables.

## Setup and generation

```go
package store

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

Place `users.tb.sql` in the same directory, then run:

```bash
go generate ./...
```

The generator combines `.tb.html` and `.tb.sql` output in `tinybind_templates_gen.go`. Discovery stops at the target directory itself; templates in a subdirectory are never picked up.

To use another naming convention, pass base-name globs with
`-html-template-pattern` and `-sql-template-pattern`, for example:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -html-template-pattern "*.page.html" -sql-template-pattern "*.query.sql"
```

The defaults remain `*.tb.html` and `*.tb.sql`.

## Choosing a dialect

A run that finds a SQL template must name its target database. There is no default:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -sql-dialect postgresql
```

`postgresql`, `mysql`, and `sqlite` are the accepted values, and omitting the flag is a generation error rather than a quiet PostgreSQL default. The reason is that the wrong placeholder token produces SQL the target engine simply rejects, while nothing in the templates hints at the mistake. A package holding only HTML templates needs no dialect.

Placeholders follow the selection: `$1`, `$2`, and so on for PostgreSQL, `?` for MySQL and SQLite. SQLite reads several placeholder spellings, and `?` is the positional one, which is what matches how arguments are bound. Generated runtime APIs accept no dialect or placeholder argument, so switching engines changes the emitted SQL text and nothing about the signatures you call. The dialect is fixed when the code is generated, not chosen when it runs.

The placeholder token is the only thing the dialect changes. Everything else you write reaches the generated SQL verbatim: tinybind will not rewrite `||` into `CONCAT`, translate `ON CONFLICT` into `ON DUPLICATE KEY UPDATE`, or work around MySQL's missing `RETURNING`. A translation layer of that kind looks correct and fails subtly — `||` is string concatenation in PostgreSQL and SQLite but logical OR in MySQL, so rewriting it can invert a predicate — and it would make the SQL you read in the template different from the SQL that runs. Write for the engine you selected. One generated package therefore serves one engine; run the generator twice to serve two.

That last point is worth weighing before you reach for SQLite in tests against a PostgreSQL production database. The two share `RETURNING` and `ON CONFLICT`, so plain CRUD often does port, but nothing checks that it did, and the generated package you exercise is not the one you ship. Selecting the dialect per generated directory is what makes running both deliberate.

## Minimal query

`users.tb.sql`:

```text
package store

type User {
  id: int
  name: string
  active: bool
}

export statement GetUser(id: int): sql.one<User> {
SELECT id, name, active
FROM users
WHERE id = {id}
}
```

The main application-facing signatures are:

```go
type User struct {
	Id     int
	Name   string
	Active bool
}

func BuildGetUser(id int) (sqlbind.Statement, error)
func GetUser(ctx context.Context, db sqlbind.Querier, id int) (User, error)
```

```go
user, err := GetUser(ctx, db, 42)
if err != nil {
	if errors.Is(err, sql.ErrNoRows) {
		// not found
	}
	return err
}
fmt.Println(user.Name)
```

## Values are always parameters

Template expressions such as `{id}` and `{name}` are never concatenated into SQL text:

```text
export statement RenameUser(id: int, name: string): sql.exec {
UPDATE users
SET name = {name}
WHERE id = {id}
}
```

```go
statement, err := BuildRenameUser(42, "Ada")
// statement.SQL  == "... SET name = $1 WHERE id = $2 ..."
// statement.Args == []any{"Ada", 42}
```

The guarantee is absolute, and it costs something. Handwritten `$1` or `?` placeholders are generation errors, and an ordinary value parameter can never stand in for a structural element — a table name, a column name, an operator, a sort direction.

## Computing a value once in Go

Declare an `external` function when a parameter needs work done in Go before it
reaches the statement, and bind its result with `{val}` when more than one
position uses it:

```text
external NormalizeName(name: string): string

export statement FindUser(name: string): sql.many<UserRow> {
{val key = NormalizeName(name)}
SELECT id, name FROM users
WHERE name = {key} OR alias = {key}
}
```

```go
func NormalizeName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
```

The binding becomes one Go local in the generated builder, so the function runs
once no matter how many placeholders read it:

```go
key := NormalizeName(name)
b.WriteString("... WHERE name = ")
b.Arg(key)
b.WriteString(" OR alias = ")
b.Arg(key)
```

Without the binding, `{NormalizeName(name)}` written twice is two calls — correct,
and worth avoiding once the function does real work.

A binding has no closing tag. It scopes whatever follows it, up to the end of the
enclosing block, which for a `{if}` branch is that branch:

```text
SELECT id, name FROM users WHERE
{if exact}
  {val key = NormalizeName(name)}
  name = {key}
{else}
  name LIKE {pattern}
{/if}
```

Three rules keep a binding from being a call you did not mean to make. Binding
the same name twice in one block is a redeclaration; the bindings of one `{val}`
are independent, so one that reads another must be split into two; and a binding
nothing reads is an error, because its call would run every time the statement is
built and the result would go nowhere.

## Declaring result cardinality

| Output | Contract | High-level result |
| --- | --- | --- |
| `sql.exec` | No row result | `sql.Result` |
| `sql.one<T>` | Exactly one row | `T`; zero rows returns `sql.ErrNoRows`, multiple rows return an error |
| `sql.optional<T>` | Zero or one row | `*T`; zero rows returns `nil, nil`, multiple rows return an error |
| `sql.many<T>` | Zero or more rows | `iter.Seq2[T, error]` |
| `sql.predicate` | Private reusable condition | No standalone API |
| `sql.relation<T>` | Private typed subquery | No standalone API |

### exec

```text
export statement DeleteUser(id: int): sql.exec {
DELETE FROM users WHERE id = {id}
}
```

```go
result, err := DeleteUser(ctx, db, 42)
if err != nil {
	return err
}
affected, err := result.RowsAffected()
```

### optional

```text
export statement FindUserByEmail(email: string): sql.optional<User> {
SELECT id, name, active
FROM users
WHERE email = {email}
}
```

```go
user, err := FindUserByEmail(ctx, db, "ada@example.com")
if err != nil {
	return err
}
if user == nil {
	// not found
}
```

### many

```text
export statement ListActiveUsers(active: bool): sql.many<User> {
SELECT id, name, active
FROM users
WHERE active = {active}
ORDER BY id
}
```

```go
for user, err := range ListActiveUsers(ctx, db, true) {
	if err != nil {
		return err
	}
	fmt.Println(user.Name)
}
```

No slice accumulates behind the iterator: rows are scanned and yielded one at a time. Breaking out of the range closes the underlying `sql.Rows`, and query, scan, and iteration errors are yielded once through the error value.

```go
for user, err := range ListActiveUsers(ctx, db, true) {
	if err != nil {
		return err
	}
	consume(user)
	break
}
```

## Result types and SELECT columns

The order of result fields must match the SELECT or RETURNING column order, and column names or aliases must correspond to field names. Generation checks both, so a SELECT list that drifts away from its result type fails the build rather than the query.

```text
type UserSummary {
  id: int
  displayName: string
}

export statement ListUsers(): sql.many<UserSummary> {
SELECT id, display_name AS displayName
FROM users
ORDER BY id
}
```

That check only holds if the shape is knowable statically. Runtime conditions therefore cannot add or remove SELECT/RETURNING columns; keep the result shape identical across every branch.

## Types

| Template type | Go API type |
| --- | --- |
| `string` / `decimal` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `float` | `float64` |
| `bytes` | `[]byte` |
| `datetime` / `date` / `time` | `time.Time` |
| `url` | `url.URL` |
| `T[]` | `[]T` |
| `T?` | `*T` |

The table stops at the Go type; the driver has to agree as well. Your SQL driver must be able to scan returned values into these types, so choose types that match both the schema and the driver, and use optional types wherever NULL is possible.

Two entries need more than the driver's agreement. A `url` column is carried as text in both directions: a `url.URL` parameter binds as its string form, and a returned column is parsed back through a runtime adapter, because `database/sql` can neither bind nor scan a struct. An optional `url` leaves a nil pointer for NULL; a required one reports an error, exactly as a required `string` does.

Separately, `datetime`, `date`, and `time` require the driver to hand back a `time.Time`; text and bytes do not scan into one. With MySQL that means `parseTime=true` in the DSN. With SQLite it depends on your driver and on the column's declared type, since SQLite stores no date type of its own. Either way it is driver configuration, not something the dialect selection can set for you.

## Conditional SQL

```text
export statement SearchUsers(
  name: string,
  activeOnly: bool
): sql.many<User> {
SELECT id, name, active
FROM users
WHERE name = {name}
{if activeOnly}
  AND active = {true}
{/if}
ORDER BY id
}
```

When the condition is false, the block is omitted. Only included values consume placeholders, so numbering and `Args` stay aligned no matter which branches survive.

```text
{if condition}
  ...
{else}
  ...
{/if}
```

The condition must be `bool`. Conditional SELECT or RETURNING columns are forbidden.

## Expanding slices for IN

```text
export statement FindUsers(ids: int[]): sql.many<User> {
SELECT id, name, active
FROM users
WHERE id IN ({ids})
ORDER BY id
}
```

```go
statement, err := BuildFindUsers([]int{10, 20, 30})
// ... WHERE id IN ($1, $2, $3)
// Args: []any{10, 20, 30}
```

An empty slice has no valid rendering as a value list, so the builder returns an error instead of emitting `IN ()`. Handle the empty case in the caller, or use a template condition to pick a different SQL structure.

## Reusing predicates

Private `sql.predicate` statements define reusable conditions:

```text
statement MinimumID(id: int): sql.predicate {
id >= {id}
}

export statement FindRecentUsers(minimum: int): sql.many<User> {
SELECT id, name, active
FROM users
WHERE {MinimumID(minimum)}
ORDER BY id
}
```

Predicates cannot be exported and receive neither `BuildMinimumID` nor an execution API. Call them only from other statements.

## Typed subqueries

Use a private `sql.relation<T>` in `FROM subquery` or `JOIN subquery`:

```text
type ActiveUser {
  id: int
  name: string
}

statement ActiveUsers(minimumID: int): sql.relation<ActiveUser> {
SELECT id, name
FROM users
WHERE id >= {minimumID} AND active = TRUE
}

export statement ListActiveUsers(
  minimumID: int,
  name: string
): sql.many<ActiveUser> {
SELECT active_users.id, active_users.name
FROM subquery ActiveUsers(minimumID) AS active_users
WHERE active_users.name = {name}
ORDER BY active_users.id
}
```

Composition does not fragment the parameter list: subquery and outer arguments share one placeholder sequence, ordered as they appear in the final SQL. The alias is explicit and lower snake case. Recursive relations are forbidden.

## UPDATE and DELETE safety

UPDATE and DELETE require a WHERE clause:

```text
export statement RenameUser(id: int, name: string): sql.exec {
UPDATE users SET name = {name} WHERE id = {id}
}
```

Whether a clause can come out empty is a property of the template, not of runtime data, so the whole check runs at generation time and no guard is emitted into generated code. A conditional WHERE fails to generate, because one call path would delete every row:

```text
export statement UnsafeDelete(id: int, enabled: bool): sql.exec {
DELETE FROM users
{if enabled}WHERE id = {id}{/if}
}
```

An `if` with an `else` where both branches emit a predicate does generate, because no path leaves the clause empty:

```text
export statement SafeDelete(id: int, name: string, byID: bool): sql.exec {
DELETE FROM users WHERE {if byID}id = {id}{else}name = {name}{/if}
}
```

The same proof covers a dynamic `SET` list: an UPDATE whose assignments are all conditional is a generation error.

The keyword must belong to the statement itself. A WHERE inside a subquery, a CTE body, a string literal, or a comment does not satisfy the requirement, so this is rejected:

```text
export statement StillUnsafe(): sql.exec {
DELETE FROM users USING (SELECT id FROM staged WHERE staged.flag) s
}
```

The check applies to every cardinality, not only `sql.exec`. A `DELETE ... RETURNING` declared as `sql.one<T>` is proven the same way. There is currently no opt-in for an intentional full-table UPDATE or DELETE.

A `sql.predicate` satisfies the requirement only when that predicate is itself non-empty on every path.

## export and package-private statements

`export` decides whether a statement joins the package's public Go API, not whether it is usable at all. A statement without `export` gets the same functions under unexported names:

```
statement findUser(id: int): sql.one<User> {SELECT id, name FROM users WHERE id = {id}}
```

```go
// generated: usable from anywhere in this package, invisible outside it
func findUser(ctx context.Context, db sqlbind.Querier, id int) (User, error)
func buildFindUser(id int) (sqlbind.Statement, error)
```

The generated function is named exactly as the statement is declared, so the name's own case is what decides Go visibility, and it has to agree with `export`:

| Declaration | Generated | |
|---|---|---|
| `export statement FindUser(...)` | `func FindUser(...)` | public API |
| `statement findUser(...)` | `func findUser(...)` | package-private |
| `export statement findUser(...)` | — | error: `export` cannot publish an unexported name |
| `statement FindUser(...)` | — | error: the name would be public without `export` |

`sql.predicate` and `sql.relation` are the exception. They are embedded into another statement's builder rather than executed, so they generate no function of their own name and their case is unconstrained.

## Using the low-level builder

Every exported statement receives a `Build<Name>` function (a private one receives `build<Name>`):

```go
statement, err := BuildGetUser(42)
if err != nil {
	return err
}

log.Printf("sql=%s args=%v", statement.SQL, statement.Args)
rows, err := db.QueryContext(ctx, statement.SQL, statement.Args...)
```

This is useful for SQL tests, logging, and custom database abstractions. `Statement` is declared once in the runtime package `github.com/shibukawa/tinybind-go/sqlbind` rather than per generated package, so a value crosses package boundaries unchanged:

```go
package sqlbind

type Statement struct {
	SQL  string
	Args []any
}
```

## Transactions

Explicit-executor APIs accept interfaces implemented by `*sql.DB`, `*sql.Conn`, and `*sql.Tx`, which is why the same generated function works inside a transaction and outside one:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
	return err
}
defer tx.Rollback()

if _, err := RenameUser(ctx, tx, 42, "Ada"); err != nil {
	return err
}
if _, err := DeleteUser(ctx, tx, 99); err != nil {
	return err
}
return tx.Commit()
```

## Executors outside database/sql

A generated row-returning statement does not require `database/sql` behind its executor. The generated body obtains its cursor through `sqlbind.Query`, which prefers the optional `RowsQuerier` interface — the same pattern by which `io.Copy` prefers `io.ReaderFrom` — and falls back to `Querier.QueryContext` for `*sql.DB`, `*sql.Conn`, and `*sql.Tx`:

```go
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
	Columns() ([]string, error)
}

type RowsQuerier interface {
	QueryRows(ctx context.Context, query string, args ...any) (Rows, error)
}
```

`Querier` itself must keep returning the concrete `*sql.Rows`, because that is what the standard handles return and Go interface satisfaction is exact; a backend that cannot construct a `*sql.Rows` embeds `sqlbind.UnimplementedQuerier` to satisfy `Querier` and implements `QueryRows` for the real path. A pgxpool adapter is about a screenful:

```go
type PGXExecutor struct {
	sqlbind.UnimplementedQuerier
	Pool *pgxpool.Pool
}

func (e PGXExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	tag, err := e.Pool.Exec(ctx, query, args...)
	return pgxResult{tag}, err
}

func (e PGXExecutor) QueryRows(ctx context.Context, query string, args ...any) (sqlbind.Rows, error) {
	rows, err := e.Pool.Query(ctx, query, args...)
	return pgxRows{rows}, err // wraps Close to return error, Columns over FieldDescriptions
}
```

Such an adapter satisfies `SQLExecutor`, so it also works with `WithSQLExecutor` and the Context APIs. `ForEach`, `ScanRows[T]`, and `RegisterScanRows` take `sqlbind.Rows`, so grouped JOIN scanning works over a custom cursor too. Generated code emitted before this seam existed calls `QueryContext` directly and reports a clear error against an embedded `UnimplementedQuerier`; regenerate it once to route through `sqlbind.Query`.

## Resolving an executor from Context

Threading an executor through every call stops working once framework middleware owns the transaction. For that case, enable Context APIs during generation:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -sql-context-api
```

```go
ctx := sqlbind.WithSQLExecutor(r.Context(), tx)

user, err := GetUserContext(ctx, 42)
for user, err := range ListActiveUsersContext(ctx, true) {
	// ...
}
```

Without an executor, these functions return `sqlbind.ErrNoSQLExecutor`. `WithSQLExecutor` accepts `*sql.DB`, `*sql.Conn`, `*sql.Tx`, or another `sqlbind.SQLExecutor` implementation.

The ordinary explicit-executor APIs remain available, so both styles can coexist.

### Read-only executors

When the Context carries a connection to a read replica, or a transaction begun with `sql.TxOptions{ReadOnly: true}`, add `sqlbind.AsReadOnly()`.

```go
ctx := sqlbind.WithSQLExecutor(r.Context(), replicaDB, sqlbind.AsReadOnly())

user, err := GetUserContext(ctx, 42)         // a SELECT, so it runs
res, err := DeleteUserContext(ctx, 42)       // sqlbind.ErrReadOnlyExecutor
```

Write statements are identified at generation time and return `sqlbind.ErrReadOnlyExecutor` before executing. The error names the rejected statement. No SQL is built and no round trip happens, so the failure is identical on a developer machine or in a test that never touches a replica.

A statement counts as read-only only when it opens with `SELECT`, `VALUES`, or `TABLE`, or is a `WITH` whose CTE bodies do not write and whose tail reads, and when it carries no top-level row-locking clause such as `FOR UPDATE`. A `DELETE ... RETURNING` declared as `sql.one<T>`, and a `SELECT ... FOR UPDATE`, are both writes. Anything the analysis cannot resolve is a write, so a misclassification can only cost a replica connection rather than send a write to a read-only executor.

Writes that are invisible in the SQL text, such as a `SELECT` calling a function that writes, are not detected; the database remains the final guard. The check is disabled when a custom resolver (the `SQLExecutorResolver` generator option) is configured, because that contract cannot carry the access mode.

## Context-only public API

A framework that publishes the declared statement names as its only executable
API can generate the Context-resolved form under those names:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -sql-context-only-api
```

```go
func FindUser(ctx context.Context, id int) (User, error)
```

In this mode:

- no exported function accepts `*sql.DB`, `*sql.Tx`, `sqlbind.Querier`, or `sqlbind.Execer`;
- the executor-taking function becomes unexported;
- `BuildName` stays exported and unchanged;
- no `NameContext` wrapper is generated, so that name stays free;
- the same public function is used inside and outside a transaction, because the
  executor comes from the Context.

`-sql-context-only-api` implies `-sql-context-api`. Set
`Options.SQLExecutorResolver` to resolve the executor through a framework
function instead of `sqlbind.SQLExecutorFromContext`.

## Generated SQL template signatures

In the signatures below, `p ...P` represents the mapped template parameters.

### Every exported statement

```go
func BuildName(p ...P) (sqlbind.Statement, error)
```

### `sql.exec`

```go
func Name(ctx context.Context, db sqlbind.Execer, p ...P) (sql.Result, error)
```

### `sql.one<T>`

```go
func Name(ctx context.Context, db sqlbind.Querier, p ...P) (T, error)
```

### `sql.optional<T>`

```go
func Name(ctx context.Context, db sqlbind.Querier, p ...P) (*T, error)
```

### `sql.many<T>`

```go
func Name(ctx context.Context, db sqlbind.Querier, p ...P) iter.Seq2[T, error]
```

### With `-sql-context-api`

```go
func NameContext(ctx context.Context, p ...P) (sql.Result, error) // exec
func NameContext(ctx context.Context, p ...P) (T, error)          // one
func NameContext(ctx context.Context, p ...P) (*T, error)         // optional
func NameContext(ctx context.Context, p ...P) iter.Seq2[T, error] // many
```

### With `-sql-context-only-api`

```go
func Name(ctx context.Context, p ...P) (sql.Result, error) // exec
func Name(ctx context.Context, p ...P) (T, error)          // one
func Name(ctx context.Context, p ...P) (*T, error)         // optional
func Name(ctx context.Context, p ...P) iter.Seq2[T, error] // many
```

### Private `sql.predicate` and `sql.relation<T>`

No application-facing builder or execution function is generated. They are used only from another statement.

## Common template errors

- Writing `$1` or `?` manually
- Mismatching SELECT column count and result field count
- Mismatching SELECT column names/aliases and result fields
- Changing SELECT/RETURNING columns with a runtime condition
- Omitting WHERE from UPDATE or DELETE
- Passing an empty slice to an expanded value list
- Receiving zero or multiple rows for `sql.one`
- Receiving multiple rows for `sql.optional`
- Ignoring the error value while ranging over `sql.many`

## Grouping JOIN rows with `ScanRows[T]`

A JOIN returns the parent row again for every child, and no cardinality declaration can undo that flattening. `ScanRows[T]` rebuilds the tree afterwards, and it works on any existing query — SQL templates are not involved.

```go
type Organization struct {
	ID    int    `db:"organization_id" groupkey:""`
	Name  string `db:"organization_name"`
	Users []User
}

type User struct {
	ID   int    `db:"user_id" groupkey:""`
	Name string `db:"user_name"`
}
```

Put a concrete call in the analyzed package:

```go
func LoadOrganizations(ctx context.Context, db *sql.DB) ([]Organization, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  o.id   AS organization_id,
  o.name AS organization_name,
  u.id   AS user_id,
  u.name AS user_name
FROM organizations o
LEFT JOIN users u ON u.organization_id = o.id
ORDER BY o.id, u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return sqlbind.ScanRows[Organization](rows)
}
```

Each grouped struct level must have exactly one scalar `groupkey:""` field. Those keys drive every merge decision:

- Rows with the same root key merge into one root object
- Rows with the same child key merge into one child object
- A NULL child key from an outer join means that child is absent
- A NULL root key is an error
- A scalar field without a `db` tag uses the snake-case form of its Go field name

### Multiple levels

```go
type Organization struct {
	ID    int    `db:"org_id" groupkey:""`
	Name  string `db:"org_name"`
	Users []User
}

type User struct {
	ID    int    `db:"user_id" groupkey:""`
	Name  string `db:"user_name"`
	Roles []Role
}

type Role struct {
	ID   int    `db:"role_id" groupkey:""`
	Name string `db:"role_name"`
}
```

Return a unique column alias corresponding to every scalar field in the JOIN SELECT.

## `ScanRows` constraints

- It targets host Go with `database/sql` and is excluded from TinyGo builds
- Every grouped struct requires exactly one `groupkey`
- Column aliases must match `db` tags
- It consumes all result rows to construct the tree, so account for memory use with very large results

The last constraint is what usually decides between the two workflows. Reach for a SQL template's `sql.one`, `sql.optional`, or `sql.many` for ordinary queries, where rows can stream past one at a time. Reach for `ScanRows` when a JOIN keeps repeating the same parent and the parent has to come back whole.
