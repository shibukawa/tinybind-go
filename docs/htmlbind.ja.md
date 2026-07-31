# htmlbind 利用ガイド

`htmlbind` は `.tb.html` に書いた型付き HTML テンプレートを、Go のレンダープランへ変換します。テンプレートは実行時に解析されず、値の型と HTML 上の挿入位置をコード生成時に検査します。

component ごとに、そのパラメータ構造体で型付けされた不変の命令列が生成され、共有ランタイム `htmlbind` がそれを実行します。生成コードが持つ責務はレンダリングだけで、`net/http` に依存せず、ヘッダも設定せず、コンテントエンコーディングの交渉も行いません。レスポンスはハンドラの責務です。

## 自動化されること

- `.tb.html` ファイルの自動発見
- テンプレート内の型、enum、公開 component の Go 宣言
- component ごとのレンダープラン
- text、attribute、URL、script、style の文脈検査
- 通常の文字列の HTML escape
- optional attribute の省略
- component の組み合わせ、`if`、`for` の描画処理
- 名前付き / 無名スロットの埋め込み
- component ローカル style のスコープ化と、head 寄与のドキュメントへのマージ
- 型エラーや危険な挿入位置のファイル名・行・列付き診断

生成される実装の中身を理解する必要はありません。利用者は `export component` に対応する関数へパラメータを束縛し、その結果を描画します。

## ユーザーが用意するもの

1. Go パッケージ直下の `.tb.html` ファイル
2. `package`、必要な `type` / `enum`、`component` 宣言
3. 外部関数を宣言した場合は、同じ Go パッケージの実装
4. 生成された公開 component を呼ぶハンドラーなど
5. コード生成の実行

## 導入とコード生成

```go
package pages

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

同じディレクトリに `profile.tb.html` を置きます。

```bash
go generate ./...
```

ジェネレーターは対象ディレクトリ直下だけを調べ、`.tb.html` と `.tb.sql` をまとめて `tinybind_templates_gen.go` に出力します。子ディレクトリは別パッケージとして個別に生成してください。

別の命名規則を使う場合は、ベース名に対する glob を
`-html-template-pattern` と `-sql-template-pattern` で指定します。

```go
//go:generate go run github.com/shibukaway/tinybind-go/cmd/tinybind-gen generate -dir . -html-template-pattern "*.page.html" -sql-template-pattern "*.query.sql"
```

既定値は引き続き `*.tb.html` と `*.tb.sql` です。

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

生成される公開 API:

```go
type HelloParams struct {
	Name string
}

func Hello(params HelloParams) htmlbind.Fragment
```

component の引数は常に1つで、宣言順に1フィールドずつ持つ `{ComponentName}Params` 構造体です。引数が0個でも1個でも複数でも同じ形になります。private component には非公開の `render{Name}Params` が生成されます。

`Hello` は何も書き込みません。プランとパラメータを束ねた `Fragment` を返すだけです。描画は別の手順なので、ステータス・ヘッダ・エラー処理はハンドラ側に残ります。

```go
import "github.com/shibukawa/tinybind-go/htmlbind"

func hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := htmlbind.Render(w, Hello(HelloParams{Name: r.URL.Query().Get("name")})); err != nil {
		// 途中まで書き込み済みの可能性があるため、書き直さずログに残す
		log.Printf("render failed: %v", err)
	}
}
```

`Fragment` は不変で共有しても安全なので、パラメータを持たないラッパーは起動時に1回だけ作って使い回せます。

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

type ProfileParams struct {
	User User
	Tone Tone
}

func Profile(params ProfileParams) htmlbind.Fragment
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
| `html` | `htmlbind.Fragment`（スロットが受け取る値） |

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
func Card(params CardParams) htmlbind.Fragment
```

`children: html` を持つ component は開始タグと終了タグの間の内容を受け取れます。children を取らない component は self-closing でも呼べます。

```text
<Avatar user={user} compact={true} />
```

## スロット

`<slot>` は、呼び出し側が渡した内容を挿入する位置を示します。slot 要素自体は出力されず、渡された内容か、宣言された既定値だけが出ます。

```text
component Panel(title: string, header: html?, children: html, footer: html?): html {
<section class="panel">
  <div class="head"><slot name="header"><b>{title}</b></slot></div>
  <div class="body"><slot required /></div>
  <slot name="footer" />
</section>
}
```

- `<slot />` は無名スロットで、予約名 `children` パラメータに束縛されます。
- `<slot name="header" />` は `html` 型の `header` パラメータに束縛されます。
- slot 要素の子は既定コンテンツで、引数が無いときに描画されます。
- `required` は必須スロットの印で、宣言型と一致していなければなりません。`required` なら `html`、無ければ `html?` です。
- 既定値のない任意スロットが未指定なら、要素も囲みもマーカーも残さず消えます。

呼び出し側は、同じ `name` を持つ `template` 要素で名前付きスロットを埋め、残りの内容が無名スロットに入ります。

```text
export component Page(caption: string): html {
<Panel title={caption}>
  <template name="header"><em>Guide</em></template>
  <p>body text</p>
</Panel>
}
```

埋め込みブロック間の空白は無名スロットの内容として数えません。`name` 属性を持たない `template` 要素は通常のマークアップとしてそのまま出力されます。

スロットは `if` の中に置けるので、子要素を出さない分岐も書けます。`if` の両分岐に同じスロットを置くことも可能です（実行されるのは片方だけのため）。`for` の本体に置くことはできず、同一パスで2回描画することもできません。

スロット引数は値ではありません。式の中でスロットパラメータを参照できないので、渡されたかどうかを判定したり、別 component へ転送したり、2箇所に挿入したりはできません。「渡されたか」を調べる代わりに既定コンテンツを使ってください。

## ドキュメント全体を組み立てる

ドキュメントシェル、任意個数のレイアウト、ページはそれぞれ別の component で、多くの場合別ファイルです。`RenderChain` が外側から順に、各要素の無名スロットへ次の要素を埋めながら合成します。

```go
wrappers := []htmlbind.Wrapper{
	BindDocument(DocumentParams{Title: "Docs"}),
	BindLayout(LayoutParams{}),
}
err := htmlbind.RenderChain(w, wrappers, Page(PageParams{Body: "hello"}))
```

ラッパーの数は可変です。空リストならページ単体の描画になり、それは `htmlbind.Render` と同じです。

生成される形が2種類あることで誤用がコンパイルエラーになります。無名スロットを持つ component にだけ `Wrapper` を返す `Bind<Name>` が生成されるため、葉をラッパーとして渡すことはできません。組み立ての検証は1バイトも書く前に行われるので、葉の無いチェーンはステータスコードを変更できる段階で失敗します。

## component の style と script

component は、ドキュメントシェルの外側に `head` 要素を宣言できます。その内容は書いた位置ではなく、ドキュメントの head へ巻き上げられます。

```text
export component Card(label: string): html {
<head>
<link rel="stylesheet" href="/shared.css" />
<style>
.box { color: red; animation: fade 1s }
.box .label { font-weight: bold }
@keyframes fade { from { opacity: 0 } }
</style>
</head>
<div class="box shadow"><span class="label">{label}</span></div>
}
```

この `head` の中では `style` と `script` の中身が生テキストとして扱われるので、CSS や JavaScript の波括弧がテンプレート構文と衝突しません。寄与は静的なマークアップである必要があります。マージ済み head は body の最初の1バイトより前に書かれるため、リクエストデータに依存できないからです。

描画されるチェーンから到達可能な component はすべて寄与します。本体から呼ばれる component も含まれ、同一のタグは1回だけ出力されます。

### スコープ付き style

component の style ブロックは、宣言されたクラス名をリネームし、同じ component 内の該当する `class` 属性も書き換えることでスコープ化されます。

```css
.box_dwu687 { color: red; animation: fade_dwu687 1s }
.box_dwu687 .label_dwu687 { font-weight: bold }
@keyframes fade_dwu687 { from { opacity: 0 } }
```

- style ブロックが宣言していないクラスはそのまま通るので、外部フレームワークのユーティリティクラスはそのまま使えます。
- `@keyframes` 名も、その `animation` / `animation-name` 参照ごとリネームされます。これらの名前は CSS の中からしか参照されないためです。
- `font-family` 名と CSS カスタムプロパティはグローバルのままです。`@font-face` と、component をまたぐテーマ指定を壊さないためです。
- `:global(...)` でセレクタをスコープ化の対象外にできます。
- `p { ... }` のような裸の要素セレクタは生成時エラーです。リネームする名前が無く、全ページに漏れるとスコープ化の意味が無くなるためです。`.card p { ... }` のようにクラスで限定してください。
- 式から与えられるクラスは書き換えられないため、生成時エラーになります。

サフィックスはテンプレートのパスと component 名から導出されるので、無関係な編集で生成クラス名が変わることはありません。

ドキュメントシェルとは `html`、`head`、`body` を持つ component のことで、その `head` 要素がマージ済み寄与の出力先になります。

### 静的ファイルの切り出し

`style` ブロックと、中身を持つ `script` ブロックはレスポンスには載りません。生成時にファイルとして書き出され、マージ済み head には参照タグだけが入ります。これによりクライアントキャッシュが効き、Content Security Policy でインラインスクリプトを禁止できます。

```html
<link rel="stylesheet" href="/public/generated/card.style.1f0a3c9d4b21.css">
<script src="/public/generated/card.script.7c62e0b1d938.js" defer></script>
```

- 1つのテンプレートファイル内の style ブロックは1つのスタイルシートにまとまります。script は component ごとに1ファイルになるので、`defer`、`async`、`type` などの属性がタグにそのまま残ります。
- ファイル名には内容のハッシュが入るので、URL は不変キャッシュ可能で、変更のないプロジェクトは同じ名前を再生成します。
- すでに外部 URL を指している `script` や `link` は、タグがそのまま寄与し、ファイルは作られません。
- 切り出しとハッシュ計算は生成時に行われます。リクエストごとの組み立ては一切なく、構成によって変化しないため参照タグの収集も不要です。

出力先と名前は2つのジェネレータオプションで決まります。

| オプション | デフォルト | 意味 |
| --- | --- | --- |
| `PublicDir` | `public/generated` | 生成ファイルを書き出すディレクトリ |
| `PublicURLBase` | `/public/generated` | 参照 URL の前置き |

generate コマンドでは `-public-dir` と `-public-url-base` です。両者は互いから導出されません。ファイルパスは `PublicDir` とファイル名の連結、参照は `PublicURLBase` と同じファイル名の連結で、パスの一部を推測したり付け足したり削ったりはしません。`PublicURLBase` はそのまま使われるので、`https://cdn.example.com/assets` のような完全 URL を指定すると、書き出し先を変えずに絶対 URL の参照になります。片方だけを設定すると生成は失敗します。必ず両方を設定してください。

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

## 生成される API の形

### 公開 component

テンプレート:

```text
export component Name(p1: T1, p2: T2): html { ... }
```

公開 API:

```go
type NameParams struct {
	P1 T1
	P2 T2
}

func Name(params NameParams) htmlbind.Fragment
```

### 引数なし

```text
export component Layout(): html { ... }
```

```go
type LayoutParams struct{}

func Layout(params LayoutParams) htmlbind.Fragment
```

### 無名スロットを持つ公開 component

`children: html` を持つ component には、チェーン用のバインダも生成されます。

```go
func BindName(params NameParams) htmlbind.Wrapper
```

children を自分で渡すときは `Name`、チェーンに渡させるときは `BindName` を使います。「ドキュメント全体を組み立てる」を参照してください。

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
