# sqlbind 利用ガイド

tinybind-go の SQL 機能には2つの用途があります。

1. `.tb.sql` から parameterized SQL builder と `database/sql` 実行関数を作る型付き SQL template
2. 通常の SQL で取得した JOIN 行を、`sqlbind.ScanRows[T]` で親子構造へまとめる row grouping

どちらもアプリケーション構造体のフィールド走査に reflection を使わず、必要な処理を事前生成します。

## SQL template で自動化されること

- `.tb.sql` の自動発見
- 値式から `$1`, `$2`, ... placeholder と `Args` の生成
- statement の戻り件数に応じた `database/sql` API
- SELECT / RETURNING の列数・列名と結果型の検査
- query result の scan
- optional / exactly-one の行数検査
- many result の逐次 iterator
- 条件付き SQL で変化する placeholder 番号の管理
- slice の placeholder list 展開
- predicate と typed subquery の合成
- WHERE のない UPDATE / DELETE の拒否

## ユーザーが用意するもの

1. Go パッケージ直下の `.tb.sql` ファイル
2. SQL template 内の parameter、結果型、statement
3. `*sql.DB`、`*sql.Conn`、`*sql.Tx` などの executor
4. transaction 境界、接続設定、migration、schema 管理
5. コード生成の実行

SQL template は schema migration を実行せず、接続先の table を自動作成しません。

## 導入とコード生成

```go
package store

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

同じディレクトリに `users.tb.sql` を置きます。

```bash
go generate ./...
```

`.tb.html` と `.tb.sql` は `tinybind_templates_gen.go` にまとめられます。対象ディレクトリ直下だけが探索対象です。

既定の placeholder は PostgreSQL 形式の `$1`, `$2`, ... です。dialect や placeholder を実行時に選ぶ API はありません。

## 最小の query

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

アプリケーションから使う主なシグネチャ:

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

## 値は必ず parameter として渡される

テンプレートの `{id}` や `{name}` は SQL 文字列へ直接連結されません。

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

テンプレートに `$1` や `?` を手書きすると生成エラーになります。table 名、column 名、operator、sort direction のような SQL 構造を通常の値 parameter で動的に差し替えることもできません。

## 戻り件数の宣言

| 出力型 | 契約 | 高レベル API の結果 |
| --- | --- | --- |
| `sql.exec` | 行を返さない | `sql.Result` |
| `sql.one<T>` | 必ず1行 | `T`。0行は `sql.ErrNoRows`、複数行は error |
| `sql.optional<T>` | 0または1行 | `*T`。0行は `nil, nil`、複数行は error |
| `sql.many<T>` | 0行以上 | `iter.Seq2[T, error]` |
| `sql.predicate` | private な条件部品 | 単独の実行 API なし |
| `sql.relation<T>` | private な subquery | 単独の実行 API なし |

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
	// 見つからなかった
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

結果を先に slice へ貯めず、行ごとに scan して返します。途中で `break` しても underlying `sql.Rows` は close されます。query、scan、iteration の error は iterator の error 値として1回 yield されます。

```go
for user, err := range ListActiveUsers(ctx, db, true) {
	if err != nil {
		return err
	}
	consume(user)
	break
}
```

## 結果型と SELECT 列

結果型の field 順と SELECT / RETURNING の列順を対応させます。列名または alias も field 名と対応させます。

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

結果列を runtime の `if` で増減させることはできません。どの分岐でも同じ結果 shape にしてください。

## 型

| テンプレート型 | Go API の型 |
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

SQL driver が返す値からこれらの Go 型へ `database/sql.Rows.Scan` できることも必要です。NULL を受ける列では optional 型など、driver と schema に合う型を選びます。

## 条件付き SQL

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

条件が false なら block 全体が省略され、採用された値だけで placeholder と `Args` が連番になります。

```text
{if condition}
  ...
{else}
  ...
{/if}
```

condition は `bool` である必要があります。SELECT / RETURNING の列 shape を変える条件分岐は禁止されています。

## IN の slice 展開

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

空 slice は有効な value list を作れないため、builder が error を返します。呼び出し側で空を特別扱いするか、template の条件分岐で SQL 構造を決めてください。

## predicate の再利用

繰り返す条件を private `sql.predicate` にできます。

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

predicate は `export` できず、`BuildMinimumID` や DB 実行 API は作られません。公開 statement の中から呼びます。

## typed subquery

private `sql.relation<T>` を `FROM subquery` または `JOIN subquery` で合成できます。

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

subquery の引数と外側の引数は、最終 SQL で現れる順に1つの placeholder 列へ統合されます。alias は lower snake case で明示します。recursive relation は使えません。

## UPDATE / DELETE の安全性

UPDATE と DELETE には WHERE が必要です。

```text
export statement RenameUser(id: int, name: string): sql.exec {
UPDATE users SET name = {name} WHERE id = {id}
}
```

WHERE が template 内にまったくなければ生成時に失敗します。WHERE が条件 block にあり、実行時に空になる可能性がある場合は `Build...` が実行前に拒否します。

```text
export statement UnsafeDelete(id: int, enabled: bool): sql.exec {
DELETE FROM users
{if enabled}WHERE id = {id}{/if}
}
```

この builder を `enabled == false` で呼ぶと error になり、DB へは送信されません。意図的な全件 UPDATE / DELETE の opt-in は現在ありません。

## 低レベル builder を使う

すべての exported statement には `Build<Name>` が作られます。

```go
statement, err := BuildGetUser(42)
if err != nil {
	return err
}

log.Printf("sql=%s args=%v", statement.SQL, statement.Args)
rows, err := db.QueryContext(ctx, statement.SQL, statement.Args...)
```

これは SQL のテスト、ログ、独自 DB abstraction との接続に便利です。`Statement` は次の利用者向け shape です。

```go
type Statement struct {
	SQL  string
	Args []any
}
```

## transaction

明示 executor API は `*sql.DB`、`*sql.Conn`、`*sql.Tx` と互換です。

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

## Context から executor を解決する API

framework middleware が transaction を Context に保持する場合は、生成時に `-sql-context-api` を指定します。

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

Context に executor がなければ `sqlbind.ErrNoSQLExecutor` が返ります。`WithSQLExecutor` に渡せるのは `*sql.DB`、`*sql.Conn`、`*sql.Tx` など `sqlbind.SQLExecutor` を満たす値です。

executor を引数で明示する通常 API も残るため、用途に応じて併用できます。

## SQL template で作られる関数シグネチャ一覧

以下の `P...` は template parameter 群、`p...` は対応する Go 引数です。

### すべての exported statement

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

### `-sql-context-api` を有効にした場合

```go
func NameContext(ctx context.Context, p ...P) (sql.Result, error)     // exec
func NameContext(ctx context.Context, p ...P) (T, error)              // one
func NameContext(ctx context.Context, p ...P) (*T, error)             // optional
func NameContext(ctx context.Context, p ...P) iter.Seq2[T, error]     // many
```

### private `sql.predicate` / `sql.relation<T>`

アプリケーションから呼ぶ `Build...` や実行関数は作られません。別の statement 内でだけ利用します。

## template のよくあるエラー

- `$1` や `?` を手書きした
- SELECT 列数と結果型の field 数が違う
- SELECT 列名 / alias と結果 field が対応していない
- runtime 条件で SELECT / RETURNING の列を変えた
- UPDATE / DELETE に WHERE がない
- slice parameter に空 slice を渡した
- `sql.one` の query が0行または複数行を返した
- `sql.optional` の query が複数行を返した
- `sql.many` の range 内で error を確認していない

## `ScanRows[T]` で JOIN 結果を親子構造にする

SQL template とは別に、既存の query で得た flat な JOIN 行を構造体の tree にまとめられます。

```go
type Organization struct {
	ID    int    `db:"organization_id" groupkey:""`
	Name  string `db:"organization_name"`
	Users []User
}

type User struct {
	ID    int    `db:"user_id" groupkey:""`
	Name  string `db:"user_name"`
}
```

解析対象パッケージに具体的な呼び出しを置きます。

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

各階層に scalar の `groupkey:""` field をちょうど1つ用意します。

- 同じ root key の行は同じ root object にまとまる
- 同じ child key の行は同じ child object にまとまる
- outer join で child key が NULL なら、その child は追加されない
- root key が NULL なら error
- `db` タグを省略した scalar field は field 名から snake case の列名を使う

### 複数階層

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

JOIN の SELECT では、すべての scalar field に対応する一意な列 alias を返してください。

## `ScanRows` の制約

- `database/sql` を使う host Go 向けで、TinyGo build からは除外される
- 各 grouped struct に `groupkey` が1つ必要
- 列 alias と `db` タグが一致している必要がある
- 結果行をすべて走査して tree を構築するため、非常に大きい結果ではメモリ使用量を考慮する

単純な1行 / 複数行 query には SQL template の `sql.one` / `sql.optional` / `sql.many`、JOIN の重複行を階層化したい場合には `ScanRows`、という使い分けが基本です。
