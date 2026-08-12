# jsonbind 利用ガイド

`jsonbind` は Go の構造体と JSON 文書を相互変換します。しかも `net/http` には一切触れません。API は `io.Reader` / `io.Writer` だけなので、CLI、ファイル読み書き、メッセージキューの消費側、そして HTTP 依存が単なる重荷になる WASM ビルドでも使えます。

## 自動化されること

- JSON object から型付き構造体への decode
- 型付き構造体から JSON への encode
- ネストした構造体、slice、map の変換
- フィールドごとの JSON 型エラー
- JSON document の読み込み上限
- 実際に `DecodeJSON[T]` / `EncodeJSON[T]` で使われた型だけのコード生成

HTTP status の選択や header の設定は、この境界の外側です。これらは [httpbind](httpbind.ja.md) の担当で、同じ codec の上に request / response の関心事を載せています。

## ユーザーが用意するもの

1. JSON に対応する Go の構造体
2. `jsonbind.DecodeJSON[T]` または `EncodeJSON[T]` の具体的な呼び出し
3. コード生成の実行
4. 入出力となる `io.Reader` / `io.Writer`

## 導入とコード生成

```go
package document

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

```bash
go generate ./...
```

ジェネレーターは generic の型引数を調べるため、生成物は実際の呼び出し方をそのまま反映します。decode だけを使う型には decoder、encode だけを使う型には encoder。呼び出していない codec は生成されません。

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
	Name      string             `json:"name"`
	Address   Address            `json:"address"`
	History   []Address          `json:"history"`
	Labels    map[string]string  `json:"labels"`
	AddressBy map[string]Address `json:"addressBy"`
}

func use(r io.Reader, w io.Writer) error {
	profile, err := jsonbind.DecodeJSON[Profile](r)
	if err != nil {
		return err
	}
	return jsonbind.EncodeJSON(w, profile)
}
```

ワイヤ名を明示しないフィールドは lower camel case になります。`DecodeJSON` は、リクエスト入力元を表す `query`、`path`、`header`、`cookie` のフィールドを decode 対象から外します。`EncodeJSON` はその区別をせず、構造体のフィールドをそのまま出力します。したがって JSON 専用モデルは、標準の `json` タグだけを持たせるのがもっとも見通しの良い設計です。

### タグのオプション

`json` タグは名前とそれに続くオプションからなり、codec は両方を読みます。名前が `-` のときは、そのフィールドをドキュメントから両方向で外します。出力もされず、その名前で届いたメンバーが読み込まれることもありません。オプションは2つあり、いずれもメンバーを書くかどうかを決めます。

- **`omitempty`** は、そのメンバーが空の JSON 値（`""`、`[]`、`{}`）になるときに落とします。`encoding/json` ではなく `encoding/json/v2` の解釈なので、`0` と `false` は書かれます。数値・真偽値・ネストしたオブジェクトには空の形が無く、このオプションは効きません。
- **`omitzero`** は、フィールドが Go のゼロ値のときに落とします。`0` や `false` に届くのはこちらです。ネストした構造体は、全フィールドがゼロのときにゼロとみなします。

両者が分かれるのは「空だがゼロではない」値です。`map[string]string{}` は nil ではないので `omitzero` は `{}` を書き、`omitempty` は落とします。`[]string(nil)` はどちらでもあるので、どちらでも落ちます。両方を書けば、どちらかが当てはまった時点でメンバーは出力されません。

codec が知らないオプションは、黙って何もしないタグではなく生成エラーになります。`omitempy` と綴り間違えたタグは、出力を突き合わせるまで正しいものと見分けがつかないからです。

## `encoding/json` とのワイヤ表現の差

生成 codec はドキュメントを前方に一度だけ走査して読み、書き出しはバッファへの append で行います。中間マップを組み立てることも、構造体を reflect で覗くこともありません。出力を `encoding/json` と突き合わせる前に、次の4点を把握しておいてください。

- **メンバーは構造体のフィールド順で出力されます**。名前順ではありません。マップ型フィールドと `payload:"*"` の rest マップだけは、辿るべき宣言順がないため名前順のままです。
- **フィールド名は完全一致で照合します**。`encoding/json` は大文字小文字を無視した照合にフォールバックするため `{"userId": …}` を `userid` フィールドに束縛しますが、この codec はしません。`encoding/json/v2` も同じ方向に舵を切っています。
- **重複したキーは捨てられずに decode されます**。`encoding/json` は最後の1つだけを残し、それ以前は見もしないので、型の合わない重複は黙って通り抜けます。ここでは出現順にすべて decode するので、不正なものはフィールドエラーになります。
- **nil のスライスは `[]`、nil のマップは `{}` で出力します**。`encoding/json` はどちらにも `null` を書くので、Go の型が引いていない線引きがクライアントに渡ります。Go 側で「項目がない」と「空のリスト」が区別されない以上、ワイヤ上でも区別されるべきではありません。`encoding/json/v2` は空の配列と空のオブジェクトを書きます。この codec も同じです。

文字列のエスケープと数値の書式は、`encoding/json` とバイト単位で一致します。ページに埋め込んでも安全なように `<`、`>`、`&` を HTML エスケープする挙動も含めて同じです。唯一の例外は不正な UTF-8 で、`\ufffd` のエスケープ形式で出力します。これは既定のエンコーダの挙動で、`GOEXPERIMENT=jsonv2` 下の `encoding/json` は置換文字をそのまま書きます。decode 結果はどちらも同じ文字列です。

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

`DecodeJSONLimit` に 0 以下を渡すと、アプリ全体の上限に戻ります。

## エラー処理

`jsonbind` のエラーは transport-neutral で、HTTP status を含意しません。どのエラーも code を持ち、フィールド単位の失敗ならその原因となったフィールド名も持ちます。

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

status の判断は捨てられたのではなく、先送りされているだけです。`httpbind.Bind` 経由で JSON を decode すると、同じエラーが HTTP 用の validation / bad request / payload too large エラーになります。

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

依存関係を決めるのは import パスです。JSON だけのパッケージでは root の `httpbind` を経由せず、`jsonbind.DecodeJSON` / `EncodeJSON` を直接呼びます。すると生成物が参照するのも `jsonbind` だけになり、`net/http` を引き込みません。推移的な依存がそのままバイナリサイズになる TinyGo / WASM 向けでは、この分離を意識的に守る価値があります。

## よくある生成漏れ

生成は何も言わずに通ったのに、実行時に decoder も encoder も無いと言われる。よくある原因は、その型が generic の wrapper 越しにしか `DecodeJSON` に届いていないことです。ジェネレーターからは具体的な型ではなく型パラメータしか見えていません。解析対象パッケージに具体的な呼び出しを置いてください。

```go
func DecodeUser(r io.Reader) (User, error) {
	return jsonbind.DecodeJSON[User](r)
}
```

それでもエラーが消えない場合は、2 点を確認します。その具体的な呼び出しがジェネレーターの解析したパッケージにあるか、そして生成後のファイルがビルド対象に含まれているかです。
