# tinybind-go

[English](README.md)

TinyGo と通常 Go のための、リフレクション不要・コード生成ファーストのバインディングライブラリです。HTTP・JSON・SQL・DynamoDB のランタイム依存を別パッケージに分離しています。

利用ガイド: [httpbind](docs/httpbind.ja.md) · [jsonbind](docs/jsonbind.ja.md) · [configbind](docs/configbind.ja.md) · [htmlbind](docs/htmlbind.ja.md) · [sqlbind](docs/sqlbind.ja.md) · [dynamobind](docs/dynamobind.ja.md)

この上にフレームワークを作る方へ: まず [フレームワーク向け機能一覧](docs/httpbind_framework_facilities.ja.md)（何が使えて何が無いかの索引）、次に [htmlbind フレームワーク実装者向けガイド](docs/htmlbind_frameworkowner.ja.md)

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
stream, err := httpbind.NewStream[ChatEvent](w, r)
if err != nil {
    httpbind.WriteError(w, r, err)
    return
}
defer stream.Close()

_ = stream.Write(ChatEvent{Type: "delta", Delta: "hi"})
_ = stream.Write(ChatEvent{Type: "done"})
```

- **`Write` は何度でも呼べる**（インクリメンタルなイベント送出）。
- 形式は `NewStream` で一度だけ決定（`?stream=` → `Accept` → `User-Agent` → 既定 **NDJSON**）。
- 形式:
  - **SSE** — `text/event-stream`
  - **NDJSON / JSONL** — `application/x-ndjson`（1 行 1 オブジェクト。**JSON 配列ではない**）
  - **JSON array** — `application/json` の `[obj1,obj2,...]`（末尾の `]` は `Close` が書く）
- 削除済みの `WriteNDJSON` / `WriteSSE` は使わない。

## パッケージ構成

| パス | 役割 |
|------|------|
| `.`（`package httpbind`） | ランタイム: Bind / Write / WriteError / NewStream / OpenAPI 配信 / SwaggerUI |
| `jsonbind/` | 単独の DecodeJSON / EncodeJSON。`net/http` と `database/sql` を import しない |
| `sqlbind/` | ScanRows と行変換ヘルパ。`net/http` を import しない |
| `dynamobind/` | `tinygodriver/nosql/dynamodb` 上の DynamoDB item runtime。`net/http` も `database/sql` も import しない |
| `generator/` | フィールド計画に基づくバインダ／ライタ + OpenAPI 3.1 埋め込み生成 |
| `parser/` | ルート／ハンドラ発見（`Bind`、`Write`、`NewStream`、エラー） |
| `templates/htmlbind/` | 型付きで文脈安全な HTML template compiler |
| `templates/sqlbind/` | 型付き parameterized SQL template compiler |
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

## デモ

```bash
go generate ./examples/demo
go run ./examples/demo
# http://localhost:8080/       インデックス + ブラウザ向けストリーム demo
# http://localhost:8080/docs/  Swagger UI
# http://localhost:8080/chat   NewStream（SSE / NDJSON / JSON array 自動）
```

curl 例の詳細は [`examples/demo/README.md`](examples/demo/README.md) を参照してください。

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
- JSON-only 生成コードは `jsonbind` だけを import し、`tinygo build -target wasm` で検証する。

### 既知の制限

| 項目 | 制限 |
|------|------|
| ツールチェイン | プロジェクト基準は TinyGo 0.41.1 + Go 1.26.x |
| js/wasm HTTP | TinyGo 0.41.1 + Go 1.26.x は `net/http/roundtrip_js.go` 内で失敗するため、HTTP 不要の WASM では `jsonbind` を使う |
| ストリーミング | `NewStream` はホストの `go test` を推奨。TinyGo 行列は未整備 |
| ServeMux | `DefaultOptions` は `net/http.ServeMux` と `tinygodriver/httpmux.ServeMux` の両方を探索。TinyGo で Go 1.22 のメソッド・ワイルドカードルーティングを使う場合は `httpmux` を利用 |
| Multipart `File` | `httpbind.File`（`payload`）で対応。サイズ/MIME の `check` は未対応。ボディ上限のデフォルトは **1 MiB**（`SetMaxMultipartBodyBytes`） |
| SQLマッピング | `ScanRows` と生成SQLスキャナはホストGo向けで、TinyGoビルドから除外 |
| ジェネレータ | ホスト側のみ（`go run` / `go test`） |

## ライセンス

[Apache License, Version 2.0](LICENSE) の下で提供します。
