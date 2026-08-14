# sqlbind 利用ガイド

tinybind-go は SQL に2方向から取り組みます。答える問いがそれぞれ違います。

1. `.tb.sql` から parameterized SQL builder と `database/sql` 実行関数を作る型付き SQL template
2. 通常の SQL で取得した flat な JOIN 行を、`sqlbind.ScanRows[T]` で親子構造へまとめる row grouping

どちらも実行時にアプリケーション構造体のフィールドを走査しません。必要な処理は型ごとに事前生成されます。

## SQL template で自動化されること

- `.tb.sql` の自動発見
- 値式から dialect に応じた placeholder と `Args` の生成
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

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

同じディレクトリに `users.tb.sql` を置きます。

```bash
go generate ./...
```

`.tb.html` と `.tb.sql` は `tinybind_templates_gen.go` にまとめられます。探索は対象ディレクトリ自身で止まるため、サブディレクトリに置いたテンプレートは拾われません。

別の命名規則を使う場合は、ベース名に対する glob を
`-html-template-pattern` と `-sql-template-pattern` で指定します。

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -html-template-pattern "*.page.html" -sql-template-pattern "*.query.sql"
```

既定値は引き続き `*.tb.html` と `*.tb.sql` です。

## dialect の選択

SQL template を含む生成では、対象データベースの指定が必須です。既定値はありません。

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -sql-dialect postgresql
```

指定できるのは `postgresql`、`mysql`、`sqlite` で、省略すると PostgreSQL と黙って解釈するのではなく生成エラーになります。placeholder の形式を間違えると対象エンジンがその SQL を単純に拒否しますが、template のどこを読んでもその間違いは現れないからです。HTML template しか持たない package に dialect は不要です。

placeholder は選択に従い、PostgreSQL なら `$1`, `$2`, ...、MySQL と SQLite なら `?` になります。SQLite は複数の placeholder 表記を読みますが、`?` が位置指定の形式で、引数の bind のされ方に一致します。生成される実行時 API に dialect や placeholder の引数はないので、エンジンを切り替えても変わるのは出力される SQL テキストだけで、呼び出す側の signature は変わりません。dialect が決まるのはコード生成の時点で、実行時ではありません。

dialect が変えるのは placeholder だけです。それ以外に書いたものは逐語的に生成 SQL へ届きます。`||` を `CONCAT` に書き換えたり、`ON CONFLICT` を `ON DUPLICATE KEY UPDATE` に翻訳したり、MySQL に無い `RETURNING` を回避したりはしません。この種の翻訳層は正しく見えて静かに壊れます — `||` は PostgreSQL と SQLite では文字列連結ですが MySQL では論理和なので、書き換えると述語が反転しえます — し、template で読む SQL と実際に走る SQL が別物になります。選んだエンジンに向けて書いてください。したがって生成された1つの package が対応するのは1つのエンジンです。2つ必要なら generator を2回走らせます。

この点は、本番が PostgreSQL でテストだけ SQLite にしようとする前に検討する価値があります。両者は `RETURNING` と `ON CONFLICT` を共有するので単純な CRUD なら移植できることも多いのですが、移植できたことを検証する仕組みはありませんし、テストで動かす生成 package は出荷する package とは別物です。dialect を生成ディレクトリ単位で選ぶ形にしてあるのは、両方走らせることを意識的な選択にするためです。

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

## 値は必ず parameter として渡される

テンプレートの `{id}` や `{name}` が SQL 文字列へ連結されることはありません。

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

この保証は例外なく効き、その分の代償もあります。`$1` や `?` を手書きすれば生成エラーになり、通常の値 parameter は SQL の構造要素——table 名、column 名、operator、sort direction——の代わりには決してなれません。

## Go 側で値を 1 回だけ加工する

statement に渡す前に Go で加工が必要なら `external` を宣言し、その結果を複数の位置で
使うなら `{val}` で束縛します。

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

束縛は生成される builder の中で Go のローカル変数 1 つになります。いくつの
placeholder が読んでも、関数が走るのは 1 回です。

```go
key := NormalizeName(name)
b.WriteString("... WHERE name = ")
b.Arg(key)
b.WriteString(" OR alias = ")
b.Arg(key)
```

束縛しなければ `{NormalizeName(name)}` を 2 回書けば 2 回呼ばれます。それ自体は正しい
動作ですが、関数が実際の仕事をするようになったら避けたいところです。

束縛に閉じタグはありません。及ぶのは後ろに続くもので、囲んでいるブロックの終わりま
でです。`{if}` の分岐の中で束縛すれば、その分岐が範囲になります。

```text
SELECT id, name FROM users WHERE
{if exact}
  {val key = NormalizeName(name)}
  name = {key}
{else}
  name LIKE {pattern}
{/if}
```

意図しない呼び出しにならないよう、規則が 3 つあります。同じブロックで同じ名前を 2 回
束縛するのは再宣言です。1 つの `{val}` の束縛どうしは独立なので、片方がもう片方を
読むなら 2 つに分けてください。そしてどこからも読まれない束縛はエラーです——statement
を組み立てるたびに呼ばれて、結果はどこにも行きません。

`external` は末尾に `error` を返すこともできます。失敗すると statement の組み立てが
そこで止まり、呼び出した側に届きます。

```go
func NormalizeName(name string) (string, error) { ... }
```

```go
key, err := NormalizeName(name)
if err != nil {
	return err
}
```

テンプレート側の宣言はどちらでも変わりません。この関数は `{val}` の値そのものにしか
書けません。ほかの場所では失敗の行き先がないので、生成時にそう告げられます。

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

iterator の裏に slice は溜まりません。行は1件ずつ scan されて渡されます。途中で `break` しても underlying `sql.Rows` は close され、query、scan、iteration の error は error 値として1回 yield されます。

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

結果型の field 順は SELECT / RETURNING の列順と対応し、列名または alias も field 名と対応していなければなりません。生成時にどちらも検査されるため、結果型から離れていった SELECT 列はクエリではなくビルドを落とします。

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

ただしこの検査が成り立つのは、shape が静的に分かる場合だけです。だからこそ結果列を runtime の `if` で増減させることはできません。どの分岐でも同じ結果 shape に保ってください。

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

この表が示すのは Go の型までで、driver も同意している必要があります。使用する SQL driver が返す値をこれらの型へ `database/sql.Rows.Scan` できることが前提なので、schema と driver の両方に合う型を選び、NULL がありうる列では optional 型を使ってください。

2つだけ、driver の同意以上のものが要る型があります。`url` 列は両方向ともテキストとして運ばれます。`url.URL` の parameter は文字列形式で bind され、返ってきた列は runtime の adapter で parse し直されます。`database/sql` は struct を bind することも scan することもできないからです。optional な `url` は NULL のとき nil pointer になり、必須の `url` は必須の `string` と同じくエラーになります。

もう1つは `datetime` / `date` / `time` で、driver が `time.Time` を返してくれる必要があります。テキストやバイト列は `time.Time` へ scan できません。MySQL なら DSN の `parseTime=true` がそれにあたります。SQLite は日付型を持たないので、driver と列の宣言型次第です。いずれにせよ driver の設定であって、dialect の選択が代わりに面倒を見られる範囲ではありません。

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

条件が false なら block 全体が省略されます。placeholder を消費するのは採用された値だけなので、どの分岐が残っても番号と `Args` はずれません。

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

空 slice を value list として書き下す方法はありません。そのため builder は `IN ()` を出力せず error を返します。呼び出し側で空を特別扱いするか、template の条件分岐で別の SQL 構造を選んでください。

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

predicate は `export` できず、`BuildMinimumID` も DB 実行 API も作られません。呼べるのは別の statement の中からだけです。

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

合成しても parameter が分断されることはありません。subquery の引数と外側の引数は、最終 SQL に現れる順で1つの placeholder 列へ統合されます。alias は lower snake case で明示します。recursive relation は使えません。

## UPDATE / DELETE の安全性

UPDATE と DELETE には WHERE が必要です。

```text
export statement RenameUser(id: int, name: string): sql.exec {
UPDATE users SET name = {name} WHERE id = {id}
}
```

clause が空になりうるかどうかは template の性質であって実行時データの性質ではないため、検査はすべて生成時に行われ、生成コードにガードは入りません。条件 block の中だけにある WHERE は、片方の経路が全件削除になるため生成に失敗します。

```text
export statement UnsafeDelete(id: int, enabled: bool): sql.exec {
DELETE FROM users
{if enabled}WHERE id = {id}{/if}
}
```

`else` があって両分岐とも述語を出す場合は、空になる経路がないので生成できます。

```text
export statement SafeDelete(id: int, name: string, byID: bool): sql.exec {
DELETE FROM users WHERE {if byID}id = {id}{else}name = {name}{/if}
}
```

同じ証明が動的な `SET` list にも適用されます。代入がすべて条件付きの UPDATE は生成エラーです。

keyword はその statement 自身のものでなければなりません。subquery、CTE 本体、文字列リテラル、コメントの中にある WHERE は条件を満たさないため、次は拒否されます。

```text
export statement StillUnsafe(): sql.exec {
DELETE FROM users USING (SELECT id FROM staged WHERE staged.flag) s
}
```

検査は `sql.exec` だけでなくすべての cardinality に適用されます。`sql.one<T>` として宣言した `DELETE ... RETURNING` も同じように証明されます。意図的な全件 UPDATE / DELETE の opt-in は現在ありません。

`sql.predicate` が条件を満たすのは、その predicate 自身がすべての経路で空にならない場合だけです。

## export とパッケージ内限定の statement

`export` が決めるのは「パッケージの公開 Go API に載るかどうか」であって、「使えるかどうか」ではありません。`export` の無い statement にも同じ関数が非公開名で生成されます。

```
statement findUser(id: int): sql.one<User> {SELECT id, name FROM users WHERE id = {id}}
```

```go
// 生成される：同じパッケージ内からは呼べる、外からは見えない
func findUser(ctx context.Context, db sqlbind.Querier, id int) (User, error)
func buildFindUser(id int) (sqlbind.Statement, error)
```

生成される関数名は**宣言した名前そのまま**です。したがって名前の大文字小文字が Go の可視性を決め、`export` と一致している必要があります。

| 宣言 | 生成 | |
|---|---|---|
| `export statement FindUser(...)` | `func FindUser(...)` | 公開 API |
| `statement findUser(...)` | `func findUser(...)` | パッケージ内限定 |
| `export statement findUser(...)` | — | エラー：非公開の名前は公開できない |
| `statement FindUser(...)` | — | エラー：`export` 無しでも公開されてしまう |

例外は `sql.predicate` と `sql.relation` です。これらは実行されず他の statement の builder に埋め込まれるだけなので、自分の名前の関数を持たず、大文字小文字の制約もありません。

## 低レベル builder を使う

exported statement には `Build<Name>`、private statement には `build<Name>` が作られます。

```go
statement, err := BuildGetUser(42)
if err != nil {
	return err
}

log.Printf("sql=%s args=%v", statement.SQL, statement.Args)
rows, err := db.QueryContext(ctx, statement.SQL, statement.Args...)
```

これは SQL のテスト、ログ、独自 DB abstraction との接続に便利です。`Statement` が宣言されるのは生成パッケージごとではなく runtime package `github.com/shibukawa/tinybind-go/sqlbind` に一度だけなので、値はパッケージ境界をそのまま越えられます。

```go
package sqlbind

type Statement struct {
	SQL  string
	Args []any
}
```

## transaction

明示 executor API が受け取るのは `*sql.DB`、`*sql.Conn`、`*sql.Tx` が満たす interface です。だからこそ、生成された同じ関数が transaction の内でも外でも動きます。

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

## database/sql 以外の executor

生成された行返し statement は、executor の裏側が `database/sql` であることを要求しません。生成コードは cursor を `sqlbind.Query` 経由で取得します。`sqlbind.Query` は optional interface の `RowsQuerier` を優先し(`io.Copy` が `io.ReaderFrom` を優先するのと同じパターンです)、なければ `Querier.QueryContext` にフォールバックします。`*sql.DB`、`*sql.Conn`、`*sql.Tx` はフォールバック側で今までどおり動きます。

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

`Querier` 自体は具象型 `*sql.Rows` を返し続けます。標準の handle がそう返す以上、Go の interface 充足は厳密一致なので、戻り値を interface に変えると `*sql.DB` が `Querier` を満たさなくなるためです。`*sql.Rows` を構築できない backend は `sqlbind.UnimplementedQuerier` を埋め込んで `Querier` を満たし、本来の経路として `QueryRows` を実装します。pgxpool の adapter は 1 画面程度で書けます:

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
	return pgxRows{rows}, err // Close を error 返しに wrap し、Columns を FieldDescriptions から作る
}
```

この adapter は `SQLExecutor` を満たすので、`WithSQLExecutor` と Context API でもそのまま使えます。`ForEach`、`ScanRows[T]`、`RegisterScanRows` は `sqlbind.Rows` を受け取るため、JOIN 結果の親子 scan も custom cursor の上で動きます。この seam 以前に生成されたコードは `QueryContext` を直接呼ぶため、`UnimplementedQuerier` 埋め込みの executor に対しては明確なエラーを返します。一度再生成すれば `sqlbind.Query` 経由になります。

## Context から executor を解決する API

transaction を framework middleware が持つようになると、executor を毎回引数で引き回す書き方は成り立たなくなります。その場合は、生成時に Context API を有効にします。

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

Context に executor がなければ `sqlbind.ErrNoSQLExecutor` が返ります。`WithSQLExecutor` に渡せるのは `*sql.DB`、`*sql.Conn`、`*sql.Tx` など `sqlbind.SQLExecutor` を満たす値です。

executor を引数で明示する通常 API も残るため、用途に応じて併用できます。

### 読み取り専用 executor

read replica への接続や `sql.TxOptions{ReadOnly: true}` で開始した transaction を Context に入れるときは、`sqlbind.AsReadOnly()` を付けます。

```go
ctx := sqlbind.WithSQLExecutor(r.Context(), replicaDB, sqlbind.AsReadOnly())

user, err := GetUserContext(ctx, 42)         // SELECT なので実行される
res, err := DeleteUserContext(ctx, 42)       // sqlbind.ErrReadOnlyExecutor
```

書き込み statement は生成時に判定され、実行前に `sqlbind.ErrReadOnlyExecutor` を返します。エラーには弾かれた statement 名が入ります。SQL の組み立てもデータベースへの往復も発生しないため、read replica に繋がっていない開発環境やテストでも同じように失敗します。

読み取りと判定されるのは、先頭が `SELECT` / `VALUES` / `TABLE`、または CTE 本体に書き込みを含まず末尾が読み取りの `WITH` で、かつトップレベルに `FOR UPDATE` などの行ロック句がない statement だけです。`DELETE ... RETURNING` を `sql.one<T>` で宣言したものや `SELECT ... FOR UPDATE` は書き込みとして扱われます。判定できないものはすべて書き込みに倒れるので、誤判定は「read replica を使えたはずが writer を使う」方向にしか起きません。

`SELECT` から書き込みを行う関数を呼ぶ場合など、静的に判定できない書き込みは検出できません。最終的な防御はデータベース側に残ります。カスタム resolver（generator オプションの `SQLExecutorResolver`）を指定した場合、その契約は読み取り専用かどうかを運べないため、このチェックは無効になります。

## context のみの公開 API

宣言した statement 名をそのまま唯一の実行 API として公開する framework 向けに、
Context 解決版をその名前で生成できます。

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -sql-context-only-api
```

```go
func FindUser(ctx context.Context, id int) (User, error)
```

このモードでは:

- `*sql.DB`、`*sql.Tx`、`sqlbind.Querier`、`sqlbind.Execer` を受け取る公開関数を生成しません
- executor を受け取る関数は非公開になります
- `BuildName` は従来どおり公開されます
- `NameContext` を生成しないため、その名前は空いたままです
- executor は Context から解決されるため、transaction の内外で同じ公開関数を使えます

`-sql-context-only-api` は `-sql-context-api` を含みます。
`sqlbind.SQLExecutorFromContext` の代わりに framework の関数で解決する場合は
`Options.SQLExecutorResolver` を設定します。

## SQL template で作られる関数シグネチャ一覧

以下の `P...` は template parameter 群、`p...` は対応する Go 引数です。

### すべての exported statement

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

### `-sql-context-api` を有効にした場合

```go
func NameContext(ctx context.Context, p ...P) (sql.Result, error) // exec
func NameContext(ctx context.Context, p ...P) (T, error)          // one
func NameContext(ctx context.Context, p ...P) (*T, error)         // optional
func NameContext(ctx context.Context, p ...P) iter.Seq2[T, error] // many
```

### `-sql-context-only-api` を有効にした場合

```go
func Name(ctx context.Context, p ...P) (sql.Result, error) // exec
func Name(ctx context.Context, p ...P) (T, error)          // one
func Name(ctx context.Context, p ...P) (*T, error)         // optional
func Name(ctx context.Context, p ...P) iter.Seq2[T, error] // many
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

JOIN は child の数だけ parent 行を繰り返して返し、この平坦化は戻り件数の宣言では取り消せません。`ScanRows[T]` は取得後に tree を組み直します。対象は既存の任意の query で、SQL template は関与しません。

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

各階層に scalar の `groupkey:""` field をちょうど1つ用意します。マージの判断はすべてこの key が決めます。

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

2つの使い分けを決めるのは、たいていこの最後の制約です。行が1件ずつ流れていける普通の query には SQL template の `sql.one` / `sql.optional` / `sql.many` を選びます。JOIN が同じ parent を繰り返し返し、その parent を丸ごと受け取りたいときに `ScanRows` を選びます。
