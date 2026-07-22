# sqlbind User Guide

tinybind-go provides two SQL workflows:

1. Typed SQL templates that turn `.tb.sql` files into parameterized builders and `database/sql` execution functions
2. Row grouping that turns flat JOIN rows into object trees with `sqlbind.ScanRows[T]`

Both workflows generate type-specific code ahead of time rather than reflecting over application struct fields.

## What SQL templates automate

- Discovering `.tb.sql` files
- Turning value expressions into `$1`, `$2`, ... placeholders and `Args`
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

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

Place `users.tb.sql` in the same directory, then run:

```bash
go generate ./...
```

The generator combines `.tb.html` and `.tb.sql` output in `tinybind_templates_gen.go`. Only files directly inside the target directory are discovered.

The default placeholder style is PostgreSQL `$1`, `$2`, and so on. Generated runtime APIs do not accept a dialect or placeholder option.

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

func BuildGetUser(id int) (Statement, error)
func GetUser(ctx context.Context, db SQLQuerier, id int) (User, error)
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

Handwritten `$1` or `?` placeholders are generation errors. Ordinary value parameters also cannot dynamically replace structural SQL elements such as table names, column names, operators, or sort directions.

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

Rows are scanned and yielded without first accumulating a slice. Breaking out of the range closes the underlying `sql.Rows`. Query, scan, and iteration errors are yielded once through the error value.

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

The order of result fields must match the SELECT or RETURNING column order. Column names or aliases must also correspond to field names.

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

Runtime conditions cannot add or remove SELECT/RETURNING columns. Keep the result shape identical across branches.

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

The selected SQL driver must also be able to scan returned values into these Go types. Use optional types where NULL is possible and choose types that match the schema and driver.

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

When the condition is false, the block is omitted. Only included values consume placeholders, so numbering and `Args` remain aligned.

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

An empty slice cannot form a valid value list, so the builder returns an error. Handle the empty case in the caller or use a template condition to choose the SQL structure.

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

Predicates cannot be exported and do not receive `BuildMinimumID` or execution APIs. Call them only from other statements.

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

Subquery and outer arguments share one placeholder sequence in final SQL order. The alias is explicit and lower snake case. Recursive relations are forbidden.

## UPDATE and DELETE safety

UPDATE and DELETE require a WHERE clause:

```text
export statement RenameUser(id: int, name: string): sql.exec {
UPDATE users SET name = {name} WHERE id = {id}
}
```

Generation fails when the template contains no WHERE at all. If WHERE is conditional and may disappear at runtime, `Build...` rejects the unsafe statement before it reaches the database.

```text
export statement UnsafeDelete(id: int, enabled: bool): sql.exec {
DELETE FROM users
{if enabled}WHERE id = {id}{/if}
}
```

Calling this builder with `enabled == false` returns an error. There is currently no opt-in for intentional full-table UPDATE or DELETE.

## Using the low-level builder

Every exported statement receives a `Build<Name>` function:

```go
statement, err := BuildGetUser(42)
if err != nil {
	return err
}

log.Printf("sql=%s args=%v", statement.SQL, statement.Args)
rows, err := db.QueryContext(ctx, statement.SQL, statement.Args...)
```

This is useful for SQL tests, logging, and custom database abstractions. The application-facing shape is:

```go
type Statement struct {
	SQL  string
	Args []any
}
```

## Transactions

Explicit-executor APIs accept interfaces implemented by `*sql.DB`, `*sql.Conn`, and `*sql.Tx`:

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

## Resolving an executor from Context

When framework middleware stores a transaction in Context, enable Context APIs during generation:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir . -sql-context-api
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

## Generated SQL template signatures

In the signatures below, `p ...P` represents the mapped template parameters.

### Every exported statement

```go
func BuildName(p ...P) (Statement, error)
```

### `sql.exec`

```go
func Name(ctx context.Context, db SQLExecer, p ...P) (sql.Result, error)
```

### `sql.one<T>`

```go
func Name(ctx context.Context, db SQLQuerier, p ...P) (T, error)
```

### `sql.optional<T>`

```go
func Name(ctx context.Context, db SQLQuerier, p ...P) (*T, error)
```

### `sql.many<T>`

```go
func Name(ctx context.Context, db SQLQuerier, p ...P) iter.Seq2[T, error]
```

### With `-sql-context-api`

```go
func NameContext(ctx context.Context, p ...P) (sql.Result, error) // exec
func NameContext(ctx context.Context, p ...P) (T, error)          // one
func NameContext(ctx context.Context, p ...P) (*T, error)         // optional
func NameContext(ctx context.Context, p ...P) iter.Seq2[T, error] // many
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

Independently of SQL templates, existing queries can map flat JOIN rows into object trees.

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

Each grouped struct level must have exactly one scalar `groupkey:""` field.

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

As a rule of thumb, use SQL template `sql.one`, `sql.optional`, or `sql.many` for ordinary queries, and use `ScanRows` when repeated JOIN rows must be grouped into a hierarchy.
