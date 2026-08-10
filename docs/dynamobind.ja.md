# dynamobind 利用ガイド

`dynamobind` は Go の構造体を DynamoDB の item に bind するパッケージです。基盤は
[`github.com/shibukawa/tinygodriver/nosql/dynamodb`](https://github.com/shibukawa/tinygodriver)
です。構造体とアクセスパターンを一度宣言して generator を実行すれば、呼び出し側で
`map[string]dynamodb.AttributeValue` を触ることも、属性名を書くこともなくなります。

driver はそのままです。`dynamobind` が足すのは型だけで、何も奪いません。driver の
error も、retry の判断も、page の境界も、すべて利用者の手元に残ります。

- [書くものと、得られるもの](#書くものと得られるもの)
- [`dynamo` tag](#dynamo-tag)
- [属性の型](#属性の型)
- [クエリ宣言](#クエリ宣言)
- [client は Context から来ます](#client-は-context-から来ます)
- [ランタイム操作](#ランタイム操作)
- [page と iterator](#page-と-iterator)
- [batch](#batch)
- [error](#error)
- [table 定義](#table-定義)
- [生成](#生成)
- [生成エラー](#生成エラー)
- [サイズ](#サイズ)
- [未実装](#未実装)

## 書くものと、得られるもの

利用者が書くのは、tag 付きの構造体、（必要なら）アクセスパターンを並べた `.tb.dynamo`
file、そして `go:generate` 行です。

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .

type Sensor string

type Reading struct {
	Sensor  Sensor    `dynamo:"sensor,partitionkey"`
	At      int64     `dynamo:"at,sortkey"`
	Celsius float64   `dynamo:"celsius"`
	Flags   []string  `dynamo:"flags,stringset,omitempty"`
	Taken   time.Time `dynamo:"taken"`
	Ignored string    `dynamo:"-"`
}
```

```text
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  table readings
  key sensor = {sensor} and at > {from}
}
```

生成されるのは 2 つの file です。

| file | 中身 |
|------|------|
| `dynamobind_gen.go` | `EncodeItem`、`DecodeItem`、`ItemKey`、`<Type>Table`、interface assertion |
| `dynamoquery_gen.go` | 宣言 1 つにつき 1 関数と、その式の定数 |

client を一度だけ入れておけば、呼び出しはこうなります。

```go
ctx = dynamobind.WithClient(ctx, client)

if err := dynamobind.Store(ctx, "readings", reading); err != nil {
	return err
}

got, err := dynamobind.Load[Reading](ctx, "readings", reading.ItemKey())

for reading, err := range ReadingsSince(ctx, "room-1", from) {
	if err != nil {
		return err
	}
	use(reading)
}
```

どこにも属性名が出てきません。属性名は tag と生成コードの中だけに存在するので、tag を
rename すると本番ではなく compile か生成が失敗します。client もどこにも出てきませんし、
宣言したクエリは table 名も書きません。宣言側が持っているからです。

### driver の `MarshalItem` を使わない理由

driver には reflection ベースの mapper が付属しています。compile 時に形の決まっている
構造体に対して使わない理由は、サイズではなく drift です。
`TableDefinition.PartitionKey.Name`、struct tag、`GetItem` に渡す `Key` は互いに何の関係も
ない 3 つの文字列で、どれか 1 つを rename しても compile は通り、実行時に
`ValidationException` で落ちます。生成すればこれが 1 つの名前になり、rename は build で
落ちます。

時間の cost もあり、item 1 件あたり約 0.8 µs・21 allocation です。ただし binary size の
話ではもうありません。[サイズ](#サイズ)のとおり、このパッケージを通る生成 path の方が
大きくなっています。

## `dynamo` tag

```text
dynamo:"<属性名>[,<option>...]"
```

名前を空にすると Go の field 名を使います。`dynamo:"-"` は field を除外します。
unexported field は常に除外されます。

| option | 意味 |
|--------|------|
| `partitionkey` | この field が table の partition key |
| `sortkey` | この field が table の sort key |
| `omitempty` | zero value のとき属性ごと書かない |
| `stringset` | slice を `L` ではなく `SS` として格納する |
| `numberset` | slice を `NS` として格納する |
| `binaryset` | slice を `BS` として格納する |
| `unixtime` | `time.Time` を epoch 秒の `N` として格納する |

この表にない option は生成 error です。ここが価値のある差です。driver の reflection
path は知らない option を何も指定されていないものとして読み、set を指定したつもりの
場所に黙って `L` を格納します。

tag の綴りは SDK の `dynamodbav` ではなく `dynamo` です。`dynamodbav` だけを持ち
`dynamo` を持たない field は生成 error になります。

## 属性の型

| Go | 属性 | 備考 |
|----|------|------|
| `string` | `S` | 空文字列は値であり、そのまま格納されます |
| `int`…`int64`, `uint`…`uint64` | `N` | `strconv` 経由。`float64` は通しません |
| `float32`, `float64` | `N` | |
| `bool` | `BOOL` | |
| `[]byte` | `B` | |
| `time.Time` | RFC 3339 nano の `S`、`unixtime` 指定で `N` | |
| `[]T` | `L`、set option 指定で `SS`/`NS`/`BS` | |
| `map[string]T` | `M` | string 以外の key は生成 error |
| nested struct | `M` | 同一 package 内で宣言されている必要があります |
| `*T` | 指し先、nil のときは `NULL` | |
| `dynamodb.AttributeValue` | そのまま格納 | escape hatch |

named type は基底型が使える場所でそのまま使えます。`type Sensor string` は `S` になり、
生成コードが変換します。

数値は最初から最後まで text です。DynamoDB の数値は有効数字 38 桁を持ち、`float64` は
持ちません。field に収まらない値は黙って wrap せず decode error になります。

```go
item["count"] = dynamodb.NString("70000") // field は uint16
err := reading.DecodeItem(item)           // 4464 ではなく error
```

どの Go の型にも収まらない桁数の数値も、`dynamodb.AttributeValue` の field を使えば
そのまま往復します。

item にその属性が無ければ field は触られません。古い版の構造体が書いた item も error
なしで decode できます。

## クエリ宣言

package の隣の `.tb.dynamo` file にアクセスパターンを宣言します。1 宣言が 1 つの名前付き
関数になります。

```text
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  table readings
  key sensor = {sensor} and at > {from}
}

export statement ReadingsBetween(sensor: Sensor, lo: int64, hi: int64): dynamo.page<Reading> {
  table readings
  key sensor = {sensor} and at between {lo} and {hi}
}

statement readingsForSensor(sensor: Sensor): dynamo.many<Reading> {
  table readings; key sensor = {sensor}
}
```

### 文法

```text
[export] statement <Name>(<param>: <GoType>, ...): dynamo.<shape><<ItemType>> {
  table <名前>
  key <属性> = {param} [and <属性> <述語>]
}
```

- `export` は名前の大文字小文字と一致している必要があります。Go は名前で可視性を決める
  ためで、`export statement ReadingsSince` と `statement readingsForSensor` はどちらも
  正しく、片方だけだと黙って rename されるのではなく生成 error になります。
- 引数の型は package で書かれているとおりの Go の型です。named type や `[]byte` も
  使えます。
- 節はどちらも必須です。`table` はこのアクセスパターンが対象にする table を指し、その結果
  として生成される関数は table を引数に取りません。
- 節の順序は自由で、1 行に並べるときは `;` で区切ります。
- `//` から行末までは comment です。

結果型は行数ではなく **request の形**を選びます。Query は常に複数返すからです。

| 形 | 生成される戻り値 | request 数 |
|----|------------------|-----------|
| `dynamo.many<T>` | `iter.Seq2[T, error]` | range が進むごとに 1 page 1 request |
| `dynamo.page<T>` | `(dynamobind.Page[T], error)` | 常に 1 回 |

sort key の述語は 1 宣言につき高々 1 つです。

| 書き方 | 送られるもの |
|--------|-------------|
| `at = {p}` | `=` |
| `at < {p}`, `at <= {p}`, `at > {p}`, `at >= {p}` | その比較 |
| `at between {lo} and {hi}` | `BETWEEN` |
| `begins_with(at, {p})` | `begins_with`。sort key が文字列のときだけ |

partition key の述語は必須で、先頭に置き、常に `=` です。DynamoDB がそれ以外を許さない
ためです。

### 生成されるシグネチャ

```go
func ReadingsSince(ctx context.Context,
	sensor Sensor, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]
```

table は引数にありません。`table` 節が与えるからです。可変長 option は driver に届くので、
`dynamodb.WithLimit`、`WithScanForward`、`WithConsistentRead`、`WithIndex` はそのまま
使えます。生成された式の名前と値は最後に追加されるので、呼び出し側の option が宣言した
条件を置き換えることはありません。

client も引数にありません。Context から来ます。
[client は Context から来ます](#client-は-context-から来ます)を参照してください。

### `table` 節が本体にある理由

型ではなく statement が持つのは、**型が 1 つの table とは限らない**からです。同じ構造体を
test 用の table と本番の table に置けるので、型に table を持たせると事実でないことを宣言
することになります。アクセスパターンはちょうど 1 つの table を指すので、書かれた場所で
事実が完結します。向きの点でも正しく、結果型は decode 先＝出力ですが、table は入力です。
入力は key 節や引数と同じく本体に属します。

省略可能ではなく必須なのは、1 つの宣言形式が 1 つのシグネチャを生むためです。省略可能に
すると、見た目の似た本体から table 引数のある関数とない関数の 2 種類ができてしまいます。

名前は DynamoDB が受け付ける範囲（英数字・`_`・`-`・`.` の 3〜255 文字）で検査されるので、
service が拒否する名前は最初の呼び出しでの `ValidationException` ではなく生成 error に
なります。

deployment 側が別の名前を使っている場合のことはここには書きません。宣言した名前は実行時に
Context で写します。[deployment のテーブル名](#deployment-のテーブル名)を参照してください。

item 操作は table 引数を保ちます。読み取る宣言が無いためで、不整合ではなく宣言が無いだけ
です。

### すべて tag と照合されます

宣言は text なので、text だけでは何も閉じません。drift を閉じているのは、**宣言に書かれた
名前を型の `dynamo` tag と生成時に突き合わせている**ことです。

```text
readings.tb.dynamo:5: statement ReadingsByNote: note is not a key of Reading;
a key condition reaches sensor and at, and a non-key attribute belongs in a filter
```

これは SQL template にはできない検査です。あちらは schema を知りませんが、こちらは tag が
schema だからです。

### 予約語は自動で処理されます

DynamoDB は 573 語を予約しており、`status`、`name`、`size`、`type`、`data`、`year`、
`count`、`timestamp` が含まれます。式にそのまま書くと `ValidationException` で拒否され
ます。生成されたクエリは全属性を無条件に alias するので、この問題自体が発生しません。

```go
const readingsSinceKeyCondition = "#k0 = :v0 AND #k1 > :v1"

var readingsSinceAttributeNames = map[string]string{"#k0": "sensor", "#k1": "at"}
```

名前が生成時に分かっているので、式も alias map も定数です。呼び出しごとの組み立ては無く、
予約語のリストを持ち歩いて最新に保つ必要もありません。

### 文字列形式も残っています

`Query` と `QueryPage` は今もキー条件を text で受け取ります。宣言で表現できないものの
ためです。その text は tag と照合されず、予約語の alias は利用者の責任です。

```go
// ValidationException: Attribute name is a reserved keyword
dynamobind.Query[Event](ctx, "events", "status = :s", values)

// 自分で alias する
dynamobind.Query[Event](ctx, "events", "#n0 = :s",
	dynamodb.WithExpressionNames(map[string]string{"#n0": "status"}),
	values)
```

## client は Context から来ます

client は process ごとに固定される事実です。既定ではどこも引数に取りません。一度入れて
おけば、呼び出し側にも生成されたシグネチャにも現れません。これは既定であって唯一の形では
ありません。[client を引数で渡す](#client-を引数で渡す)を参照してください。

```go
ctx := dynamobind.WithClient(r.Context(), client)
```

```go
WithClient(ctx context.Context, c *dynamodb.Client, options ...ClientOption) context.Context

ClientFromContext(ctx context.Context) (*dynamodb.Client, error)
TableFromContext(ctx context.Context, table string) (*dynamodb.Client, string, error)
```

このパッケージの入口はすべて `TableFromContext` を通ります。`ClientFromContext` は
escape hatch で、このパッケージが包んでいない操作のために driver へ直接届きます。

client の無い Context は `ErrNoClient` です。結果の形が許す方法で呼び出し側に届きます。
error を返す関数はそのまま返し、iterator は zero value と一緒に 1 度 yield して終わります。
page の失敗と同じ報告のしかたです。

別の client でテストする、別 region に届く、といった場合は、別のシグネチャではなく別の
Context を作ります。

### deployment のテーブル名

`table` 節に書いた名前と、item 操作に渡す名前は、コードが宣言する名前です。既定では
それがそのまま送られます。deployment 側が別の名前を使っているときは resolver を入れます。

```go
ctx := dynamobind.WithClient(r.Context(), client,
	dynamobind.WithTableNames(func(ctx context.Context, declared string) string {
		return config.Tables[declared]
	}))
```

```go
type TableResolver func(ctx context.Context, declared string) string

WithTableNames(resolve TableResolver) ClientOption
```

prefix ではなく関数にしてあるのは意図的です。prefix は deployment ツールがたまたま従って
いる規約でしかなく、他の形を表せません。CDK が生成する物理名は接尾辞が付きますし、
`orders-prod` は環境名が後ろに来ますし、環境変数から読む名前は宣言した名前と 1 文字も
共有しません。ここではどれも同じ 1 つの関数です。

Context を取るのは、写像が process だけでなく request にも依存しうるからです。テナント別の
テーブルなら、Context からテナントを読む同じ関数で済みます。

resolver を入れなければ宣言した名前がそのまま送られるので、宣言どおりの名前を使っている
deployment は何も書きません。

## client を引数で渡す

client とテーブル名の写像を合わせたものが `Handle` です。`WithClient` はこれを Context に
入れます。`NewHandle` は同じ値を直接渡すために作ります。

```go
type Handle struct{ /* 中身は非公開 */ }

NewHandle(c *dynamodb.Client, options ...ClientOption) Handle
WithHandle(ctx context.Context, h Handle) context.Context
HandleFromContext(ctx context.Context) (Handle, error)

func (h Handle) Client() *dynamodb.Client
func (h Handle) Table(ctx context.Context, table string) (*dynamodb.Client, string, error)
```

ランタイムの入口にはそれぞれ `On` を付けた双子があり、`Handle` を引数に取ります。

```go
h := dynamobind.NewHandle(client, dynamobind.WithTableNames(names))

reading, err := dynamobind.LoadOn[Reading](ctx, h, "readings", key)
err = dynamobind.StoreOn(ctx, h, "readings", reading)
for reading, err := range dynamobind.QueryOn[Reading](ctx, h, "readings", cond) {
}
```

実装を持っているのは `On` の側で、Context 版はそこへ委譲します。2 つがずれることはありません。

**`Context` は両方とも第 1 引数のままです。** deadline を運ぶのは Context であり、driver が
それを要求するからです。`On` 版が落とすのは `ctx.Value` の参照であって、`Context` では
ありません。

zero 値の `Handle` は `ErrNoClient` です。client の無い Context とまったく同じ扱いです。

メソッド形式はありません。Go はメソッドに型パラメータを許さず、item 系の入口はすべて item
の型でジェネリックなので、`h.Load[Reading](...)` は存在しえません。

引数版が向いている場面:

- すでに client を持っている呼び出し側。参照はそのぶん無駄になります
- サイズが効くバイナリ。TinyGo 0.41.1 / wasip1 で、`WithClient` も Context 版の入口も呼ばない
  プログラムは Context 周りの機構を一切リンクせず、実測で 39,189 バイト小さくなりました
- フレームワークが管理する値を 1 つの構造体にまとめていて、操作ごとではなく request ごとに
  1 回だけ解決したい場合

Context の深さ 20 での解決が約 24 ns、深さ 1 で約 5.6 ns、`Handle` が約 2.1 ns です。DynamoDB
への往復に対してはどれも見えません。**選ぶ基準は呼び出し側の書き味とバイナリサイズであって、
request のレイテンシではありません。**

## ランタイム操作

```go
Load[T](ctx, table, key, opts...) (T, error)
Store(ctx, table, v, opts...) error
Remove(ctx, table, v, opts...) error
Update(ctx, table, v, expression, opts...) error

StoreReturning(ctx, table, v, opts...) (T, bool, error)
RemoveReturning(ctx, table, v, opts...) (T, bool, error)

QueryPage[T](ctx, table, keyCond, opts...) (Page[T], error)
ScanPage[T](ctx, table, opts...) (Page[T], error)
Query[T](ctx, table, keyCond, opts...) iter.Seq2[T, error]
Scan[T](ctx, table, opts...) iter.Seq2[T, error]

StoreAll(ctx, table, vs) (unprocessed []T, err error)
LoadAll[T](ctx, table, keys, opts...) (items []T, unprocessed []dynamodb.Key, err error)
```

これらが table 名を取るのは、読み取る宣言が無いからです。宣言のあるクエリは取りません。

dispatch は registry ではなく型制約です。生成された codec を持たない型は、登録漏れによる
実行時失敗ではなく compile error になります。

`Store` は `PutItem` で、item 全体を置き換えます。`Update` は DynamoDB の update 式を
そのまま受け取り、key だけを供給します。struct tag が実際に供給できるのはそこだけです。

`StoreReturning` と `RemoveReturning` は `ALL_OLD` を要求し、置き換え／削除されたものを
decode します。bool は何も無かったときに false で、error ではありません。

## page と iterator

`QueryPage` は 1 request で、`LastEvaluatedKey`、`Count`、`ScannedCount` を返します。
`Query` は代わりに反復し、range が進むごとに次の page を要求します。

1 回の `range` が何度も request を出すことがあり、iterator は page ごとの数値を一切
報告しません。filter が返り値の 100 倍を走査している query も、そうでない query も、
見た目は同じです。中断した反復を途中から再開することもできません。それが問題になる
場面では `QueryPage`、あるいは宣言側で `dynamo.page<T>` を使ってください。`Scan` は
同じ性質で、対象が table 全体です。

loop を break すると、次の request を出さずに終わります。

## batch

`StoreAll` と `LoadAll` は入力を DynamoDB が受け付ける単位に分割します。`MaxBatchWrite`
が 25、`MaxBatchGet` が 100 で、どちらも公開されているので、入力を自分で刻む側も分割側と
同じ数を読めます。ここまでは算術なので runtime に置いてあります。

retry は算術ではないので置いていません。service が断った分はそのまま返ります。

```go
unprocessed, err := dynamobind.StoreAll(ctx, "readings", readings)
if err != nil {
	return err
}
// retry するかは利用者の判断です。driver は transport 層で既に retry しており、
// 1 回の write が attempts × 2 回配送されうると明記しています。ここで loop を
// 足すと、その回数が黙って掛け算になります。
```

`LoadAll` が返す順序は DynamoDB の応答順です。一致しなかった key は単に含まれません。
error でもなく、unprocessed key でもありません。

## error

driver の sentinel はすべて生き残ります。

```go
_, err := dynamobind.Load[Reading](ctx, "readings", key)
if errors.Is(err, dynamodb.ErrItemNotFound) {
	// 存在しない key は存在しないまま。zero value になって届くことはありません
}

var driverError *dynamodb.Error
if errors.As(err, &driverError) {
	log.Println(driverError.Op, driverError.RequestID, driverError.Retryable())
}
```

decode の失敗は、属性名と期待した型・実際の型を持ちます。

```go
if mapping, ok := dynamobind.AsError(err); ok {
	log.Println(mapping.Attribute, mapping.Expected, mapping.Got) // at N S
}
```

`AsError` は reflection を必要とする `errors.As` を使わずに chain を辿ります。

## table 定義

```go
func ReadingTable(name string) dynamodb.TableDefinition
```

`partitionkey` を宣言している型に対して、codec と同じ tag から生成されます。test では
table 作成に必要ですし、`CreateTable` を一度も呼ばない program でも key 名の出所が
1 箇所にまとまります。driver の `CreateTable` 自体は約 22 KB で、呼ばなければ linker が
落とします。

これは table の**形**であって名前ではありません。`name` が引数なのはそのためです。型は
1 つの table とは限らず、同じ定義から test 用の table も本番の table も作れます。名前を
決めるのは宣言の `table` 節で、こちらはそのどれもがどういう形かを述べます。

## 生成

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

生成は package の呼び出しに従います。`Store` があれば encoder、`Load` があれば decoder が
出ます。どこからも名指しされない型からは何も出ません。nested struct は親の操作を受け継ぎ
ます。`.tb.dynamo` の宣言も結果型の使用として数えるので、DynamoDB の利用が宣言だけの
package でも、生成されたクエリが必要とする decoder は出ます。

client の渡し方はどちらでも数えます。`StoreOn` は `Store` と同じように発見されるので、
呼び出しごとに `Handle` を渡す package でも Context 版と同じものが生成されますし、宣言済み
クエリは Context・item 操作は `Handle` という混在も、設定なしでそのまま見つかります。

key builder だけが例外で、`partitionkey` を宣言した型には、それを必要とする呼び出しが
無くても `ItemKey` と table 定義が生成されます。item を読む標準的な書き方は
`Load(ctx, table, v.ItemKey())` であり、method の使用は generator が発見できる呼び出し
ではありません。発見を待つと、呼ぶべき method が永遠に生成されないことになります。3 行の
method なので、呼ばれなければ linker が落とします。

生成 file には入力の SHA-256 が記録されます。source、`.tb.dynamo`、`go.mod`、option、
generator binary のすべてが記録値と一致する再実行は、再生成せずに終了します。`-force` は
無条件に再生成します。

`-force` は hash に関わらず再生成します。残りの調整は `generator.Options` にあります。

```go
options := generator.DefaultOptions()
options.DisableFeatures = []generator.Feature{generator.FeatureItemTable}
options.DynamoTemplatePattern = "*.query.dynamo"
```

| 設定 | 効果 |
|------|------|
| `FeatureItemCodec` | DynamoDB mode 全体を止めます。クエリも含みます |
| `FeatureItemTable` | `<Type>Table` だけを止めます。codec と key builder は残ります |
| `DynamoTemplatePattern` | 宣言 file の glob。既定は `*.tb.dynamo` |
| `DynamoParameterAPI` | 生成されたクエリが先頭に `dynamobind.Handle` を取ります |
| `DynamoHandleResolver` | 生成されたクエリが、指定した関数から Handle を得ます |

後半 2 つは、生成されたクエリがどこから client を得るかを選びます。どちらも設定しないのが
既定で、これらが存在しなかった頃とバイト単位で同じものを出力します。

`DynamoParameterAPI` は `Handle` をシグネチャへ移します。宣言した名前は変わりません。
変わるのはシグネチャだけです。

```go
// 既定
func ReadingsSince(ctx context.Context, sensor string, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]

// DynamoParameterAPI
func ReadingsSince(ctx context.Context, h dynamobind.Handle, sensor string, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]
```

`DynamoHandleResolver` はシグネチャを既定のまま保ち、`Handle` の出どころだけを変えます。
あなたの関数を名指しします。

```go
options.DynamoHandleResolver = &generator.SymbolPattern{
	PackagePath: "example.com/app/pw",
	Name:        "DynamoHandle",
}
```

```go
func DynamoHandle(ctx context.Context) (dynamobind.Handle, error)
```

これは、すでに自前の Context 値を持っているフレームワーク向けです。`WithClient` で 2 つ目の
値を入れるかわりに、管理する値を 1 つの構造体に 1 つのキーでまとめ、そこから `Handle` を
答えます。**その構造体がいくつ値を持っていても、生成されたどの呼び出しも 1 回の参照と 1 回の
型アサーションで済みます。**

両方を設定した場合は `DynamoParameterAPI` が勝ちます。すでに `Handle` を持っている
シグネチャは何も解決しないからです。package path の無い resolver や、エクスポートされた
識別子でない名前は、ビルドできないファイルを吐くのではなく生成時に失敗します。

前者の CLI flag は `-dynamo-parameter-api` です。resolver は `SQLExecutorResolver` と同じく
Go API 専用の設定です。残りの設定は `-html-template-pattern` や `-sql-template-pattern` と
違って対応する CLI flag がまだないので、当面は `generator.New` 経由で指定してください。

## 生成エラー

どの検査も型と field、あるいは statement と属性を名指しします。本番ではなくここで失敗
させる理由が、行動できる message そのものだからです。

tag と型の検査:

- 未知の `dynamo` tag option
- `dynamo` tag を持たない field の `dynamodbav` tag
- 2 つの field が同じ属性名に写る
- `partitionkey` が 2 つ、`sortkey` が 2 つ、`partitionkey` の無い `sortkey`
- key field の属性が `S`、`N`、`B` のいずれでもない
- 属性形式を持たない Go の型、string 以外を key にする map、要素型の合わない set option
- 別 package で宣言された nested struct
- `EncodeItem`、`DecodeItem`、`ItemKey` を既に手書きで宣言している型

クエリの検査:

- `table` 節が無い statement、`table` 節が 2 つある statement
- DynamoDB が拒否する table 名
- `dynamo` tag を持たない item 型、`partitionkey` を持たない型
- その型に無い属性
- key 節に書かれたキー以外の属性
- `=` でない partition key の述語、先頭でない partition key
- 2 つ以上の sort key 述語
- 文字列として格納されていない属性への `begins_with`
- 属性の Go の型と一致しない引数の型
- 宣言されていない引数を指す placeholder、使われない引数
- 同じ名前の statement が 2 つ

## サイズ

TinyGo 0.41.1 / `wasip1` での実測です。4 field の item を 1 件 store して read する
program 1 本を比較しています。属性単位の error 報告も含めてどの行も同じ仕事をしていて、
違うのは item の写し方と client の取り方だけです。

| build | byte |
|-------|------|
| driver 直、item map は手組み、error 報告なし | 3,541,365 |
| driver の `MarshalItem`（reflection） | 3,586,193 |
| 手書き codec、driver を直接呼ぶ | 3,586,568 |
| 手書き codec + `dynamobind` | 3,625,639 |
| **生成 codec + `dynamobind`** | **3,625,851** |

読み取るべきことは 2 つです。

**生成 codec は同じ形の手書き codec より 212 byte 大きい。** generator が責任を負うのは
この数字で、このプロジェクトが守ると決めた予算でもあります。

**Context から client を取る方式が約 38 KB かかっていて**、ここでは他のすべてを圧倒して
います。client が引数だった 1 つ前の API に対して同じ program を build すると 3,587,827 で、
Context への移動は +37,812 byte です。これはこのパッケージ側で削れる分ではありません。
`dynamobind` を一切使わず `context.WithValue` と型 assertion を 1 回ずつ書くだけで、同じ
program が 48,409 byte 増えます。assertion が、TinyGo なら本来落とせる型記述子の機構を
引き込むからです。

結果ははっきり書いておきます。**生成 path は driver の reflection mapper より約 40 KB
大きくなりました**。小さくはありません。型付き codec が driver を直接呼ぶ形なら reflection
とほぼ同じ（+375 byte）で、差はすべて API 面、その大半が Context です。40 KB が効く
ほど厳しい target なら、生成された `EncodeItem`・`DecodeItem`・`ItemKey` を使って driver を
直接呼べば、型の安全だけを取れます。普通の method なので、このパッケージを link する必要は
ありません。

codec を生成する drift の理由はこれに影響されません。名前が食い違わないことが論点であり、
サイズがいくつであっても成り立ちます。

`encoding/json` と `reflect` はどちらにせよ link されます。driver が request body を
`encoding/json` で marshal するからで、生成コードをいくら足しても消えません。それを取り
戻すには driver 側に byte 単位の JSON path が要りますが、それが入っても上の API は
変わりません。

## 未実装

- **filter 式・projection 式・condition 式・update 式。** `filter` 節はその旨の message
  付きで拒否されます。当面は式を自分で渡してください。実装されたとき、同じ宣言に入ります。
- **secondary index。** `gsi` tag が無いので、宣言クエリは table 自身の key に対して
  走ります。`dynamodb.WithIndex` は driver に届きますが、その index の key に対する条件の
  検査は行われません。
- **single-table 設計。** 1 struct が 1 table を所有する前提です。codec 自体はその table
  を誰と共有していようが関知しませんが、`<Type>Table` は 1 つの型を記述し、型付き read は
  全 item を 1 つの型として decode するので、共有 table ではその 2 つを手書きすることに
  なります。
- **楽観ロックと TTL。** `version` tag と `ttl` tag は設計済みで未実装です。TTL は driver
  の `UpdateTimeToLive` も待っています。
- **transaction、PartiQL、Streams、DAX。** driver が対象外としているため、提供できません。
