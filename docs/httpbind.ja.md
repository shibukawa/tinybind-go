# httpbind 利用ガイド

`httpbind` は `net/http` のハンドラーで、HTTP リクエストを Go の構造体へ変換し、構造体を JSON 応答として書き出すためのパッケージです。同じ解析結果から OpenAPI 3.1 文書も生成します。

## 自動化されること

- query、JSON、form、multipart、path、header、cookie から構造体への変換
- 文字列から `int`、`int64`、`bool`、`float64` への変換
- `check` タグによる入力検証とデフォルト値の適用
- 構造体から JSON レスポンスへの変換
- バインド・検証エラーから RFC 9457 Problem Details レスポンスへの変換
- `net/http` のルート登録とハンドラー利用型の静的な発見
- ルート、入力、出力、エラーを反映した OpenAPI 3.1 文書の生成
- SSE、NDJSON、JSON array のストリーミング形式選択

別のルート DSL やスキーマ定義は不要です。通常の Go の型、`net/http` の登録コード、ハンドラー内の `Bind` / `Write` 呼び出しが入力になります。

## ユーザーが用意するもの

1. リクエストとレスポンスの構造体
2. `httpbind.Bind`、`Write` などを使う `net/http` ハンドラー
3. `http.ServeMux` などへのルート登録
4. コード生成の実行方法
5. 実際の業務処理、認証、DB アクセスなど

## 導入とコード生成

対象パッケージに生成指示を置く例です。

```go
package api

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

```bash
go generate ./...
```

既定では次のファイルが生成されます。

- `tinybind_gen.go` — 利用されている型の HTTP / JSON バインディング
- `tinybind_openapi_gen.go` — OpenAPI JSON / YAML の埋め込み
- `tinybind_templates_gen.go` — 同じディレクトリにテンプレートがある場合

CI では、静的解析できないルート候補も失敗として扱う `-check` が便利です。

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir . -check
```

ジェネレーターは対象ディレクトリの1つの Go パッケージを解析します。別パッケージにあるハンドラーの中までは追跡しないため、ルート登録と解析対象ハンドラーは同じパッケージに置くのが基本です。

## 最小の API

```go
package api

import (
	"net/http"

	httpbind "github.com/shibukawa/tinybind-go"
)

type HelloRequest struct {
	Name string `query:"name" check:"required"`
}

type HelloResponse struct {
	Message string `json:"message"`
}

func hello(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[HelloRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}

	out := HelloResponse{Message: "Hello, " + in.Name}
	if err := httpbind.Write(w, r, out); err != nil {
		httpbind.WriteError(w, r, httpbind.Internal(err))
	}
}

func Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", hello)
	return mux
}
```

```bash
curl 'http://localhost:8080/hello?name=Ada'
# {"message":"Hello, Ada"}
```

## リクエストフィールドの入力元

タグを省略したフィールドは `input` として扱われます。ワイヤ上の名前は、タグがなければ lower camel case です。たとえば `DisplayName` は `displayName` になります。

| タグ | 入力元 | 用途 |
| --- | --- | --- |
| タグなし / `input:"name"` | query または body | 一般的な入力。query に値があれば query を優先 |
| `query:"page"` | query のみ | 検索条件、ページ番号など |
| `payload:"name"` | body のみ | JSON / form / multipart に限定したい値 |
| `path:"id"` | パス値 | `GET /users/{id}` のような値 |
| `header:"Authorization"` | HTTP ヘッダー | 認証情報など |
| `cookie:"session"` | cookie | セッション ID など |
| `method:"method"` | HTTP method | `GET`、`POST` などを文字列で受ける |

`input` のスカラー値は query を先に調べ、存在しないときだけ body を読みます。ネストした構造体、slice、map は body から読みます。入力元を曖昧にしたくない API では `query` と `payload` を明示してください。

```go
type SearchRequest struct {
	Keyword string `query:"keyword" check:"required"`
	Page    int    `query:"page" check:"min=1,default=1"`
	Filter  string `payload:"filter"`
}
```

```bash
curl 'http://localhost:8080/search?keyword=go&page=2' \
  -H 'Content-Type: application/json' \
  -d '{"filter":"active"}'
```

### 対応する body

- `application/json`
- `application/x-www-form-urlencoded`
- `multipart/form-data`

```go
type CreateUserRequest struct {
	Name  string `payload:"name"`
	Email string `payload:"email"`
}
```

同じ型を JSON または form で送れます。

```bash
curl http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada","email":"ada@example.com"}'

curl http://localhost:8080/users \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'name=Ada' \
  --data-urlencode 'email=ada@example.com'
```

### path、header、cookie

```go
type GetUserRequest struct {
	ID      string `path:"id" check:"required,uuid"`
	Token   string `header:"Authorization" check:"required"`
	Session string `cookie:"session"`
}

func getUser(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[GetUserRequest](r)
	// ...
}

mux.HandleFunc("GET /users/{id}", getUser)
```

### multipart ファイル

ファイルは `httpbind.File` と `payload` タグで受けます。

```go
type UploadRequest struct {
	Title string        `payload:"title" check:"required"`
	Image httpbind.File `payload:"image"`
}

func upload(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[UploadRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}

	_ = in.Image.Filename
	_ = in.Image.ContentType
	_ = in.Image.Size
	_ = in.Image.Content
}
```

```bash
curl http://localhost:8080/uploads \
  -F 'title=avatar' \
  -F 'image=@avatar.png'
```

multipart body の既定上限は 1 MiB です。アプリ起動時に必要量へ変更できます。

```go
httpbind.SetMaxMultipartBodyBytes(8 << 20) // 8 MiB
```

### 未宣言フィールドをまとめて受ける

JSON / form の既知フィールド以外を `payload:"*"` で収集できます。

```go
type EventRequest struct {
	Type   string         `payload:"type"`
	Extras map[string]any `payload:"*"`
}
```

生の JSON が必要なら `map[string]json.RawMessage` を使います。

```go
type EventRequest struct {
	Type   string                     `payload:"type"`
	Extras map[string]json.RawMessage `payload:"*"`
}
```

## 入力チェック

`check` はコード生成時に解釈され、実行時には生成済みの検証処理が動きます。

| ルール | 対象 | 例 |
| --- | --- | --- |
| `required` | 入力の存在。string は空文字、file は空内容も拒否 | `check:"required"` |
| `default=value` | scalar | `check:"default=1"` |
| `min` / `max` | 数値 | `check:"min=1,max=100"` |
| `minlen` / `maxlen` / `len` | string | `check:"minlen=3,maxlen=64"` |
| `enum=a\|b` | scalar | `check:"enum=asc\|desc"` |
| `pattern=...` | string | `check:"pattern=^[A-Z]{3}$"` |
| `email` | string | `check:"email"` |
| `uuid` | string | `check:"uuid"` |
| `date` | string | `YYYY-MM-DD` |
| `time` | string | `HH:MM:SS` |
| `datetime` | string | RFC 3339 |

`pattern` はカンマを含む正規表現を区切れないため、タグ内の最後に置きます。

```go
type CreateAccountRequest struct {
	Name     string `check:"required,minlen=1,maxlen=64"`
	Email    string `check:"required,email,maxlen=254"`
	Age      int    `check:"min=0,max=150"`
	Plan     string `check:"enum=free|pro,default=free"`
	PostCode string `check:"pattern=^[0-9]{3}-[0-9]{4}$"`
}
```

デフォルト値は「値が送られなかったとき」に検証後に適用されます。このため `check:"min=1,default=-1"` は、未指定なら `-1`、明示的に `-1` を送れば検証エラー、という sentinel 用途にも使えます。

非 pointer の数値や bool は、Go のゼロ値だけから「未指定」と「明示的な 0 / false」を区別できない場面があります。存在自体を必須にする設計では、この制約を考慮してください。

## レスポンス

### 200 JSON

```go
type UserResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

err := httpbind.Write(w, r, UserResponse{ID: "u_1", Name: "Ada"})
```

`Write` は `200 OK` と `application/json` を書きます。

`json` タグの名前部分は利用できますが、現在の生成 encoder は `omitempty` と `json:"-"` による省略を適用しません。レスポンス型の field はすべて出力される前提で設計してください。

### 200 以外の成功

```go
err := httpbind.WriteStatus(
	w,
	r,
	http.StatusCreated,
	UserResponse{ID: "u_1", Name: "Ada"},
)
```

`204 No Content` を指定した場合は body を書きません。

## エラー応答

業務処理では HTTP 用エラーを返し、ハンドラー境界で `WriteError` に渡します。

```go
func findUser(id string) (UserResponse, error) {
	if id == "" {
		return UserResponse{}, httpbind.BadRequest(httpbind.Problem{
			Code:    "missing_id",
			Message: "id is required",
		})
	}
	return UserResponse{}, httpbind.NotFound(httpbind.Problem{
		Code:    "user_not_found",
		Message: "user was not found",
	})
}
```

用意されている constructor は次のとおりです。

- `BadRequest` — 400
- `Unauthorized` — 401
- `Forbidden` — 403
- `NotFound` — 404
- `Conflict` — 409
- `PayloadTooLarge` — 413
- `Internal` — 500
- `Validation` — 400、フィールド別の詳細付き

```go
err := httpbind.Validation(
	httpbind.Field("email", "payload", "already registered"),
)
httpbind.WriteError(w, r, err)
```

クライアントには `application/problem+json` が返ります。5xx では内部原因や内部メッセージはレスポンスに公開されません。

## ストリーミング

```go
type ChatEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
}

func chat(w http.ResponseWriter, r *http.Request) {
	stream, err := httpbind.NewStream[ChatEvent](w, r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	defer stream.Close()

	if err := stream.Write(ChatEvent{Type: "delta", Delta: "hello"}); err != nil {
		return
	}
	_ = stream.Write(ChatEvent{Type: "done"})
}
```

形式は `NewStream` の時点で一度だけ決まります。

1. `?stream=`
2. `Accept` ヘッダー
3. User-Agent
4. 既定の NDJSON

```bash
# SSE
curl -N 'http://localhost:8080/chat?stream=sse'

# NDJSON
curl -N -H 'Accept: application/x-ndjson' http://localhost:8080/chat

# 1つの JSON array 文書
curl -H 'Accept: application/json' http://localhost:8080/chat
```

JSON array は閉じ `]` を `Close` が書くため、必ず `defer stream.Close()` を設定してください。

## OpenAPI と Swagger UI

ジェネレーターは、発見したルート、`Bind` の型、`Write` / `WriteStatus` / `NewStream` の型、HTTP エラーを OpenAPI に反映します。

```go
mux.HandleFunc("GET /openapi.json", httpbind.OpenAPIJSON)
mux.HandleFunc("GET /openapi.yaml", httpbind.OpenAPIYAML)
mux.Handle("GET /docs/{$}", httpbind.SwaggerUI("/openapi.json"))
```

Swagger UI のアセットは CDN から読み込まれます。オフライン環境では OpenAPI JSON / YAML の配信だけを使うか、別途 UI をホストしてください。

## よくある生成漏れ

ジェネレーターは通常、ソース中の具体的な generic 呼び出しから対象型を発見します。

```go
httpbind.Bind[CreateUserRequest](r)
httpbind.Write[CreateUserResponse](w, r, out)
```

呼び出しが別パッケージに隠れている、独自 wrapper 経由でしか使わない、静的に型を特定できない場合は発見できません。まず `-check` の診断を確認してください。全構造体の全マッピングを生成する必要がある場合は `-generate-all` を使えますが、通常は利用箇所を直接記述する方法が小さな生成物になります。
