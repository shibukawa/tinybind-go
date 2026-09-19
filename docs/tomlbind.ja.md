# tomlbind ガイド

`tomlbind` は TOML ドキュメントを1つ、構造体にデコードします。
[configbind](configbind.ja.md) の TOML 層だけを切り出したもので、環境変数も、コマ
ンドラインオプションも、設定ディレクトリの探索もなく、それらが必要とするパーサも
バイナリにリンクされません。

```go
settings, err := tomlbind.DecodeTOML[Settings](file)
```

ジェネレータがこの呼び出しを読み、デコーダを書き、型に `DecodeTOMLFrom` メソッド
を付けます。呼び出しを追加した直後の1回目のビルドは通りません(メソッドがまだ存在
しないため)。ジェネレータを走らせてからビルドし直してください。これは他の生成
モードと同じブートストラップで、その代わりに「デコーダが無い」が実行時の事故では
なく**コンパイルエラー**になります。

- [どちらを使うか](#どちらを使うか)
- [エントリポイント](#エントリポイント)
- [デコーダの宣言](#デコーダの宣言)
- [生成されるもの](#生成されるもの)
- [構造体タグ](#構造体タグ)
- [扱えるフィールド型](#扱えるフィールド型)
- [TOML ファイル](#toml-ファイル)
- [エラー](#エラー)
- [生成](#生成)

## どちらを使うか

| | tomlbind | configbind |
| --- | --- | --- |
| 読むもの | `io.Reader` かバイト列の TOML ドキュメント1つ | パスや設定ディレクトリで見つけたファイル、次に env、次に CLI |
| 重ね合わせ | なし | `default < TOML < env < CLI` で固定 |
| 文字列中の `${NAME}` | そのまま | 環境変数から展開 |
| provenance、scaffold、サブコマンド | なし | あり |
| リンクされるもの | `minitoml` のみ | `minitoml`、`cliparser`、`configpath`、`configdir`、`go-envparse` |

構造体は両方で同じものが使えます。`tomlbind` は configbind と同じ `key` と
`default` タグを読み、configbind が読み込むドキュメントは同じように束縛されるの
で、tomlbind から始めて、他のソースが必要になったら configbind に移れます。

## エントリポイント

```go
tomlbind.DecodeTOML[T](r io.Reader) (T, error)
tomlbind.DecodeTOMLBytes[T](data []byte) (T, error)
```

`DecodeTOML` は `r` を最後まで読みます。ドキュメントは起動時に一度読むものなので、
ストリーミングパーサはありません。サイズ上限は呼び出し側の `io.LimitReader` で
かけてください。どちらも新しい `T` を返します。デコードは値を置き換えるもので、既
存の値に重ねるものではありません。

呼び出し側が書くのは型引数だけです。エントリポイントはポインタ経由で生成メソッド
に制約をかけるので、デコーダの無い型は制約を満たさず、呼び出しがビルドに失敗しま
す。

## デコーダの宣言

生成はパッケージ単位で、メソッドは型に付くので、別パッケージからデコードされる型
は、その型のそばでデコーダを宣言します。

```go
package config

var _ = tomlbind.GenerateDecoder[Settings]()
```

この呼び出しは init 時に実行され、何もしません。宣言そのものが目的で、ジェネレー
タは他の呼び出しと同じように見つけ、型を所有するパッケージにメソッドを書きます。

別パッケージの型を名指しする宣言は、対処法付きの生成エラーになります。通常の
`DecodeTOML[other.T]` 呼び出しで別パッケージの型を名指しした場合はコンパイラに任
され、呼び出し箇所で制約を満たさないと報告されます。

## 生成されるもの

`DecodeTOML[Settings](r)` に対して、ジェネレータは `tomlbind_gen.go` に次を書きます。

```go
func decodeSettingsTOML(dst *Settings, doc minitoml.Document) error
func (v *Settings) DecodeTOMLFrom(data []byte) error
```

前者は `minitoml/codegen` が configbind 向けに出力する apply 関数を、ドキュメント
のルートを prefix にして出したものです。ネストした構造体と配列テーブルの要素はそ
の中にインライン展開されるので、メソッドが付くのは名指しした型だけです。後者は
`minitoml` でバイト列をパースし、`*v` をリセットして apply します。

登録も `init` も生成されません。生成ファイルの import は `fmt`、`minitoml`、そし
て duration フィールドがあるときの `time` です。

## 構造体タグ

| タグ | 意味 |
| --- | --- |
| `key:"name"` | フィールド名の snake_case と違うときの TOML キー |
| `default:"value"` | キーが無いときの値。TOML に書くのと同じ表記 |

configbind が読む他のタグ、`opt`、`env`、`help`、`enum`、`falsy`、`dependon`、
`summary`、`secret` は他のソースやレポートのためのもので、ここでは無視されます。
それらを付けた構造体もそのままデコードできます。

`default` が無く、ドキュメントにもキーが無いフィールドはゼロ値のままです。

## 扱えるフィールド型

`string`、`bool`、すべての幅の整数(`int` から `uint64`)、`float64` と
`float32`、`"250ms"` のような文字列からの `time.Duration`、`[]string`、テーブルと
してのネストした構造体、配列テーブルとしての同一パッケージ構造体のスライスです。

TOML の整数は float フィールドに拡張されます。整数フィールドに TOML の float を書
くとエラーで、フィールドの幅を超える値もエラーです。対応型のエイリアスはその型と
して扱われます。

`float64` と `float32` は tomlbind が受け付けて configbind が受け付けない唯一のも
のです。configbind の env 層と CLI 層に float の形式が無いためで、configbind と共
有する構造体には float を入れないでください。

## TOML ファイル

パーサは `minitoml` で、configbind が読むサブセットを読みます。

- テーブル、ネストしたテーブル、ドット区切りの bare key
- string、bool、整数、float のスカラー
- スカラーの配列
- 配列テーブル `[[name]]`。ヘッダ1つにつき要素1つ
- コメント

引用符付きキー、インラインテーブル、ネストした配列、日時は拒否されます。同じ
キーの二重定義はエラーです。形の規則の全体は
[configbind の TOML ファイル](configbind.ja.md#toml-ファイル) を参照してくださ
い。同じ規則です。

## エラー

パースエラーは行と列を示します。apply のエラーは `tomlbind: listeners.port: ...`
のように完全なキーパスと、値の何が悪かったかを示します。どちらも通常のエラーで
す。デコードはドキュメントごとに一度しか走らないので、比較するための sentinel は
ありません。

## 生成

```bash
tinybind-gen generate -dir .
```

エントリポイントを呼ばないパッケージには `tomlbind_gen.go` は書かれません。この
パスは `-disable` で `decode-toml` として無効化でき、`tomlbind` 自体は `io` 以外
を import しません。インターフェースはバイト列で書かれていて、`minitoml` を名指
しするのは生成コードだけです。

生成されたデコーダは TinyGo の `wasm` と `wasip1` でビルドでき、動作します。
