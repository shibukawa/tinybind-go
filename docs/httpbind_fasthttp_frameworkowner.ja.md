# fasthttp バックエンド フレームワーク実装者向けガイド

このガイドは、tinybind の**上に**ウェブフレームワークを作っていて、その利用者に
fasthttp 向けのビルドを提供したい人のためのものである。扱うのは、変換が実装者に
何を要求するか、実装者自身のタグ境界がどこに落ちるか、どのヘルパーが2つ必要に
なるか。

アプリケーション作者が見るものは [httpbind_fasthttp.ja.md](httpbind_fasthttp.ja.md)
にある。ここでは繰り返さない。ルーティング一般は
[httpbind_frameworkowner.ja.md](httpbind_frameworkowner.ja.md)。

## 変換は宣言されたものしか知らない

生成は、書かれた net/http ハンドラーを書き換えて fasthttp 版を導出する。2つの
トランスポート引数が1つの context に畳まれ、writer や request を取っていた呼び出し
からはその引数が落ちる。`httpbind.Bind` とその仲間についてそれが成立するのは、
ジェネレータが最初からその形を知っているからだ。

実装者の関数の形は知らない。`framework.Render(w, r, page)` を呼ぶハンドラーは、
追跡不能なサードパーティのロガーを呼ぶハンドラーと見分けがつかない。変換は同じ
理由で両方を拒否する —— どの引数が消えるのか判断できない。そして利用者のコードに
あるそういうハンドラーは、実装者が動かない限り利用者には直せないビルドエラーに
なる。

だから呼び出しを登録し、トランスポートがどこに座っているかを言う。

```go
calls := generator.NewCallRegistry()
err := calls.Register(
	// func Render(w http.ResponseWriter, r *http.Request, page Page) error
	generator.ResponseWriteCall(
		generator.Function("example.com/fw", "Render"),
		generator.ArgumentType("response", 2),
		generator.WriterArgument(0),
		generator.RequestArgument(1),
	),
)
```

`WriterArgument` と `RequestArgument` が新しい半分である。他の role は意味のある値を
**どこから読むか**を言う。この2つは逆に、意味を何も運ばず、net/http の形が両方の
半分を別々に渡すからという理由だけで存在している引数はどれか、を言う。単一値の
トランスポートが落とすのはちょうどそれだ。

トランスポートを取ってモデルを名指さない呼び出しにもパターンが要る。無ければ、
それを呼ぶハンドラーが全部拒否される。`TransportCall` はそのためにある。

```go
generator.TransportCall(
	generator.Function("example.com/fw", "Abort"),
	generator.WriterArgument(0),
	generator.RequestArgument(1),
)
```

このモジュール自身の `WriteError` もこの形で登録されている。それが無かったとき、
エラーを報告するハンドラーが全部拒否された。必要性はそうやって見つかった。

## ヘルパーを2つのパッケージに

スロットの宣言は、どの引数を落とすかを変換に教える。落とされる先の関数を作って
くれるわけではない。`framework.Render` は `*fasthttp.RequestCtx` の上にも存在する
必要があり、それは実装者が書く。

同じ名前で2つ目のパッケージに出し、ペアを登録する。

```go
transform := generator.DefaultTransformOptions()
transform.ImportRewrites["example.com/fw/render"] = "example.com/fw/render/fast"
```

生成されるファイルは、2つ目のパッケージを**1つ目のローカル名で** import する。
書き換えられた本体には `render.Page(ctx, p)` と書いてあり、それは書き換え前と同じ
ことを言っている。動いたのは import 行だけだ。この対応付けに組み込みは無い ——
このモジュール自身のランタイムのペア以外は何も入っていないし、いくつ足しても
かまわない。

fasthttp 側を書くときは、アクセサの規約に従うとよい。トランスポート値を第1引数に
取り、パラメータ名は net/http 版のものを使う。このモジュールのランタイムがそう
していて、その見返りとして生成バインダーの本体が両トランスポートで同じテキストに
なる。動くのはシグネチャ行だけだ。引数の順序を変えたヘルパーは書き換え規則を1つ
要求する。元の形を鏡写しにしたヘルパーはそれを要求しない。

## タグ境界をどこに置くか

アプリケーションはハンドラーのファイルにタグを付ける。実装者が抱えるのは同じ問題の
難しいほうだ。パッケージの大半はトランスポートと関係が無く、タグを付けられるのを
嫌がる。

パッケージで割ると、そのトランスポート非依存の大多数が代金を払う。設定関数も
オプション構造体もログヘルパーも、両側にエイリアスが要る。import path を1つに
保って中でタグを切れば、それらの費用はゼロになる。これは tinybind 自身とは逆の
配置だ —— `httpbind` と `fasthttpbind` は別パッケージである。同じ判断基準が違う
答えを出しているだけで、あちらは表面のほとんどがトランスポート形、こちらはほとんど
そうでない。

タグ層を薄く保つ手は、**型にタグを付け、その利用者には付けない**ことである。

```go
//go:build !fasthttp
type Handler func(w http.ResponseWriter, r *http.Request)

//go:build fasthttp
type Handler func(ctx *fasthttp.RequestCtx)
```

```go
// タグなし。シグネチャの字面が変わらないので1コピーで済む
func (a *App) GET(path string, h Handler) { ... }
```

シグネチャが自前の型の名前しか書いていない関数は、その型の定義が違っていても、
両方のタグで同じシグネチャを保つ。ルーティングテーブルもオプション構造体も登録
API もそれで単一コピーのままになる。2つ必要なのは、リクエストの**中に**手を伸ばす
コードだけだ。

ただし境界は伝播する。タグなしの関数がタグ付きの関数を呼べるのは、呼び先の
シグネチャが両方のタグで同一である間だけで、同一でなくなった瞬間に呼び出し元にも
タグが付く。これは変換がアプリケーションコードに対して計算する閉包と同じもので、
実装者は手でそれをやることになる。タグがリクエスト処理層より外へ広がり始めたら、
transport-free にできたはずのシグネチャがそうなっていない。そちらを直すほうが、
2つ目のコピーを保守するより安い。

## エラーモデルは1つの型である

`Problem`、`FieldError`、`HTTPError` は共有 leaf にあり、両方のランタイムがそれを
エイリアスしている。一致する2つの定義ではなく、同じ型だ。だからフレームワークが
一方のサーフェスで組んだエラーは、他方が検査したときも一致する。再宣言しては
いけない。再エクスポートするならエイリアスにすること。

継ぎ目をまたぐものすべてに同じことが言える。今日一致している2つの定義は、後日
食い違う機会が2つあるということであり、その失敗は静かだ。`errors.As` が一致
しなくなる。`Problem` が unwrap されなくなる。

アプリケーションに見せる問題値は `Problem` より豊かなもの（ステータス、
タイトル、フィールド、原因）が欲しくなるはずだが、それにも `Problem` と名付ける
と、同名で別物の型が2つ利用者の前に並ぶ。自分のものは両ランタイムがエイリアス
する leaf に置くこと。ここで `FieldError` に対して取ったのと同じ手であり、
どちらのバックエンドでビルドしても名前が1つのものを指す。

更新の面も同じ割り方をしていて、実例として読む価値がある。`htmlupdate` と
`fasthttpupdate` はメソッドが必要な `Options` と `Response` だけを再宣言し、
残りは1つのコアからエイリアスしている。見返りは具体的だ。`Registry`、
`Reloadable`、`Update` は両バックエンドで同一の型なので、それらを組み立てる
コードにビルドタグは一切要らない。各シェルは `updatecore.Options(o)` で変換して
いるので、片方だけにフィールドが増えれば、ずれるのではなくコンパイルが止まる。

## ルータを選ぶ

`RouterTarget` が、生成されるルート登録の使うパッケージ、修飾子、型、登録関数名、
catch-all の綴りを指定する。

```go
transform.Router = generator.RouterTarget{
	Import:         "example.com/fw/mux",
	Qualifier:      "mux",
	Type:           "mux.Router",       // そのまま書き出されるので interface ならポインタ不要
	RegisterFunc:   "Wire",
	CatchAllSuffix: ":*",
}
```

既定は tinygodriver の fasthttprouter フォークである。フレームワークがルーティングを
所有しているなら、これを自前の型に向ければ生成はそちらに登録する。

`CatchAllSuffix` を空にすると、catch-all を持たないルータに対して、綴りを勝手に
でっち上げる代わりにそのルートを名指しで拒否する。

## 拒否契約から継承するもの

アダプタは無い。変換が書き換えられないハンドラーはビルドを止め、診断が該当箇所と
remedy を名指しする。ここから2つの帰結が実装者に落ちてくる。

利用者は remedy の中に**あなたのフレームワークの名前**を読むことになる ——
「passes r to fw.Render, whose transport arguments are undeclared」。そして修正は、
あなたにしか登録できない call pattern である。それを登録し切っているかどうかが、
利用者が採用できるバックエンドと、できないバックエンドの差になる。

もう1つの帰結は静かだ。トランスポートを取るフレームワークヘルパーはどれも拒否の
種になる。つまりその形で公開している表面が、2度保守して1度宣言しなければならない
表面である。transport-free なシグネチャを与えられるヘルパー ——
`func Render(ctx context.Context, page Page) (Body, error)` のような —— は、
どちら側にも費用を発生させず、何も拒否しない。アプリケーション作者も同じ助言を
受けるが、実装者にとってのほうが価値は大きい。その選択は、上に載る全アプリケー
ションに掛け算で効くからだ。

## 確かめる

自分のフレームワークで書かれたパッケージにレポートを向けて、返ってきたものを
読む。

```bash
tinybind-gen generate -dir ./example -backend fasthttp -transport-report
```

何も出なければ、そのフレームワークを普通に使ったアプリケーションは fasthttp で
ビルドできる。何か出たなら、登録していない call pattern か、移植していないヘルパー
か、そもそもトランスポートを取るべきでなかったシグネチャのいずれかだ。どれかは
メッセージが言う。
