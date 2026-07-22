# jsonbind 利用ガイド

`jsonbind` は HTTP に依存せず、Go の構造体と JSON 文書を相互変換するパッケージです。`io.Reader` / `io.Writer` だけを使うため、CLI、ファイル、メッセージキュー、WASM などでも利用できます。

## 自動化されること

- JSON object から型付き構造体への decode
- 型付き構造体から JSON への encode
- ネストした構造体、slice、map の変換
- フィールドごとの JSON 型エラー
- JSON document の読み込み上限
- 実際に `DecodeJSON[T]` / `EncodeJSON[T]` で使われた型だけのコード生成

`jsonbind` 自体は HTTP status や HTTP header を扱いません。HTTP request / response が必要なら [httpbind](httpbind.ja.md) を使います。

## ユーザーが用意するもの

1. JSON に対応する Go の構造体
2. `jsonbind.DecodeJSON[T]` または `EncodeJSON[T]` の具体的な呼び出し
3. コード生成の実行
4. 入出力となる `io.Reader` / `io.Writer`

## 導入とコード生成

```go
package document

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

```bash
go generate ./...
```

ジェネレーターは generic の型引数を調べます。decode だけを使う型には decoder、encode だけを使う型には encoder が生成されます。

## 基本例

```go
package document

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

type User struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Active bool     `json:"active"`
	Tags   []string `json:"tags"`
}

func decodeExample() error {
	in := strings.NewReader(`{
  "id": 1,
  "name": "Ada",
  "active": true,
  "tags": ["admin", "author"]
}`)

	user, err := jsonbind.DecodeJSON[User](in)
	if err != nil {
		return err
	}
	fmt.Println(user.Name)
	return nil
}

func encodeExample(user User) (string, error) {
	var out bytes.Buffer
	if err := jsonbind.EncodeJSON(&out, user); err != nil {
		return "", err
	}
	return out.String(), nil
}
```

## 対応するモデル

主に次の組み合わせを利用できます。

- `string`
- `int`
- `int64`
- `bool`
- `float64`
- 上記の slice
- ネストした構造体
- 構造体の slice
- `map[string]string` など scalar の map
- `map[string]Struct`

```go
type Address struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

type Profile struct {
	Name       string             `json:"name"`
	Address    Address            `json:"address"`
	History    []Address          `json:"history"`
	Labels     map[string]string  `json:"labels"`
	AddressBy  map[string]Address `json:"addressBy"`
}

func use(r io.Reader, w io.Writer) error {
	profile, err := jsonbind.DecodeJSON[Profile](r)
	if err != nil {
		return err
	}
	return jsonbind.EncodeJSON(w, profile)
}
```

ワイヤ名を明示しないフィールドは lower camel case になります。`DecodeJSON` では、リクエスト入力元を表す `query`、`path`、`header`、`cookie` のフィールドは decode 対象外です。一方、`EncodeJSON` は構造体のフィールドを出力対象にするため、JSON 専用モデルでは標準の `json` タグだけを使うのが分かりやすい設計です。

現在の生成 codec が利用するのは `json` タグのフィールド名部分です。`omitempty` と `json:"-"` による省略は適用されないため、field はすべて出力される前提でモデルを設計してください。

## 未知のフィールドを保持する

既知のフィールド以外を `payload:"*"` で集められます。

```go
type Envelope struct {
	Kind  string         `json:"kind" payload:"kind"`
	Extra map[string]any `payload:"*"`
}
```

JSON 値を decode せず保持したい場合は `json.RawMessage` を使います。

```go
type RawEnvelope struct {
	Kind  string                     `json:"kind" payload:"kind"`
	Extra map[string]json.RawMessage `payload:"*"`
}
```

`Extra` に入るのは、明示的に宣言したフィールドを除いたプロパティです。

## 読み込み上限

既定の上限は 1 MiB です。

アプリ全体の上限を変更する例:

```go
func init() {
	jsonbind.SetMaxJSONBodyBytes(4 << 20) // 4 MiB
}
```

1回の呼び出しだけ上限を変更する例:

```go
doc, err := jsonbind.DecodeJSONLimit[Document](reader, 64<<10) // 64 KiB
```

`DecodeJSONLimit` に 0 以下を渡すと、アプリ全体の上限を使用します。

## エラー処理

`jsonbind` のエラーは transport-neutral です。HTTP status へは変換しません。

```go
doc, err := jsonbind.DecodeJSON[Document](reader)
if err != nil {
	if jsonErr, ok := jsonbind.AsError(err); ok {
		log.Printf("code=%s field=%s message=%s",
			jsonErr.Code,
			jsonErr.Field,
			jsonErr.Message,
		)
	}
	return err
}
```

代表的な code は次のとおりです。

| code | 意味 |
| --- | --- |
| `json_parse` | JSON 構文、object / array、値の型が不正 |
| `json_field` | 特定フィールドの値が不正 |
| `payload_too_large` | 読み込み上限を超えた |
| `body_read` | reader からの読み込みに失敗 |
| `internal` | nil writer など呼び出し側の問題 |

`httpbind.Bind` 経由で発生した JSON エラーは、HTTP 用の validation / bad request / payload too large エラーへ変換されます。

## ファイルの読み書き

```go
func load(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	return jsonbind.DecodeJSON[Config](f)
}

func save(path string, value Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jsonbind.EncodeJSON(f, value)
}
```

## HTTP なしの生成を保つ

JSON だけのパッケージでは root の `httpbind` を import せず、`jsonbind.DecodeJSON` / `EncodeJSON` を直接呼びます。すると生成物も `jsonbind` 用だけになり、`net/http` への依存を持ちません。TinyGo / WASM 向けコードでは特にこの分離が有効です。

## よくある生成漏れ

型を wrapper の外側から動的に渡すだけでは、ジェネレーターが具体的な型を見つけられないことがあります。解析対象パッケージに具体的な呼び出しを置いてください。

```go
func DecodeUser(r io.Reader) (User, error) {
	return jsonbind.DecodeJSON[User](r)
}
```

`jsonbind: no generated decoder` または encoder 相当のエラーが出た場合は、対象型の呼び出しが同じパッケージにあり、生成後のファイルがビルド対象に含まれているかを確認します。
