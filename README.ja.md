# tinybind-go

[English](README.md)

TinyGo と通常 Go のための、リフレクション不要・コード生成ファーストのバインディングライブラリです。HTTP・JSON・SQL のランタイム依存を別パッケージに分離しています。

利用ガイド: [httpbind](docs/httpbind.ja.md) · [jsonbind](docs/jsonbind.ja.md) · [htmlbind](docs/htmlbind.ja.md) · [sqlbind](docs/sqlbind.ja.md)

リクエスト／レスポンスの構造体を一度定義するだけで、ジェネレータが型専用のバインダとライタを出力します。同じモデルで **JSON・form・multipart・query**（タグにより path / header / cookie も）を扱えます。レスポンスはクライアントの **`Accept`** に合わせて適応します（ストリーミング時は content negotiation も）。同じ解析結果から **OpenAPI 3.1 も生成**し、バインダ／ライタと常に同期します。ルート登録は別 DSL ではなく、実際の **`net/http` の書き方を静的解析**して発見します（`HandleFunc`、`Handle`、メソッド値、ラッパーなど）。

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
go run ./cmd/tinybind-gen -dir . -openapi
```

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
| `generator/` | フィールド計画に基づくバインダ／ライタ + OpenAPI 3.1 埋め込み生成 |
| `parser/` | ルート／ハンドラ発見（`Bind`、`Write`、`NewStream`、エラー） |
| `cmd/tinybind-gen` | CLI: パッケージ dir からバインダ + OpenAPI を生成 |
| `examples/demo` | 一通り触れるサンプルアプリ |
| `internal/*` | テスト用フィクスチャ |
| `testdata/cmd/*` | 開発用ヘルパ（配布対象外。`testdata` 配下のため `go get` / `./...` の対象外） |

```bash
go run ./cmd/tinybind-gen -dir ./path/to/package
```

独自ジェネレータのコマンドは `generator.Main` を呼ぶだけで作れます。
`DefaultOptions` から始め、プロジェクトで許可する探索先を各 `Set` にすべて
列挙します。

```go
package main

import "github.com/shibukawa/tinybind-go/generator"

func main() {
    options := generator.DefaultOptions()
    options.ServeMuxes.Set = []generator.TypePattern{
        {PackagePath: "net/http", Name: "ServeMux"},
        {PackagePath: "github.com/shibukawa/petitweb-go/handler", Name: "ServeMux"},
    }
    options.RuntimePackages.Set = []string{
        "github.com/shibukawa/tinybind-go",
        "github.com/shibukawa/tinybind-go/jsonbind",
        "github.com/shibukawa/tinybind-go/sqlbind",
        "github.com/shibukawa/petitweb-go/handler",
    }
    generator.Main(options)
}
```

`RuntimePackages` は同名の `Bind`、`Write`、`WriteStatus`、`DecodeJSON`、
`EncodeJSON`、`NewStream`、`ScanRows` を展開します。操作単位の
`options.DecodeJSON.Set` などを指定すると、その操作だけ展開結果を置換します。
`Set` は追加ではなく完全置換なので、標準と互換パッケージの両方が必要なら
両方を列挙します。`generator.Options{}` の探索先は空です。各パターンの
`Disabled` または `DisableFeatures` で、`-generate-all` 使用時も機能を
無効化できます。

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
