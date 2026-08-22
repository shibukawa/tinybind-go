# tinybind-go

[English](README.md)

TinyGo と通常 Go のための、リフレクション不要・コード生成ファーストのバインディングライブラリです。HTTP・JSON・SQL・DynamoDB のランタイム依存を別パッケージに分離しています。

利用ガイド: [httpbind](docs/httpbind.ja.md) · [jsonbind](docs/jsonbind.ja.md) · [cborbind](docs/cborbind.ja.md) · [configbind](docs/configbind.ja.md) · [htmlbind](docs/htmlbind.ja.md) · [sqlbind](docs/sqlbind.ja.md) · [dynamobind](docs/dynamobind.ja.md) · [firestorebind](docs/firestorebind.ja.md) · [cachekeybind](docs/cachekeybind.ja.md) · [リロード可能な component](docs/httpbind_reloadable_componet.ja.md) · [fasthttp バックエンド](docs/httpbind_fasthttp.ja.md)

この上にフレームワークを作る方へ: まず [フレームワーク向け機能一覧](docs/httpbind_framework_facilities.ja.md)（何が使えて何が無いかの索引）、次に [htmlbind フレームワーク実装者向けガイド](docs/htmlbind_frameworkowner.ja.md)、利用者が fasthttp 向けにビルドするなら [fasthttp バックエンド フレームワーク実装者向けガイド](docs/httpbind_fasthttp_frameworkowner.ja.md)

リクエスト／レスポンスの構造体を一度定義するだけで、ジェネレータが型専用のバインダとライタを出力します。同じモデルで **JSON・form・multipart・query**（タグにより path / header / cookie も）を扱えます。レスポンスはクライアントの **`Accept`** に合わせて適応します（ストリーミング時は content negotiation も）。同じ解析結果から **OpenAPI 3.1（JSON）も生成**し、バインダ／ライタと常に同期します。godoc コメントは `summary` / `description` として取り込まれます。ルート登録は別 DSL ではなく、実際の **`net/http` の書き方を静的解析**して発見します（`HandleFunc`、`Handle`、メソッド値、ラッパーなど）。

```go
type CreateUserRequest struct {
	// input = query + payload（JSON / form / multipart）。タグは省略可。
	Name  string `input:"name"`  // タグなし Name string と同じ
	Email string `input:"email"` // タグなし Email string と同じ
	OrgID string `path:"org_id"`
	Token string `header:"Authorization"`
}

type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	OrgID string `json:"org_id"`
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	// Name/Email: query および/または JSON/form/multipart ボディ（input）
	// OrgID は path、Token は Authorization ヘッダ
	out := CreateUserResponse{
		ID:    "u_1",
		Name:  input.Name,
		Email: input.Email,
		OrgID: input.OrgID,
	}
	_ = httpbind.Write[CreateUserResponse](w, r, out)
}
```

パッケージに対してジェネレータを実行します（バインダ + OpenAPI 埋め込み）:

```bash
go run ./cmd/tinybind-gen generate -dir . -openapi
```

同じ生成処理で `configbind.SubCommand[T]` によるCLI専用のapplication
subcommandも生成できます。必須・任意・残余のposition引数に対応します。
詳しくは [configbindのsubcommandガイド](docs/configbind.ja.md#cli-subcommand)
を参照してください。

### 構造体タグ リファレンス

タグ値を省略した場合、ワイヤ上の名前はフィールド名の lower-camel になります（例: タグなし `Name` → `"name"`）。

| タグ | 入力元 | 説明 |
|------|--------|------|
| （なし）または `input:"name"` | **query + payload** | デフォルト。payload は JSON・`application/x-www-form-urlencoded`・`multipart/form-data` を含む。通常のユーザー入力フィールドではタグ省略可。 |
| `query:"page"` | query のみ | ボディからは読まない。 |
| `payload:"name"` | ボディのみ | `Content-Type` に応じて JSON / form / multipart。query 文字列からは読まない。 |
| `payload:"image"` と `httpbind.File` | multipart のファイルパート | 名前付きパートからファイル名・Content-Type・サイズ・バイト列を bind。payload のみ（query 不可）。multipart ボディ上限はデフォルト **1 MiB**。`httpbind.SetMaxMultipartBodyBytes` で変更可。 |
| `path:"org_id"` | path パラメータ | ルートパターンの `{org_id}`（相当）と対応。 |
| `header:"Authorization"` | リクエストヘッダ | タグ値がヘッダ名。 |
| `cookie:"session"` | cookie | タグ値が cookie 名。 |

**`input` / `payload` / `query` の使い分け**

- 通常フィールド（query *または* body のどちらでも来うる）には **`input`**（またはタグなし）を使う。
- 入力元を制限したいときだけ **`query`** / **`payload`** を使う（例: 検索条件は query、一部フィールドは body のみ）。
- `payload` は `input` と異なり、**query パラメータは受け付けない**。

制限を混ぜる例:

```go
type SearchRequest struct {
	Keyword string `query:"keyword"`   // query のみ
	Page    int    `query:"page"`
	Filter  string `payload:"filter"`  // ボディのみ（JSON/form/multipart）
}
```

レスポンス構造体ではエンコード用に標準の `json:"..."` をよく使います。リクエストのバインド元は上記のソース用タグです。

### ストリーミング（理想 API）

```go
httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error {
    if err := s.Write(ChatEvent{Type: "delta", Delta: "hi"}); err != nil {
        return err
    }
    return s.Write(ChatEvent{Type: "done"})
})
```

- **`Write` は何度でも呼べる**（インクリメンタルなイベント送出）。
- 形式はストリームが開くときに一度だけ決定（`?stream=` → `Accept` → `User-Agent` → 既定 **NDJSON**）。
- ストリームを閉じるのはエントリ側なので、コールバックが途中で失敗しても JSON array の末尾 `]` は書かれます。
- 形式:
  - **SSE** — `text/event-stream`
  - **NDJSON / JSONL** — `application/x-ndjson`（1 行 1 オブジェクト。**JSON 配列ではない**）
  - **JSON array** — `application/json` の `[obj1,obj2,...]`（末尾の `]` はストリームを閉じるときに書かれる）
- 削除済みの `WriteNDJSON` / `WriteSSE` は使わない。

## パッケージ構成

| パス | 役割 |
|------|------|
| `.`（`package httpbind`） | ランタイム: Bind / Write / WriteError / WriteStream / OpenAPI 配信 / SwaggerUI |
| `jsonbind/` | 単独の DecodeJSON / EncodeJSON。`net/http` と `database/sql` を import しない |
| `sqlbind/` | ScanRows と行変換ヘルパ。`net/http` を import しない |
| `dynamobind/` | `tinygodriver/nosql/dynamodb` 上の DynamoDB item runtime。`net/http` も `database/sql` も import しない |
| `firestorebind/` | `tinygodriver/nosql/datastore` 上の Firestore（Datastore mode）entity runtime。`net/http` も `database/sql` も import しない |
| `cachekeybind/` | キャッシュキーのフレーミング runtime。stdlib のみ |
| `generator/` | フィールド計画に基づくバインダ／ライタ + OpenAPI 3.1 埋め込み生成 |
| `parser/` | ルート／ハンドラ発見（`Bind`、`Write`、`WriteStream`、エラー） |
| `templates/htmlbind/` | 型付きで文脈安全な HTML template compiler |
| `templates/sqlbind/` | 型付き parameterized SQL template compiler |
| `templates/firestorebind/` | 型付き Firestore アクセスパターン宣言（`.tb.firestore`） |
| `cmd/tinybind-gen` | CLI: パッケージ dir からバインダ + OpenAPI を生成 |
| `examples/demo` | 一通り触れるサンプルアプリ |
| `internal/*` | テスト用フィクスチャ |
| `testdata/cmd/*` | 開発用ヘルパ（配布対象外。`testdata` 配下のため `go get` / `./...` の対象外） |

```bash
go run ./cmd/tinybind-gen generate -dir ./path/to/package
```

生成ファイルには、生成に使った入力の SHA-256 を持つ `// tinybind:generated`
コメントが記録されます。パッケージのソース・テンプレート・`go.mod`・オプション・
ジェネレータのバイナリがすべて記録と同じハッシュになる実行は、再生成せずに
終了します。`-force` を付けると常に再生成します。詳細は
[docs/httpbind.ja.md](docs/httpbind.ja.md#変更のないパッケージのスキップ) を
参照してください。

フレームワーク側ですべてのランタイム関数をラップしても、呼び出しを
ジェネレータに認識させられます。ラッパーのパッケージ上の識別子、操作の意味、
ジェネレータが必要とする型・値の位置だけを 0 始まりで登録します。

```go
package main

import "github.com/shibukawa/tinybind-go/generator"

func main() {
    calls := generator.NewCallRegistry()
    if err := calls.Register(
        // func RegisterConfig[T any](ctx context.Context, name string) *T
        generator.ConfigBindCall(
            generator.Function("example.com/framework", "RegisterConfig"),
            generator.GenericType("config", 0),
            generator.Argument("prefix", 1),
        ),
        // func Created(ctx context.Context, w http.ResponseWriter, value any) error
        generator.ResponseWriteStatusCall(
            generator.Function("example.com/framework", "Created"),
            generator.ArgumentType("response", 2),
            generator.Constant("status", 201),
        ),
    ); err != nil {
        panic(err)
    }
    options, err := calls.Options(generator.DefaultOptions())
    if err != nil {
        panic(err)
    }
    generator.Main(generator.MustCommandSet(generator.GenerateCommand(options)))
}
```

追加の引数は登録不要です。モデル型が型引数にある場合は `GenericType`、
値引数の静的な型にある場合は `ArgumentType`、設定prefixやroute patternの
ような値は `Argument`、ラッパー内に隠したHTTP 201のような固定値は
`Constant` で指定します。操作ごとのコンストラクタは
`RequestBindCall`、`ResponseWriteCall`、`ResponseWriteStatusCall`、
`StreamCreateCall`、`JSONDecodeCall`、`JSONEncodeCall`、`RowsScanCall`、
`ConfigBindCall`、`ConfigSubCommandCall`、`RouteRegisterCall`、
`ErrorResponseCall` です。
パッケージ関数は `Function`、名前付きreceiverのmethodは `Method` で
対象にします。

各コンストラクタの必須role名は同じ順に、`request`、`response`、
`response` + `status`、`stream`、`decode`、`encode`、`row`、
`config` + `prefix`、`config` + `name` + `help`、`pattern` + `handler`、
`status` です。

`RuntimePackages` は標準tinybindと同じ関数名・signatureを使う場合の短縮指定
として残っています。関数名の変更、引数の並べ替え・追加、固定値の隠蔽がある
ラッパーには明示的なcall patternを使います。`generator.Options{}` の探索先は
空です。`DisableFeatures` へ追加した機能は `-generate-all` 使用時も無効です。

組み込みの `generate` とフレームワーク独自のライフサイクルコマンドを
一つのCLIにまとめられます。

```go
commands := generator.MustCommandSet(
    generator.GenerateCommand(options),
    generator.Command{Name: "init", Summary: "プロジェクトを初期化", Run: runInit},
    generator.Command{Name: "build", Summary: "generateしてbuild", Run: runBuild},
    generator.Command{Name: "watch", Summary: "監視してgenerateとbuild", Run: runWatch},
)
generator.Main(commands)
```

各コマンドには `context.Context`、引数、stdin/stdout/stderr・作業directory・
環境変数を持つ `CommandIO` が渡されます。`build` や `watch` からCLIを
起動せず、同一process内で生成できます。

```go
result, err := generator.New(options).GeneratePackage(ctx, generator.GenerateRequest{
    Dir: dir, OpenAPI: true,
})
```

`GeneratePackage` はtemplate、mapping、configbind、指定時のOpenAPIを実行し、
出力したpathを返します。`generator.Main` はprocess境界だけを担当するため、
テストや合成したコマンドでは `CommandSet.Run` または `GeneratePackage` を
直接呼びます。

生成は利用箇所単位で絞られ、`DecodeJSON[T]` だけを使うコードには JSON
デコーダだけが生成され、root の HTTP runtime と `net/http` へ依存しません。従来どおり有効な全
マッピングを生成する場合は `Options.GenerateAll`、互換File型は
`Options.FileTypes.Set` で明示できます。

単独 JSON API は `jsonbind.DecodeJSON` / `jsonbind.EncodeJSON` です。
JSON の読み込み上限はデフォルト 1 MiB で、全体設定は
`jsonbind.SetMaxJSONBodyBytes`、呼び出し単位では
`jsonbind.DecodeJSONLimit` を使います。`jsonbind` は transport-neutral な
エラーを返し、HTTP request の上限超過は `httpbind.Bind` が 413 に変換します。

JOIN 結果は生成されたreflection-freeコードを使う `ScanRows[T]` で木構造にまとめられます。各階層で一つの
スカラーフィールドに `groupkey:""`、列名には `db:"column_name"` を付けます。
同じキーの行は同じ親・子へ集約され、outer join の子キーが NULL ならその子を
追加しません。

```go
type Organization struct {
    ID    int    `db:"organization_id" groupkey:""`
    Users []User
}
type User struct {
    ID int `db:"user_id" groupkey:""`
}

organizations, err := sqlbind.ScanRows[Organization](rows)
```

## テンプレートの整形

`.tb.html` / `.tb.sql` / `.tb.dynamo` は本モジュールが定義した独自形式なので、既存のエディタは整形方法を知りません。ジェネレータがフォーマッタを同梱します。

```bash
go run ./cmd/tinybind-gen fmt -w -dir ./store
```

`-l` は変更が必要なファイルを列挙して非ゼロで終了します（CI 用）。`-as sql`（`html` / `dynamo`）は標準入力の 1 ソースを標準出力へ流すフィルタで、エディタの保存時整形はこれを使います。

形式ごとの動作:

- **SQL** — 1 clause 1 行。CTE 本体とサブクエリは自分の `SELECT` の下に字下げし、`JOIN` と `ON` を分け、条件が長いときは `AND` / `OR` を行頭に揃えます。キーワードの大文字小文字・リテラル・コメントは書かれたままです。
- **HTML** — `head` や `table` など、HTML パーサ自身が空白のみの run を捨てる位置では 1 タグ 1 行にします。それ以外では既にある空白を改行に置き換えるだけなので、`<b>a</b><i>b</i>` は貼り付いたまま、描画結果は変わりません。`pre` / `textarea` / `script` / `style` と `preserve-whitespace` 配下はバイト単位でそのままです。
- **DynamoDB** — `table` の次に `key`、1 clause 1 行。

構文エラーのあるソースは報告のみで書き換えません。コマンドの機能はすべてライブラリとしても使えます。

```go
import "github.com/shibukawa/tinybind-go/templates/templatefmt"

formatted, err := templatefmt.Source("users.tb.sql", source, templatefmt.Options{})
results, err := templatefmt.Dir("./store", templatefmt.Options{Width: 120})
```

`templatefmt.Dir` は読むだけで書き込みません。各 `Result` が変更の有無を持ち、適用したいときだけ `Write()` を呼びます。

## 生成 Go のテンプレート位置

生成された Go は出力であってソースではないので、その中のエラーは書いた覚えのないファイルを名指しします。`-template-line-directives` は、生成コードをそれを生んだテンプレートの行へ Go の `//line` ディレクティブで写します。

```bash
go run ./cmd/tinybind-gen generate -dir ./store -template-line-directives
```

テンプレートの式に型エラーがあれば、`tinybind_templates_gen.go` の行ではなく `store/users.tb.sql:5` が報告されます。ディレクティブを尊重する読み手はすべて追随します — コンパイラ、`go vet`、delve、gopls、エディタ。テンプレート由来でない生成行は、生成ファイル自身の位置のままです。

ディレクティブに書かれるパスは絶対パスです。ツールチェインがコマンドを実行した場所を基準に短縮するので、同じ 1 本の文字列がモジュールルートからは `store/users.tb.sql`、パッケージの中からは `./users.tb.sql` と読めます。`go build` と `go vet` の両方が同じものを出すのはこの形だけです。バイナリの中では `-trimpath` が他のソースパスと同じように正規化するので、リリースビルドが持つのは `yourmodule/store/users.tb.sql` です。

どこまで届くかは方言によって違います。

- **SQL** — statement は本物の Go 関数として出るので、その中で panic するとスタックフレームも `.tb.sql` を名指しします。
- **HTML** — コンパイル時のみ。描画は共有の `htmlbind` コーディネータが命令列を歩く中で起きるため、フレームはそのパッケージの中にあり、生成コードに打ったディレクティブでは動かせません。
- **DynamoDB** — 宣言 1 つにつき 1 写像。パーサが持つのは行だけで桁がないので、これ以上細かくはできません。

既定はオフです。オンにする前に 2 点。有効化するとテンプレートを持つ生成ファイルがすべて書き換わります。中身に絶対パスが入るので、生成 Go のバイトはチェックアウト位置に依存するようになります。生成 Go をバージョン管理から外していれば代償はありませんが、コミットしている場合は他のマシンで誤ったパスを指すことになります。写像された部分は 3 割ほど長くなります。ディレクティブは 1 行ごとに繰り返します — 繰り返さないと、直下の 1 行しか正しく写らないからです。コメントはバイナリに入らないので、増えるのはソースサイズだけです。もう 1 点、カバレッジ付きのテスト実行では、プロファイルが生成ファイルのパスを保ったまま写像後の行番号を書くため、`go tool cover` はそのファイルに存在しない行を描きます。カバレッジを取るときはオフにしてください。

書き込み済みファイルではなく `Artifact` を受け取る場合、写像の終わりを示すディレクティブは書き出すファイル名を名乗る必要があり、その名前を知っているのは呼び出し側だけです。

```go
content := generator.ResolveTemplatePositions(artifact.Content, artifact.OutputBase+"_pw_gen.go")
```

この呼び出しを飛ばすと生成の足場の位置が誤って報告されます。テンプレート側の位置は影響を受けません。

## デモ

```bash
go generate ./examples/demo
go run ./examples/demo
# http://localhost:8080/       インデックス + ブラウザ向けストリーム demo
# http://localhost:8080/docs/  Swagger UI
# http://localhost:8080/chat   WriteStream（SSE / NDJSON / JSON array 自動）
```

curl 例の詳細は [`examples/demo/README.md`](examples/demo/README.md) を参照してください。

## ベンチマーク

生成コードには駆動すべき reflection がなく、中間の `map[string]any` を組み立てる必要もありません。差はそこから出ています。計測環境は Apple M3・Go 1.26.5・`darwin/arm64`、10回計測の最良値です。

各ペアの出力は一致します。JSON codec は差分ファザで `encoding/json` と突き合わせており、ハンドラとテンプレートのペアはベンチマークに並ぶテストで等価性を検証しています。再現は次のコマンドです。

```bash
go test ./internal/benchfixture -run xxx -bench . -benchmem
```

### スループット

ドキュメントは 312 バイトの注文データで、ネストしたオブジェクト・オブジェクト3要素の配列・文字列配列を含みます。ページは5行のユーザー一覧です。

| 経路 | 標準ライブラリ | 生成コード |
|------|----------------|------------|
| JSON decode（`io.Reader`） | 3447 ns · 1688 B · 30 allocs | **777 ns · 856 B · 15 allocs** |
| JSON decode（`json.Unmarshal`、バイト列が手元にある場合） | 3287 ns · 888 B · 25 allocs | — |
| JSON encode | 579 ns · 144 B · 1 alloc | **272 ns · 0 B · 0 allocs** |
| `Bind` + `Write`（リクエスト再利用） | 850 ns · 1584 B · 17 allocs | **584 ns · 1021 B · 16 allocs** |
| `Bind` + `Write`（リクエスト構築込み） | 1695 ns · 7445 B · 31 allocs | **1422 ns · 6883 B · 30 allocs** |
| HTML レンダリング（`html/template` 対 `htmlbind`） | 7346 ns · 2705 B · 107 allocs | **930 ns · 464 B · 4 allocs** |

JSON の各行は `encoding/json` との比較、ハンドラの行は同じ body decode・path・header 読み取りを手書きした `net/http` ハンドラとの比較、HTML の行は同じドキュメントを出力する `html/template` テンプレートとの比較です。

エンコードのアロケーションはゼロです。生成エンコーダはプールしたバッファに append するので、レスポンス1本あたりのゴミが出ません。デコードの15回は、結果に残る文字列とスライス13個に body バッファとその reader を足した数で、呼び出し側が受け取る値を変えずに削れるものはもう残っていません。HTML レンダリングの4回は行ごとではなくレンダリング1回ごとの固定費（bind した fragment・オプション・レンダラ・変換バッファ）なので、ページが長くなっても4回のままです。

### バイナリサイズ

同じ小さな JSON プログラムを2通りにビルドしたものです。片方は `encoding/json`、もう片方は生成された `jsonbind` codec を使います。`jsonbind` は `encoding/json` を一切 import しないため、reflection ベースの codec がバイナリに入りません。Go 1.26.5 と TinyGo 0.41.1 でビルドし、ネイティブの行は `darwin/arm64` です。

| ビルド | `encoding/json` | `jsonbind` | 削減 |
|--------|-----------------|------------|------|
| `go build` | 3,075,666 | **2,565,106** | −511 KB（−16.6%） |
| `go build -ldflags="-s -w"` | 2,061,138 | **1,708,034** | −353 KB（−17.1%） |
| `tinygo build`（ネイティブ） | 474,256 | **293,632** | −181 KB（−38.1%） |
| `tinygo build`（ネイティブ）+ `strip` | 287,856 | **187,968** | −100 KB（−34.7%） |
| `tinygo build -target wasi` | 1,264,464 | **738,966** | −525 KB（−41.6%） |
| `tinygo build -target wasi -no-debug` | 488,762 | **222,564** | −266 KB（−54.5%） |

strip すると差は縮むどころか効いてきます。デバッグ情報が落ちた後は、残りに占める reflection 機構の割合が上がるためです。strip した TinyGo wasm ビルドではバイナリの約半分がそれにあたります。

wasm とネイティブで strip のかかり方が違うのは、デバッグ情報の置き場所が違うためです。wasm バイナリは DWARF を同梱していて、`-no-debug` が取り除くのはそれです。Mach-O バイナリはそもそも DWARF を持たず（macOS では別ファイルの dSYM に分離されます）、`-no-debug` を付けても何も変わりません。ネイティブでこのフラグに相当するのは、シンボルテーブルを落とす `strip` です。

### encoding/json/v2

`encoding/json/v2` は Go 1.26 でもまだ `GOEXPERIMENT=jsonv2` の裏にあり、ライブラリから無条件に import することはできません。それでも計測したのは、「生成 codec は v2 を対象にすべきではないか」という当然の疑問があるからです。

```bash
GOEXPERIMENT=jsonv2 go test ./internal/benchfixture -run xxx -bench JSON -benchmem
```

| 経路 | v1・フラグなし | v1・フラグあり | v2 API | 生成コード |
|------|----------------|----------------|--------|------------|
| decode（`io.Reader`） | 3543 ns · 1688 B · 30 | 2536 ns · 1889 B · 18 | 1650 ns · 544 B · 11 | **799 ns · 856 B · 15** |
| decode（バイト列が手元にある） | 3352 ns · 888 B · 25 | 1871 ns · 496 B · 10 | 1525 ns · 496 B · 10 | — |
| encode | 587 ns · 144 B · 1 | 1330 ns · 1824 B · 11 | 943 ns · 288 B · 2 | **274 ns · 0 B · 0** |

フラグを立てるだけで decode は実際に改善します。v1 API が v2 の上に再実装されているためです。ただし encode の行を見てください。2.3倍遅く、メモリは12倍になります。フラグはどちらに転んでもタダではありません。

v2 のトークナイザ `jsontext` を対象にコード生成する案が本命候補でした。同じキー switch を `ReadToken` で駆動する形にすると、decoder を使い回した場合のアロケーションは13回 — `jsonbind.Parser` とちょうど同じ — になりますが、所要は 1320 ns です。codec のエントリポイントがそうせざるを得ないように呼び出しごとに decoder を作る場合は 1804 ns · 1600 B · 38 allocs になります。

決め手はサイズです。同じ小さなプログラムで、experiment のコストは次のとおりです。

| ビルド | フラグなし | フラグあり |
|--------|------------|------------|
| `go build` | 3,075,522 | 3,887,730（+26%） |
| `go build -ldflags="-s -w"` | 2,061,010 | 2,598,722（+26%） |
| `tinygo build -target wasi` | 1,345,144 | 2,217,774（+65%） |
| `tinygo build -target wasi -no-debug` | 496,869 | 881,891（+78%） |

experiment を有効にした strip 済み wasm ビルドは、同じプログラムを `jsonbind` で作った場合の3.5倍のサイズです。TinyGo を第一級ターゲットとするライブラリにとって、これは v2 を依存に取れないという結論を意味します。速度の列にも、build tag で第二の実装を抱えてまで取りにいく理由は見当たりません。

## TinyGo

生成バインディングコードは TinyGo を第一級の対象とします。JSON runtime は `net/http` から独立しており、TinyGo の HTTP 標準ライブラリ経路が使えない js/wasm でも利用できます。

検証済み: **TinyGo 0.41.1 + Go 1.26.x**。

```bash
./scripts/tinygo-check.sh
```

### TinyGo 関連のランタイム注意

- `AsHTTPError` は `errors.As` を使わない（一部 TinyGo で `AssignableTo` 未実装のため）。
- `WriteError` は problem JSON を手組み（`encoding/json` と RawMessage の組み合わせの脆さを避ける）。
- レジストリの `reflect.Type` は **型の識別キー**のみで、フィールド走査には使わない。
- 生成される bind/write コードは `reflect` を import しない。
- `jsonbind` は JSON の解析と出力を自前で行い `encoding/json` を import しない。JSON だけを扱うバイナリに reflect ベースの codec が載らないので、`tinygo build -target wasi` なら約4割、`-no-debug` 付きなら約半分が削れる。[ベンチマーク](#バイナリサイズ)を参照。
- `GOEXPERIMENT=jsonv2` を付けてビルドしないこと。Go 1.26 でも `encoding/json/v2` は experiment の裏にあり、TinyGo では同じ wasi バイナリが約60%膨らむ。`jsonbind` はそもそも呼ばない。

### 既知の制限

| 項目 | 制限 |
|------|------|
| ツールチェイン | プロジェクト基準は TinyGo 0.41.1 + Go 1.26.x |
| js/wasm HTTP | TinyGo 0.41.1 + Go 1.26.x は `net/http/roundtrip_js.go` 内で失敗するため、HTTP 不要の WASM では `jsonbind` を使う |
| ストリーミング | `WriteStream` はホストの `go test` を推奨。TinyGo 行列は未整備 |
| ServeMux | `DefaultOptions` は `net/http.ServeMux` と `tinygodriver/httpmux.ServeMux` の両方を探索。TinyGo で Go 1.22 のメソッド・ワイルドカードルーティングを使う場合は `httpmux` を利用 |
| Multipart `File` | `httpbind.File`（`payload`）で対応。サイズ/MIME の `check` は未対応。ボディ上限のデフォルトは **1 MiB**（`SetMaxMultipartBodyBytes`） |
| SQLマッピング | `ScanRows` と生成SQLスキャナはホストGo向けで、TinyGoビルドから除外 |
| ジェネレータ | ホスト側のみ（`go run` / `go test`） |

## ライセンス

[Apache License, Version 2.0](LICENSE) の下で提供します。
