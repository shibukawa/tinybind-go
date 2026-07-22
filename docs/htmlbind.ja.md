# htmlbind 利用ガイド

`htmlbind` は `.tb.html` に書いた型付き HTML テンプレートを、`io.Writer` へ描画する Go 関数へ変換します。テンプレートは実行時に解析されず、値の型と HTML 上の挿入位置をコード生成時に検査します。

## 自動化されること

- `.tb.html` ファイルの自動発見
- テンプレート内の型、enum、公開 component の Go 宣言
- component ごとの描画関数
- text、attribute、URL、script、style の文脈検査
- 通常の文字列の HTML escape
- optional attribute の省略
- component の組み合わせ、`if`、`for` の描画処理
- 型エラーや危険な挿入位置のファイル名・行・列付き診断

生成される実装の中身を理解する必要はありません。利用者が呼ぶのは、テンプレートの `export component` に対応する関数です。

## ユーザーが用意するもの

1. Go パッケージ直下の `.tb.html` ファイル
2. `package`、必要な `type` / `enum`、`component` 宣言
3. 外部関数を宣言した場合は、同じ Go パッケージの実装
4. 生成された公開 component を呼ぶハンドラーなど
5. コード生成の実行

## 導入とコード生成

```go
package pages

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

同じディレクトリに `profile.tb.html` を置きます。

```bash
go generate ./...
```

ジェネレーターは対象ディレクトリ直下だけを調べ、`.tb.html` と `.tb.sql` をまとめて `tinybind_templates_gen.go` に出力します。子ディレクトリは別パッケージとして個別に生成してください。

## 最小の component

`hello.tb.html`:

```text
package pages

export component Hello(name: string): html {
<!DOCTYPE html>
<html lang="ja">
  <body>
    <h1>Hello, {name}</h1>
  </body>
</html>
}
```

生成される公開関数シグネチャ:

```go
func Hello(w io.Writer, name string) error
```

`http.ResponseWriter` は `io.Writer` を満たすため、そのまま渡せます。

```go
func hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := Hello(w, r.URL.Query().Get("name")); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}
```

## 型を宣言する

```text
package pages

type User {
  name: string
  active: bool
  nickname: string?
  profileURL: url
  tags: string[]
}

enum Tone { Primary, Secondary }

export component Profile(user: User, tone: Tone): html {
<article>
  <a href={user.profileURL}>{user.name}</a>
</article>
}
```

この例でアプリケーションから使う主な宣言は次の形です。

```go
type User struct {
	Name       string
	Active     bool
	Nickname   *string
	ProfileURL url.URL
	Tags       []string
}

type Tone string

const (
	TonePrimary   Tone = "Primary"
	ToneSecondary Tone = "Secondary"
)

func Profile(w io.Writer, user User, tone Tone) error
```

テンプレートで宣言した型は生成後の同じ Go パッケージに属します。Go 側ではその型を使って引数を組み立てます。

### 型対応

| テンプレート型 | Go から渡す型 |
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
| `html` | `HTML`、component の children 用 |

## 条件分岐

```text
export component Status(active: bool): html {
{if active}
  <span class="active">active</span>
{else}
  <span class="inactive">inactive</span>
{/if}
}
```

`else if` も使えます。

```text
{if score >= 80}
  <strong>A</strong>
{else if score >= 60}
  <strong>B</strong>
{else}
  <strong>C</strong>
{/if}
```

condition は `bool` である必要があります。

## 繰り返し

```text
type User { name: string }

export component UserList(users: User[]): html {
<ul>
{for user, index in users}
  <li data-index={index}>{user.name}</li>
{/for}
</ul>
}
```

index が不要なら省略できます。

```text
{for user in users}
  <p>{user.name}</p>
{/for}
```

## component を組み合わせる

`export` のない component は同じテンプレートモジュール内だけで使う private component です。

```text
type User { name: string }

component Badge(label: string, children: html): html {
<span class="badge"><strong>{label}</strong>{children}</span>
}

export component Card(user: User): html {
<Badge label={user.name}>
  <em>member</em>
</Badge>
}
```

アプリケーションから呼べるシグネチャは公開 component だけです。

```go
func Card(w io.Writer, user User) error
```

`children: html` を持つ component は開始タグと終了タグの間の内容を受け取れます。children を取らない component は self-closing でも呼べます。

```text
<Avatar user={user} compact={true} />
```

## attribute

### 通常 attribute

```text
<p title={user.nickname}>{user.name}</p>
<p class="user {user.active ? 'active' : 'inactive'}">...</p>
```

`string?` の値を attribute 全体に指定した場合、`nil` なら attribute 自体が省略されます。

```text
<p title={user.nickname}>...</p>
```

optional 値を固定文字列と混ぜることはできません。

```text
<!-- 不可: nickname が optional -->
<p title="User: {user.nickname}">...</p>
```

### boolean attribute

```text
<article hidden={not user.active}>...</article>
```

値が true のときだけ `hidden` が出力されます。値なしの静的 boolean attribute も書けます。

```text
<input disabled>
```

### URL attribute

`href` や `src` には `string` ではなく `url` を要求します。

```text
type Link { label: string, destination: url }

export component LinkView(link: Link): html {
<a href={link.destination}>{link.label}</a>
}
```

Go 側では `url.URL` を渡します。

## escape と信頼済みコンテンツ

通常の `string` は HTML text / attribute の文脈で自動 escape されます。

```text
export component Safe(message: string): html {
<p title={message}>{message}</p>
}
```

たとえば `<script>` を含む文字列を渡しても HTML として実行されません。

HTML、CSS、JavaScript を意図的にそのまま挿入する場合だけ、明示的な intrinsic を使います。

```text
type Payload {
  message: string
  count: int
  enabled: bool
}

export component Document(
  markup: string,
  css: string,
  javascript: string,
  payload: Payload
): html {
{RawHTML(markup)}
<style>{RawCSS(css)}</style>
<script>{RawJavaScript(javascript)}</script>
<script>window.payload = {JsonForScript(payload)};</script>
}
```

| intrinsic | 許可される位置 | 意味 |
| --- | --- | --- |
| `RawHTML(string)` | HTML の子要素位置 | 信頼済み HTML を無加工で出す |
| `RawCSS(string)` | `<style>` 内 | 信頼済み CSS を無加工で出す |
| `RawJavaScript(string)` | `<script>` 内 | 信頼済み JavaScript を無加工で出す |
| `JsonForScript(value)` | `<script>` 内 | 型付きデータを script 用に安全な JSON へ変換 |

`Raw*` は sanitizer ではありません。外部入力をそのまま渡さず、アプリケーションが信頼できる固定値または事前に安全性を保証した値に限定してください。データを JavaScript へ渡す用途では `RawJavaScript` ではなく `JsonForScript` を使います。

## 外部関数

表示用の値変換を Go で実装したい場合は `external` でシグネチャを宣言します。

```text
enum Tone { Primary, Secondary }

external Decorate(value: string, tone: Tone): string

export component Label(value: string, tone: Tone): html {
<span>{Decorate(value, tone)}</span>
}
```

同じ Go パッケージに対応する関数を実装します。

```go
func Decorate(value string, tone Tone) string {
	if tone == TonePrimary {
		return "★ " + value
	}
	return value
}
```

## 作られる関数シグネチャ一覧

### 公開 component

テンプレート:

```text
export component Name(p1: T1, p2: T2): html { ... }
```

呼び出し API:

```go
func Name(w io.Writer, p1 T1, p2 T2) error
```

### 引数なし

```text
export component Layout(): html { ... }
```

```go
func Layout(w io.Writer) error
```

### private component

`export` がない component には、アプリケーションから利用する公開 API は作られません。同じテンプレートから component tag として呼びます。

### external

`external` は関数を生成しません。宣言した型に対応する Go 関数をユーザーが同じパッケージに実装します。

## 複数ファイルを使うとき

同じディレクトリのテンプレートは1つの Go ファイルへまとめられます。

- すべて同じ Go package 名にする
- 公開 component、type、enum、external の名前を重複させない
- private component も生成後の宣言名が衝突しないよう、分かりやすい固有名にする

package 宣言を省略できる場合もありますが、Go パッケージと一致する `package pages` のような宣言を各ファイルに置くと意図が明確です。

## 診断の読み方

生成に失敗すると、テンプレートの位置付きで原因が表示されます。

```text
profile.tb.html:12:8: html:url requires url, got string
```

よくある原因は次のとおりです。

- `href` / `src` に `string` を渡した
- `<script>` に通常の `string` を挿入した
- optional 値を複合 attribute の一部にした
- `if` に bool 以外を渡した
- 宣言していない field / function / component を参照した
- `RawHTML` などを許可されていない文脈で使った

診断はコード生成時に出るため、テンプレートを変更したら `go generate ./...` を実行してからビルド・テストしてください。
