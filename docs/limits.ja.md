# サイズと形の上限

リクエストがぶつかりうる上限を一箇所にまとめます。何の上限か、既定値、変更方法、
超えたときにクライアントが受け取るもの。

これまでは各ボディ形式の節に分かれて書かれていました。一つを設定している間はそれで
読めますが、「このサービスは何を受け付けるのか」に答えるときには読めません。運用者が
一度は訊き、セキュリティレビューが毎回訊く質問です。

## 一覧

| 上限 | 既定値 | 変更方法 | 超えたとき |
|---|---|---|---|
| JSON ボディ | 1 MiB | `SetMaxJSONBodyBytes`、呼び出し単位なら `jsonbind.DecodeJSONLimit` | 413 |
| JSON のネスト深度 | 90 | `jsonbind.SetMaxNestingDepth` | 400 |
| CBOR ボディ | 1 MiB | `SetMaxCBORBodyBytes` | 413 |
| multipart ボディ | 1 MiB | `SetMaxMultipartBodyBytes` | 413 |
| multipart のファイルパート | multipart ボディの上限 | 同じつまみ | 413 |
| multipart をメモリに置く量 | 32 MiB、ボディ上限で頭打ち | 固定（`DefaultMultipartMaxMemory`） | — |
| WebSocket メッセージ | 1 MiB | `SocketOptions.ReadLimit` | 接続が閉じる |
| 固定長配列・`[N]byte` フィールド | Go の型が宣言した長さ | 型そのもの | 400 |
| urlencoded フォームボディ | **このモジュールの管轄外** — [トランスポート側の上限](#トランスポート側の上限)を参照 | | |

設定関数はプロセス全体に効き、両トランスポートランタイムの下層に置かれています。
起動時に一度呼べば net/http と fasthttp の双方に効きます。

```go
func main() {
	httpbind.SetMaxJSONBodyBytes(4 << 20)      // 4 MiB
	httpbind.SetMaxMultipartBodyBytes(8 << 20) // 8 MiB
	httpbind.SetMaxCBORBodyBytes(2 << 20)      // 2 MiB
	httpbind.SetSocketDefaults(httpbind.SocketOptions{ReadLimit: 256 << 10})
	// ...
}
```

引数はバイト数で、0 以下を渡すと上限が外れるのではなく既定値に戻ります。「無制限」は
ありません。誰も選ばなかった上限は、クライアントが選ぶ上限になるからです。

## リクエストボディ

### JSON — 1 MiB

`httpbind.SetMaxJSONBodyBytes(n)` / `httpbind.MaxJSONBodyBytes()`。HTTP の外側では
`jsonbind.DecodeJSONLimit(r, n)` がプロセス既定値に触らずに一回分だけを縛ります。

net/http では読み取り自体が上限で縛られるので、Content-Length のないボディも、嘘の
Content-Length を付けたボディも上限で止まります。fasthttp ではバインダが動く時点で
サーバがボディを読み終えているため、上限は届いたものに対する検査になり、読み取り自体は
サーバの `MaxRequestBodySize` が縛っています。ここを下げてバイトを早い段階で拒否したい
なら、そちらも設定してください。

`jsonbind` は transport-neutral なエラーを返し、生成されたバインダがそれを 413 に
変換します。

### JSON のネスト深度 — 90

`jsonbind.DefaultMaxNestingDepth`。`jsonbind.SetMaxNestingDepth` で引き上げられます。

これだけはサイズではなく形の上限です。読み取りは再帰で、入れ子の値は入れ子の呼び出しに
なり、生成されたデコーダは一段下のデコーダを呼びます。つまり文書の深さがそのまま
goroutine のスタックの深さになります。上限がなければ 1 MB の `[` は 50 万フレーム、
リクエストあたり数十 MB のスタックであり、それはボディサイズ上限がメモリについて何も
守れていないということです。

既定値はホストではなく、このパーサが動く最小のスタックで決めています。TinyGo の
goroutine スタックは固定長で、wasm ターゲットは 64 KiB から始まり、そこではブラケットが
100 段ほど開いた時点でパーサが溢れます。しかも wasm の溢れは終了時にしか検出されない
ので、原因になったリクエストはエラーではなく間違った答えを受け取ります。90 はその下に、
パーサの周りのフレーム分の余裕を残した値です。アプリケーションが作る文書がここに届く
ことはありません。人が書く JSON が 90 段ネストすることもありません。

これを超える文書はパースエラーになり、生成されたバインダは他の不正なボディと同じく
400 に変換します。スタックが伸びるホストでは起動時に引き上げられます。
`jsonbind.SetMaxNestingDepth(10000)` が `encoding/json` の許す値です。TinyGo ターゲット
で引き上げるなら `-stack-size` とセットで。

### CBOR — 1 MiB

`httpbind.SetMaxCBORBodyBytes(n)` / `httpbind.MaxCBORBodyBytes()`。両ランタイムが
同じ値を尊重します。CBOR を有効にして生成したビルドにのみ存在します。
[httpbind ガイド](httpbind.ja.md)のオプショナル CBOR ボディの節も参照してください。

### multipart/form-data — 1 MiB

`httpbind.SetMaxMultipartBodyBytes(n)` / `httpbind.MaxMultipartBodyBytes()`。

一つの値が三つを縛ります。ボディ全体、そこから読み出す各ファイルパート、そして
パートが一時ファイルへ落ちる前にフォームをメモリに置く量です。最後のものが
`DefaultMultipartMaxMemory`（32 MiB）で、これは下限ではなく上限として働き、ボディ上限で
頭打ちになります。既定値のままなら一時ファイルへは何も落ちません。

クライアントが Content-Length を送っていれば、まずそれを検査します。net/http では
続けてボディをラップするので、Content-Length がない、あるいは間違っていても上限を
すり抜けられません。fasthttp ではハンドラに届く前にサーバ自身の `MaxRequestBodySize` が
読み取りを縛っています。

ボディが上限を超えた場合も、単一のファイルパートが超えた場合も 413 です。読めない
パートはフィールド名を伴う 400 になります。

### application/x-www-form-urlencoded — ここでは縛られない

`SetMaxMultipartBodyBytes` はここに届きません。urlencoded ボディを解析するのは
トランスポートなので、縛るのもトランスポート自身の上限です。net/http なら 10 MB、
fasthttp なら `MaxRequestBodySize`。multipart の上限を小さくすることが脅威モデルの
一部なら、トランスポート側の上限も設定してください。そうしないと、ヘッダを一つ
書き換えるだけで同じペイロードが一桁大きく届きます。

## 形の上限

固定長の Go 配列として宣言したフィールドは、そこへデコードできるものを縛ります。

```go
type Board struct {
	Cells [9]string `json:"cells"`
}
```

短い配列は届いた分だけを埋め、残りはゼロ値のままにします。Go の型が述べている長さを、
文書が述べ直す必要はないからです。長い配列はエラーで、`jsonbind.ErrArrayTooLong` を
cause に持ちます。先頭 `N` 個を格納して残りを捨てるのは、デコーダが黙ってデータを
失うということであり、長さを宣言するのはまさにそれを防ぐためだからです。base64 から
デコードする `[N]byte` フィールドも同じ両端の規則に従います。

クエリパラメータの個数もフォームフィールドの個数も、ここでは縛っていません。どちらも
トランスポートが課すボディと URL の上限で縛られます。

## WebSocket メッセージ — 1 MiB

`SocketOptions.ReadLimit`。プロセス単位なら `httpbind.SetSocketDefaults`、エンドポイント
単位なら `httpbind.WebSocketWith` で設定します。これを超えて送ってきたピアは、メッセージを
切り詰められるのではなく接続を閉じられます。

ライフサイクルの上限（`IdleTimeout`、`PingInterval`、`WriteTimeout`）と同じ場所にあり、
そちらはソケット面と一緒に [httpbind ガイド](httpbind.ja.md)に書かれています。ゼロの
フィールドはプロセス既定値を、ゼロのままのプロセス既定値は定数を取るので、上限なしで
ドライバに届く値はありません。

## トランスポート側の上限

以下はこのモジュールが設定するものではなく、上記すべての下層にあります。ここの値が
上記より緩いこと自体は穴ではありませんが、バインダが拒否する時点でプロセスがすでに
何バイト触っているかを決めるのはこの値です。

| | net/http | fasthttp |
|---|---|---|
| リクエストボディ全体 | 自分で包まない限り無制限 | `Server.MaxRequestBodySize`、4 MiB |
| urlencoded フォームボディ | 10 MB 固定 | `Server.MaxRequestBodySize` |
| リクエストヘッダ | `Server.MaxHeaderBytes`、1 MB | `Server.ReadBufferSize`、4 KiB |
| リクエストラインと URL | `Server.MaxHeaderBytes` | `Server.ReadBufferSize` |

## 関連

- [httpbind ガイド](httpbind.ja.md) — これらの上限が適用されるボディ形式
- [jsonbind ガイド](jsonbind.ja.md) — HTTP の外側の JSON
- [fasthttp バックエンド](httpbind_fasthttp.ja.md) — もう一方のランタイムでの違い
