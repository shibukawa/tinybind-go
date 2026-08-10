# fasthttp バックエンド 利用ガイド

fasthttp のコードは書きません。[httpbind.ja.md](httpbind.ja.md) にあるとおりの
`net/http` ハンドラーを書くと、生成がそこから fasthttp 版を導出します。1つの
パッケージから2つのビルドが出て、build tag で切り替わり、両方ともコンパイラが
検証します。

ただしそれが成り立つのは、writer と request のすべての使われ方を導出が説明できる
間だけです。たいていは説明できます。できなかった場合は生成が止まり、その行を
名指しします。書き換えられなかったハンドラーを黙って吸収するアダプタは無いから
です。自分のハンドラーがどちら側に落ちるかを知ることが最初の作業で、それはこの
ガイドの最後の節で扱います。

## 手に入るもの

- `Bind`、`Write`、`WriteStatus`、`WriteError` とリクエストアクセサ群が、
  `*fasthttp.RequestCtx` 上に、同じ名前で
- 同じモデルから生成されたバインダーとライター。fasthttp ランタイムに登録済み
- 書き換えられたハンドラー。2つのトランスポート引数が1つの context に畳まれる
- fasthttp ルータへのルート登録。元になるのは discovery がすでに読んでいる
  `net/http` の登録
- 同じ OpenAPI 文書。これはフィールド計画から出るもので、トランスポートには
  依存しません
- 部分更新の面のすべて。ストリーミングとライブの描画を含みます。挙動が一点だけ
  異なるので[後述](#fasthttp-での部分更新)します

保守するソースは変わりません。書く形は1つで、それは net/http の形です。

## 生成の前に —— タグ境界がどこに落ちるか

build tag はファイル単位でしか効きません。この一点がファイル構成を決めます。

ハンドラーは fasthttp タグの下では置き換わるので、それを収めたファイルはその
ビルドから除外されます。同じファイルにある他のものも一緒に消えます。両方の
ビルドが必要とする型宣言も含めて、です。トランスポートハンドラーは専用ファイルに
置いてください。

```
api/
├── models.go      // タグなし: リクエスト・レスポンス構造体
├── handlers.go    //go:build !fasthttp —— ハンドラーと ServeMux の配線
└── service.go     // タグなし: writer も request も出てこないもの
```

`handlers.go` の `!fasthttp` は自分で書きます。生成は付けません。ただし構成の
検査はして、両方のビルドが必要とする宣言とハンドラーが同居しているファイルを
警告します。

2度書くのは配線だけです。`handlers.go` が `ServeMux` を組み、タグ付きの小さな
ファイルが fasthttp サーバーを組む。その間にあるものは共有か、導出されたものかの
どちらかです。

## 生成する

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir ./api -backend fasthttp
```

5つのファイルが出ます。

| ファイル | 制約 | 内容 |
| --- | --- | --- |
| `tinybind_gen.go` | `!fasthttp` | net/http のバインダーとライター |
| `tinybind_fasthttp_gen.go` | `fasthttp` | fasthttp のバインダーとライター |
| `tinybind_transport_gen.go` | `fasthttp` | 導出されたハンドラー |
| `tinybind_routes_gen.go` | `fasthttp` | ルート登録 |
| `tinybind_openapi_gen.go` | なし | 変わりません |

2つのバインダーファイルは同じモデルを別々のランタイムに登録します。だから片方が
除外されていないともう片方はコンパイルできません。`-backend` を付けなければ何も
起きず、このフラグが存在する前とバイト単位で同じ出力になります。

## 動かす

```go
//go:build fasthttp

package main

import (
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/fasthttprouter"

	_ "github.com/shibukawa/tinygodriver/netdev" // TinyGo のソケット層。ホスト Go では no-op

	"example.com/app/api"
)

func main() {
	r := router.New()
	api.RegisterRoutes(r)
	_ = fasthttp.ListenAndServe(":8080", r.Handler)
}
```

```bash
go build -tags fasthttp ./...
```

ルータは tinygodriver が fasthttp フォークと並べて持っているフォークです。それで
なければなりません。フォークの `RequestCtx` を取るハンドラーは
`valyala/fasthttp` の `RequestCtx` を取るハンドラーではないので、上流のルータは
生成コードを受け付けません。名前付きパラメータはどちらも `{name}` と綴るので、
ルートパターンはそのまま移ります。catch-all の `{rest...}` だけが `{rest:*}` に
なります。

## 知っておくべき差が3つ

**値はリクエストからコピーして返されます。** `RequestCtx` とそこから辿れるバイト
スライスはプールされ、ハンドラーが戻った時点で再利用されます。`Bind` が返すものは
すべてコピーされていて、パースした JSON 文書も含みます。だから束ねた値はその後も
有効です。ただし context をハンドラーの外へ持ち出すとこの保証は壊れます。それを
握った goroutine は、次にそのスロットを占めたリクエストを読みます。

**ストリーミングは反転します。** `WriteStream` は両方のトランスポートでコール
バックを取ります。

```go
httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error {
	if err := s.Write(ChatEvent{Type: "delta", Delta: "hi"}); err != nil {
		return err
	}
	return s.Write(ChatEvent{Type: "done"})
})
```

fasthttp ではこのコールバックがハンドラーの復帰後に走るので、その中で起きた
エラーはハンドラーのコードへ戻る道がありません。だから両方のトランスポートが
`SetStreamErrorHandler` で登録したハンドラーへ渡します。ストリームを閉じるのも
両方ともランタイム側です。これが、コールバックが途中で失敗したときも JSON array
文書が閉じている理由です。保持型のエントリは deprecated ではなく削除しました。
コンパイルが通るまま残っていると、fasthttp 側に対応物のない呼び出し箇所が
ビルド時ではなくデプロイ時に見つかることになるからです。

**WebSocket も同じ形で反転し、そしてこちらのほうが安く済みます。** `WebSocket` は
両方のトランスポートでコールバックを取り、その中身は1つのソースです。

```go
_ = httpbind.WebSocket(w, r, func(s *httpbind.Socket[ClientMsg, ServerMsg]) error {
	for {
		in, err := s.Read()
		if err != nil {
			return err
		}
		if err := s.Write(ServerMsg{Type: "message", Text: in.Text}); err != nil {
			return err
		}
	}
})
```

戻り値はハンドシェイクのエラーだけです。コールバックが返したものは 101 の後なので
`SetStreamErrorHandler` へ回ります。ストリームと同じくコールバックはハンドラーより
長く生きるので、コンテキストを読んではいけません。先に取り出してください。

fasthttp が節約するのは、その下の層です。`RequestCtx.Hijack` は同期的な受け渡しな
ので、アップグレードは TinyGo でも手当てなしに動きます。`net/http` バックエンドの
ほうは前段に `tinygodriver/httpserver` が要ります。TinyGo 自身のサーバーはそもそも
アップグレードを完了できないからです。fasthttp はコールバックが返ると接続を閉じま
すが、これはコールバック形の契約そのものなので `KeepHijackedConns` は off のままに
します。

**使えなくなる機能があります。** fasthttp は HTTP/2 を実装していません。TinyGo の
下では TLS 終端もできないので、前段に置いてください。TinyGo はパッケージが
コンパイルできるという意味でのみサポートしていて、サイズやスループットの約束は
ありません。実際、そのツールチェーンでは fasthttp フォークのほうが `net/http`
より大きくなります。

## 何が拒否されるかを先に知る

踏み切る前に訊けます。このレポートは何も書かず、exit 0 で終わります。

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate \
    -dir ./api -backend fasthttp -transport-report
```

```
handlers.go:31:2: createUser passes r to otel.Attach, whose transport arguments are
  undeclared; remedy: move the call behind a function taking neither the writer nor
  the request, or register it as a call pattern declaring its transport slots (unknown_call)
handlers.go:78:1: listUsers calls renderError, which is not transformable;
  handlers.go:72:6 reads r.URL, which no rewrite covers; remedy: fix the refusal
  reported below this one (inherited)
3 handler(s) would be refused by the fasthttp backend
```

各行の末尾が分類で、位置は宣言ではなく**該当箇所**を指します。継承された拒否は、
実際の原因になった行まで各ホップを名指しします。

導入は all-or-nothing なので、総額が一度に見えることが普通より重要です。拒否は
固まる傾向もあります。共有のエラーヘルパー1つがパッケージの大半の原因になって
いて、それを直すと呼び出していたハンドラーが全部通る、というのはよくある形です。

ハンドラーが受理されるのは、writer と request のすべての出現がリライタの知って
いる形であるときです。ランタイム呼び出しの引数、同じく書き換え対象になっている
関数の引数、そして `r.Context()` —— `RequestCtx` が `context.Context` を満たすので
context に対応します。`_` への代入も問題ありません。それ以外は拒否されます。

| 種別 | 意味 |
| --- | --- |
| `unknown_call` | 値がパッケージ外の関数に渡る。トレーシング、メトリクス、セッションの形 |
| `unknown_selector` | 書き換え表に無いフィールドやメソッド。`r.RemoteAddr` など |
| `escapes` | 代入、保存、返却、クロージャによる捕捉、アドレス取得 |
| `type_assertion` | `w.(http.Flusher)` の類。代わりに `WriteStream` を使う |
| `inherited` | 呼び先が拒否されただけ。チェーンが実際の該当箇所を名指しする |

どの拒否にも、それを解消する remedy が付きます。

## fasthttp での部分更新

更新の面は、トランスポートに何を要求するかで二つに割れます。そして大きいほうは
そのまま移ります。

リクエストを読んで `Response` を返すものはすべて動きます。アクションの2つ、
再描画、シーケンス、`Negotiate`、CSRF の読み取り、ヘッダー計算のすべてです。
ハンドラーは同じハンドラーのままで、`options.WantsUpdate(r)` が
`options.WantsUpdate(ctx)` になるだけ。分岐の構造は動きません。

```go
func addToCart(w http.ResponseWriter, r *http.Request) {
    if !options.WantsUpdate(r) {
        htmlupdate.Redirect(w, r, "/cart", http.StatusSeeOther)
        return
    }
    answer, err := options.WriteUpdate(r, updates)
    if err != nil {
        httpbind.WriteError(w, r, err)
        return
    }
    _, _ = answer.WriteTo(w)
}
```

`http.Redirect` ではなく `htmlupdate.Redirect` を使ってください。net/http では
標準ライブラリにそのまま委譲する同じ呼び出しですが、fasthttp のリダイレクトは
コンテキストのメソッドで、関数呼び出しをメソッド呼び出しに変える書き換えは
transform の守備範囲外です。`http.Redirect` を呼ぶハンドラーは名指しで拒否
されます。

配置について2点あります。

`Options` の値はハンドラーの中で組み立てるか、タグ付きのファイル対で宣言して
ください。これは各バックエンドが再宣言する唯一の型で、パッケージレベルの `var`
は関数ではなく宣言なので transform は書き換えず、それが置かれたファイルはタグに
除外されます。

`Registry` にその問題はありません。生成されるコンポーネント登録も同様です。
`Registry`、`Reloadable`、`Update`、`Failure` は両バックエンドで同一の型なので、
それらを組み立てるヘルパーはタグなしのファイルに置けば両方からコンパイル
されます。

### ストリーム系はコールバックを取ります

`OpenStream` と `OpenLiveStream` は削除しました。代わりが `WriteStream` と
`WriteLiveStream` で、ストリームを返すのではなくプロデューサを受け取ります。

```go
options.WriteStream(w, r, head, func(stream *htmlupdate.DeltaStream) error {
    stream.Replace("feed", markup, entry)
    return nil
})
```

理由は fasthttp です。ストリーミングボディはハンドラーが**戻った後**に走る
コールバックから書かれるので、文をまたいで保持するストリームはあちらに翻訳
できません。この形から落ちてくる利点が両バックエンドで二つあります。プロデューサ
が成功したかどうかに関わらずエントリがストリームを閉じるので、`Close` の
書き忘れが切り詰められたレスポンスを送ることはもうありません。そして返した
エラーは in-band で報告された上で `SetStreamErrorHandler` に届き、捨てられません。

`Render`、`RenderStream`、`RenderStreamAsync`、`RenderLiveStream` の署名は
そのままです。それぞれがリクエストから読むものはすべて最初のレコードより前に
読まれるので、そこまでの失敗はステータスに変換できる通常のエラーのままです。
その後はステータスが確定しているので、失敗はターミネータに乗ります。

一点だけ実挙動が違うので、ライブ配信に依存する前に知っておいてください。
**fasthttp にリクエスト単位のキャンセルはありません**。`Done` チャネルが閉じる
のはサーバのシャットダウン時だけです。net/http ではクライアント切断でリクエスト
コンテキストが cancel されるためライブストリームは即座に終わりますが、fasthttp
では切断はレコードの書き込み失敗で気づくので、終了は次の配信時になります。以後
一度も配信しない購読はサーバが止まるまでリソースを保持します。上限が必要なら
それを持つコンテキストを渡してください。

同じ理由で、`RenderLiveStream` にキャンセル用として渡された
`*fasthttp.RequestCtx` は、シャットダウンのシグナルを持つコンテキストに置き換え
られます。書き換えられたハンドラーは必ずこうなり（transform は `r` と
`r.Context()` を同じ識別子に畳みます）、その値はプール由来なので、ハンドラーが
戻った後に読むと別のリクエストを読むことになります。

## まだ出来ていないこと

ルート単位のボディ上限は移りません。`http.MaxBytesHandler` は1つのハンドラーを
縛りますが、fasthttp はサーバーを縛ります。ルート単位の上限を復元するヘッダー
フックの生成はまだ書かれていません。ルート単位の上限に依存しているなら、それを
待つか、`Server.MaxRequestBodySize` を最小値に合わせて、それを全体の天井として
受け入れてください。
