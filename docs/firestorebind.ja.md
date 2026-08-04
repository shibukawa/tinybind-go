# firestorebind 利用ガイド

`firestorebind` は Go の構造体を Firestore（Datastore mode）の entity に bind する
パッケージです。基盤は
[`github.com/shibukawa/tinygodriver/nosql/datastore`](https://github.com/shibukawa/tinygodriver)
です。構造体とアクセスパターンを一度宣言して generator を実行すれば、呼び出し側で
`datastore.Value` を組み立てることも、プロパティ名を書くこともなくなります。

DynamoDB では属性名を間違えるとサービスが教えてくれます。最初の呼び出しで
`ValidationException` が返る。Datastore は教えてくれません。filter は通り、クエリは
実行され、何にもマッチしない。空の batch が失敗の合図のすべてで、そして空の batch は
「本当に該当がなかった」ときにも返ります。この静けさが、このパッケージが相手にして
いるものです。

driver はそのままです。`firestorebind` が足すのは型だけで、何も奪いません。driver の
error も、retry の判断も、transaction の再実行も、batch の境界も、すべて利用者の手元
に残ります。

- [書くものと、得られるもの](#書くものと得られるもの)
- [`firestore` tag](#firestore-tag)
- [プロパティの型](#プロパティの型)
- [key はプロパティではなくパスです](#key-はプロパティではなくパスです)
- [クエリ宣言](#クエリ宣言)
- [複合インデックス](#複合インデックス)
- [client と namespace は Context から来ます](#client-と-namespace-は-context-から来ます)
- [ランタイム操作](#ランタイム操作)
- [条件付き書き込み](#条件付き書き込み)
- [transaction](#transaction)
- [page と iterator](#page-と-iterator)
- [batch](#batch)
- [error](#error)
- [生成](#生成)
- [生成エラー](#生成エラー)
- [サイズ](#サイズ)
- [未実装](#未実装)

## 書くものと、得られるもの

利用者が書くのは、tag 付きの構造体、（必要なら）アクセスパターンを並べた
`.tb.firestore` file、そして `go:generate` 行です。

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .

type SensorID string

type Reading struct {
	ID      SensorID      `firestore:"-,name"`
	Site    datastore.Key `firestore:"-,parent"`
	Version int64         `firestore:"-,version"`

	Sensor  SensorID  `firestore:"sensor"`
	At      time.Time `firestore:"at"`
	Celsius float64   `firestore:"celsius"`
	Note    string    `firestore:"note"`
	Tags    []string  `firestore:"tags,omitempty"`
	Body    string    `firestore:"body,noindex"`
	Ignored string    `firestore:"-"`
}
```

```text
export statement ReadingsSince(sensor: SensorID, from: time.Time): firestore.many<Reading> {
  where sensor == {sensor} and at > {from}
  order at desc
}
```

生成されるのは 2 つの file です。

| file | 中身 |
|------|------|
| `firestorebind_gen.go` | `EncodeEntity`、`DecodeEntity`、`Kind`、`EntityKey`、`EntityVersion`、interface assertion |
| `firestorequery_gen.go` | 宣言 1 つにつき 1 関数、その kind 定数、宣言された index |

client を一度だけ入れておけば、呼び出しはこうなります。

```go
ctx = firestorebind.WithClient(ctx, client)

key, err := firestorebind.Store(ctx, reading)

got, err := firestorebind.Load[Reading](ctx, reading.EntityKey())

for reading, err := range ReadingsSince(ctx, "room-1", from) {
	if err != nil {
		return err
	}
	use(reading)
}
```

どこにもプロパティ名がありません。kind もありません。この後者が DynamoDB 版と対応する
ものを持たない部分です。table はデプロイの都合なので `dynamobind` の宣言は `table` 句を
持ちますが、kind は型に属します。結果型が `Reading` を名指し、`Reading` は自分の kind を
知っている。宣言と codec が「何を問い合わせているか」で食い違う、という書き方ができません。

### driver の `MarshalEntity` を使わない理由

driver は reflection ベースの mapper を持っていて、それが読む tag は `datastore` です。
`cloud.google.com/go/datastore` と同じ綴りなので、公式 client 向けに書かれた例がそのまま
動きます。

その親切さがそのまま危険でもあります。1 つのフィールドに 2 つの tag が付くと、見た目は
交換可能で、リネームしたプロパティすべてで食い違う 2 つのマッピングになる。しかも両方
コンパイルが通り、両方 `Entity` を作る。クエリが filter するのはそのうち片方だけです。
なので `datastore` tag があって `firestore` tag がないフィールドは、両方の綴りを名指し
した生成エラーにしています。driver 自身のドキュメントが、上に乗る generator に対して
まさにそう扱うよう求めています。

## `firestore` tag

```text
firestore:"<プロパティ名>[,<オプション>...]"
```

名前が空ならフィールド名を使います。`firestore:"-"` はスキップですが、下の identity
オプションが付いている場合は別です。非公開フィールドは常にスキップされます。

| オプション | 意味 |
|------------|------|
| `name` | この string フィールドが key の name になる |
| `id` | この `int64` フィールドが key の数値 id になる |
| `parent` | このフィールドが祖先パスを供給する |
| `version` | この `int64` フィールドが、読み取り時の entity version を受け取る |
| `noindex` | プロパティとして保存するが、どの index にも入れない |
| `omitempty` | フィールドがゼロ値のとき、プロパティ自体を書かない |

この表にないオプションは生成エラーです。ここでの打ち間違いを吸収してくれるものは何も
ありません。driver の mapper はそもそも別の tag を読むので、綴りを間違えたオプションは
「一生現れないプロパティ」になるだけです。

コストを知っておく価値のあるオプションが 2 つあります。

`omitempty` はプロパティを *absent* にします。null ではありません。Datastore にとって
「プロパティが無い」と「プロパティが null」は filter に対して別物で、どちらも表現できる
ので、この選択は見た目の問題ではありません。

`noindex` は見た目より安く、見た目より狭い。Datastore のプロパティは既定で全部
index されます。index は書き込みスループットとストレージを消費するので、filter しない
長いテキストを外すのは効きます。ただし外れたプロパティは *どの* index にも無いので、
クエリがそれにマッチすることは二度とありません。生成はそこを強制します。`noindex` の
フィールドを filter、order、projection に書いた宣言は落ちます。

## プロパティの型

| Go | 値 | 備考 |
|----|----|------|
| `string` | `stringValue` | 空文字列は値であり、保存されます |
| `int`…`int64` | `integerValue` | ワイヤ上はテキスト。`float64` を経由しません |
| `uint8`、`uint16`、`uint32` | `integerValue` | どの値も `int64` に収まります |
| `float32`、`float64` | `doubleValue` | 本物の JSON 数値 |
| `bool` | `booleanValue` | |
| `[]byte` | `blobValue` | ワイヤ上は base64 |
| `time.Time` | `timestampValue` | 保存はマイクロ秒まで。往復で切り詰められます |
| `datastore.Key` | `keyValue` | 外部キーに一番近いもの。何も強制はしません |
| `datastore.LatLng` | `geoPointValue` | |
| `[]T` | `arrayValue` | |
| ネストした構造体 | `entityValue` | 同じパッケージで宣言され、key を持ちません |
| `*T` | 指す先、nil なら null | |
| `datastore.Value` | そのまま格納 | 逃げ道 |

名前付き型は元の型が使える場所で使えるので、`type SensorID string` は string として
マップされます。

説明の要る拒否が 3 つあります。どれも「素直なマッピングが存在して、それでも間違い」
という場合です。

**integer と double は別の型です。** DynamoDB は `N` ひとつで、数値の Go 型を全部そこに
畳みます。Datastore は違います。`integerValue` と `doubleValue` を別々に保存し、別々に
並べ、別々に比較する。だから `int64` フィールドは integer からしか、`float64` フィールド
は double からしかデコードしません。古いスキーマがもう一方で書いた値は、変換ではなく
デコードエラーになります。ここで変換してしまうと、その値を見つけたクエリが二度と
見つけられない値ができあがります。

**`uint`、`uint64`、`uintptr` は生成エラーです。** Datastore の integer は `int64` で、
driver はそれより広いものの marshal を拒否します。`math.MaxInt64` を超える値はワイヤに
届きません。`uint64` を受け付けるフィールドは、コンパイルが通り、小さい値を何ヶ月も保存し、
最初の大きい値で落ちます。`int64` を使うか、本当にその幅が必要で並べ替えに使わないなら
string プロパティにしてください。

**map は生成エラーです。** map は「プロパティ名が構造体ではなく実行時データから来る」
ネスト entity になります。tag 以外の場所からプロパティ名が来ること、それがこの codec の
存在理由そのものです。名前が宣言されているネスト構造体を使うか、本当に名前が動的なら
`datastore.Value` フィールドを使ってください。driver 自身の mapper も同じ理由で map を
拒否します。

## key はプロパティではなくパスです

DynamoDB の partition key は item の中の属性です。Datastore の key はプロパティですら
ありません。プロパティの *隣* にあり、kind と識別子の対を並べたパスで、手前の要素は
祖先です。このパッケージと `dynamobind` の構造的な違いは、すべてこの 1 点から出ています。

```go
type Reading struct {
	ID   SensorID      `firestore:"-,name"`   // key パスの末尾
	Site datastore.Key `firestore:"-,parent"` // その手前すべて
	// ...
}
```

生成されるのは 4 つです。

```go
func (v Reading) Kind() string             { return "Reading" }
func (v Reading) EntityKey() datastore.Key // 祖先を含めたパス全体
func (v Reading) EncodeEntity() datastore.Entity
func (v *Reading) DecodeEntity(e datastore.Entity) error
```

identity のフィールドは **プロパティマップに現れません**。プロパティとしても書くと
identity が二重に保存され、2 つの写しがずれていきます。だから encoder はそれを外に出し、
decoder は key から埋め戻します。`Load` が返した値は、もう一度読まなくても自分の identity
を持っています。

これはクエリに帰結します。保存されていないプロパティで filter はできません。`where` 句に
`ID` を書いた宣言はその旨の生成エラーになり、代わりに ancestor 句を指します。

もし重複を *欲しい* なら — filter したいなら — tag に実際の名前を与えてください。

```go
ID SensorID `firestore:"sensor,name"` // key の末尾であり、保存もされる
```

これは明示的な選択で、2 つを揃え続けるのは利用者の責任になります。

kind の既定は Go の型名です。設定するものはなく、必要もありません。`Reading` はどこに
保存されていても `Reading` で、table 名とは違います。`dynamobind` が断らざるを得なかった
single-table の問いがここで起きないのも同じ理由です。1 つの型が 1 つの kind、というのが
構造から出ています。

識別子フィールドがゼロ値のままだと **incomplete key** になります。insert では正当で、
サーバが id を採番する場所です。`Store` と `Insert` は保存された key を返すので、呼び出し
側が代入し直します。

```go
task := Task{Title: "write it down"}       // Number は 0
key, err := firestorebind.Insert(ctx, task)
task.Number = key.Path[0].ID
```

incomplete key に対する `Remove` は、リクエストを送る前にエラーになります。何を指すのか
決まらないからです。

## クエリ宣言

パッケージの隣に置いた `.tb.firestore` file がアクセスパターンを宣言します。生成は
それぞれを名前付き関数 1 つに変えます。

```text
export statement RecentReadings(from: time.Time, size: int): firestore.batch<Reading> {
  where at > {from}
  order at desc
  limit {size}
}

export statement HotOrNamed(sensor: SensorID, note: string, from: time.Time): firestore.batch<Reading> {
  where (sensor == {sensor} or note == {note}) and at > {from}
  index sensor, at
}

statement readingsUnder(parent: datastore.Key): firestore.many<Reading> {
  ancestor {parent}; order at
}
```

### 文法

```text
[export] statement <Name>(<param>: <GoType>, ...): firestore.<shape><<EntityType>> {
  where <条件>
  ancestor {param}
  select <プロパティ>, ...
  distinct <プロパティ>, ...
  order <プロパティ> [asc|desc], ...
  start {param}
  end {param}
  limit <n>|{param}
  offset <n>|{param}
  index <プロパティ> [asc|desc], ...
}
```

どの句も省略できます。`export` は名前自身の大文字小文字と一致していなければなりません。
Go は名前で可視性を決めるので、片方だけ書くのは静かなリネームではなく生成エラーです。
パラメータの型はパッケージが書くとおりの Go の型です。句の順序は自由、`;` で 1 行に
並べられ、`//` は行末までのコメントです。

`kind` 句はありません。結果型が束縛された型を名指し、その型が kind を供給するので、宣言が
codec と食い違えません。item 操作も kind を取らないので、両者は「たまたま違う」のでは
なく一貫しています。

結果の shape はリクエストの形を選びます。クエリは常に batch を返すからです。

| shape | 生成される戻り値 | リクエスト |
|-------|------------------|------------|
| `firestore.batch<T>` | `(firestorebind.Page[T], error)` | ちょうど 1 回 |
| `firestore.many<T>` | `iter.Seq2[T, error]` | range が進むたび batch 1 つ |
| `firestore.count<T>` | `(int64, error)` | 集計クエリ 1 回。entity はデコードしません |
| `firestore.keys<T>` | `(firestorebind.KeyPage, error)` | keys-only 1 回 |

### 条件

```text
where sensor == {s} and at > {from}
where sensor == {s} or note == {n}
where (sensor == {s} or note == {n}) and at > {from}
where sensor in {sensors}
where sensor not in {retired}
```

比較は `==`、`!=`、`<`、`<=`、`>`、`>=`、`in`、`not in` です。`=` を書くと、Go と同じく
`==` と書くよう言われます。

`and` は `or` より強く結び、これも Go と同じです。括弧で上書きできます。両方の演算子が
要るのは Datastore が両方で合成するからですが、昔からそうだったわけではありません。この
文法はしばらく `or` を名指しで拒否し、AND のみのワイヤだと説明していました。書いた時点
では本当で、driver が選言クエリを得た時点で本当ではなくなりました。

`in` と `not in` は候補を取るので、パラメータはプロパティが保存する型のスライスです。
`sensor in {sensors}` は `sensors: []SensorID` を要求し、それ以外は両方の型を名指しした
生成エラーになります。

選言クエリは見た目よりコストがかかります。Datastore は filter 全体を選言標準形に展開し、
`datastore.MaxDisjunctions` で打ち切ります。`and` の中に入れた `or` は加算ではなく乗算に
なる。ここでは数えていません。展開の規則はサービスのもので、食い違う数え方は動くクエリを
拒否するからです。生成される godoc が定数名を出すので、数字までは 1 手です。

### 順序、上限、カーソル

`order` は 1 つ以上のプロパティを取り、`desc` と書かない限り昇順です。

`limit` と `offset` はリテラルかパラメータを取ります。offset は避ける価値があります。
飛ばした entity も読まれ、課金されるからです。再開はカーソルのほうが安く、それが
`start` と `end` です。

```text
export statement ReadingsFrom(cursor: datastore.Cursor, size: int): firestore.batch<Reading> {
  order at
  start {cursor}
  limit {size}
}
```

パラメータは `datastore.Cursor` でなければならず、string を間違って渡せません。渡し戻す
のは `Page[T].EndCursor` です。

### projection

`select` は返るものを指定したプロパティに絞ります。結果型は変わりません。選ばなかった
ものがゼロ値のまま、束縛された型が返ってきます。これは decoder が「プロパティが無い」
ときに既にやっていることです。projection は帯域の話であって、形の話ではありません。

危険なのは入り口ではなく出口です。Datastore に部分更新はないので `Store` と `Update` は
entity 全体を置き換えます。projection の結果を書き戻すと、選ばなかったプロパティが全部
消えます。生成される godoc が、projection を含む関数すべてでそう書きます。防いではいません。
値は束縛された型のままで、`EntityEncoder` を満たしたままです。

projection は index から読むので、選んだプロパティはすべて index されている必要があります。
`noindex` のフィールドを select すると生成エラーです。配列プロパティを select すると、
サービスは entity ごとではなく要素ごとに 1 件返します。generator はフィールドの Go の型を
見てそれを godoc に書きます。1 つだけ検査していない規則があります。等値 filter で既に
固定されているプロパティの projection をサービスは拒否します。これは公開された規則ですが
このリポジトリで実測しておらず、間違った検査は動くクエリを拒否する側に転ぶので、godoc で
名前を挙げるに留めています。

`distinct` は指定プロパティが同じ結果をまとめます。Datastore はその プロパティが order の
先頭に並ぶことを要求し、両方の句が同じ宣言の中にあるので、これは推測ではなく構造として
検査しています。

### すべて tag に対して検査されます

宣言はテキストで、テキストだけでは何も塞げません。塞いでいるのは、生成がその中のすべての
名前を型の `firestore` tag に突き合わせることです。

```text
readings.tb.firestore:5: statement ReadingsByNote: Reading has no property "nte"
readings.tb.firestore:9: statement A: parameter s is string, but property sensor is stored from SensorID
readings.tb.firestore:14: statement B: body is tagged noindex on Reading, so it is
in no index and a query naming it can never match
```

検査は条件のツリーの中まで届きます。`or` の内側に入れ子になったリネームも、トップレベル
のものとまったく同じ大きさで落ち、メッセージはその比較を書いた行を指します。

### 生成されるシグネチャ

```go
func RecentReadings(ctx context.Context,
	from time.Time, size int, opts ...datastore.ReadOption) (firestorebind.Page[Reading], error)
```

kind もなく、client もなく、クエリビルダもありません。可変長オプションは driver に届くので
`datastore.WithEventualConsistency` や `WithReadTime` が使えます。

宣言は、transaction が扱える shape — `batch`、`count`、`keys` — については transaction 版も
生成します。

```go
func RecentReadingsTx(ctx context.Context, tx *firestorebind.Tx,
	from time.Time, size int) (firestorebind.Page[Reading], error)
```

iterator の shape には作りません。transaction の中の `range` は、コミットしなければならない
ものの内側で往復回数を無制限に発生させます。それを楽にするラッパーは、コストを束ねている
のではなく隠しています。transaction の中で全件が要るなら batch 形で明示的にページングして
ください。

### driver のビルダも残っています

`Query`、`QueryPage`、`Count`、`QueryKeysPage` は手で組んだ `*datastore.Query` を取ります。
実行時に形が決まるクエリのためです。`datastore.Query` のメソッドはすべて対応する句を
持つようになったので、これはもう「文法で言えないことを言う」ための口ではありません。
「リクエストが来るまで分からないことを言う」ための口です。手で組んだクエリは tag に対して
検査されません。

```go
q := datastore.NewQuery("Reading").Filter("sensor", datastore.Equal, datastore.String(id))
page, err := firestorebind.QueryPage[Reading](ctx, q)
```

## 複合インデックス

単一プロパティの index は自動です。複合 index は自動ではありません。等値 filter と不等号を
組み合わせたクエリ、あるいは別のプロパティで並べ替えるクエリは、別途宣言して `gcloud` で
適用した index を必要とします。無いと、そのクエリはコンパイルが通り、ここでの検査を全部
通過し、最初の実行で `FAILED_PRECONDITION` になります。

`index` 句は、デプロイ手順が適用できる値を出します。

```text
export statement WarmReadings(sensor: SensorID, from: time.Time): firestore.many<Reading> {
  where sensor == {sensor} and at > {from}
  order at desc
  index sensor, at desc
}
```

```go
yaml, err := datastore.MarshalIndexYAML([]datastore.Index{WarmReadingsIndex})
// gcloud datastore indexes create index.yaml に渡す
```

statement が export なら index も export されます。デプロイ手順が届く必要があるからです。

導出はしません。generator は宣言の filter と order を見て、必要な index を割り出すことが
できます。そして driver がまさにそれを見送りました。規則が微妙で、静かに間違った導出は
「そのクエリを直さない index」を名指すことになる、という理由です。この論拠は下流のほうが
強く効きます。ビルドログの間違った診断は権威に見え、名指された index を追加した著者の
クエリは依然として壊れたままだからです。

代わりにあるのは、何も名指さないヒントです。複合 index が要りそうな形で、index 句を書いて
いない宣言には、「必要かもしれない」と述べる godoc の行が付きます。確実性を主張しないので、
それを根拠に間違った行動を取ることができません。

宣言はデプロイではありません。`index` 句を書かない著者が得るのは、せいぜいヒントです。

## client と namespace は Context から来ます

client は 1 プロセスの事実です。パラメータとして受け取るものはありません。一度入れておけば、
呼び出し側にも生成されたシグネチャにも現れません。

```go
ctx := firestorebind.WithClient(r.Context(), client)
```

```go
WithClient(ctx context.Context, c *datastore.Client, options ...ClientOption) context.Context
ClientFromContext(ctx context.Context) (*datastore.Client, error)
```

`ClientFromContext` は逃げ道で、このパッケージが包んでいない操作のために driver へ直接
届くためのものです。

`dynamobind` はここに table 名の resolver を必要とします。宣言した table 名とデプロイされた
table 名が違うからです。このパッケージには要りません。kind は型に内在していて、デプロイが
それを改名することはないからです。

代わりに変わるのはテナントです。namespace は「誰が訊いているか」であって「その型が何か」
ではないので、型に載せると 1 つの構造体が 2 つ目のテナントで使えなくなります。プロセスに
固定の namespace は driver の `datastore.WithNamespace` が担い、リクエストごとに変わるものを
こちらが担います。

```go
ctx := firestorebind.WithClient(r.Context(), client,
	firestorebind.WithNamespace(func(ctx context.Context) string {
		return tenantOf(ctx)
	}))
```

生成された key は namespace を持ちません。ランタイムが出ていく時点で解決したものを刻むので、
束縛された型はテナントをまたいで持ち運べます。既に namespace を持つ key はそのままです。

client が入っていない Context は `ErrNoClient` で、結果の形が許すやり方で報告されます。
error を返す関数は返し、iterator はゼロ値とともに 1 度だけ yield して止まります。

2 つ目のプロジェクト、2 つ目のデータベース、テスト用の client は、2 つ目のシグネチャでは
なく 2 つ目の Context です。

## ランタイム操作

```go
Load[T](ctx, key, opts...) (T, error)
Store[T](ctx, v, opts...) (datastore.Key, error)
Insert[T](ctx, v, opts...) (datastore.Key, error)
Update[T](ctx, v, opts...) error
Remove[T](ctx, v, opts...) error

QueryPage[T](ctx, q, opts...) (Page[T], error)
Query[T](ctx, q, opts...) iter.Seq2[T, error]
Count(ctx, q, opts...) (int64, error)
QueryKeysPage(ctx, q, opts...) (KeyPage, error)

LoadAll[T](ctx, keys, opts...) (values []T, missing, deferred []datastore.Key, err error)
StoreAll[T](ctx, vs, opts...) ([]datastore.Key, error)
InsertAll[T](ctx, vs, opts...) ([]datastore.Key, error)
RemoveAll[T](ctx, vs, opts...) error
```

どれも kind を取りません。identity は key で完結しているので、シグネチャが運ぶものが
残っていない。ここが `dynamobind` の対応物より短い唯一の場所です。あちらはすべての入り口が
table を名指します。

ディスパッチは registry ではなく型制約です。生成コードのない型はコンパイルに失敗します。
誰も書かなかった登録のせいで実行時に落ちるのではなく。

`Store` は upsert です。`Insert` と `Update` は前提条件を名前に持ちます。そしてそれで
全部です。これらはワイヤ自身の動詞であって、このパッケージが組み立てた条件ではありません。
部分更新はありません。Datastore に無いので、すべての書き込みが entity を置き換えます。

returning 形はありません。commit は以前の entity を返さないので、`StoreReturning` や
`RemoveReturning` にはデコードするものがない。古い値が要るなら transaction の中で読んで
ください。それが正直なコストです。

## 条件付き書き込み

コストの小さい順に 3 段階あります。

**動詞。** `Insert` は key が存在すれば失敗し、`Update` は存在しなければ失敗します。
put-if-absent と put-if-present は呼び出し側が最も頻繁に書く 2 つの条件で、ここではコストが
ゼロです。`dynamobind` は前者を得るために条件式を生成しなければなりません。

**`version` tag。** decoder が読み取り時の entity version をそこに入れ、後の `Store` や
`Update` がそれを前提条件として送ります。

```go
r, err := firestorebind.Load[Reading](ctx, key) // r.Version が入る
r.Celsius = 21.5
_, err = firestorebind.Store(ctx, r)            // 他が書いていなければ適用
```

衝突は `datastore.ErrFailedPrecondition` で、そのまま届きます。一度も読んでいない値は
version がゼロなので前提条件を送らず、最初の書き込みはただの `Store` です。`Insert` は
取りません。key が存在すれば既に失敗するので、まだ存在してはいけない entity への前提条件は
何も言っていないからです。

2 つのバックエンドが最もきれいに分かれるのがここです。`dynamobind` は唯一の
`ConditionExpression` の枠を生成条件のために確保しなければならず、呼び出し側自身の条件と
`version` tag は共存できません。こちらの `baseVersion` は mutation のフィールドなので、
filter や ancestor も使いたい呼び出し側から tag が何も奪いません。

**transaction。** 述語の形をしたものすべて。プロパティの値に対する述語をこのワイヤ上で
評価するものは何もないので、transaction の中で読んで Go で判断するのは代替手段ではなく、
唯一の経路です。

## transaction

```go
err := firestorebind.Run(ctx, func(tx *firestorebind.Tx) error {
	task, err := firestorebind.LoadTx[Task](ctx, tx, key)
	if err != nil {
		return err
	}
	task.Title = "after"
	tx.Store(task)
	return nil
})
```

```go
Run(ctx, fn func(*Tx) error, opts ...datastore.TxOption) error
RunReadOnly(ctx, fn func(*Tx) error, opts ...datastore.TxOption) error

LoadTx[T](ctx, tx, key, opts...) (T, error)
LoadAllTx[T](ctx, tx, keys) (values []T, missing, deferred []datastore.Key, err error)
QueryPageTx[T](ctx, tx, q) (Page[T], error)
QueryKeysPageTx(ctx, tx, q) (KeyPage, error)
CountTx(ctx, tx, q) (int64, error)

func (tx *Tx) Store(v EntityEncoder, opts ...datastore.WriteOption)
func (tx *Tx) Insert(v EntityEncoder, opts ...datastore.WriteOption)
func (tx *Tx) Update(v EntityEncoder, opts ...datastore.WriteOption)
func (tx *Tx) Remove(v Keyer, opts ...datastore.WriteOption)
```

`dynamobind` は transaction を提供しません。DynamoDB の driver が宣言していないからです。
あちらで除外した理由が、こちらでは採用する理由になります。read-modify-write を表現できる
唯一の手段だからです。

読み取りがメソッドではなく別の関数なのは、Go のメソッドが型パラメータを取れないためで、
`Load` 自身と別なのは、transaction 内の読み取りがハンドルを経由しなければならないためです。
Context にハンドルを載せると、どの Context が届いたかで 1 つの呼び出し箇所が 2 つの意味を
持つことになります。

隠していない性質が 2 つあります。隠すと嘘になるからです。

**クロージャは複数回実行されることがあります。** 競合するとサーバが `ABORTED` を返し、
driver は commit を再送するのではなくクロージャ全体を再実行します。それが構築の土台に
した読み取りが古くなっているからです。だからクロージャは transaction の外に副作用を持って
はいけません。中で送ったメッセージや書いたファイルは、複数回起きることがあります。

**キューされた書き込みは何も返しません。** `tx.Store` に返すべき error がないのは、まだ
何も起きていないからです。mutation は commit と一緒に運ばれます。だから error を返した
クロージャは何も書かず、ロールバックも要りません。

retry ループはここで足しません。driver 自身の再実行予算が効き、
`datastore.WithTxRetries` がそれを設定します。

## page と iterator

`QueryPage` はリクエスト 1 回です。`Query` は代わりに反復し、range が進むたびに batch を
要求します。

1 回の `range` が何回もリクエストを出すことがあり、iterator は batch の数値を何も報告
しません。それが気になるときは `QueryPage` か、宣言の `firestore.batch<T>` に手を伸ばして
ください。ループを break すれば、次のリクエストを出さずに止まります。

`Page[T]` は bool にすると失われる 2 つを保っています。

```go
type Page[T any] struct {
	Values         []T
	EndCursor      datastore.Cursor
	More           datastore.MoreResults
	SkippedResults int32
}
```

`More` は batch が *なぜ* 終わったかを言います。尽きたのか、limit に当たったのか、cursor に
当たったのか。limit で終わった batch には訊く価値のある続きがあり、尽きた batch にはあり
ません。`SkippedResults` は offset が飛ばした件数で、それらは読まれ、課金されています。

`QueryKeysPage` は同じ形の `KeyPage` を返します。keys-only クエリのためのものです。
`KeysOnly` を代わりに設定はしません。設定するラッパーは、呼び出し側のクエリが entity と
言っているところに key を返すことになります。

## batch

`LoadAll` は driver 自身の定数 `datastore.MaxLookupKeys` で分割し、2 つではなく 3 つの
リストを返します。

```go
values, missing, deferred, err := firestorebind.LoadAll[Reading](ctx, keys)
```

これらは 3 つの別々の事実です。**missing** な key は何も保持していません。**deferred** な
key はサーバが今回読まないことにしたもので、再試行するかは利用者の判断です。「無かった」に
畳むと、まさにそこが失われます。値はサーバの応答順で返り、渡した key の順ではありません。

`StoreAll` は件数ではなく、エンコード後のサイズを `datastore.MaxRequestBytes` に対して
見て分割します。これは実装上の手抜きではありません。Google は commit あたりの mutation
件数の上限を文書化しておらず、commit の境界はバイト数で、件数でのチャンク分割はこの
パッケージが作った数字になってしまいます。サイズ算出は entity ごとに JSON marshal 1 回分の
コストで、これは driver がその後に行う marshal とは別です。

サービスの上限値はここに 1 つも書かれていません。`MaxLookupKeys`、`MaxRequestBytes` ほかは
driver の定数です。写した上限値こそが、サービスが変わったときにずれるものだからです。

分割された batch は transaction **ではありません**。大きな書き込みは複数回に分けて commit
され、失敗すると手前の分は書かれたまま残ります。全部か無かにしたいなら `Run` を使ってく
ださい。`datastore.MaxTransactionBytes` の範囲で。

## error

driver の sentinel はすべて生き残ります。

```go
_, err := firestorebind.Load[Reading](ctx, key)
if errors.Is(err, datastore.ErrNoSuchEntity) {
	// 見つからないことは見つからないまま。ゼロ値としては届きません
}

var driverError *datastore.Error
if errors.As(err, &driverError) {
	log.Println(driverError.Op, driverError.Status, driverError.Retryable())
}
```

判別は `Status` で行ってください。HTTP のコードでは決してやらないこと。`ALREADY_EXISTS` と
`ABORTED` はどちらも 409 で、意味は正反対です。片方は終端、片方は再試行可能。409 で分岐する
コードは、重複 insert を永遠に再試行します。このパッケージの中にもステータスコードで分岐
している箇所はありません。

デコードの失敗はプロパティ名と両方の kind を名指します。

```go
if mapping, ok := firestorebind.AsError(err); ok {
	log.Println(mapping.Property, mapping.Expected, mapping.Got) // scale double integer
}
```

`AsError` は `errors.As` を使わずにチェーンを辿ります。`errors.As` は reflection を必要と
するからです。

## 生成

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

生成はパッケージが何を呼んでいるかに導かれます。`Store` があれば encoder、`Load` があれば
decoder、どこからも名指されない型からは何も出ません。ネストした構造体は親の操作を継承
しますが、key は継承しません。`entityValue` は key を持たないからです。`.tb.firestore` の
宣言は結果型の使用としてカウントされるので、Firestore の使用が宣言だけのパッケージでも、
生成されたクエリが必要とする decoder は出ます。

例外が 3 つあり、呼び出しの発見ではなく tag から生成されます。`Kind`、`EntityKey`、
`EntityVersion` です。entity を読む文書化された方法は `Load(ctx, v.EntityKey())` であり、
ランタイムは version を interface assertion で訊きます。どちらも generator が発見できる
呼び出しではないので、発見を待つとそのメソッドは呼ばれるために存在することが一度もない
ままになります。いずれも葉の関数で、誰も呼ばなければリンカが落とします。

生成される file はすべて入力の SHA-256 を記録します。ソース、`.tb.firestore` file、
`go.mod`、オプション、generator の binary がすべて記録どおりに hash されるなら、再実行は
何も生成せずに終わります。`-force` は無条件に再生成します。

```go
options := generator.DefaultOptions()
options.DisableFeatures = []generator.Feature{generator.FeatureEntityCodec}
options.FirestoreTemplatePattern = "*.query.firestore"
```

| 設定 | 効果 |
|------|------|
| `FeatureEntityCodec` | Firestore モード全体を止めます。クエリも含めて |
| `FirestoreTemplatePattern` | 宣言の glob。既定は `*.tb.firestore` |

`tinybind-gen fmt` は他の 3 つのテンプレート言語と並んで `.tb.firestore` を整形します。
`--firestore-template-pattern` と `-as firestore` が使えます。整形は冪等で、コメントを
保ち、句を書かれた順序によらず固定の順に並べ替えます。読む順序が、著者がたまたま先に
打った順ではなく、クエリが何をするかに従うようにです。

## 生成エラー

どの検査も型とフィールド、あるいは statement とプロパティを名指します。行動に移せる
メッセージであることが、本番ではなくここで落とす理由のすべてだからです。

tag と型の検査:

- 未知の `firestore` tag オプション
- `firestore` tag のないフィールドに付いた `datastore` tag
- 2 つのフィールドが 1 つのプロパティ名に対応する
- `name`、`id`、`parent`、`version` が 2 つ以上、あるいは `name` と `id` の併用
- string でないフィールドの `name`、`int64` でないフィールドの `id` / `version`、
  `datastore.Key` でも束縛された型でもないフィールドの `parent`
- 自分自身の型に到達する parent の連鎖
- key を持たないネスト型への identity オプション
- プロパティとして保存されないフィールドへの `noindex`
- プロパティの形を持たない Go の型: map、`uint` / `uint64` / `uintptr`、スライスのスライス、
  channel、関数、interface
- 別パッケージで宣言されたネスト構造体
- `EncodeEntity`、`DecodeEntity`、`EntityKey`、`Kind`、`EntityVersion` を手で宣言済みの型

クエリの検査:

- `firestore` tag を持たない entity 型
- 型が持たないプロパティ（条件ツリーのどの深さでも）
- `noindex` のプロパティを名指す filter、order、projection、index
- 保存されない identity / version フィールドを名指す filter
- プロパティの Go の型と一致しないパラメータ、`in` が要求するスライスでないパラメータ
- 宣言されていないパラメータを指すプレースホルダ、一度も使われないパラメータ
- `datastore.Key` でない ancestor パラメータ、`datastore.Cursor` でない cursor パラメータ
- `count` への `select` / `start` / `end`、`keys` クエリへの `select`
- order の先頭に並んでいない `distinct` のプロパティ
- 比較が要る場所に書かれた `or`、`==` が要る場所の `=`
- 同名の statement 2 つ

## サイズ

測っていません。`dynamobind` に実測の表があるのは、あちらの事情がそれを必要としたから
です。DynamoDB の driver は reflection の mapper を持っているので、生成することを動いている
代替に対して正当化する必要があり、測った結果は「生成された経路のほうが大きい」でした。

1 つだけ持ち越せる数字があります。同じ仕組みだからです。client を Context から解決する
コストは、`dynamobind` の `wasip1` 上の実測で約 38 KB でした。素の `context.WithValue` と
型アサーション 1 つが、TinyGo が普段落とす型記述子の機構を引き込みます。こちらも同程度と
見てください。パッケージ固有ではなく、パターンに内在するものです。

それが問題になるほど厳しいターゲットなら、生成された `EncodeEntity`、`DecodeEntity`、
`EntityKey` はただのメソッドです。それらを使って driver を直接呼べば、このパッケージは
1 バイトもリンクされません。

## 未実装

- **GQL。** 同じクエリに対する 2 つ目のリクエスト形式で、独自のエスケープの話が要ります。
- **宣言からの SUM と AVG。** driver には `Sum` と `Avg` があります。まだそこへ届く結果
  shape がないので、手で組んだクエリで呼んでください。
- **`AllocateIDs`。** 型付きのヘルパーはありません。書き込み前に key が要るときは driver の
  ものを使ってください。
- **kind の上書き。** kind は Go の型名です。`kind=` オプションは、もし要るようになった
  ときの形として残していますが、必要としたものがありません。
- **TTL。** Datastore mode のこのワイヤ上では表現できません。expiry は
  `gcloud firestore fields ttls update` で別途適用する、通常の timestamp プロパティに対する
  フィールドレベルのポリシーです。だから期限付き entity に tag は要らず、ただの
  `time.Time` フィールドがすべてです。ここは Datastore のほうが DynamoDB より単純な唯一の
  場所で、`dynamobind` の `ttl` tag は driver の `UpdateTimeToLive` を待っています。
- **watch、listener、property transformation。** driver が除外しています。transformation の
  除外は意図的です。サーバ側の増分と配列追加は、driver の retry ポリシーが避けるために
  作られている「再送が冪等でない」危険をそのまま呼び戻します。このパッケージが書き込みを
  再送可能だと文書化できているのは、その除外の上に立っています。
