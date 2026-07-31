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
| 完了の framing 全般と、それを適用するクライアントスクリプト | 実装者 |
| live 接続をいつ開くか、どう張り直すか、張り直せないときにどうするか | 実装者 |

最後の行は見た目より広い範囲です。`htmlbind.Content` が持つのは境界 ID とレンダ
リング済み HTML だけで、それ以外は何もありません。これは意図的で、ストリーミング
中のドキュメントにも、JSON ペイロードにも、フレームワークが考えた任意の形にも載せ
られるようにするためです。モジュールはどの経路でも `<script>` を書かず、マージ済み
head にも何も差し込みません。完了がワイヤ上でどう見えるかは、配信するランタイムと
セットで一度だけ決める設計判断です。

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

この要素と ID まではモジュールの担当です。その先は実装者の担当で、確定した境界は
レンダリング済みフラグメントと、それが置き換わるプレースホルダの ID を持つ
`Content` として届きます。`Content.WriteTo` が書くのはフラグメント本体だけです。

したがって以下はモジュールが強制するものではなく推奨です。ただしモジュールはこの
形を前提に設計されており、後述のマーカーの規則は本質的です。確定した境界は、不活
性な template とマーカーの組として追記します。

```html
<template data-tb-boundary="tb-1">…resolved…</template><tb-apply for="tb-1"></tb-apply>
```

Go 側はこうなります。

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

適合するクライアントスクリプトが守るべき契約は次のとおりです。

- **template ではなくマーカーをトリガにする。** これだけは好みの問題ではありませ
  ん。防いでいる不具合は「[なぜマーカーが必要か](#なぜマーカーが必要か)」にあります
- 境界 ID はマーカーの `for` 属性から読む
- `id` が一致する要素を template の content で置換する
- 置換後にマーカーと template の両方を取り除く
- 各境界の適用は高々1回。完了はドキュメント順ではなく確定順で届く
- template やプレースホルダが見つからない場合は何もしない。切断されたレスポンス
  では、コミット済みの fallback が残らなければならない

transport が違うなら形を変えて構いません（ナビゲーションレスポンスの JSON に載せる
場合、マーカーを起動するパーサがそもそも走りません）。ただし「フラグメントを運ぶ
バイト列が完成する前に何も適用しない」という規則だけは持ち込んでください。

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

上記の形に対するリファレンス実装です。

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

完了チャンクにインライン化せず、すでに配信しているバンドルに入れてください。そう
すれば完了チャンクはスクリプトを一切運ばず、nonce も `unsafe-inline` も無しに、
インラインスクリプトを禁止するポリシー下で動かせます。

### スクリプトをページに載せる

これも実装者の担当です。以前は `RenderChainAsync` がこのランタイムをマージ済み
head に自分で差し込んでいましたが、現在は差し込みません。判断と注入が同じ場所に
揃ったということです。判断は `HasAwaitBlock` で行い、注入は document シェルが出す
`script` タグ — シェル component の head 寄与、あるいはそのテンプレートのリテラル
markup — で行います。

スクリプトが読まれなかったレスポンスは壊れているのではなく、改善されないだけです。
どのプレースホルダもコミット済みの fallback を保ったままで、これは JavaScript 無し
のクライアントに見えるものと同じです。

## 初回ロード

特別な処理は要りません。シェルがマージ済み head と自前のランタイムタグを書き、初期
パスが全ての fallback を伴ってドキュメントをコミットし、完了が後からストリームされ
ます。

```go
for content, err := range htmlbind.RenderChainAsync(ctx, w, wrappers, page) {
	if err != nil {
		log.Printf("boundary failed: %v", err)
		break
	}
	if err := writeCompletion(w, content); err != nil {
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

## live な境界

`live` なソースを束縛した境界は、ソースが値を yield するたびに同じ領域を置き換え
ます。ワイヤプロトコルで変わる点が 2 つ、実装者が決める点が 1 つあります。

まず framing が変わります。初回レスポンス上の完了チャンクは、HTML パーサが消費
している最中のマークアップです。template とマーカーという形が存在する理由がそこに
あります。ドキュメントが完成した後の配信を読んでいるパーサはいないので、マーカー
規則には発火する相手がおらず、あの framing は何も買っていません。代わりにレコード
を送ります。

```go
_, err := w.Write(content.AppendJSON(nil))
```

出力は `{"id":"tb-1","html":"…"}` で、断片は JSON 文字列としてもスクリプト文脈と
しても安全にエスケープされます。append 形にしてあるのは、2 つ目のバッファを使わず
に framing を前置できるようにするためです（イベントストリームなら
`content.AppendJSON([]byte("data: "))`）。レコードの外側の framing は、読む側の
クライアントと揃える必要があるので実装者のものです。

次に識別子が変わります。境界 ID が表すのは確保された順ではなく**描画ツリー上の
位置**です。境界の中の境界は `tb-1-1` になり、live な境界のサブツリーは配信ごとに
新しい ID を作るのではなく、毎回同じ ID を配ります。したがってクライアントは同じ
要素を繰り返し置き換えることになり、長寿命の購読が「二度と埋まらないプレース
ホルダ」を溜め込むことはありません。ID が配り直される前に、超過した配信のネスト
した処理はランタイムが打ち切ります。遅いネスト境界が、置き換え後のプレース
ホルダに着弾できないようにするためです。

実装者が決めるのは、live な接続を張り直したときの振る舞いです。同じページを再実行
すれば同じ ID になるので、**クライアントが持っていない ID が来たということは構造
そのものが変わった**ことを意味します。監視中のダッシュボードにパネルが増えた、と
いった場合です。これを正しく取り込むには、クライアントが描いていないドキュメント
の中で新しい境界の位置を決める必要があり、それは再接続の問題ではなくナビゲーション
の問題です。近似でやればパネルは変な場所に出ます。接続を止めて、リロードを促して
ください。素の `alert()` でも最初の実装としては筋が通ります。稀なケースであり、
画面が間違っているほうが、無粋であるより悪いからです。

このクライアントを書く前に知っておく挙動が 2 つあります。

`RenderAsync` 上の live な境界は、1 回配信を取って購読を解除します。live な領域を
含む初回ロードでも、ローディング状態ではなく実データを見せたうえでレスポンスが
終わります。ただし「この後もまだ来る」ことは初回レスポンスからは分かりません。
それを答えるのが `htmlbind.HasLiveBlock` で、同じチェーンに対して、描画を始める
前に聞けます。

```go
if htmlbind.HasLiveBlock(wrappers, page) {
	// この画面は変わり続ける。クライアントは live 接続を開くべき
}
```

聞いておけば、二度と変わらない画面が無駄なリクエストを払うことはありません。
`HasAwaitBlock` の部分集合で、そちらは「そもそも境界を適用するランタイムが必要か」
という別の問いのままです。

もう 1 つ、静かなソースがレスポンスを開いたままにすることはできません。
`WithAsyncTimeout` は、応答を返す義務のあるエントリで「境界が何も出していない
時間」を打ち切ります。時間切れは commit 済みの fallback を残し、`recover` は描き
ません。まだ言うことがないソースは、失敗したわけではないからです。
`RenderChainLive` にこの打ち切りはありません。

## recover を持たない境界が失敗したとき

`recover` 節を宣言していない `await` ブロックの束縛が失敗すると、シーケンスは
`*htmlbind.UnrecoveredError` を yield して終わります。この error はコミット済み
プレースホルダの `BoundaryID` と、元の Go error を持ちます。

テンプレート側にこの失敗を置く場所がないので、失敗は境界を抜けてこちらに来ます。
画面に残っているのは「読み込み中…」のプレースホルダで、それを置き換えるものはもう
来ません。**ドキュメント全体を差し替えてエラーを表示してください。** ページの一部
だけを失敗として見せたいなら、テンプレートが `recover` を書きます。書かなかった
ブロックについては、どこがどれだけ欠けたか作者が想定していない画面を取り繕って操作
させるより、このページは失敗したという1つの事実を返す方が正しい。シーケンスが終わ
る以上、他に未確定の境界が残っていればそれも永久に fallback のままなので、なおさら
です。

```go
for content, err := range htmlbind.RenderChainAsync(ctx, w, wrappers, page) {
	if err != nil {
		var unrecovered *htmlbind.UnrecoveredError
		if errors.As(err, &unrecovered) {
			log.Printf("boundary %s failed: %v", unrecovered.BoundaryID, unrecovered.Err)
		} else {
			log.Printf("render failed: %v", err)
		}
		writeFailureScreen(w) // 初期パスはコミット済み。書き換えではなく差し替え
		htmlbind.Flush(w)
		break
	}
	if err := writeCompletion(w, content); err != nil {
		break
	}
	htmlbind.Flush(w)
}
```

`writeFailureScreen` が書くのは、完了の framing と同じくフレームワークが決めた形で
す。マーカー1つで足ります。

```html
<tb-failed></tb-failed>
```

```js
customElements.define("tb-failed", class extends HTMLElement {
	connectedCallback() {
		this.remove();
		document.body.replaceChildren(failureScreen());
	}
});
```

初期パスがフラッシュした時点でステータスコードは確定しているので、これはレスポンス
の書き換えではなく、画面の差し替えです。切断されたレスポンスにはこのマーカーも届か
ないので、そのときは何も起きず、コミット済みの fallback が残ります。既存の規則どお
りです。

エラー画面の文言はサーバの error から作らないでください。`UnrecoveredError.Err`
も `WithErrorReporter` が受け取るのも生の Go error で、そのままページに載せれば
サーバ内部が漏れます。コードやメッセージを見せたいなら `PublicError` の投影だけを
マーカーの属性に載せてください。なおレポータは境界ごとの goroutine から並行に呼ば
れるので、複数の失敗を集約するなら自分でロックが要ります。

### 同期エントリ

`Render` と `RenderChain` も同じ失敗で `*UnrecoveredError` を返します。こちらは
fallback を書かずに返るので、完成したように見えて永久に解決しないローディング表示
を含んだドキュメントは出来上がりません。

見返りは、まだ何もステータスを確定していないことです。バッファに描いていれば、その
バッファを捨ててエラーステータスを返せます。`http.ResponseWriter` に直接描いている
と、失敗した境界より前のバイトは既に出ています。エラーレスポンスに切り替えたい
レンダリング — ナビゲーションレスポンスや、ストリームしないページ — はバッファ経由
にしてください。

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
返さず同じ診断で失敗する。[httpbind_discovered_router.ja.md](httpbind_discovered_router.ja.md)
のファイルシステムルータが、生成ハンドラの復号対象を決めるために読むのもこれである。

### サーバーアクションを解決する

テンプレートは URL ではなく Go のハンドラを名指しできる。

```html
<button server-action="Rename" data-target="#name">rename</button>
```

コンパイラはこれを自力で下げられない。URL はハンドラがどこにマウントされているかに
依存し、このモジュールはルーティングについて何も知らないからだ。だから解決は、
実装者を真ん中に挟んだ2つのパスになる。

```go
refs, err := htmlbind.ActionRefs("page.tb.html", source)
// refs[0] == {Component: "Page", Handler: "Rename", Element: "button", Pos: ...}

out, err := htmlbind.Generate("page.tb.html", source, htmlbind.GenerateOptions{
	Package:          "id_",
	ServerActions:    map[string]string{"Rename": "/_action/00369cf962b6/Rename"},
	ServerActionAttr: "hx-post",   // 任意。既定は data-tb-action
})
```

`ActionRefs` はモジュールが参照している名前を、診断がテンプレートを引用できるよう
位置つきで報告する。実装者はそれをテンプレートが属するパッケージに対して解決し、
答えを `ServerActions` で返す。解決しなかった参照はコンパイルエラーになる。黙って
死んだ要素が出ることはない。

マップが持たない名前には `ServerActionResolver` が答える。列挙するよりオンデマンドで
解決したい名前のための口である。

```go
	ServerActionResolver: func(name string) (string, bool) { ... },
```

マップがリゾルバより優先されるので、リゾルバを足しても既に供給した名前が別の宛先に
向くことはない。設定した時点から、未解決の診断は試した両方の供給元を名指しする。

下げ方は意図的に薄い。`server-action` は URL を運ぶ属性1つになり、その要素の
それ以外の属性は読まれずに残る。だから `data-target` や `hx-swap` の意味は実装者の
クライアントランタイムが決められる。`ServerActionAttr` があるのは、生成された
アクションが tinybind のものではなく既に使っているライブラリを動かせるようにする
ためである。

`GenerateOptions.ContextExternals` も同じ形で、こちらが前例として見る価値がある。
どちらも呼び出し側しか決められないテンプレートの事実であり、パスの間に Go の
パッケージを読んで解決される。

探索型ルータはこれらを全部やってくれる。ハッシュ、エンドポイントのパス、どの
ハンドラが公開されるか —— それらの導出は
[httpbind_discovered_router.ja.md](httpbind_discovered_router.ja.md) にある。

## ルーティング

ルーティングは `htmlbind` の責務ではない。このモジュールはレスポンスボディを書いて
そこで止まるので、どちらのルータもここには無い。登録を読む側は
[httpbind_frameworkowner.ja.md](httpbind_frameworkowner.ja.md) に、テンプレートの
ディレクトリから登録を生成する側は
[httpbind_discovered_router.ja.md](httpbind_discovered_router.ja.md) にある。
