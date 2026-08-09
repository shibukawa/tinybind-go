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
- `await` 境界の並行実行と、解決した順でのストリーミング
- `live` なソースを束縛した境界の、配信ごとの再描画
- `@cache` を付けた component の出力を、渡されたストア経由で再利用すること
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
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -html-template-pattern "*.page.html" -sql-template-pattern "*.query.sql"
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

テンプレートで宣言した型は生成後の同じ Go パッケージに属し、呼び出し側はその生成された型で引数を組み立てます。手書きの構造体をテンプレートの型へ変換する仕組みはないので、形が食い違えばそれは Go のコンパイルエラーになります。

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

条件式は `bool` 型でなければなりません。

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

`export` のない component は、テンプレートの組み立ての中だけで使う private component です。

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

寄与にできるのは `link`、`meta`、`style`、`script`、`title`、`noscript` です。要素の子を持てるのは `noscript` だけで、その子は `link`、`style`、`meta` に限られます。それ以外はボディの内容だからです。

```html
<head>
<noscript><meta http-equiv="refresh" content="0; url=/no-script"></noscript>
</head>
```

この書き方の寄与は無条件です。常に出したいページには正しく、一部のレスポンスにだけ出したいタグは、代わりに render 呼び出しで `htmlbind.WithHead` から渡します。

描画されるチェーンから到達可能な component はすべて寄与します。本体から呼ばれる component も含まれ、同一のタグは1回だけ出力されます。同一性はタグ単位なので、2 つの component が両方 `/shared.css` をリンクしてそれぞれ独自の style を宣言した場合、link は 1 つ、style ブロックは 2 つになります。

その寄与には行き先が必要です。ドキュメントシェルとは `html`、`head`、`body` を持つ component のことで、その `head` 要素が出力先になります。

#### 寄与を調べる

束縛済みの component は、自身の寄与をタグ単位で報告します。各タグを宣言した component も一緒に返ります。

```go
fragment := Badge(BadgeParams{Label: "new"})

fragment.Head()
// []string{`<link rel="stylesheet" href="/shared.css">`, `<style>.badge_uvkb9m { color: red }</style>`}

fragment.HeadSources()
// []string{"Badge (components.tb.html:5:1)", "Badge (components.tb.html:6:1)"}
```

`Head` と `HeadSources` は 1 つのリストに対する 2 つのビューです。長さも順序も同じなので、どちらの添字 `i` も同じタグを指します。`Wrapper` も同じ組を持つので、チェーンの要素がどちらの形かを知らずに調べられます。

これは、寄与を配送できない呼び出し側のためのものです。ドキュメントシェルを含まないレスポンス — たとえば swap ライブラリ向けの部分描画 — にはマージ先の head がなく、寄与を捨てればスタイルの当たっていない領域が、どのログにも何も残さずに差し込まれます。拒否するのが安全側の選択で、`HeadSources` は、その報告が head のマークアップを出して読み手に grep させるのではなく、直すべき component を名指しするための手段です。

```go
if head := fragment.Head(); len(head) > 0 {
	// component Badge (components.tb.html:5:1) declares <link rel="stylesheet" ...>
	return fmt.Errorf("component %s declares %s", fragment.HeadSources()[0], head[0])
}
```

`MergeHead` は重複を落としたマージ済みタグを返すので、対応する出自リストを持ちません。チェーンの各要素に自身の分を尋ねてください。

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
type Link {
  label: string
  destination: url
}

export component LinkView(link: Link): html {
<a href={link.destination}>{link.label}</a>
}
```

Go 側では `url.URL` を渡します。描画時には値のスキームも検査されるので、パースを通った `javascript:` URL であっても属性には届きません。属性の名簿、許可されるスキーム、その設定方法は [htmlbind_security.ja.md](htmlbind_security.ja.md) にあります。

## 空白の扱い

インデントは生成物に持ち込みません。静的マークアップ中の空白の連続は、生成時に
スペース 1 個へ畳まれます。ブラウザがその連続を 1 個のスペースとして描画するのと
同じ結果なので表示は変わらず、生成された Go・バイナリ・毎回のレスポンスから
1 行あたり 1 連続ぶんの空白が消えます。

```text
export component Card(): html {
<div class="card">
    <h1>Title</h1>
</div>
}
```

は `" <div class=\"card\"> <h1>Title</h1> </div> "` を出力します。改行と 4 スペースの
インデントはそのまま残りません。

削除ではなく 1 個へ畳むのは、インラインボックス同士の間の空白が見えるためです。

```text
<span>a</span>
<span>b</span>
```

は `a b` と描画されます。改行を消せば `ab` になりますが、ある要素がインラインか
どうかは CSS の話で、ジェネレータには判断できません。

次のものはバイト単位でそのまま保たれます。

- `<pre>` と `<textarea>`、およびその中身すべて
- `<script>` と `<style>` の本文。改行は行コメントの終端であり、自動セミコロン挿入の
  根拠でもあります
- `preserve-whitespace` を付けた部分木

マークアップではなくスタイルシートによって空白が意味を持つようになった要素、
つまりジェネレータからは見えないケースでこの目印を使います。

```text
<div id="log" preserve-whitespace>
  first line
  second line
</div>
```

この属性は予約語で、出力には現れません。値を取らない属性なので、
`preserve-whitespace="false"` は黙って無視されるのではなく生成エラーになります。

空白だけの連続を完全に削除するのは、HTML パーサ自身がそれを捨てる位置に限ります。
`<html>`・`<head>`・table 系要素の直下と、ドキュメント全体を描画する
component の doctype 周辺です。

実行全体で元の空白をバイト単位で保ちたい場合 — 既存の golden ファイルと生成
マークアップを突き合わせる場合など — はジェネレータオプションの
`PreserveTemplateWhitespace` を指定します。

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
| `RawJavaScript(string)` | `<script>` 内、および `on*` イベントハンドラ属性 | 信頼済み JavaScript を無加工で出す |
| `JsonForScript(value)` | `<script>` 内 | 型付きデータを script 用に安全な JSON へ変換 |

`Raw*` は sanitizer ではありません。外部入力をそのまま渡さず、アプリケーションが信頼できる固定値または事前に安全性を保証した値に限定してください。データを JavaScript へ渡す用途では `RawJavaScript` ではなく `JsonForScript` を使います。

イベントハンドラ属性は JavaScript の文脈なので `trusted_javascript` しか受け付けません。`onclick={message}` に `string` を渡すと生成が失敗します。振る舞いを付けるなら `server-action` を推奨します。[htmlbind_security.ja.md](htmlbind_security.ja.md) を参照してください。

### `<script>` と `<style>` の中の波括弧

`<script>` と `<style>` の内容は JavaScript と CSS であり、そこでの `{` はその言語の構文です。この 2 つの要素の中では、波括弧が**間に空白を置かずに**次の形のいずれかで書かれたときだけテンプレートの挿入になります。

```text
{name}                    値そのもの
{record.field}            メンバアクセス
{JsonForScript(payload)}  呼び出し
{(active ? on : off)}     丸括弧で囲んだ式
{if ready} ... {/if}      制御ブロック
```

それ以外の波括弧は内容です。区別しているのは直後の空白なので、`{ name }` は内容、`{name}` は挿入です。

#### 内容として残るもの

以下はどれもエスケープ不要で、1 行ずつそのまま出力されます。

```text
export component Widget(): html {
<script>
class X {}                                  // 空ブロック
function f() {
  return 1
}                                           // 行末の波括弧
function g(){return 1}                      // minify された関数
const o = { a: 1 };                         // オブジェクトリテラル
const n = {0: 'a'};                         // 数値キー
const p = {a, b};                           // 短縮記法の複数指定
class C { m() { this.v = 1; } }             // this へ代入するメソッド本体
if (x) { render() }                         // 1 文だけのブロック
const s = `hi ${name}`;                     // テンプレートリテラル
</script>
<style>
.a { color: red; }                          /* 宣言ブロック */
.b{color:red}                               /* minify された宣言 */
@media print {
  .c { color: #000; }
}                                           /* 入れ子の at-rule */
</style>
<script type="speculationrules">
{"prerender": [{"where": {"href_matches": "/*"}}]}
</script>
}
```

特筆すべきは `${name}` の行です。そのまま残るので、JavaScript のテンプレートリテラルはブラウザ側の意味を保ちます。この規則を入れる前は挿入として読まれていて、`name` という名前の `trusted_javascript` パラメータがスコープにあると**診断なしでコンパイルが通り**、クライアントコードとして書いたものにサーバの値が埋め込まれていました。

#### 挿入になるもの

以下はいずれも式の文法に到達し、子ノード位置の挿入とまったく同じように、文脈に対して型検査されます。

```text
type Payload { id: int }
type Config { js: trusted_javascript }

export component Widget(
  js: trusted_javascript,
  cfg: Config,
  css: string,
  payload: Payload,
  ready: bool,
  on: trusted_javascript,
  off: trusted_javascript
): html {
<script>{js}</script>                          <!-- 値そのもの -->
<script>{cfg.js}</script>                      <!-- メンバアクセス -->
<script>{JsonForScript(payload)}</script>      <!-- 呼び出し -->
<script>{(ready ? on : off)}</script>          <!-- 丸括弧で囲んだ式 -->
<style>{RawCSS(css)}</style>                   <!-- style 内容での呼び出し -->
<script>{if ready}console.log(1){/if}</script> <!-- 制御ブロック -->
}
```

上の形で表せない式は丸括弧で囲みます — `{items[0]}` ではなく `{(items[0])}`、`{ready ? on : off}` ではなく `{(ready ? on : off)}`。

#### 内容と形が衝突する場合

タイトに書かれるため形に一致してしまう記述が 2 つあります。どちらも黙って置換されるのではなく、診断されます。

```text
<script>const o = {name};</script>     ⟶  unknown identifier name
<script>if(x){render()}</script>       ⟶  unknown function render
```

逃げ道は `{{` ... `}}` です。ここに限らずテンプレート全体でのリテラル波括弧のエスケープで、`{` ... `}` を 1 組だけ出力し、中身は解析されません。

```text
<script>const o = {{name}};</script>
```

は `const o = {name};` を出力します。

1 つだけ黙って通るケースが残ります。タイトな短縮記法の名前が、挿入可能な型のパラメータ名と一致した場合です。`payload: script_json` がスコープにある状態の `const o = {payload};` はコンパイルが通り、置換されます。対処は上のエスケープで、また、実際のコードでずっと多く書かれる空白付きの `const o = { payload };` が内容として扱われる理由もここにあります。

`<html>` の外側に宣言した `<head>` は [head 寄与](#component-の-style-と-script)であり、その `<script>` と `<style>` の内容は verbatim です。そこには形の規則が一切適用されません。上の規則が対象にしているのは、ドキュメントシェル自身の head と、要素の内容です。

波括弧が意図せず挿入として読まれた場合、診断が要素名と逃げ道を示します。

```text
tasks.tb.html:13:65: unknown identifier name; this is inside <script> content,
where {...} is a template insertion. Write {{...}} to keep a literal brace,
insert a value with RawJavaScript or JsonForScript, or move the script to a file
under the public asset directory
```

数行を超えるスクリプトなら、最後の選択肢がたいてい正解です。アプリケーションが配信するファイルに出して参照してください。

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

### リクエストを読む

先頭に `context.Context` を宣言すると、ページが描画されている context — async 系
エントリに渡した `ctx`、または同期系エントリに `WithContext` で渡した値 — を受け
取ります。

```go
func RequestID(ctx context.Context) string { return traceFrom(ctx) }
```

テンプレート側の宣言はどちらでも `external RequestID(): string` のままです。つま
りこれは Go を書く人が関数ごとに決めることです。引数を書かなければ、これまでどお
り素朴に呼ばれます。

ページではなくリクエストに属する値 — リクエスト ID、nonce、ロケール — を、
必要なページすべてのパラメータ構造体を経由させずに markup へ届けるための仕組みで
す。これは読み取りであって、この形で呼ばれる関数がレスポンスに書いてはいけません。

`: html` と宣言した external は `htmlbind.Fragment` を返し、部分木として描画され
ます。トークンだけでなく hidden input 全体を返せます。

```text
external RequestBanner(): html
```

```go
func RequestBanner(ctx context.Context) htmlbind.Fragment { ... }
```

> [!NOTE]
> **CSRF トークンはこれではありません。** かつてはここの例でしたが、今はモジュールが
> 自動で出します。unsafe form はすべて hidden field を自動的に持ち、トークンは context
> ではなく描画呼び出しの `htmlbind.WithCSRFToken(token)` で渡します。
> [htmlbind_frameworkowner.ja.md](htmlbind_frameworkowner.ja.md#csrf-トークンのライフサイクルを持つ) を参照してください。

```go
```

## 非同期 component

`external async` で宣言した関数は、ページの描画中に並行して実行されます。Go 側の
実装は普通のブロッキング関数のままで構いません。通常の external との違いは error
を返す点だけで、これは境界が recover する対象を必要とするためです。

```text
external async LoadUser(id: string): User
external async LoadPosts(id: string): Post[]
```

```go
func LoadUser(id string) (User, error)
func LoadPosts(id string) ([]Post, error)
```

このコードは並行処理を何も知りません。goroutine 化と完了待ちはランタイム側が行う
ので、1 つの `await` に遅い呼び出しが 2 つあっても、合計ではなく遅い方の時間で
済みます。

### context を受け取る

第1引数に `context.Context` を宣言すると、その境界の context が渡されます。

```go
func LoadPosts(ctx context.Context, id string) ([]Post, error)
```

テンプレートの宣言は変わりません。コード生成時にパッケージ内の Go ソースを読み、
context を受け取る関数にだけ渡します。関数ごとに、実装を書く側が決められます。
パラメータを持たない関数は、そのまま呼ばれます。

DB クエリや外部リクエストなど、本当に中断できる呼び出しでは受け取ってください。
中断できない呼び出しでは省略して構いません。待ち時間はどちらでも制限されます。

非同期の結果は、それを待っている境界の内側にしか存在しません。そのため async
関数は `await` の束縛でのみ呼び出せます。それ以外の場所での呼び出しは生成時エラー
になります。

### await / fallback / recover

`await` ブロックは 3 つの節から構成されます。

```text
export component Profile(id: string): html {
<section>
{await user = LoadUser(id), posts = LoadPosts(id)}
  <h1>{user.name}</h1>
  <ul>{for post in posts}<li>{post.title}</li>{/for}</ul>
{fallback}
  <p class="pending">読み込み中…</p>
{recover err}
  <p class="failed">{err.message}</p>
{/await}
</section>
}
```

- `await` の後ろの束縛は同時に開始します。それぞれが 1 つの非同期呼び出しに名前を
  付け、束縛された値は primary 側で普通の型付き識別子として扱えます。
- `fallback` は必須です。最初にレスポンスへ確定するのがこの内容なので、遅い依存が
  ページの残りを止めることはありません。
- `recover` は任意で、安全な `error` 値を束縛します。読めるフィールドは `code`、
  `message`、`retryable`、`timeout` です。

`recover` を省略したブロックが失敗すると、その失敗はテンプレートの外に出て、ページ
全体の失敗になります。`Render` は fallback を描かずに `*htmlbind.UnrecoveredError`
を返し、`RenderAsync` はそれを yield してシーケンスを終えます。逐次描画では
fallback が既にレスポンスへ乗っているので、画面を差し替えるのは呼び出し側 — 多くは
上に載っているフレームワーク — の仕事です。ページの一部だけを失敗として見せたいなら
`recover` を書いてください。永久に終わらないローディング表示が残ることはありません。

束縛は primary 側だけ、エラー名は `recover` 側だけで見えます。そのため、描画時点
で存在しない値をどの節からも読めません。

`await` ブロックの中に `<slot>` は書けません。fallback と置き換え後の両方が同じ
スロットを描画してしまうためです。

### 呼び出し側が始める値

`external async` の呼び出しが始まるのは、境界がそこに到達した時点です。つまり、
それより前に走っていたもの — リクエストの解析にも、上位のレイアウトの描画にも —
重ねられません。パラメータ側を `async` と宣言すれば、処理を始める場所は呼び出し
側が決められ、テンプレートに残るのは結果を待つことだけになります。

```text
type Customer {
  name: string
  orders: async Order[]
}

export component Profile(customer: Customer, headline: async string?): html {
<h1>{customer.name}</h1>
{await orders = customer.orders}
  <ul>{for order in orders}<li>{order.id}</li>{/for}</ul>
{fallback}
  <p>{customer.name} の注文を読み込み中…</p>
{/await}
}
```

```go
customer := Customer{
	Name:   "ada",
	Orders: htmlbind.Go(ctx, func(ctx context.Context) ([]Order, error) {
		return store.Orders(ctx, id)
	}),
}
err := htmlbind.Render(w, Profile(ProfileParams{Customer: customer}))
```

`async T` は任意のパラメータと record フィールドに付く前置修飾子で、Go では
`htmlbind.Pending[T]` になります。関数ではなく、呼び出すこともできません。読める
のは `await` の束縛だけで、そこでは async な呼び出しと同じ節に並び、一緒に確定
します。

修飾子は型全体にかかるので、`async Order[]` は未完了の値の配列ではなく、1 つの
未完了なスライスです。行ごとに個別に届いてほしいなら、行の型に `async` フィールド
を持たせて `for` の中で await します。

record は確定済みのメンバと未完了のメンバを同時に持てます。上の例が fallback の
中で `customer.name` を描画できるのはそのためで、待っている注文だけが境界の向こう
に残ります。

ハンドルを作るコンストラクタは 3 つです。

| コンストラクタ | 用途 |
| --- | --- |
| `htmlbind.Go(ctx, work)` | 専用の goroutine で処理を開始する |
| `htmlbind.Resolved(v)` | 既に手元にある値、およびテスト |
| `htmlbind.Failed(err)` | 既に分かっている失敗 |

チャネルを受け取るコンストラクタはありません。既にチャネルを返すサービスは、
`Go` のクロージャの中で受け取れば取り込めます。こうすると、すべてのハンドルが
このパッケージ自身の起こした goroutine のものになり、その中で起きた panic は
プロセスの終了ではなくハンドルのエラーになります。

ハンドルは一度だけ確定し、その後も読めます。だからレイアウトとその中のページが
同じ値を持ってよく、両方の境界が同じ結果を見て、処理は 1 回しか走りません。
チャネルなら、先に受信した側にだけ値が渡っていたところです。

`Go` に渡す context が縛るのは処理そのもので、その処理を止める責任は呼び出し側に
残ります。描画側が縛るのは待ち時間だけです。

待つ型が optional なら、未設定のハンドルは正当な値です。即座に「不在」として確定
し、境界を開かず、`recover` にも行きません。不在は失敗ではなくデータだからです。

一方、必須の側を未設定のまま渡すのは呼び出し側のバグで、終わらない待ちではなく
エラーとして表面化します。チェーンのメンバ自身のパラメータから辿れる値なら 1
バイトも書く前に検査されるので、そのレスポンスはまだエラーステータスを返せます。
ループ項を経由して辿る値は、ループがそこに到達した時点で検査されます。いずれの
場合も、エラーはテンプレートが宣言したとおりの名前を伝えます。

```go
var unset *htmlbind.UnsetPendingError
if errors.As(err, &unset) {
	log.Printf("%s が設定されていません", unset.Path)
}
```

キャッシュ component は `async` パラメータも、`async` フィールドに到達する record
も宣言できません。保存されたバイト列は再描画の代わりを務めるものですが、未完了の
値はそれを開始した 1 リクエストのものだからです。

### 更新され続けるソース

`external async` の関数は一度だけ答えます。ゲージ、チャットのログ、フィードが
欲しいのはその逆で、値はサーバ側の都合で変わり、それを映す領域も一緒に変わって
ほしい。ソースを `live` と宣言すると、値ではなくシーケンスを返すようになります。

```text
external live WatchMetrics(id: string): Point
```

```go
func WatchMetrics(ctx context.Context, id string) iter.Seq2[Point, error]
```

束縛するのは普通の `await` ブロックです。専用のキーワードはありません。`await`
節はもともと「変化は一度きり」を制約していたわけではなく、**どの値を待つか**を
言っているだけで、**何回来るか**は宣言側が言っているからです。

```text
{await point = WatchMetrics(id)}
  <p class="value">{point.label}: {point.value}</p>
{fallback}
  <p class="pending">待機中…</p>
{recover err}
  <p class="failed">{err.code}</p>
{/await}
```

先に commit されるのは fallback で、これは一度だけ確定する束縛と同じです。その
あと、ソースが値を yield するたびに primary が描き直され、毎回同じ領域を置き換え
ます。

先頭の `context.Context` は、ここでは任意ではなく必須です。終わらないソースは
止められる必要があり、context が無いと購読が終わったときに戻らせる手段がありま
せん。

1 回の配信が運ぶのは、増分ではなく**その領域の状態全体**です。チャットを見張る
ソースは現在のメッセージ一覧を、ゲージを取るソースは現在のウィンドウを yield し
ます。これが描画を何度でも繰り返せる性質を保ち、値を取りこぼしても何も失われない
理由になります。裏を返すと、蓄積した状態を持つのはソース側で、テンプレートは配信
と配信の間に何も持ちません。

配信は pull です。ソースは自分の `yield` の中で、境界が次の値を受け取れるように
なるまでブロックします。画面が使い切れない速さで値を作るソースはキューを膨らませ
るのではなくティックを取りこぼし、サイズを決めるバッファもありません。range を
抜けることがそのまま購読解除になります。

yield されたエラーは**失敗の配信**であって、ソースの終わりではありません。境界は
`recover` を描き、次の値がそれを置き換えます。一時的な障害は表示されて、そして
消えます。終端の合図はシーケンスが返ることだけです。`recover` 節を書かなかった
ブロックは最初の失敗で購読を終えます。これは一度だけ確定する束縛と同じ規則です。

1 つの節に両方を束縛できます。

```text
{await title = LoadTitle(id), point = WatchMetrics(id)}
  <h1>{title}</h1>
  <p>{point.label}: {point.value}</p>
{fallback}
  <p>待機中…</p>
{/await}
```

`LoadTitle` は一度で確定し、`WatchMetrics` は配信を続けます。毎回の描画が両方を
読むので、**どのソースが動いたかを言う必要がありません**。最初の描画は全部の束縛
に値が揃うまで待ちます。primary はすべてを読むので、揃う前に描くとどれかがゼロ値
になるためです。他の束縛を待っている間に届いた値は合体されます。最新の 1 つが
あれば足りるからです。

live な領域が描くのは入力ではなく出力です。primary の中の `<form>`、`<input>`、
`<textarea>`、`<select>` は生成エラーになります。配信はユーザーが入力している
最中にも届き、部分木の置き換えは入力内容を何の合図もなく捨てるからです。コント
ロールは境界の外に置き、中では live なデータだけを変えてください。規則は束縛では
なく境界に従うので、live な束縛が 1 つあればブロック全体に適用されます。`fallback`
と `recover` は配信で置き換わらないので対象外です。

フォーカス、スクロール位置、メディアの再生位置も同じ危険にさらされますが、静的な
規則では禁じられません。タイマーで描き直される領域には置かないでください。

### 非同期 component の描画

`Render` は束縛の完了を待ち、確定した内容をその場に書き出します。`await` を含む
テンプレートでも、クライアント側の JavaScript なしで完全な HTML が得られます。

```go
err := htmlbind.Render(w, Profile(ProfileParams{Id: id}))
```

もう一方の選択肢は fallback を先に流すことです。そちらを選ぶには、その構成が
そもそも境界を開き得るのかを事前に知る必要があります。答えるのが
`HasAwaitBlock` で、`Fragment` と `Wrapper` にあり、メンバを合算するチェーン形も
あります。

```go
if htmlbind.HasAwaitBlock(wrappers, page) {
	// このレスポンスはストリームする
}
```

フラグは推移的なので、async な component を呼ぶだけの component も `true` に
なります。読んでも何もレンダリングされません。パラメータ経由で渡した Fragment
は数えられないので、自分で作った値と合算してください。

`RenderAsync` は先に fallback を送り、確定した境界を順に yield します。返るのは
シーケンスで、書き込むのはハンドラ側です。

```go
func profile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := Profile(ProfileParams{Id: r.PathValue("id")})
	for content, err := range htmlbind.RenderAsync(r.Context(), w, page,
		htmlbind.WithAsyncTimeout(3*time.Second),
		htmlbind.WithErrorReporter(func(err error) { log.Printf("boundary failed: %v", err) }),
	) {
		if err != nil {
			// レスポンスは確定済みなので、書き直さずログに残す。recover を持た
			// ない境界の失敗も *htmlbind.UnrecoveredError としてここに届く
			log.Printf("render failed: %v", err)
			break
		}
		if err := writeCompletion(w, content); err != nil {
			break
		}
		htmlbind.Flush(w)
	}
}
```

`writeCompletion` はモジュール側ではなく利用側のコードです。次節で書きます。

ラッパーチェーンには `RenderChainAsync` を使います。このループを隠すエントリは
用意していません。境界がいくつ出るかは事前に分からず、リクエスト時に組み立てる
チェーンならなおさらなので、ストリーミングするハンドラは結局シーケンスに対して
書くことになるためです。

live な境界は、この 2 つのエントリでは最初の配信を取って購読を解除します。ロー
ディング状態ではなく実データを初回描画に載せつつ、レスポンスは終われるようにする
ためで、クローラや JavaScript を持たないブラウザが受け取るのもこれです。購読を
開いたままにするエントリが `RenderLive` と `RenderChainLive` です。

```go
for content, err := range htmlbind.RenderLive(ctx, io.Discard, page) {
	if err != nil {
		log.Printf("delivery failed: %v", err)
		break
	}
	// content をクライアントへ届ける
}
```

シーケンスが終わるのは、すべてのソースが終わったとき、range を抜けたとき、
context がキャンセルされたときです。境界が確定したときではありません。live な
境界は確定しないからです。配信だけが欲しいときは `io.Discard` を渡します。ソース
の引数を評価するには、それを持つ component を実行する必要があるのでドキュメントの
バイト列自体は生成されますが、転送はされません。境界 ID は位置で決まるので、同じ
チェーンをもう一度描けば同じ ID になり、クライアントは既に画面にあるプレース
ホルダを指せます。

`WithAsyncTimeout` は、応答を返す義務のあるエントリで「境界が何も出していない
時間」を打ち切ります。時間切れはソースの失敗ではないので、`recover` に置き換わる
のではなく fallback がそのまま残ります。まだ届いていない値について、fallback は
正直だからです。`RenderLive` にこの打ち切りはありません。live なソースは、データ
が静かな間はいくらでも静かでいてよいためです。

レスポンスへ書き込むのは range している呼び出し側だけで、途中で range を抜けると、
残っている境界を待たずに描画が終わります。初回パス後の flush はランタイムが行い
ます。各チャンクの後は `htmlbind.Flush` で同じことをします（flush できない writer
では何もしません）。

### 完了チャンクの framing

逐次描画では、未確定の境界は fallback を `<!--tb:...-->` と `<!--/tb:...-->` で挟む形で
書き出されます。モジュールが担うのはここまでです。残りの半分は利用側の担当で、
yield される `Content` は確定したフラグメントと、それが埋まるプレースホルダの ID
だけを持ちます。`WriteTo` が書くのはフラグメント本体だけで、囲みもマーカーも
スクリプトもありません。head へ何かを差し込むこともありません（同期・非同期の
どちらの入口でも）。

これは意図的な分割です。完了をどう運ぶかと、それを適用するクライアントコードは
一つの設計なので、持ち主も一つ — 上に載せるフレームワーク、あるいはハンドラ自身
— であるべきだからです。

既定として妥当なのは次のレシピです。フラグメントを inert な template で包み、
後ろにマーカー要素を続けます。

```go
func writeCompletion(w io.Writer, content htmlbind.Content) error {
	if _, err := io.WriteString(w, `<template data-tb-boundary="`+content.BoundaryID+`">`); err != nil {
		return err
	}
	if _, err := content.WriteTo(w); err != nil {
		return err
	}
	_, err := io.WriteString(w, `</template><tb-apply for="`+content.BoundaryID+`"></tb-apply>`)
	return err
}
```

ワイヤ上はこうなります。

```html
<template data-tb-boundary="tb-1">…</template><tb-apply for="tb-1"></tb-apply>
```

`tb-apply` の定義は、すでに配信しているランタイムスクリプトに一度だけ置き、
`connectedCallback` で置換します。

```js
customElements.define("tb-apply", class extends HTMLElement {
	connectedCallback() {
		const id = this.getAttribute("for");
		this.remove();
		const template = document.querySelector(`template[data-tb-boundary="${id}"]`);
		if (!template) return;
		const placeholder = document.getElementById(id);
		if (placeholder) placeholder.replaceWith(template.content);
		template.remove();
	}
});
```

配信済みのファイルに入っているので、完了チャンク側にはスクリプトが一切乗らず、
CSP に nonce も `unsafe-inline` も不要です。

このマーカーが置換の安全性を担保しています。HTML パーサは**開始タグの時点で**要素を
挿入するので、template の出現に反応する実装では、中身がまだ届いていない template を
読んでプレースホルダを空で置き換えてしまい、結果どころか fallback まで失う可能性が
あります。`<tb-apply>` はバイト列上 `</template>` の後にあるため、プロキシ・TLS
レコード・圧縮エンコーダがどう分割しても、存在する時点で template は完成しています。
マーカーが届かなかった完了は適用されず、fallback が残ります。トリガにするのは
マーカーであって template ではありません。

どのスクリプトにも適用されなかったレスポンスは fallback がそのまま残ります。
JavaScript を切ったクライアントに見えるものと同じです。`Render` なら境界はその場で
確定するので、ランタイム無しで動く必要があるルートにはそちらの入口があります。

### キャンセルが打ち切るのは「待ち」

リクエストのキャンセルや `WithAsyncTimeout` の満了で、ランタイムは待つのをやめ
ます。キャンセルされた境界は完了を出しません。読む相手がもういないからです。
タイムアウトは他と同じ失敗なので、`code: "timeout"` で recover を描画するか、
`recover` を持たないブロックならページ全体の失敗になります。

処理そのものが止まるかどうかは external 次第です。context を受け取っていれば
キャンセルを見て早く戻れます。受け取っていなければ中断できないので、放置されます
（自然に終わり、結果は捨てられます）。

### エラーはサーバ側に留まる

recover 節から生の Go の error は見えません。既定では失敗は `code: "internal"`
（message は空）に、タイムアウトは `code: "timeout"` になります。より具体的な情報を
公開したい場合は、error 自身に安全な射影を持たせます。

```go
type UpstreamError struct{ Service string }

func (e UpstreamError) Error() string { return "upstream " + e.Service + " unreachable" }

func (e UpstreamError) PublicError() htmlbind.AsyncError {
	return htmlbind.AsyncError{Code: "upstream", Message: "時間をおいて再試行してください。", Retryable: true}
}
```

`WithErrorReporter` にはどちらの場合も元の error が渡ります。`recover` 節で処理した
失敗も届くので、ログや計測から漏れることはありません。

## キャッシュ component

component に `@cache` を付けると、同じパラメータに対する描画結果を再利用できます。

```text
@cache(ttl: "5m")
export component Sidebar(userId: string, tone: Tone): html {
<aside>...</aside>
}
```

`ttl` は生成時に解釈されるので、不正な duration はリクエスト時ではなくビルド時に
失敗します。`ttl` を書くことが「保存する」という指定でもあります。`ttl` のない注釈は
スコープを宣言するだけで何も保存しません。これは後述の
[キャッシュスコープ](#キャッシュスコープ)で扱います。

キャッシュはテンプレートの書き換えではなく運用上の選択です。呼び出し側がストアを
渡すまで、何も保存されません。

```go
var pageCache = htmlbind.NewMemoryCache(1024)

err := htmlbind.Render(w, Page(params), htmlbind.WithCache(pageCache))
```

`WithCache` は `RenderChain`、`RenderAsync`、`RenderChainAsync` でも使えます。渡さなければ、
注釈付き component も注釈がない場合とまったく同じように描画されます。

### キーが表すもの

キーには、component のパッケージとファイル、生成されたプランのフィンガープリント、
宣言された全パラメータの正規化された表現が入ります。パラメータの変更、テンプレートの
編集、その component が描画する内容の編集は、いずれも別のキーになります。再生成した
コードが古い出力を読むことはありません。

宣言されたパラメータでないものはキーから見えません。例外が一つだけあり、private な
component のキーにはレンダーのスコープ値も入ります。これが
[キャッシュスコープ](#キャッシュスコープ)です。ロケールなど、スコープ値が覆わない
ものは、パラメータとして渡すか、その component をキャッシュしないでください。

### キャッシュスコープ

レスポンスは、何も言われなければ private です。どこにも注釈のないチェーンは private を
報告し、`scope: "public"` だけがその唯一のオプトアウトです。

```go
if htmlbind.IsPrivate(wrappers, leaf) {
	w.Header().Set("Cache-Control", "private, no-store")
}
```

読んでも何も描画しないので、ヘッダは最初のボディバイトより前に決められます。4 階層下の
private な component が描画されるのはヘッダを確定したずっと後なので、この順序が要ります。

既定が非対称なのは意図的です。実際にはユーザごとなのに共有扱いにした component は、
ある読み手の出力を別の読み手に配ります。逆の間違いはキャッシュミス 1 回で済みます。
さらに、誰にも見えないものを覆えます。`context.Context` からリクエスト識別子を読む Go
関数を呼ぶ component は、コンパイラが行えるどの検査からも共有可能に見えます。

状態は 3 つあり、周囲の `public` 表明に対する態度で分かれます。

| 書いたもの | 報告 | 周囲の `public` に対して |
| --- | --- | --- |
| 何も書かない | private | 継承する |
| `@cache(scope: "private")` | private | 生成時に拒否する |
| `@cache(scope: "public")` | 共有 | — |

保存する `@cache(ttl: "5m")` に `scope` を書かなければ private 宣言になり、キーは
読み手ごとに分かれます。値は `WithCacheScope` で渡します。

```go
err := htmlbind.Render(w, Page(params),
	htmlbind.WithCache(pageCache), htmlbind.WithCacheScope(accountID))
```

値は不透明です。エントリが属する読み手を識別できるものを渡してください。こちらは何も
解釈しません。**渡さなければ private な component は何も保存しません。** 空のスコープで
書かれたエントリは、private の札を下げた共有エントリになってしまうので、ミスにするのが
意図的な答えです。

layout にも `ttl` なしで付けられます。slot の持ち主は保存できないので `ttl` は生成
エラーになり、適格性の条件はどれも適用されません。認証済み layout に 1 つ書けば、その
下の全ページを覆えます。

```text
@cache(scope: "private")
export component AuthLayout(children: html): html {
<html>...<slot required /></html>
}
```

チェーンでは、private がどこにあっても勝ちます。`public` 宣言はいちばん外側の
メンバーに書かないとレスポンス全体を覆えません。ラッパーはその下の全部を含みますが、
内側のメンバーは自分を包んでいるマークアップについて何も言えないからです。

`PrivateSource` は、チェーンを private にした宣言の component 名を返します。共有される
はずだと思っていた作者に見せるのはこれです。

ストアの大きさはこれに合わせてください。private な component はスコープごとに 1
エントリを持つので、共有キー向けに決めたエントリ上限が読み手の数だけ分割されます。
`NewMemoryCache` は挿入順に近い順序で追い出すため、上限が小さすぎると緩やかに劣化する
のではなくスラッシングします。

呼び出しグラフが private 宣言に届く component に `scope: "public"` を書くと、スコープを
書いた位置で生成エラーになります。宣言のない component は表明を妨げません。妨げたら、
何一つ public にできなくなります。

### 独自ストアを渡す

`CacheStore` はメソッド 2 つのインタフェースです。Redis や memcached のアダプタは
普通のアプリケーションコードとして書けます。

```go
type CacheStore interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration)
}
```

`Set` は何も返しません。すでに正しく描画できたレスポンスを、キャッシュ書き込みの失敗
で壊さないためで、失敗の報告は実装側の責務です。ストアは 1 回の描画中に複数の
goroutine から使われるため、並行安全である必要があります。キーは長くなりうる普通の
文字列なので、ストア側でハッシュ化して構いません。

### 制約

保存されたバイト列で置き換えられない component は、生成時に拒否されます。

- `html` パラメータを宣言できません。スロット引数は値ではなく束縛された継続であり、
  キーに入れられないためです。
- 直接でも呼び出し先経由でも、`await` 境界に到達できません。境界はプレースホルダと
  置き換え内容の 2 回に分けて出力されるため、1 つの連続したバイト列にならないため
  です。
- ドキュメントの `head` を所有できません。マージ後の head はパラメータではなく
  チェーンに依存するためです。
- 直接でも呼び出し先経由でも、unsafe な `<form>` と、provider を持つ登録済み要素に
  到達できません。どちらもリクエストごとの値を運ぶので、保存されたボディはあるセッ
  ションの値を次に来た人へ配ることになります。

## フォームと CSRF トークン

post / put / patch / delete するフォームは CSRF トークンを運びます。**書くことは何も
ありません。**

```html
<form method="post" action="/orders">
  <input name="title" value="{title}">
  <button>作成</button>
</form>
```

hidden field はフォームの第一子として生成されます。トークン自体はフレームワークから
来ます（描画オプションであって、テンプレートが名指しするものではありません）。同じ値
がページ上のすべてのフォームに届き、ブラウザランタイムはリクエストヘッダにも同じ値を
載せます。

4つの規則が続きます。うち3つはブラウザで気づく不具合ではなく生成エラーです。

| ケース | 挙動 |
| --- | --- |
| `method="get"`、または method 無し | トークンなし。GET のフィールドはクエリ文字列になり、URL に載ったトークンは履歴・ログ・リファラに残ります。 |
| `action` が静的な絶対 URL | **生成エラー。**挿入するとセッションの秘密を別オリジンに渡すことになります。 |
| `method` が式 | **生成エラー。**生成時に読めず、外すとトークンを漏らすかフォームを無防備にするかのどちらかです。 |
| 同名のフィールドが既にある | 触りません。手書きのトークンがそのまま動きます。 |

本当に別オリジンへ post するフォームには印を付ければ飛ばされます。この印は設定された
属性接頭辞に従い、ブラウザには届きません。

```html
<form method="post" action="https://payments.example.com/checkout" data-tb-no-csrf>
```

`Origin` と Fetch Metadata の検査だけで行くと決めた配信は、生成時にこの仕組みごと切
れます。上のキャッシュ制約も一緒に外れます。

## ハイフン付き要素

ハイフンは HTML 自身のカスタム要素の目印なので、ハイフン付き要素の空間はビルドが宣言
するホワイトリストです。そこに無い要素はファイル・行・桁を名指しする生成エラーになり
ます。これが目的です。認識されないハイフン付き要素はそのまま出力されても何も描画せず
何も報告しないので、`<csrf-toekn />` は今まで見えないままでしたが、今は意図した名前を
提案するコンパイルエラーになります。

ホワイトリストには2種類が入ります。フレームワークは **builtin** 要素を登録し、これは
生成時にプランステップへ書き換えられます。アプリケーションは使っている Web Components
を **passthrough** として登録します（名前、または `sl-*` のような接頭辞グロブ）。こち
らはそのまま出力され、プランステップを作りません。

`<svg>` と `<math>` の中のハイフン付きの名前は標準の外部名前空間の要素なので、ホワイ
トリストの外にあります。

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

`package` 宣言を省略できる場合もありますが、各ファイルに書いてください。`package pages` は生成ファイルがどの Go パッケージに属するかを明言し、読む側にディレクトリ名からの推測を残しません。

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
- スロットの `required` の有無が、パラメータの宣言型と食い違っている
- 対象 component が宣言していないスロットを埋めようとした
- スコープ付き style ブロックに裸の要素セレクタを書いた
- `external async` の関数を `await` の束縛以外の場所で呼んだ
- `async` なパラメータ／フィールドを `await` の束縛以外の場所で読んだ
- `await` ブロックに `fallback` 節を書かなかった
- `async` パラメータを持つ、`await` 境界に到達する、あるいは document head を持つ
  component の `@cache` に `ttl` を書いた
- slot の持ち主の `@cache` に `ttl` を書いた（保存しないため）
- `@cache` に `ttl` も `scope` も書かなかった（何も言っていないため）
- 呼び出しグラフが private 宣言に届く component に `@cache(scope: "public")` を書いた
- JavaScript や CSS が挿入の形と衝突した
  → [`<script>` と `<style>` の中の波括弧](#script-と-style-の中の波括弧)

`<script>` / `<style>` の内容で出た診断は、要素名と逃げ道を示します。そこでの波括弧はテンプレート構文よりも、書かれた内容であることのほうが多いからです。

```text
tasks.tb.html:13:65: unknown identifier name; this is inside <script> content,
where {...} is a template insertion. Write {{...}} to keep a literal brace,
insert a value with RawJavaScript or JsonForScript, or move the script to a file
under the public asset directory
```

ドキュメントシェル自身の `<head>` の中では、さらに 1 文が付きます。`<html>` の外側に宣言した `<head>` なら、同じマークアップが verbatim として読まれるからです。

```text
. A <head> declared outside <html> is a contribution, whose script and style
bodies are verbatim
```

診断が出るのはコード生成時です。テンプレートを変更したら、ビルドやテストの前に必ず `go generate ./...` を実行してください。実行するまで Go のビルドが見ているのは以前のプランで、そこにはテンプレート側で既に直した診断も含まれています。
