# htmlbind フレームワーク実装者向けガイド

これは tinybind を**土台にしてフレームワークを作る人**向けの資料です。tinybind
を使ってアプリケーションを作る人向けではありません。アプリケーション作者が触れ
ない部分 — 境界のワイヤプロトコル、レスポンスに必要なクライアントランタイムの判
定、そして自分で実装しなければならない範囲 — を扱います。

テンプレート言語、component 構文、通常のレンダリングは
[htmlbind.ja.md](htmlbind.ja.md) を先に読んでください。ここでは繰り返しません。

## モジュールの責務と実装者の責務

`htmlbind` はレスポンスボディで意図的に止まっています。`net/http` に依存せず、
ヘッダを書かず、ファイルを配信せず、ルーティングを決めません。

| 関心事 | 担当 |
| --- | --- |
| レンダープラン、エスケープ、スロット、チェーン合成 | モジュール |
| await 境界の識別子、プレースホルダ markup、完了ペイロード | モジュール |
| head 寄与のマージと重複排除 | モジュール |
| リクエストスコープの値を context に入れる | 実装者 |
| レスポンスのステータス、Content-Type、エンコーディング、フラッシュ方針 | 実装者 |
| ナビゲーション、履歴、SPA 的な挙動全般 | 実装者 |
| HTML 以外のレスポンスにおける完了の transport framing | 実装者 |

最後の行は見た目より狭い範囲です。`htmlbind.Content` が持つのは境界 ID とレンダ
リング済み HTML だけで、それ以外は何もありません。これは意図的で、ストリーミング
中のドキュメントにも、JSON ペイロードにも、フレームワークが考えた任意の形にも載せ
られるようにするためです。

## クライアントランタイムが必要かの判定

境界を開き得るものが含まれる場合にだけ、確定した境界を適用するスクリプトが要り
ます。手元の値に聞いてください。

```go
page := pages.Home(pages.HomeParams{})

if page.HasAwaitBlock() {
	// このレスポンスは境界をストリームする
}
```

`HasAwaitBlock` は `Fragment` と `Wrapper` の両方にあり、メンバを合算するチェー
ン形もあります。

```go
document := pages.BindDocument(pages.DocumentParams{Title: "Home"})
layout := pages.BindLayout(pages.LayoutParams{})
page := pages.Home(pages.HomeParams{})

if htmlbind.HasAwaitBlock([]htmlbind.Wrapper{document, layout}, page) {
	// チェーン全体で1回判定するので、ランタイムのタグも1つ
}
```

このフラグにどこまで頼れるかは、次の3つの性質が決めます。

**推移的です。** 生成時に component 呼び出しグラフを辿るので、自身が `await`
を宣言していなくても、async な component を呼ぶだけで `true` になります。

**何もレンダリングしません。** フラグは生成されたプラン上の定数で、束縛値に写さ
れるだけです。読んでも goroutine は起きず、シーケンスも消費しないので、エントリ
ポイントを選ぶ前に聞けます。

**渡した Fragment は見えません。** パラメータ構造体を経由して component に渡
した `Fragment` は数えられません。リフレクションなしに呼び出し側のパラメータ構造
体の中は覗けないからです。その Fragment は手元にあるはずなので、自分が作った値
を合算してください。

```go
sidebar := pages.Sidebar(pages.SidebarParams{})
home := pages.Home(pages.HomeParams{Sidebar: sidebar})

needsRuntime := sidebar.HasAwaitBlock() || home.HasAwaitBlock()
```

document - layout - page という通常の構成については、チェーン形のヘルパがこれを
やってくれます。

## 境界のワイヤプロトコル

プログレッシブレンダリングは、未確定の境界を fallback を抱えたプレースホルダとし
て書きます。

```html
<tb-boundary id="tb-1" style="display:contents">…fallback…</tb-boundary>
```

確定した境界は、不活性な template とマーカーの組として追記されます。

```html
<template data-tb-boundary="tb-1">…resolved…</template><tb-apply for="tb-1"></tb-apply>
```

適合するクライアントスクリプトが守るべき契約は次のとおりです。

- **template ではなくマーカーをトリガにする。** これだけは好みの問題ではありませ
  ん。防いでいる不具合は「[なぜマーカーが必要か](#なぜマーカーが必要か)」にあります
- 境界 ID はマーカーの `for` 属性から読む
- `id` が一致する要素を template の content で置換する
- 置換後にマーカーと template の両方を取り除く
- 各境界の適用は高々1回。完了はドキュメント順ではなく確定順で届く
- template やプレースホルダが見つからない場合は何もしない。切断されたレスポンス
  では、コミット済みの fallback が残らなければならない

`Content.WriteTo` は上記の markup をそのまま出力します。ナビゲーションレスポンス
の JSON に載せるなど自前で framing したい場合のために、`Content` は ID と HTML を
分けて持っています。

### なぜマーカーが必要か

HTML パーサは**開始タグ**を読んだ時点で要素を挿入します。したがって template の出
現に反応するランタイムは、中身がまだ届いていない template を読み、プレースホルダ
を空で置換し、template を削除してしまう可能性があります。結果と一緒に fallback ま
で失われます。

これは机上の話ではなく、template の開始タグが単独のネットワークチャンクに載った
ときに実際に観測された不具合です。開発中は見えません。小さい完了は1チャンクで届
き1タスクでパースされるためです。プロキシ、TLS レコード境界、圧縮エンコーダのい
ずれかがバイト列を分割したときにだけ現れます。

`<tb-apply>` はバイト列上で `</template>` より後ろにあるので、マーカーが存在する
時点で template は必ず完成しています。バイトがどう分割されていても同じです。

**規則はトリガ元についてであり、API についてではありません。** マーカーを監視す
る `MutationObserver` は適合します — マーカーは template の完成前には現れ得ないか
らです。template を監視する `MutationObserver` は適合しません。カスタム要素の
`connectedCallback` を推奨するのは、パース中に実行されインラインスクリプトと同じ
即時性が得られるためです。observer も正しく動きますが、マイクロタスク1つ分遅れま
す。

### 適合するクライアントスクリプト

現在モジュールが同梱しているランタイムを展開したものです。

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

完了チャンクはスクリプトを一切運びません。そのため nonce も `unsafe-inline` も無
しに、インラインスクリプトを禁止するポリシー下で動かせます。

### 現状の制約

現在は `RenderChainAsync` がこのランタイムをマージ済み head に自分で差し込むので、
フレームワークが自前のものに差し替えることはまだできません。`HasAwaitBlock` はそ
れを変えるための前半です — 注入より先に、判断が呼び出し側に移りました。差し替えが
入るまでは、上記プロトコルは固定として扱ってください。

なおスクリプトは*マージ済み head* に載るので、document シェルを持たないチェーン
（`head` 要素を出すものが無いチェーン）には届きません。その場合 fallback がそのま
ま最終的な内容になります。

## 初回ロード

特別な処理は要りません。シェルがマージ済み head を書き、初期パスが全ての fallback
を伴ってドキュメントをコミットし、完了が後からストリームされます。

```go
for content, err := range htmlbind.RenderChainAsync(ctx, w, wrappers, page) {
	if err != nil {
		log.Printf("boundary failed: %v", err)
		break
	}
	if _, err := content.WriteTo(w); err != nil {
		break
	}
	htmlbind.Flush(w)
}
```

このループを隠すエントリポイントは意図的に用意していません。1回のレンダリングが
いくつ境界を produce するかは事前に分からず、リクエスト毎に組み立てられるチェーン
では特にそうなので、ストリーミングするハンドラはいずれにせよシーケンスに対して書
くことになるためです。

初期パスがフラッシュした時点でステータスコードは確定します。それ以降の失敗はログ
のためのものであり、レスポンスを書き換えるためのものではありません。

## SPA 的なナビゲーション

ここが最も未完成な領域です。サポート済みの機能の説明ではなく、**利用できる部品の
説明**として読んでください。

**今日動くもの。** ナビゲーションレスポンスは、document シェルを含まないチェーン
の通常のレンダリングです。head を出すのはシェルだけなので、追加 API なしにボディ
だけが出ます。

```go
// wrapper リストに document シェルを入れないので <html>, <head>, <body> は出ない
err := htmlbind.RenderChain(w, []htmlbind.Wrapper{layout}, page)
```

そしてレンダリングがマージ*したはずの* head は、レンダリングせずに取得できます。

```go
tags := htmlbind.MergeHead([]htmlbind.Wrapper{document, layout}, page)
```

`Fragment.Head()` と `Wrapper.Head()` は、そのメンバ自身の寄与を返します。スコー
プ済みでそのまま書ける形です。

**自分で作る必要があるもの。** それを生きたドキュメントに適用する部分すべてです。
前ページの `title`、`meta`、`link`、`script` をブラウザが置き換えてくれることはあ
りません。また、ドキュメントにはアプリケーションが自分で足したタグも存在し、ナビ
ゲーションはそれを消してはいけません。したがって、自分のタグと他人のタグを区別す
る手段と、消して作り直すのではない差分適用が必要になります。前後どちらにも存在す
るスタイルシートを再取得させず、ちらつかせないためです。

**まだ設計中のもの。** マージ済み head タグに所有マーカー属性と内容由来の ID を付
け、クライアントがシリアライズされた markup ではなく同一性で差分を取れるようにす
ること。head の変更を運ぶナビゲーションレスポンスの形。そしてドキュメント寿命で一
意な境界識別子です。現在の境界 ID はレンダリング毎の連番（`tb-1`, `tb-2`, …）で、
空のドキュメントへのフルロードには十分ですが、以前の境界が残っているドキュメント
にナビゲーションが境界を挿入すると衝突します。今ナビゲーションを作るなら、これら
がドキュメント寿命で一意だと仮定せず、自分の ID を上に載せてください。

もう1点、ナビゲーション中に適用する完了ではマーカー機構がそもそも使えません。ナ
ビゲーションレスポンスのボディにパーサは走らないので、connect されるものが存在し
ないためです。新しい内容を設置した後に、境界 ID で適用してください。

## head のマージ

寄与は外側から順にマージされ、後から来た重複は落ちます。2つの component が同
じスタイルシートを宣言してもタグは1つです。同一性はマージ済み文字列の完全一致で
す。

制約を1つ把握しておいてください。`Bind` はプラン自身の寄与しか写さないので、パラ
メータ構造体を経由して component に渡された `Fragment` は、その head 寄与をマー
ジ済みドキュメント head に届けません。テンプレート内で静的に呼ばれる component
は問題ありません — コンパイラが呼び出し側のプランに畳み込みます。穴は、実行時に
パラメータ経由で渡された Fragment、つまりまさにファイル跨ぎの合成のケースに固有で
す。修正されるまでは、その形でスロットを合成するフレームワークは、それらの
Fragment の `Head()` を自分でマージしてください。

## ジェネレータとの統合

フレームワークは tinybind のものを同梱するのではなく、自前の generate コマンドを
作ります。コールレジストリで自分のラッパを登録すれば、tinybind の API ではなく自
分の API に対して discovery が働きます。

```go
calls := generator.NewCallRegistry()
if err := calls.Register(generator.ConfigBindCall(
	generator.Function("example.com/framework", "RegisterConfig"),
	generator.GenericType("config", 0),
	generator.Argument("prefix", 1),
)); err != nil {
	return err
}
options, err := calls.Options(generator.DefaultOptions())
```

生成は1回の実行につき1パッケージディレクトリです。再帰モードも複数ディレクトリ指
定もありません。したがってフレームワークが生成時に導出する識別子は、実行内の連番
に依存してはいけません。2つのディレクトリは2つのプロセスです。

`Options` はテンプレートファイルのパターン、SQL API の形、そして使わない生成フェー
ズを止めるためのフィーチャースイッチも持ちます。

なお、component のスタイルとスクリプトは現在インライン markup としてドキュメ
ント head にマージされます。public ディレクトリ配下へのファイル抽出は実装されてい
ないので、配信すべきものも設定すべきアセット URL もまだありません。キャッシュのた
めや、インラインスタイルを禁止するポリシーのために外部スタイルシートが必要なら、
現状ではフレームワーク側の担当になります。

### コンポーネントの署名を読む

tinybind のものを置き換えるのではなく、テンプレートの周りにコードを生成する場合、
コンポーネントの引数を **Go** の型で知る必要がある。そのマッピングを再実装すると
コンパイラより常に1リリース遅れることになるので、公開されている。

```go
sigs, err := htmlbind.Signatures("page.tb.html", source)
page, ok := htmlbind.Lookup(sigs, "Page")
// page.Parameters[0] == {Name: "orders", GoType: "htmlbind.Pending[[]Order]", Async: true}
```

`Generate` と同じ解析を通すので、コンパイルできないモジュールは部分的な答えを
返さず同じ診断で失敗する。[httpbind_frameworkowner.ja.md](httpbind_frameworkowner.ja.md)
のファイルシステムルータが、生成ハンドラの復号対象を決めるために読むのもこれである。

## ルーティング

ルーティングは `htmlbind` の責務ではない。このモジュールはレスポンスボディを書いて
そこで止まるので、どちらのルータもここには無い。登録を読む側と、テンプレートの
ディレクトリから登録を生成する側の両方が
[httpbind_frameworkowner.ja.md](httpbind_frameworkowner.ja.md) にある。
