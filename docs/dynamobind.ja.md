# dynamobind 利用ガイド

`dynamobind` は Go の構造体を DynamoDB の item に bind するパッケージです。基盤は
[`github.com/shibukawa/tinygodriver/nosql/dynamodb`](https://github.com/shibukawa/tinygodriver)
です。構造体を一度定義して generator を実行すれば、呼び出し側で
`map[string]dynamodb.AttributeValue` を触ることはなくなります。

driver はそのままです。`dynamobind` が足すのは型だけで、何も奪いません。driver の
error も、retry の判断も、page の境界も、すべて利用者の手元に残ります。

## 自動化されること

- reflection を使わない `EncodeItem` と `DecodeItem` の生成
- table 定義と同じ tag から作る `ItemKey`
- `<Type>Table` の生成。schema と request の key 名が食い違わなくなります
- `.tb.dynamo` の宣言 1 つにつき 1 つの名前付きクエリ関数
- 型が runtime interface を満たすことの compile 時 assertion

## ユーザーが用意するもの

1. `dynamo` tag を持つ構造体
2. その型を名指しする `dynamobind` の呼び出しを 1 つ以上
3. `go:generate` 行、または `tinybind-gen generate` の実行

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .

type Reading struct {
	Sensor  string    `dynamo:"sensor,partitionkey"`
	At      int64     `dynamo:"at,sortkey"`
	Celsius float64   `dynamo:"celsius"`
	Flags   []string  `dynamo:"flags,stringset,omitempty"`
	Taken   time.Time `dynamo:"taken"`
	Ignored string    `dynamo:"-"`
}
```

```go
got, err := dynamobind.Load[Reading](ctx, client, "readings", want.ItemKey())
```

## driver の `MarshalItem` を使わない理由

driver には reflection ベースの mapper が付属しています。動作はしますが、compile
時に形の決まっている構造体に対しては 2 つ問題があります。

1 つは drift です。`TableDefinition.PartitionKey.Name`、struct tag、`GetItem` に渡す
`Key` は、互いに何の関係もない 3 つの文字列です。どれか 1 つを rename しても compile
は通り、実行時に `ValidationException` で落ちます。生成すれば、3 つとも 1 つの宣言から
出てきます。

もう 1 つは cost で、こちらは小さい方の問題です。reflection path は binary で約 24 KB、
item 1 件あたり約 0.8 µs かかります。後述の[サイズ](#サイズ)の実測では、生成 codec は
reflection より 19 KB 小さく、手書き codec との差は 200 byte 未満です。

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
`dynamo` を持たない field は生成 error になります。黙って Go の field 名で格納される
よりは、その場で止まる方が安全だからです。

## 型

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

数値は最初から最後まで text です。DynamoDB の数値は有効数字 38 桁を持ち、`float64`
は持ちません。したがってどの経路も float を経由しません。field に収まらない値は黙って
wrap せず、decode error になります。

```go
item["count"] = dynamodb.NString("70000") // field は uint16
err := reading.DecodeItem(item)           // 4464 ではなく error
```

どの Go の型にも収まらない桁数の数値も、`dynamodb.AttributeValue` の field を使えば
そのまま往復します。

## 操作

```go
Load[T](ctx, c, table, key, opts...) (T, error)
Store(ctx, c, table, v, opts...) error
Remove(ctx, c, table, v, opts...) error
Update(ctx, c, table, v, expression, opts...) error

StoreReturning(ctx, c, table, v, opts...) (T, bool, error)
RemoveReturning(ctx, c, table, v, opts...) (T, bool, error)

QueryPage[T](ctx, c, table, keyCond, opts...) (Page[T], error)
ScanPage[T](ctx, c, table, opts...) (Page[T], error)
Query[T](ctx, c, table, keyCond, opts...) iter.Seq2[T, error]
Scan[T](ctx, c, table, opts...) iter.Seq2[T, error]

StoreAll(ctx, c, table, vs) (unprocessed []T, err error)
LoadAll[T](ctx, c, table, keys, opts...) (items []T, unprocessed []dynamodb.Key, err error)
```

dispatch は registry ではなく型制約で行います。codec が生成されていない型は compile
error になります。誰も登録しなかった registry を実行時に探して失敗する、ということは
起きません。

`Store` は `PutItem` です。item 全体を置き換えます。`Update` は DynamoDB の update
式をそのまま受け取り、key だけを供給します。struct tag から導けるのは key だけだから
です。

`StoreReturning` と `RemoveReturning` は `ALL_OLD` を要求し、置き換えた／削除した item
を decode して返します。bool は「元の item が無かった」を表し、これは error ではありま
せん。

## page と iterator

`QueryPage` は 1 request で、`LastEvaluatedKey`、`Count`、`ScannedCount` を返します。
`Query` は代わりに反復します。

```go
for reading, err := range dynamobind.Query[Reading](ctx, c, "readings", "sensor = :s",
	dynamodb.WithExpressionValues(values)) {
	if err != nil {
		return err
	}
	use(reading)
}
```

1 回の `range` が何度も request を出すことがあり、iterator は page ごとの数値を一切
報告しません。filter が返り値の 100 倍を走査している query も、そうでない query も、
見た目は同じです。中断した反復を途中から再開することもできません。それが問題になる
場面では `QueryPage` を使ってください。`Scan` は同じ性質で、対象が table 全体です。

loop を break すると、次の request を出さずに終わります。

## 宣言済みクエリ

クエリは package の隣の `.tb.dynamo` file で宣言し、生成が 1 宣言につき 1 つの名前付き
関数にします。

```text
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  key sensor = {sensor} and at > {from}
}
```

```go
for reading, err := range ReadingsSince(ctx, client, "readings", "room-1", from) {
	if err != nil {
		return err
	}
	use(reading)
}
```

結果型は行数ではなく **request の形**を選びます。Query は常に複数返すからです。
`dynamo.many<T>` は全 page を反復し、`dynamo.page<T>` は 1 request で `Page[T]` を返します。

宣言に書いた属性名は `dynamo` tag と照合されるので、tag を rename すると本番ではなく
生成が失敗します。key 節が受け付けるのは DynamoDB がそこで受け付けるものだけです。
partition key は `=` のみ、sort key の述語は `=`, `<`, `<=`, `>`, `>=`, `between`,
`begins_with` から高々 1 つ。キー以外の属性を書くとそう言われます。

```text
readings.tb.dynamo:5: statement ReadingsByNote: note is not a key of Reading;
a key condition reaches sensor and at, and a non-key attribute belongs in a filter
```

filter 式は未実装なので、この message は今は存在しない節を指しています。実装されたとき、
同じ宣言に入ります。

### 予約語は自動で処理されます

DynamoDB は 573 語を予約しており、`status`、`name`、`size`、`type`、`data`、`year`、
`count`、`timestamp` が含まれます。式にそのまま書くと拒否されます。生成されたクエリは
全属性を無条件に alias するので、この問題自体が発生しません。

```go
const eventsByStatusKeyCondition = "#k0 = :v0"

var eventsByStatusAttributeNames = map[string]string{"#k0": "status"}
```

式も alias map も定数です。属性名が分かった時点で確定するので、呼び出しごとの組み立ては
ありません。

### 文字列形式も残っています

`Query` と `QueryPage` は今もキー条件を text で受け取ります。宣言で表現できないものの
ためです。その text は tag と照合されず、予約語の alias は利用者の責任です。

```go
// ValidationException: Attribute name is a reserved keyword
dynamobind.Query[Event](ctx, c, "events", "status = :s", values)

// 自分で alias する
dynamobind.Query[Event](ctx, c, "events", "#n0 = :s",
	dynamodb.WithExpressionNames(map[string]string{"#n0": "status"}),
	values)
```

## batch

`StoreAll` と `LoadAll` は入力を DynamoDB が受け付ける単位に分割します。write は 25
件、read は 100 件です。ここまでは算術なので runtime に置いてあります。

retry は算術ではないので置いていません。service が断った分はそのまま返ります。

```go
unprocessed, err := dynamobind.StoreAll(ctx, c, "readings", readings)
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
_, err := dynamobind.Load[Reading](ctx, c, "readings", key)
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

table を CloudFormation や Terraform に完全に任せる場合は `-disable item-table` を
指定してください。codec と key builder は残ります。`-disable item-codec` は mode 全体を
止めます。

## 何が、いつ生成されるか

生成は package の呼び出しに従います。`Store` があれば encoder、`Load` があれば decoder
が出ます。どこからも名指しされない型からは何も出ません。nested struct は親の操作を
受け継ぎます。

key builder だけが例外です。`partitionkey` を宣言した型には、それを必要とする呼び出しが
無くても `ItemKey` と table 定義が生成されます。item を読む標準的な書き方は
`Load(ctx, c, table, v.ItemKey())` であり、method の使用は generator が発見できる呼び出し
ではありません。発見を待つと、呼ぶべき method が永遠に生成されないことになります。3 行の
method なので、呼ばれなければ linker が落とします。

## サイズ

TinyGo 0.41.1 / `wasip1` での実測です。4 field の item を 1 件 store して read する
program 1 本を比較しています。

| build | byte | 手書き codec との差 |
|-------|------|--------------------|
| driver 直、item map は手組み | 3,543,805 | — |
| 手書き codec + `dynamobind` | 3,568,434 | — |
| **生成 codec + `dynamobind`** | **3,568,604** | **+170** |
| driver の `MarshalItem`（reflection） | 3,588,094 | +19,660 |

生成 codec は同じ形の手書き codec より 170 byte 大きく、reflection mapper より約 19 KB
小さいという結果です。上 2 行の間にある 24 KB は codec ではなく `dynamobind` の API 面
そのものです。どちらも要らない program は、生成された method を直接呼べます。

`encoding/json` と `reflect` はどちらにせよ link されます。driver が request body を
`encoding/json` で marshal するからで、生成コードをいくら足しても消えません。この分を
回収するには driver 側に byte 単位の JSON path が必要ですが、それが入っても上記の API は
変わりません。

## 制約

- transaction、PartiQL、Streams、DAX は対象外です。driver が対象外としているためです。
- nested struct は同一 package で宣言されている必要があります。他人の package に codec
  は生成できません。
- secondary index の tag は未実装なので、宣言クエリは table 自身の key に対して走ります。
- filter 式・projection 式・condition 式・update 式は生成しません。自分で渡してください。
