# update サーフェス 利用ガイド

**対象読者:** `tinybind-go` の上にフレームワークを作り、HTML の update エンドポイントを自前のルーターと自前のブラウザランタイムに繋ぎ込む人。

ここに書くのは使い方です。なぜこの形なのかは [`httpbind_render_modes.md`](httpbind_render_modes.md)（英語）を、ワイヤ上の正確なバイト列は [`httpbind_update_wire_contract.md`](httpbind_update_wire_contract.md)（英語）を参照してください。

---

## 全体像

1 つの URL が複数の答え方をします。レンダーヘッダのないリクエストには完全なドキュメントが返るので、クローラーもランタイムを持たないブラウザも影響を受けません。ヘッダが付くと、同じハンドラが変化した領域だけを返します。マークアップとして返すか、クライアントが既に持っている静的な形を埋める値として返すかのどちらかです。どのエントリも自分のハンドラの中から呼ぶものであり、このパッケージはルートを一切マウントせず、URL を所有せず、**ヘッダもステータスも書きません**。

## このパッケージが書くもの、あなたが書くもの

書くのはバイト列です。それだけです。

このパッケージが知っていて利用者側では導出できないもの —— レスポンスがどのリクエストヘッダに依存するか、Content-Type は何か、実際にどのモードが提供されたか、ボディのダイジェストは何か —— は**計算して手渡します**。デプロイ側が決めるもの —— キャッシュポリシー、条件付きリクエストに答えるかどうか、失敗時の見え方 —— には一切触れません。

| | 担当 |
| --- | --- |
| `Vary`、`Content-Type`、レンダーのエコー、`X-Tinybind-Live`、`ETag` | ここで計算し、**書くのは利用者** |
| `Cache-Control`、304 の返却、最終的に送るステータス | 利用者 |
| ボディ | ここで書く |

ボディより先にヘッダが確定するかどうかで、2 つの形があります。

**ヘッダが先、次にボディ** —— 直接書き込むエントリとストリームするエントリ:

```go
htmlupdate.ApplyTo(update.Headers(r, wrappers, leaf), w)   // または StreamHeaders / LiveHeaders
w.Header().Set("Cache-Control", "no-store")                // 利用者側
err := update.Render(w, r, wrappers, leaf, renderOptions(r)...)
```

**答え全体を受け取って送る** —— ヘッダがレンダー結果に依存するエントリ。redraw は自身のボディをダイジェストするためです:

```go
answer, ok := update.Redraw(r, registry, renderOptions(r)...)
if ok {
    w.Header().Set("Cache-Control", "private, no-cache")
    if answer.NotModified(r) {
        htmlupdate.ApplyTo(answer.Header, w)
        w.WriteHeader(http.StatusNotModified)
        return
    }
    _, _ = answer.WriteTo(w)
    return
}
```

> **`Vary` は好みではなく正しさの制御です。** これを落としたレスポンスは、共有キャッシュからページを求めているブラウザに手渡されうるものになります。`Headers` も `Response.Header` も `Vary` を計算し、`ApplyTo` も `WriteTo` も書き出すので、省くことは「漏れ」ではなく「判断」になります —— そしてその判断は今や利用者ができます。

**`ETag` だけは自力で計算できないヘッダです。** このパッケージが組み立てたボディをダイジェストするため、利用者側で作るならコンポーネントを二度レンダーすることになります。これが上の原則の例外であり、redraw がヘッダを先に渡さず `Response` 全体を返す理由でもあります。redraw のヘッダはボディより先には存在できません。

拒否のレスポンスも `Vary` を持ちます。`404` は `Cache-Control` が一切なければヒューリスティックにキャッシュ可能なので、軸の付いていない拒否は保存され、ページへのリクエストに返されうるからです。

## セットアップ

```go
var update = htmlupdate.Options{
    Key:          []byte(os.Getenv("TB_VALIDATOR_KEY")), // validator を認証する
    ServeRuntime: true,                                  // または CallerOwnsRuntime
}

func main() {
    if err := update.Validate(); err != nil {
        log.Fatal(err) // 設定の誤りをまとめて報告する
    }
}
```

`Key` は重要です。ブラウザに公開される validator が鍵付きでなければ、エンドポイントに到達できる者はダイジェストを比較することで、その領域の内容に関する推測を確認できてしまいます。鍵をローテートすると完全なドキュメントが返るようになりますが、それがローテーションの意図した効果です。

`ServeRuntime` と `CallerOwnsRuntime` はちょうど一方だけを設定します。どちらも設定しなければコンパイルは通り、そのうえで更新が静かに止まったページを配ることになります。`Validate` がそれを拒否するのはそのためです。

## ページのレンダー

ハンドラでチェーンをレンダーし、リクエストが何を求めているかの判断は `Options.Render` に任せます:

```go
mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
    if !authorized(r) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    wrappers := []htmlbind.Wrapper{
        documentPlan.BindWrapper(documentParams{}, setChildren),
        layoutPlan.BindWrapper(layoutParams{Section: r.URL.Query().Get("section")}, setChildren),
    }
    leaf := pagePlan.Bind(pageParams{Query: r.URL.Query().Get("q")})
    htmlupdate.ApplyTo(update.Headers(r, wrappers, leaf), w)
    if err := update.Render(w, r, wrappers, leaf); err != nil {
        http.Error(w, http.StatusText(500), 500)
    }
})
```

`Bind` と `BindWrapper` は `*Plan[P]` のメソッドです。パッケージレベルの `htmlbind.Bind(plan, params)` と `htmlbind.BindWrapper(plan, params, set)` も引き続き動作し、意味も完全に同じです。これらは deprecated なラッパーとして残してあり、生成コードも手書きのコードも移行を強制されません。生成されるプランは今も関数形を書きます。自分のコードは都合のよいときに移せばよく、移さなくても構いません。

`Headers` にはレンダーが受け取るのと同じ wrapper と leaf を渡します。その合成が live 境界を持つかどうかは、合成そのものの性質だからです。何も渡さなければ live マーカーが落ち、live 境界を持つページに対して live リクエストが開かれることが二度となくなります。

レンダーのエントリはすべてオプションを取ります —— `Render`、`RenderStream`、`RenderStreamAsync`、`RenderLiveStream`。

ヘッダのアクセサはエントリの形ごとに 3 つあります。違うのは live リクエストが何に解決されるかだけですが、その違いは重要です。レスポンスは自分が何であるかを名乗らなければならず、さもないとプロキシによる差し替えが検出できなくなります。

| アクセサ | 対象 | live リクエストの解決先 |
| --- | --- | --- |
| `Headers` | `Render` | ドキュメント。そのエントリが提供するもの |
| `StreamHeaders` | `RenderStream`、`RenderStreamAsync` | ナビゲーション（終端済み） |
| `LiveHeaders` | `RenderLiveStream` | live |

`Render` はバッファリングします。`await` 境界を持つページでは `RenderStreamAsync` を使えば、遅い境界が遅らせるのはそれ自身だけになります:

```go
err := update.RenderStreamAsync(r.Context(), w, r, wrappers, leaf, renderOptions(r)...)
```

## レンダーオプションと、それを必ず渡すべき理由

フラグメントをレンダーするエントリはすべて `[]htmlbind.Option` を取ります。**どこでも同じものを渡してください。** そうしないと、コンポーネントがページの中では一つの形で、それを置き換えるレスポンスでは別の形でレンダーされます:

```go
func renderOptions(r *http.Request) []htmlbind.Option {
    return []htmlbind.Option{
        htmlbind.WithCSRFToken(session.CSRFToken(r)),
        htmlbind.WithCache(store),
        htmlbind.WithURLSchemes("http", "https", "myapp"),
    }
}
```

- **`WithCSRFToken` は実質的に必須です。** unsafe なフォームを含むコンポーネントは CSRF フィールドを出力し、トークンがなければそのレンダーは*失敗*します。空トークンのフォームではなく 500 です。セッションの背後にないレンダー（メール本文、静的エクスポート、ゴールデンテスト）には `WithoutCSRFToken()` を使ってください。
- **`WithURLSchemes`** —— これがないと、独自スキームを持つ `url` は `#tb-blocked-url` に無害化されます。
- **`WithCache`** —— これがないと `@cache` コンポーネントは毎回本体を実行します。

境界の prefix、ビルド識別子、リクエストの context は `Options` から供給されるので渡す必要はありません。利用者のオプションは最後に適用されるので、context を上書きすることは依然として可能です。

## Redraw: ブラウザが 1 領域を再要求する

公開するコンポーネントを登録します。登録がレビューポイントです。コンポーネントのパラメータの前に HTTP エンドポイントを置くことになり、そのパラメータは誰でも渡せるからです:

```go
var registry = &htmlupdate.Registry{}

func init() {
    registry.MustRegister(pages.CounterReloadable)
}
```

> 渡された値を整形するだけのコンポーネントは登録して安全です。識別子でレコードを読み込むものは、通常のハンドラとまったく同じように**自分で所有権を検査しなければなりません**。

ページハンドラの中で分岐させれば、redraw はそのハンドラ自身の検査を引き継ぎます:

```go
mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
    if !authorized(r) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    // ページと、その上のコンポーネントの redraw はこの URL を共有するので、
    // このリクエストがどちらに転んでもキャッシュは redraw の軸をキーに含める必要がある。
    htmlupdate.ApplyTo(update.RedrawHeaders(r), w)
    if answer, ok := update.Redraw(r, registry, renderOptions(r)...); ok {
        w.Header().Set("Cache-Control", "private, no-cache")
        _, _ = answer.WriteTo(w)
        return // redraw であり、上の検査を引き継いでいる
    }
    // 通常のページレンダー
})
```

`private, no-cache` は、かつてこのパッケージが全員に対して選んでいた値です。redraw は通常ユーザーごとの内容をレンダーするので private、そして `no-store` ではなく `no-cache` なのは、`no-store` が `ETag` の存在意義である条件付きリクエストを禁じてしまうからです。今はデフォルトではなく提案です。

`Registry.RequiredHead()` を起動時にドキュメントシェルへ入れてください。redraw はこのエンドポイントがレンダーしたことのないページの領域を書き換えるので、それを必要とするマークアップより先にスタイルシートを設置することができません:

```go
shell := documentParams{Head: registry.RequiredHead()}
```

## Action: 状態を変えて再描画する 1 往復

```go
func addToCart(w http.ResponseWriter, r *http.Request) {
    count, err := cart.Add(r.Context(), itemID)
    if err != nil {
        httpbind.WriteError(w, r, err)
        return
    }
    if update.WantsUpdate(r) {
        answer, err := update.WriteUpdate(r, []htmlupdate.Update{
            htmlupdate.Replace("cart", CartBadge(CartBadgeParams{ID: "cart", Count: count})),
        }, renderOptions(r)...)
        if err != nil {
            httpbind.WriteError(w, r, err)
            return
        }
        w.Header().Set("Cache-Control", "no-store") // action のレスポンスは決してキャッシュ可能ではない
        _, _ = answer.WriteTo(w)
        return
    }
    httpbind.Write(w, r, result) // エンドポイント本来の JSON
}
```

`WriteUpdateStatus` はステータスを明示できる同等の関数で、却下された送信に対して 422 **と**その理由を示す領域の両方を返せます。前述の CSRF オプションが最も効いてくるのがこのケースです。書き換えられる領域がフォームそのものだからです。

## Sequence: マークアップの代わりに値を送る

フラグメントの静的な半分 —— リテラルのテキスト —— はどのレンダーでも同一です。リクエストヘッダを 1 つ設定すれば、それは流れなくなります:

```
X-Tinybind-Sequences: 1
```

以降、オペレーションはアドレスとそれを埋める値を運びます。マークアップより小さくなる場合に限られます:

```json
{"kind":"replace","id":"panel","seq":"Yb3_x…","values":["Inbox","30","data-tb-id","r0", …]}
```

アドレスの背後にあるツリーの要求には、redraw と同じ形で答えます:

```go
mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
    if answer, ok := update.Sequence(r); ok {
        w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
        _, _ = answer.WriteTo(w)
        return
    }
    // redraw、その後に通常のページレンダー
})
```

sequence はここで唯一**ユーザーごとではない**レスポンスです。リクエストではなくテンプレートから導出されるためで、だからこそ `public, max-age=31536000, immutable` を返せる唯一のものであり、ユーザーを越え、ビルドを越えて共有キャッシュに保持されます。自身の内容のダイジェストでアドレス付けされるので、テンプレートを編集すると古いアドレスに新しいボディが乗るのではなく新しいアドレスが生まれ、無効化すべきものは何もありません。このプロセスがレンダーしたことのないアドレスには 404 を返し、クライアントは代わりにマークアップを要求します。

**割に合うかどうかは形次第です。** 100 行のパネルで計測したところ、値はマークアップ全体の 40% でしたが、小さなフラグメントではアドレス＋値のほうがマークアップ*より*高くつきます。選択がフラグメント単位で行われるのはそのためであり、この分割が損失にならないのもそのためです。

## クライアントがすべきこと

レコードの形では強制できない 4 つの規則です。

**1. `If-None-Match` を自分で設定しないこと。** 通常の fetch を発行し、再検証はブラウザに任せてください。ブラウザは自身のストアから完全なボディを再構成するので、head、live マーカー、manifest が必ず届きます。自分で条件付きリクエストを発行すると、ボディのない 304 が返り、3 つとも失われます。

**2. 同一ドキュメント内の履歴ナビゲーションでは、いま画面に出ている DOM を記述する manifest を送ること。** 移動先のエントリに保存されているものではありません。A → B → A と辿り、画面には B が出ている状態で A の manifest を返すと、すべての境界が等しいと判定され、何も送られなくなります。

**3. 追い越されたリクエストを abort すること。** あるインスタンスに対して次の redraw を発行する前に、保留中のものを。ナビゲーションのレコードを適用する前には、保留中のすべての redraw を。abort したリクエストのレスポンスは、バイトが届いていても適用されません。これを怠ると、キーストロークごとに redraw する検索ボックスで、長いクエリの入力欄の下に短いクエリの結果が残ります。

**4. live ページへのナビゲーションでは:** 出ていくページの live リクエストを abort し、ナビゲーションのレコードを適用し、*それから*新しい live リクエストを開きます。この順序で。

## レスポンスの適用

レコードの形は 3 つ、適用経路は 1 つです。

**`replace`** —— 領域のマークアップを差し替えます。入れ子の境界がある位置には**穴**を持ち、`boundaries` がその名前を挙げます:

```json
{"kind":"replace","id":"panel","html":"<section …><template data-tb-id=\"r0\"></template></section>","boundaries":["r0","r1"]}
```

このレスポンスの中に**同じ id のオペレーションがある**穴は、それで埋めます。**ない**穴は既に手元にある領域なので、差し替えの前にその live なノードを取り出し、穴へ移してください。中にあるフォーカス、フォームの値、再生中のメディアが保たれるのはこの操作によります。

穴は `<template>` です。HTML パーサーが書かれた位置に保ち、かつ描画しない唯一の要素だからです。未知の要素では代用できません。テーブルのコンテキストではパーサーがそれをテーブルの直前へ foster-parent するので、テーブルの行が残した穴はすべてテーブルの外に出てしまい、それを埋める行がページ上に散らばります。フラグメントを `template` 要素の中でパースすべき理由も同じです。裸の `<tr>` は他の場所でパースするとタグを失います。

**`children`** —— 領域自身のマークアップは変わらず、入れ子の境界がこの順序でこれらになった、という意味です。マークアップは一切ありません:

```json
{"kind":"children","id":"panel","boundaries":["r0","r1","r2"]}
```

リストが保持するものは保持し、移動したものは移動させ、省かれたものは捨て、独自のオペレーションとして届いたものは埋めます。リストが「行が 1 つ追加された」ことを、リスト全体を再送せずに伝える方法です。

**`html` の有無ではなく `kind` で分岐してください。** `children` レコードは `html` を持たず、このパッケージのストリーム経路はどれも一度はこれを間違えました。

**`seq` + `values`** —— 同じフラグメントを分割したものです。ツリーを歩きながら、穴ごとに 1 値、条件分岐ごとに 1 値（どちらの枝か）、ループごとに 1 値（何回か）、コンポーネント呼び出しごとに 1 値（境界かインラインか）を消費します。連結して一度だけパースします。

**エスケープがこのモジュールの外に出ることはありません。** 値は HTML に書き込まれるのと同じ形で、既にエスケープされた状態で届きます。利用者は連結してパースするだけで、エスケーパーを適用することも値を判断することもありません。特に URL スキームの許可リストはここに留まります。

## 失敗

生成できなかった update は `application/problem+json` で答えます:

```json
{"type":"about:blank","title":"Bad Request","status":400,
 "detail":"invalid redraw arguments","code":"invalid_arguments",
 "errors":[{"field":"page","location":"query","message":"is not an integer"}]}
```

**判別子はメディアタイプです。** `application/json` は適用すべき update です。2xx でないものも含みます。バリデーションエラーを運ぶ 422 は*成功した* update だからです。`application/problem+json` は update を生成しなかったリクエストです。何も適用せずフォールバックしてください。

`code` は失敗の種別を運びます。古くなったページとレンダーの失敗は、プロキシから見れば同じステータスですが、オンコールの担当者から見れば別の事象だからです。

拒否は `Failure` フィールドが設定された通常の `Response` として返るので、種別を読んで独自のエラーページを送ることもできます。`Options.OnFailure` は答えるのではなく観測します。答えを変えるかどうかにかかわらず、すべての拒否に対して残したいログ行と span のためのものです。

## ヘッダ

| ヘッダ | 方向 | 意味 |
| --- | --- | --- |
| `X-Tinybind-Render` | request | モード: `navigation`、`live`、`redraw`、`action`、`sequence`。レスポンスにエコーされる |
| `X-Tinybind-Build` | request | ページをレンダーしたビルド。不一致なら完全なドキュメントを返す |
| `X-Tinybind-Manifest` | request | `id:frame:children:parent` のエントリをカンマ区切りで |
| `X-Tinybind-Kind` / `-Instance` | request | redraw がどのコンポーネントを指すか |
| `X-Tinybind-Sequences` | request | このクライアントは sequence ツリーを歩ける |
| `X-Tinybind-Sequence-Address` | request | どの sequence を返すか |
| `X-Tinybind-Live` | response | この合成は live 境界を持つ。live リクエストを開くこと |

ストリーム上のオペレーションレコードは manifest のエントリ全体 —— `frame`、`children`、`parent` —— を運びます。3 つとも保存してください。`children` は次の次のリクエストでリストが並べ替えられたように見えるのを防ぐものであり、`parent` は削除を、最も外側を差し替えるフォールバックではなく、生存者を報告する境界に帰属させるためのものです。

prefix は `Options.HeaderPrefix` です。すべてがそこから合成されるので、名前を変えてもランタイムを再ビルドする必要はありません。

## このパッケージがしないこと

- **ヘッダやステータスを書くこと。** 両方を計算して手渡します。キャッシュポリシーはライブラリではなくデプロイに属するものであり、1 箇所で書かれたヘッダは 1 箇所まで辿れるヘッダです。
- **ワイヤのバージョンを選ぶこと。** 出力される形の隣に独自のフィールドを足してください。このパッケージが扱う互換性の軸はビルド識別子だけです。
- **ブラウザランタイムを配ること。** `CallerOwnsRuntime` を設定して `RuntimeSource` を自前のアセットにマージするか、ワイヤ契約に対して自分で書いてください。
- **ルートをマウントすること。** どのエントリも利用者が呼ぶものです。redraw や sequence が提供される URL は利用者のものであり、だからこそ 2 本目のパスパターンを 1 本目と同期させ続ける必要がなく、ページハンドラの認可をそのまま引き継げます。
