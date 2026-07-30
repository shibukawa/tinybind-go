# 探索型ルータ

このガイドは、tinybind の**上に**ウェブフレームワークを作る人のためのものである。
扱うのは探索型ルータ —— ディレクトリがどうやってルートになるか、変更操作がどう
Go に届くか、どこから先が実装者の責務か。

このルータが通常の登録型ルータとどう関係するか、それぞれで生成が何を見られるかは
[httpbind_frameworkowner.ja.md](httpbind_frameworkowner.ja.md) を先に読むこと。
描画側 —— 境界プロトコル、クライアントランタイム、head のマージ —— は
[htmlbind_frameworkowner.ja.md](htmlbind_frameworkowner.ja.md) にある。ここでは
どちらも繰り返さない。

このモジュールの上に作られたフレームワークは、どれも結局同じループを書く。
テンプレートのディレクトリを歩き、パスから URL を導き、ハンドラを登録する。
ループは簡単だ。守るべき制約はそうではなく、そのうち2つは、触ってもいない
パッケージのビルドを Go のツールチェーンが拒むまで姿を見せない。

`routetree` はそのループと、それらの制約である。探索して生成し、そこで止まる。
ページのメタデータ、サイトマップ、`robots.txt` については何の意見も持たない。
それらは実装者のもので、後述のルートテーブルがその材料になる。

## ページが取り得る3つの形

ルートとは、ルート直下のどこかで `page.tb.html` を持つディレクトリのこと。
そのファイル1つで足りる。生成ハンドラが URL を復号し、コンポーネントを描画し、
レスポンスを書く。データはテンプレート自身の `external` 呼び出しが取ってくる。

`page.go` を足すと、リクエストと描画の間で Go が走る。どれだけ走るかはシグネチャ
で決まる。

| ファイル | 形 | 得られるもの |
| --- | --- | --- |
| `page.tb.html` のみ | テンプレートのみ | ハンドラは全部生成される。データはテンプレート自身の `external` 呼び出しが取る |
| `+ page.go` の `func Load(id string, page int) (User, error)` | 型付き | 生成ハンドラが復号し、`Load` を呼び、その戻り値を描画する |
| `+ page.go` の `func Load(w http.ResponseWriter, r *http.Request)` | ハンドラ | 登録だけが生成される。レスポンスは全部実装者のもの |

形はシグネチャで決まるので、どちらにも一致しない `Load` は生成エラーになり、
いまどうなっていて取り得た2つの契約が何かを名指しする。

`Load` はページのエントリポイントの名前としては妙で、最初の案は `Page` だった。
それはコンパイラとの接触に耐えない。テンプレートコンパイラが同じパッケージに
`func Page(params PageParams) htmlbind.Fragment` を既に出力するので、その隣の
2つ目の `Page` は Go の再宣言になる。ファイル名は `page.go` のまま、コンポーネント
名も `Page` のまま。横にずれたのはエントリポイントだけである。

入力の規則は2つの段で1つ。先頭の引数はルートの動的セグメントで順序もルート順、
それ以降は引数名をキーとするクエリパラメータ。`page.go` がなければ規則は
コンポーネントの引数リストを読み、あれば `Load` の引数リストを読む。だからページを
1段上げても入力の書き方は変わらない。URL はオブジェクトを運ばないので、受け付ける
のはスカラーだけである。

クエリパラメータは省略可能にできる。綴りは、テンプレート側の optional マーカーが
すでに生成しているポインタそのものである。

```text
export component Page(topic: string, page: int?): html { ... }
```

`page` は `*int` で届く。キーが無いか値が空なら nil、それ以外はパースした数値への
ポインタになる。`?page=0` と「ページ指定なし」を区別する方法はこれだけであり、
パースできない `?page=x` は描画前に失敗するままである。パスセグメントは省略可能に
できない。単一セグメントはルートが一致した時点で常に存在し、キャッチオールは空の
残余を文字列として束縛するからだ。

変わるものが1つある。型付きの段では、コンポーネントの引数リストが `Load` の
戻り値リストになる。生成はこれを検査し、個数・順序・型のいずれかが食い違えば
両方のリストを名指しして失敗する。

## セグメント記法と、`[id]` にできない理由

末尾のアンダースコア1つが動的セグメント、2つが catch-all。

```
pages/users/id_/page.tb.html      → GET /users/{id}
pages/files/rest__/page.tb.html   → GET /files/{rest...}
```

ファイルベースのルータを使ったことがあれば、ブラケットを期待するはずで、最初の
疑問はなぜ `users/[id]/` ではないのかだろう。答えは趣味の問題ではない。ルート
ディレクトリは Go パッケージでもあり、ツールチェーンはパッケージパターンの照合中に
—— ビルド制約を評価するより前に —— 不正な import パス要素を拒否する。だから
`pages/users/[id]/page.go` が1つあっても、そのパッケージが壊れるのではない。
モジュール全体の `go build ./...` が壊れる。`{id}` `$id` `@id` `:id` `=id`
`(group)` `-id` `~id` も同じように失敗し、探索はそれらを先に拒否して理由を述べる。

除外も同じ権威に従う。探索は Go のツールチェーンが既に無視するものを無視する。
先頭の `_` と `.`、そして `testdata` である。プライベートフォルダの規約はそれに
付いてくる。

自明な綴りが間違っているもう一箇所がルートページである。`GET /` ではなく
`GET /{$}` で登録される。標準ライブラリでは裸の `/` は前方一致パターンであり、
未マッチのパスを 404 にせず全部拾ってしまうからだ。

## レイアウト

祖先ディレクトリの `layout.tb.html` が、その下の全ページを外側から順に包む。
そのレイアウトが取り得る形は2つの規則で縛られる。

`children: html` を宣言しなければならない。テンプレートコンパイラが `BindLayout`
ラッパーを出力するのはその形に対してだけなので、宣言がなければバインダは無く、
生成された呼び出しはコンパイルできない。探索は宣言の欠落を報告し、Go コンパイラに
任せない。

読めるのは自分のディレクトリ以上の動的セグメントだけである。`pages/users/` の
レイアウトは `/users/{id}` の `id` を読めない。深いセグメントに依存するラッパーは
そのセグメントが変わったときに再利用できず、それこそが祖先レイアウトを再利用する
価値そのものだからだ。

## どこに何が出るか

```
pages/                       ← Config.Root、既定は "pages"
├── layout.tb.html
├── layout_gen.go            ← コンパイル済み Layout コンポーネント
├── page.tb.html
├── page_gen.go              ← コンパイル済み Page コンポーネント
├── route_gen.go             ← "/" の RouteParams と DecodeRoute
├── routes_gen.go            ← Register / NewServeMux / Routes / Actions
└── users/id_/
    ├── page.tb.html
    ├── page.go              ← 任意の func Load と、サーバーアクション
    ├── page_gen.go
    └── route_gen.go
```

テンプレートごとに生成ファイル名が決まるので、同じディレクトリの `page.tb.html`
と `layout.tb.html` が1つの出力を奪い合うことはない。

登録はルートパッケージに置かれ、ルートパッケージ側には置かれない。自然な設計は
各ページの隣に composer を置くことだが、それは動かない。リーフは祖先レイアウトの
ためにルートを import し、ルートはページのためにリーフを import する。Go はそれを
循環と呼ぶ。したがって合成はレジストリに置かれ、生成される import はすべて
ツリーの下向きになり、上向きの辺はどこにも存在しない。

この制約はハンドラの段まで届く。手書きの `Load` は自分より上の composer を呼べない
ので、レイアウトチェーンが欲しいハンドラは `htmlbind.RenderChain` で自分で組む。
レスポンスを所有することが存在理由の段にとっては、これは正しい側の取引である。

## サーバーアクション

ページは `GET` だが、ウェブサイトはそうではない。フォームやボタン操作をどこかが
受けなければならず、その送信先はここでは URL として書かない。テンプレートが
export された Go のハンドラを名指しし、生成がアドレスを供給する。

```html
<button server-action="Rename" data-target="#name">rename</button>
```

```go
func Rename(w http.ResponseWriter, r *http.Request) { /* レスポンスを全部所有する */ }
```

属性はエンドポイントを運ぶ属性に下がり、それ以外の属性は手を付けられずに残る。

```html
<button data-tb-action="/_action/00369cf962b6/Rename" data-target="#name">rename</button>
```

契約はこれで全部である。`server-action` は名前を URL に解決して書き下すだけで、
クライアントのプロトコルを一切モデル化しない。だから `data-target` —— あるいは
`hx-target` でも何でも —— の意味は実装者のランタイムが決められる。
`Emitter.ActionAttr` を `hx-post` に向ければ、生成されたアクションはグルーコード
なしで HTMX を動かす。

手書きの `action="/users/42/rename"` に対してこれが買うものはコンパイラである。
URL は、それが指すハンドラと照合されることのない文字列にすぎない。名前は解決を
要求されるシンボルだ。Go の関数をリネームすれば、それを参照したテンプレートの
位置で生成が落ちる。

ハンドラは普通の `http.HandlerFunc` なので、`httptest` でテストでき、動かすのに
登録は要らない。周りには何も生成されない。書いたものがそのままレスポンスになる。

入力の読み方も他のハンドラと同じである。

```go
func Rename(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[RenameRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	// ...
}
```

`Bind` はリクエスト型を宣言したパッケージ向けに生成されたバインダ経由でディス
パッチする。つまりそのルートパッケージはジェネレータを走らせた対象でなければ
ならない。そのループは[実行](#実行)にある。

### ツリーが持たないハンドラを名指しする

ルートツリーの外にあるテンプレート — フレームワークが自前でコンパイルする classic
側のページなど — は、探索が歩かないハンドラを名指しする。アドレスを供給すれば、
他と同じように lowering される。

```go
files, err := routetree.Generate(routetree.GenerateOptions{
	Config: cfg,
	ActionResolver: func(name string) (string, bool) {
		url, ok := myRouteTable[name]
		return url, ok
	},
})
```

テンプレート自身のルートパッケージが export するハンドラが常に勝つので、リゾルバ
を足しても既にエンドポイントを持つアクションが黙って別の宛先に向くことはない。
どちらの供給元も答えられない名前は生成エラーのままで、試した両方の供給元を名指し
する。

### 何が到達可能か

ルートパッケージの export されたハンドラ形の関数は、テンプレートが言及するか
どうかに関わらず、すべてエンドポイントを得る。広すぎるように聞こえるが、ルート
パッケージが何であるかを思い出すと違って見える。生成レジストリ以外に import され
ないパッケージであり、そこで export されたシンボルは一般的な API ではなく、その
ルートの表面なのだ。

非公開にしたければ関数を小文字にする。それがオプトアウトであり、宣言は一切要らない。
別パッケージの生成コードは unexported なシンボルに到達できないからだ。

`Load` は除外される。ページ自身のエントリポイントであり、ハンドラの段でシグネチャ
が同じになるだけである。

生成される `Actions` テーブルが全エンドポイントを列挙する。それが、この表面を
暗黙ではなく検査可能なものにしている。パスは構造を隠すが何の権限も与えない ——
ケイパビリティトークンではない —— ので、各ハンドラは依然として自分の呼び出し元を
認証・認可する。

### アドレス

`/_action/<hash>/<HandlerName>`。ハッシュは宣言側ディレクトリとハンドラ名の
ダイジェストの先頭12桁の16進数である。ビルドソルトが無いので、変更のない
プロジェクトを再生成すれば同じ値が再現され、デプロイをまたいで開いたままのページも
サーバーが認識できる先に POST する。可読な名前が一緒に乗るので、ネットワークトレース
が走った Go の関数を名指しする。

ダイジェストに入るのが配信ルートのパスではなく宣言側ディレクトリである点は、
レイアウトで意味を持つ。レイアウトは1回だけコンパイルされて配下の全ページで
描画されるので、ルートパスをハッシュすると1つのハンドラがページごとに違う
アドレスを持ち、ハッシュの存在理由である決定性が壊れる。

`Emitter.ActionPrefix` が空間全体を動かす。既定値はタダで安全である。探索が
アンダースコアで始まるディレクトリを無視するので、どんなルートツリーも `/_action`
を生成しえない。設定されたプレフィックスにはその保護が無いので、生成は既存の
ルートが占めるプレフィックスを拒否する。`ServeMux` の panic として現れるのに
任せない。

## 実行

```go
files, err := routetree.Generate(routetree.GenerateOptions{
	Config: routetree.Config{
		Root:       "pages",
		ImportBase: "example.com/app/pages",
	},
	RootPackage: "pages",
})
if err != nil {
	return err
}
return routetree.Write(files)
```

`ImportBase` を明示しなければならないのは、導出できないからである。ディレクトリは
モジュール内の自分の位置を明かさない。`Generate` 自体は何も書き込まない ——
ファイルを返すだけで `Write` は便宜的なもの —— ので、後処理と出力先の差し替えが
実装者の手に残る。

ページやサーバーアクションが `httpbind.Bind` を呼ぶなら、バインダも生成する。
バインダはパッケージ単位で、その中の `Bind` 呼び出し位置から生成されるので、ツリー
が自分のパッケージを報告し、実装者がそれを回す。

```go
tree, err := routetree.Discover(cfg)   // すでに探索済みならそれを使い回す
// ... 先に Generate の結果を書き出しておく
for _, pkg := range tree.Packages() {
	_, err := generator.Generate(pkg.Dir, "", "tinybind_gen.go")
	if err != nil && !errors.Is(err, generator.ErrNothingToGenerate) {
		return err
	}
}
```

順序が意味を持つ。解析は各パッケージを型検査するので、ツリー自身の生成ファイルが
先にディスクに載っている必要がある。束縛するものが無いルートパッケージは多い ——
テンプレートだけのページはリクエスト型を宣言しない —— が、それは空の生成ファイルでは
なく上の `ErrNothingToGenerate` として返る。

これでページが OpenAPI に入ることはない。ツリー内の登録は生成レジストリの中だけで、
探索は tinybind が生成したものを読み飛ばす。判定の鍵はファイル名ではなくヘッダなので、
出力先を自分の名前に変えても何も変わらない。

1行足りなくなるのはヘッダ自体を差し替える場合である。生成ファイルに自分のブランドを
付けるなら、そのプレフィックスを探索に伝えること。伝えないと、書いたばかりのレジストリ
が実装者の手書きコードとして解析され、ページの登録が文書化されたルートになる。

```go
e.GeneratedHeader = "// Code generated by Popcorn Wave via TinyBind; DO NOT EDIT."

options := generator.DefaultOptions()
options.GeneratedHeaders = []string{e.GeneratedHeaderPrefix()}
```

`GeneratedHeaderPrefix` が対になっているので、登録する文字列がヘッダからずれることは
ない。登録するプレフィックスも慣習どおり `DO NOT EDIT.` で終わっている必要がある。
他ツールの生成コードは既定では探索対象のままである —— そこの登録はアプリケーション
自身の API 表面であり、黙って落とすとドキュメントからルートが消えるからだ。

アプリケーションが触るシンボルは1つになる。

```go
mux := pages.NewServeMux()        // または pages.Register(existingMux)
```

`Register` は `htmlbind.Option` を受け取ってすべての描画に渡すので、リクエスト
ごとのキャッシュ・タイムアウト・エラーフックは一度設定すれば済む。ページルートと
アクションのエンドポイントはまとめて登録される。

## ルートテーブル

レジストリはファイルシステムが知っていることを出力し、それ以上は意図的に出さない。

```go
var Routes = []RouteInfo{
	{Pattern: "GET /{$}", Path: "/", Dir: "", Params: nil},
	{Pattern: "GET /users/{id}", Path: "/users/{id}", Dir: "users/id_", Params: []string{"id"}},
}

var Actions = []ActionInfo{
	{Pattern: "POST /_action/00369cf962b6/Rename", Path: "/_action/00369cf962b6/Rename",
		Dir: "users/id_", Handler: "Rename", Hash: "00369cf962b6"},
}
```

`Routes` がサイトマップやルートインスペクタの接合部になる。パターン、メソッド、
どのセグメントが動的かはツリーから来る。動的セグメントが実際に展開される値は
アプリケーションのデータなので、実装者のもとに残る。

どちらのテーブルも何ではないかも押さえておきたい。OpenAPI の入力源ではない。
ページルートは OpenAPI ドキュメントに入らない。OpenAPI は公開された API 契約を
記述するものであり、HTML ページはそれではないからだ。アクションのエンドポイントも
同じ理由で入らない。1つのページの実装詳細だからである。どちらかを文書化したい
フレームワークは、これらのテーブルから自分のアーティファクトとして足す。

## 出力のカスタマイズ

3段階のうち、たいていのフレームワークは最初のものだけで足りる。

**シンボルの差し替え。** 生成コードは `Symbols` が指すパッケージを呼ぶので、
自分のランタイムに向けるだけならテンプレートは要らない。

```go
e := routetree.NewEmitter()
e.Symbols.RuntimeImport = "example.com/framework/render"
e.Symbols.RuntimeAlias = "render"
e.Symbols.ErrorImport = "example.com/framework/web"
e.Symbols.ErrorAlias = "web"
e.Symbols.BadRequest = "Invalid"
e.Symbols.Problem = "Fault"
e.Symbols.WriteError = "WriteProblem"   // ハンドラが失敗を書く入口
```

生成される宣言名も同じように、`ParamsType` `DecodeFunc` `RenderFunc`
`RegisterFunc` `MuxFunc` `TableVar` `ActionTableVar` で変えられる。
`ActionPrefix` と `ActionAttr` がアクションの URL 空間と、それが書き込まれる属性を
動かす。

ルータはリクエストのパッケージとは別のシンボル対を持つ。

```go
e.Symbols.MuxImport = "example.com/framework/web"
e.Symbols.MuxAlias = "web"
e.Symbols.MuxType = "web.Router"           // そのまま書かれるので interface ならポインタは不要
e.Symbols.MuxConstructor = "web.NewRouter" // 空ならコンストラクタ関数を出力しない
```

別になっているのは、1つの alias がルータと `Request` の両方を供給していると、ルータ
を動かすだけでリクエストのパッケージまで連れて行ってしまい、自分のルータだけを使い
たいフレームワークがレジストリ丸ごとの差し替えに追い込まれるからである。生成コード
が登録に使うのは `HandleFunc` だけなので、`MuxType` は1メソッドの interface で満たせる。

**ブロック単位の差し替え。** 組み込みテンプレートは名前付きブロックの合成なので、
残りを引き受けずに一箇所だけ変えられる。

| ブロック | 書き出すもの |
| --- | --- |
| `imports` | 生成ファイルの import 文 |
| `convert` | 生文字列1つからスカラー1つを読む処理 |
| `error` | デコーダが生成しうるすべてのエラー値 |
| `render` | レジストリとコンポーザにおける描画呼び出しそのもの |
| `handler` | レジストリ内の1ルートのハンドラ本体 |

```go
err := e.Parse("error", `web.Invalid(web.Fault{Code: {{ .Code | quote }}})`)
```

エントリポイントが writer 以上を要求するときに手を伸ばすのが `render` である。
`Symbols` の変更はパッケージを差し替えるがシグネチャは差し替えないので、レスポンス
入口がリクエストを必要とするフレームワーク —— bot モードの選択、圧縮、document
シェル、エラーページ —— は、呼び出しの名前ではなく形を変える。

```go
err := e.Parse(routetree.TemplateRender,
	`web.WriteHTML({{ .Writer }}, {{ .Request }}, {{ .Chain }}, {{ .Leaf }})`)
```

スコープにあるものは名前で届く（`Writer` `Request` `Chain` `Leaf` `Options`）。
ブロックが書くのは `error` 型の式なので、失敗をどう扱うかは呼び出し側に残る。`Chain`
はレイアウトチェーンで、祖先レイアウトを持たないページでは `nil` になる。`Wrappers`
は同じ値で、その場合は空になる —— `nil` を渡すより分岐したい override 向けである。

`Request` はスコープに無ければ空で、コンポーザがまさにその場合にあたる。コンポーザ
の契約は writer だからだ。ステータスを選ぶためにバッファへ描画するハンドラには、
引き渡すレスポンスが存在しない。必要なときは2つの設定がそれを変える。

```go
e.RenderWriterType = "http.ResponseWriter" // 既定は io.Writer
e.RenderRequestParam = "r"                 // 既定はリクエスト引数なし
```

コンポーザは自分のシグネチャが名指しするものを import するので、writer の型が `io`・
リクエストのパッケージ・自分のランタイムのいずれかで修飾されていれば追加設定は要らない。
同じ制約がブロック自体にもかかる。名指しできるのは、そのファイルが既に import して
いるパッケージ（自分のランタイムとエラーのパッケージ、リクエストのパッケージ、`io`）
だけである。`Symbols.RuntimeImport` を自分の入口があるパッケージに向けておけば、入口と
オプション型が同じパッケージにある通常のケースはそれで足りる。

これは `error` ブロックがすでに立てていた議論と同じである。失敗を書く入口は `(w, r)`
を受け取っていて、いまページを書く入口も受け取れる。

**ファイル全体の差し替え。** `TemplateDecoder` `TemplateComposer`
`TemplateRegistry` がそれぞれ1ファイルを端から端まで描画する。

```go
err := e.Parse(routetree.TemplateRegistry, myRegistryTemplate)
```

どのテンプレートも、フィールドが export され `DecoderModel` `ComposerModel`
`RegistryModel` に文書化されたモデルを受け取る。ヘルパーとして `quote` と `dict`
が付く。`dict` があるのは、Go のテンプレートがネストしたブロックに2つ以上の値を
渡す手段を他に持たないため。設定済みのベース1つを実行ごとに特殊化したいときは
`Clone` で独立したエミッタが取れる。

`Parse` が失敗した場合はエミッタを触らないので、壊れたテンプレートで動いていた
セットを失うことはない。

tinybind のものを置き換えるのではなくテンプレートの周りにコードを生成する場合、
コンポーネントの引数を Go の型で知る必要があり、テンプレートが行うサーバーアクション
の参照も知る必要がある。`htmlbind.Signatures` と `htmlbind.ActionRefs` は
[htmlbind_frameworkowner.ja.md](htmlbind_frameworkowner.ja.md) にある。

## まだ無いもの

- CSRF は未配線である。ambient credentials で到達可能な `POST` エンドポイントな
  ので、入るまでは自分で包むこと。
- スクリプト無しモードは設計済みだが未実装である。いまは `<form server-action>` も
  他の要素と同じように下がり、横取りするランタイムを必要とする。ページ自身へ POST
  して `303` で戻る形が第2段階であり、JavaScript を無効にして送信されたフォームが
  必要とするのはそれである。
- `document.tb.html` は探索されるが、まだ適用されない。ドキュメントシェルは
  実装者のもの。
- URL セグメントを持たないルートグループには記法が無い。他のフレームワークが使う
  括弧の綴りが、import パスとして不正な文字だから。
- catch-all は文字列としてバインドされる。より豊かな型付けは未決。

最後の2つは同じ衝突を2度見ているにすぎない。Go の規則とルーティングの慣習が
食い違うとき、勝つのは Go の規則のほうだ。
