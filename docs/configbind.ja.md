# configbind 利用ガイド

`configbind` は、アプリケーション設定を Go の構造体へ読み込むパッケージです。構造体を一度定義すると、default、TOML、環境変数、CLI option を同じ field へ重ね合わせます。

この重ね合わせ方は設定で変えられません。優先順位は固定で、右側ほど優先されます。

```text
default < TOML file < environment variable < CLI option
```

> [!IMPORTANT]
> configbind の TOML parser は標準 TOML のフルセットではなく、設定用途に絞った subset です。quoted key、inline table、nested array などは利用できません。既存の一般的な TOML file をそのまま読み込む用途ではなく、対応範囲に合わせて設定 file を用意してください。詳しくは「[TOML file](#toml-file)」を参照してください。

## 自動化されること

- 設定構造体と `configbind.Bind[T]` の利用箇所の発見
- 構造体 field から TOML key、CLI option、環境変数名の決定
- `default`、`key`、`opt`、`env`、`help`、`falsy`、`dependon` tag の反映
- nested struct、`[]string`、array of tables から作る struct slice の設定 mapping
- default → TOML → env → CLI の merge
- string、bool、int、`time.Duration`、`[]string` への型変換
- 各設定値が最終的にどの入力元から来たかの、定義順かつ secret を mask した記録

生成コードの内部を利用者が実装することは一切ありません。アプリケーションがすることは、`Bind` で設定 pointer を取得し、起動時に一度 `Load` を呼ぶことだけです。

## ユーザーが用意するもの

1. 設定を表す Go の構造体
2. literal prefix を指定した `configbind.Bind[T]("prefix")` 呼び出し
3. アプリケーション起動時の `configbind.Load`
4. 必要に応じた TOML file、環境変数、CLI option
5. コード生成の実行

## 導入とコード生成

```go
package main

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

解析対象 package に具体的な `Bind` 呼び出しを置きます。

```go
func registerConfig() *ServerConfig {
	return configbind.Bind[ServerConfig]("server")
}
```

```bash
go generate ./...
```

configbind の対象がある場合、既定では `configbind_gen.go` が生成されます。生成は `Bind` の type parameter と prefix を静的に読み取ります。だからこそ prefix は計算結果ではなく文字列 literal でなければなりません。

## 設定 file の雛形生成

各 package の生成コードは、その package 内の `Bind` ごとに定義を1つ登録します。公開された `configbind` の関数が、framework と import 済みの全 application package から雛形 field を統合します。

```go
func ScaffoldTOML() (string, error)
func ScaffoldEnv() (string, error)
func WriteScaffoldTOML(w io.Writer) error
func WriteScaffoldEnv(w io.Writer) error
```

TOML 出力は対応 subset 内の文法を使います。どちらの形式も `default` があればその値を、なければ型に応じた zero value を使い、`help` tag は comment になります。環境変数の雛形には `opt`、`env:"NAME"`、`env:"-"` も反映されます。

`[prefix]` table 内の key は構造体の定義順に並びます。table 自体は prefix と型名の順なので、雛形の出力順が package の初期化順に左右されることはありません。環境変数の雛形は table による grouping がないため、従来どおり変数名順のままです。

たとえば次の定義がある場合:

```go
// ServerConfig は公開 listener の設定です。
type ServerConfig struct {
	Port     int    `default:"8080" opt:"port,p" help:"HTTP listen port"`
	Host     string `default:"localhost" help:"listen host"`
	Internal string `env:"-"`
}

func serverConfig() *ServerConfig {
	return configbind.Bind[ServerConfig]("server")
}
```

統合後の出力には、次と同等の内容が含まれます。

```toml
# ServerConfig は公開 listener の設定です。
[server]
# HTTP listen port
port = 8080
# listen host
host = "localhost"
internal = ""
```

構造体の godoc は table の comment になります。`.env` の雛形は変数名で全体を sort するため、field の comment だけが付きます。

```dotenv
# HTTP listen port
PORT=8080
# listen host
SERVER_HOST="localhost"
```

server framework package とモジュラモノリスの各 package で、別々に generator を実行できます。生成済み package を import すると定義がすべて登録されるため、最終 application が依存 package の source を再解析する必要はありません。出力順は deterministic で、key や環境変数名が重複した場合は error になります。

generator は実行時に file を作成せず、application に雛形出力用 subcommand も追加しません。application に合う command をユーザー側で用意して、公開出力関数を呼び出してください。

```go
import (
	"fmt"
	"os"

	"github.com/shibukawa/tinybind-go/configbind"
)

func printConfigScaffold(format string) error {
	if format == "env" {
		return configbind.WriteScaffoldEnv(os.Stdout)
	}
	if format == "toml" {
		return configbind.WriteScaffoldTOML(os.Stdout)
	}
	return fmt.Errorf("unknown scaffold format %q", format)
}
```

file が必要なときは redirect します。

```bash
./myserver scaffold-config toml > config.toml
./myserver scaffold-config env > .env
```

ひとつ見落としやすい隙間があります。`configbind.Load` が読むのは process の環境変数だけで、`.env` file を parse することはありません。雛形から作った `.env` は、任意の dotenv loader や shell の仕組みで、`Load` より前に process へ届けておく必要があります。

## 最小例

```go
package main

import (
	"fmt"
	"log"

	"github.com/shibukawa/tinybind-go/configbind"
)

type ServerConfig struct {
	Port int    `default:"8080" help:"HTTP listen port"`
	Host string `default:"localhost" help:"listen host"`
}

func main() {
	cfg := configbind.Bind[ServerConfig]("server")
	if _, err := configbind.Load(configbind.LoadOptions{
		Vendor: "acme",
		Tool:   "myserver",
	}); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("listen on %s:%d\n", cfg.Host, cfg.Port)
}
```

値を何も指定しなければ、`localhost:8080` になります。

```bash
# 環境変数
SERVER_HOST=0.0.0.0 SERVER_PORT=9000 ./myserver

# CLI。CLI は環境変数より優先
SERVER_PORT=9000 ./myserver --server-port 10000
```

## 構造体 tag

| Tag | 役割 | 例 |
| --- | --- | --- |
| `default:"value"` | どの入力元にも値がない場合の値 | `default:"8080"` |
| `key:"name"` | TOML と内部 key の field 名を変更 | `key:"listen_port"` |
| `opt:"long"` | CLI long option 名を上書き | `opt:"port"` |
| `opt:"long,p"` | long option と1文字の short option を指定 | `opt:"port,p"` |
| `env:"NAME"` | 環境変数名を正確な名前で上書き | `env:"OTEL_SERVICE_NAME"` |
| `env:"-"` | その field の環境変数入力を無効化 | `env:"-"` |
| `help:"text"` | option の説明 metadata | `help:"HTTP listen port"` |
| `falsy:"value"` | string option において「off」を意味する選択肢 | `falsy:"off"` |
| `dependon:"key"` | 指定 key が空の間、この field を provenance から隠す | `dependon:"webserver.tls.enabled"` |

`falsy` と `dependon` は安定した設定 key を必要とするため、array of tables の要素 field には指定できません。要素の key は設定全体ではなく個々の要素に属するからです。

### godoc を説明の source にする

`help` tag のない field は godoc comment を説明として使い、generator がその内容を struct tag に書き戻します。

```go
type ServerConfig struct {
	// Port is the HTTP listen port.
	Port int `default:"8080"`
}
```

generator を1回実行すると source は次のようになります。

```go
type ServerConfig struct {
	// Port is the HTTP listen port.
	Port int `default:"8080" help:"Port is the HTTP listen port"`
}
```

以降は tag が唯一の source of truth です。既存の `help` tag は常に comment より優先され、再実行しても内容は変わりません。使われるのは最初の段落だけで、`//go:` や lint directive は除去され、末尾の句点も1つ削られます。行末 comment（`Host string // listen address`）も同様に使えます。

同じ text は生成された CLI の usage にも渡ります。help 文字列を空にして登録した `SubCommand` は、struct の godoc に fallback します。

手書きの source を generator に書き換えさせたくない場合は feature を無効化してください。生成結果には godoc 由来の help が入ったままになります。

```go
options := generator.DefaultOptions()
options.DisableFeatures = append(options.DisableFeatures, generator.FeatureHelpBackfill)
```

tag は組み合わせて使えます。そしてその組み合わせが、すべての表層での名前を一度に決めます。

```go
type ServerConfig struct {
	Port int `key:"listen_port" default:"8080" opt:"port,p" help:"HTTP listen port"`
}
```

prefix が `server` のとき、この1つの field は4つの名前で現れます。

| 種類 | 名前 |
| --- | --- |
| 安定した設定 key | `server.listen_port` |
| TOML | `[server] listen_port = 8080` |
| CLI | `--port 8080` または `-p 8080` |
| 環境変数 | `PORT=8080` |

`opt` を指定すると、既定の `--server-listen_port` は登録されません。環境変数名も `opt` の long option から決まります。

## 名前の決まり方

prefix が `webserver` の場合:

```go
type WebServerConfig struct {
	Port int
	Host string
	TLS  TLSConfig
}

type TLSConfig struct {
	Enabled  bool
	CertPath string
}
```

| Field | 設定 key | CLI option | 環境変数 |
| --- | --- | --- | --- |
| `Port` | `webserver.port` | `--webserver-port` | `WEBSERVER_PORT` |
| `Host` | `webserver.host` | `--webserver-host` | `WEBSERVER_HOST` |
| `TLS.Enabled` | `webserver.tls.enabled` | `--webserver-tls-enabled` | `WEBSERVER_TLS_ENABLED` |
| `TLS.CertPath` | `webserver.tls.cert_path` | `--webserver-tls-cert_path` | `WEBSERVER_TLS_CERT_PATH` |

Go field 名は snake case の key になります。CLI では nested key の `.` が `-` へ変わり、環境変数はさらに踏み込んで `-` と `.` の両方を `_` にし、全体を大文字にします。

prefix 自体に `.` を含めることもできます。prefix と field key の階層は設定 key と TOML では `.` のまま保持され、CLI ではすべて `-` へ正規化されます。

```go
cache := configbind.Bind[CacheConfig]("middleware.cache")
```

`MaxEntries` field の名前は次のようになります。

| 種類 | 名前 |
| --- | --- |
| 設定 key | `middleware.cache.max_entries` |
| TOML table | `[middleware.cache]` |
| CLI | `--middleware-cache-max_entries` |
| 環境変数 | `MIDDLEWARE_CACHE_MAX_ENTRIES` |

## TOML file

```toml
[webserver]
port = 8080
host = "127.0.0.1"
cors_origins = ["https://app.example.com", "https://admin.example.com"]
tls.enabled = true
tls.cert_path = "/etc/myserver/server.crt"
```

nested table でも書けます。

```toml
[webserver.tls]
enabled = true
cert_path = "/etc/myserver/server.crt"
```

繰り返す設定は array of tables で書きます。`[[...]]` header 1つが1要素になり、struct の slice へ入ります。

```toml
[[webserver.routes]]
path = "/"
dir = "./public"

[[webserver.routes]]
path = "/files"
dir = "./files"
listing = true
```

`[[...]]` header 以降の key はすべてその要素に属するため、その table 自身の key は最初の要素より前に書きます。開いている要素の下の standard table header、たとえば `[webserver.routes.rewrite]` はその要素の sub-table です。同じ nest は dotted key（`rewrite.from = "/old"`）でも書けます。

configbind が読む TOML は意図的に限定された subset です。

- table、nested table、bare dotted key
- string、bool、integer、float の scalar
- primitive scalar の array
- array of tables
- comment

quoted key、inline table、nested array は利用できません。ここにある制限は1つではなく2つです。parser が受け付ける範囲と、struct field が受け取れる範囲。狭いのは後者です。float の TOML 値は parse できても、float field へ直接 bind することはできません。

## 設定 file の探索

```go
result, err := configbind.Load(configbind.LoadOptions{
	Vendor:   "acme",
	Tool:     "myserver",
	FileName: "settings.toml",
})
```

`FileName` の既定は `config.toml` です。configbind は次の順で読み取り可能な
file を探し、最初に見つかった1つだけを読みます。

1. `ExplicitConfigPath`。field が空なら `--config-path`
2. `ExtraConfigReadPaths` の配列順
3. `Vendor` / `Tool` 配下の OS user config directory
4. `Vendor` / `Tool` 配下の OS system config directory

複数 file はマージしません。だからこそ local test 用の設定は、production の
system 設定に混ざるのではなく、それを置き換えられます。
`ExtraConfigReadPaths` の存在しない、または読めない項目は skip します。
どの候補も見つからなければ、default、env、CLI だけで load します。

実行時に file を明示するには `--config-path` を使います。

```bash
./myserver --config-path ./local.toml
```

明示した file が存在しない、読めない、directory である場合は error になり、通常の config directory へ fallback しません。

test などでは `ExplicitConfigPath` も利用できます。

```go
result, err := configbind.Load(configbind.LoadOptions{
	ExplicitConfigPath: "/tmp/test-config.toml",
	Args:               []string{},
	Environ:            []string{},
})
```

`ExplicitConfigPath` は `--config-path` より優先されます。本番では通常、`Args` から `--config-path` を受ける方法を使います。

任意の local file や deployment 固有 file には `ExtraConfigReadPaths` を使います。

```go
result, err := configbind.Load(configbind.LoadOptions{
	Vendor:               "acme",
	Tool:                 "myserver",
	ExtraConfigReadPaths: []string{"./config.test.toml", "/run/secrets/app.toml"},
})
```

`./config.test.toml` があれば、読むのはその TOML だけです。なければ
`/run/secrets/app.toml`、user config、system config の順に探索します。

### `LoadOptions` 一覧

| Field | 意味 | 既定 |
| --- | --- | --- |
| `Vendor` | OS config directory 内の vendor 名 | configdir 探索まで進む場合は必須 |
| `Tool` | application / tool 名 | configdir 探索まで進む場合は必須 |
| `FileName` | 探索する TOML basename | `config.toml` |
| `Args` | program 名を除いた CLI arguments | `nil` なら `os.Args[1:]` |
| `Environ` | `KEY=value` 形式の環境 | `nil` なら `os.Environ()` |
| `ExplicitConfigPath` | 強制的に使う file path | 空なら `--config-path`、extras、directory 探索 |
| `ExtraConfigReadPaths` | 配列順に探索する任意の file path | 存在しない項目は skip |

`nil` と空 slice の違いは test で効いてきます。`nil` は「process にフォールバックする」という意味だからです。CLI や環境の入力を完全に止めたいときは空 slice を渡します。

```go
Args:    []string{},
Environ: []string{},
```

## 環境変数

環境変数名は CLI の最初の long option 名から生成されます。

```go
type ServerConfig struct {
	Port int `opt:"port,p"`
	Host string
}
```

```bash
PORT=8080
SERVER_HOST=127.0.0.1
```

prefix だけを見て `SERVER_PORT` にするのではなく、`opt:"port,p"` により long option が `port` なので環境変数も `PORT` になります。

### 環境変数名を上書きする

外部の標準や既存の運用規約に合わせる場合は `env` tag を使います。TOML key とCLI optionはそのままに、環境変数名だけを独立して変更できます。

```go
type ObservabilityConfig struct {
	ServiceName string `env:"OTEL_SERVICE_NAME"`
	Endpoint    string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

observability := configbind.Bind[ObservabilityConfig]("observability")
```

`ServiceName` は次の名前になります。

| 種類 | 名前 |
| --- | --- |
| TOML | `[observability] service_name = "checkout"` |
| CLI | `--observability-service_name checkout` |
| 環境変数 | `OTEL_SERVICE_NAME=checkout` |

`env` の値は大文字・小文字を含めてそのまま利用され、英字または `_` で始まる環境変数名を指定します。同じ環境変数名を複数 field に割り当てると生成 error になります。環境変数から設定されたくない field には `env:"-"` を指定できます。

## CLI subcommand

`SubCommand[T]` は、生成される CLI 専用の command branch を宣言します。その
field は TOML も環境変数も一切読みません。`arg` のない field は option になり、
position field には `arg:"required"`、`arg:"optional"`、`arg:"*"` を指定します。

```go
type MigrateOptions struct {
	Path   string   `arg:"required" help:"migration directory"`
	Label  string   `arg:"optional" help:"migration label"`
	DryRun bool     `default:"false" help:"変更を適用せず表示"`
	Extra  []string `arg:"*" help:"追加のmigration input"`
}

server := configbind.Bind[ServerConfig]("server")
migrate := configbind.SubCommand[MigrateOptions]("migrate", "database migrationを実行")

if _, err := configbind.Load(configbind.LoadOptions{
	Vendor: "acme",
	Tool:   "myserver",
}); err != nil {
	// *configbind.UsageError には生成されたtop-levelまたはcommand usageが含まれます。
	log.Fatal(err)
}
if migrate != nil {
	runMigrations(*migrate)
	return
}
runServer(*server)
```

`go generate` 後は、次のどちらでも `MigrateOptions` が選択・設定されます。

```bash
./myserver migrate ./migrations
./myserver migrate ./migrations --dry_run release extra-a extra-b
```

選択された `SubCommand` だけが non-nil を返します。必須 position の不足、未知の
command や option、`--help` では、生成された usage を含む
`*configbind.UsageError` が返ります。option は position 引数の前後どちらにも
置けます。

選択と parse は同じ引数列を読みます。これは test で覚えておく価値があります。
本番では `LoadOptions.Args` を nil のままにして両方に `os.Args[1:]` を使わせ、
`Args` を上書きする test では、`SubCommand` を呼ぶ前に同じ内容を `os.Args` に
設定してください。

## CLI option

scalar option は分離形と `=` 形を利用できます。

```bash
./myserver --server-port 8080
./myserver --server-port=8080
```

bool field は値を省略すると true です。明示的な false も指定できます。

```bash
./myserver --webserver-tls-enabled
./myserver --webserver-tls-enabled=false
```

`[]string` は option を繰り返します。

```bash
./myserver \
  --webserver-cors_origins https://app.example.com \
  --webserver-cors_origins https://admin.example.com
```

未定義の option、値が必要な option の値不足、不正な bool は `Load` error になります。

非対称なのは TOML です。未知の key は parse を通り、対応する struct field がないまま黙って適用されません。つまり CLI の option を打ち間違えれば派手に失敗するのに、設定 file の key の打ち間違いは静かに失敗します。typo を厳密に拒否したい場合は、起動時に `LoadResult.Overlay.Keys()` と期待する key を照合してください。

## nested 設定と `[]string`

```go
type WebServerConfig struct {
	Port        int      `default:"8080"`
	Host        string   `default:"localhost"`
	CorsOrigins []string
	TLS         TLSConfig
}

type TLSConfig struct {
	Enabled  bool   `default:"false"`
	CertPath string
}
```

```toml
[webserver]
port = 8080
cors_origins = ["a.example", "b.example"]
tls.enabled = true
tls.cert_path = "server.crt"
```

```bash
WEBSERVER_TLS_CERT_PATH=production.crt \
  ./myserver --webserver-cors_origins cli.example
```

この場合、`CertPath` は env、`CorsOrigins` は CLI、`Enabled` は TOML、`Host` は default から取得されます。

## 繰り返す設定

struct の slice は array of tables から読み込まれます。

```go
type WebServerConfig struct {
	Routes []RouteConfig `help:"static routes"`
}

type RouteConfig struct {
	Path    string
	Dir     string
	Listing bool `default:"false"`
}
```

```toml
[[webserver.routes]]
path = "/"
dir = "./public"

[[webserver.routes]]
path = "/files"
dir = "./files"
listing = true
```

要素数そのものが data であるため、要素の field には CLI option も環境変数もありません。入力元は TOML file だけです。`default` は要素ごとに適用され、上の例では最初の route が `listing = false` になります。要素の field に `opt` や `env` を付けると、黙って無視されるのではなく生成時のエラーになります。subcommand は struct の slice を受け取れません。

要素の型は同一 package の named struct を値で持つ必要があります。`[]*RouteConfig` と、自分自身へ到達する struct はどちらも生成時に拒否されます。scaffold は slice ごとに `[[...]]` block の例を1つ出力します。

## 複数の設定構造体

複数の `Bind` target を登録し、1回の `Load` でまとめて適用できます。

```go
server := configbind.Bind[ServerConfig]("server")
database := configbind.Bind[DatabaseConfig]("database")

_, err := configbind.Load(configbind.LoadOptions{
	Vendor: "acme",
	Tool:   "myserver",
})
if err != nil {
	return err
}

_ = server.Port
_ = database.URL
```

すべての `Bind` は `Load` より前に呼びます。返された pointer は `Load` が成功すると値の入った状態になります。

## 入力元を確認する

`LoadResult.Provenance()` は、そのまま log に流せる形の実効設定を返します。

```go
result, err := configbind.Load(options)
if err != nil {
	return err
}

for _, entry := range result.Provenance() {
	log.Printf("%s = %s (%s)", entry.Key, entry.Value, entry.Place)
}
```

この slice は sort されているのではなく、順番が保たれています。binding は `Bind` を呼んだ順、その中の key は構造体の定義順で、nested struct は宣言された位置に展開されます。どの binding にも属さない key — たとえば誰かの TOML file に紛れ込んだ entry — は、既知の key のあとに辞書順で続きます。

slice を受け取る前に filter が 2 つ走ります。key path に `password`、`secret`、`token`、`apikey`、`api_key`、`credential`、`access_key` を含む key は、値の代わりに `*****` を返します。もう 1 つが `dependon` tag による抑制で、次節で説明します。

### 無効な機能の設定を隠す

使っていない subsystem も、放っておけば default 値の塊をそのまま出力し、実際に効いている設定を埋もれさせます。`dependon` は、その field が意味を持つかどうかを決める親を指定します。

```go
type WebServerConfig struct {
	Tracing    string `enum:"off,otlp,jaeger" falsy:"off" help:"tracing exporter"`
	TracingURL string `dependon:"webserver.tracing" help:"collector URL"`
}
```

親は prefix を含む完全な設定 key なので、別 package が bind した key にも依存できます。`webserver.tracing` が空と読める間、`webserver.tracing_url` は provenance の slice に現れません。`webserver.tracing` 自身は出力されます。親が空であること自体が、子が消えた理由だからです。親が隠れている場合は、その親に依存する field も連鎖して隠れます。

「空」とは空文字と `false` です。`int` の 0、空の list、0 秒の duration は「設定されていない」ではなく意図した設定なので、空とは扱いません。enum 型の option にはもう 1 つの形が要り、それが `falsy` です。「off」を意味する選択肢を宣言すると、その値は依存 field にとって空として扱われ、さらに何も値を設定しなかった場合の値としても使われます。

- `default` tag がなく、どの入力元も key を設定しない場合: `off` になります。
- 入力元が key を `""` に設定した場合: `off` になり、`Place` はその入力元のままです。
- `default` tag がある場合: default が優先され、`falsy` は使われません。

いずれも bind 先の構造体には影響しません。`TracingURL` は入力元の値で populate されますし、CLI flag や help も変わりません。雛形も全 field を出力し続けます。初回 load より前に option を発見できなくなっては困るためです。

### 生の overlay

`LoadResult.Overlay` には、filter を通していない merge 後の値と勝った入力元が入っています。

```go
result, err := configbind.Load(options)
if err != nil {
	return err
}

entry, ok := result.Overlay.Get("server.port")
if ok {
	log.Printf("server.port came from %s", entry.Place)
}
```

`Place` は次のいずれかです。

- `configbind.PlaceDefault`
- `configbind.PlaceFile`
- `configbind.PlaceEnv`
- `configbind.PlaceCLI`

table 全体を走査したい場合は `Overlay.All()` が key の辞書順で entry を返します。

`LoadResult.ConfigPath` は選ばれた file path、`FoundFile` は TOML file がそもそも見つかったかを示します。overlay の値は mask されないため、raw value をまとめて log すれば credential も一緒に log されます。log に出すものは `Provenance()` を使ってください。

## 利用する API

configbind は template のような新しい公開関数を生成しません。利用者が呼ぶ API は次の2つです。

```go
func Bind[T any](prefix string) *T

func Load(opts LoadOptions) (*LoadResult, error)
```

`Bind` に必要な型登録と設定適用処理は生成 file の `init` で準備されます。

## 対応する field 型

実用上の v1 対応型は次のとおりです。

- `string`
- `bool`
- `int`
- `time.Duration`
- `[]string`
- 上記を持つ named nested struct
- 同一 package の named struct を要素とする `[]T`（array of tables から読み込み）

float、map、その他の slice、pointer などは直接 bind できません。必要な場合は対応型で受け、`Load` 後にアプリケーション側で変換してください。

### duration

`time.Duration` の field は、どの入力元でも Go の duration 文法のみを受け付けます。

```go
type ServerConfig struct {
	ReadTimeout time.Duration `default:"5s" help:"request read timeout"`
}
```

```toml
[webserver]
read_timeout = "1h30m"
```

裸の数値は拒否します。`5` では秒なのか nanosecond なのか判断できないためです。これは `default` tag も同じで、parse できない値は `Load` ではなく `go generate` で失敗します。雛形は duration を quote された文字列として出力し、`default` のない field は `"0s"` から始まります。

この扱いを受けるのは `time.Duration` そのものだけです。underlying type が `time.Duration` の独自 named type は整数として bind されます。

array of tables の要素内でも duration は使えます。`default` は要素ごとに適用されます。

```toml
[[webserver.routes]]
path = "/static"
max_age = "15m"

[[webserver.routes]]
path = "/assets"   # max_age は default になる
```

## よくある問題

### `type not registered; run go generate`

`configbind.Bind[Type]` を追加・変更した後に生成していない場合に発生します。

```bash
go generate ./...
```

それでも発生する場合は、呼び出しが解析対象 package にあり、prefix が文字列 literal で、生成された `configbind_gen.go` が build 対象に入っているか確認します。

### 環境変数が反映されない

環境変数名は設定 key ではなく、CLI long option から決まります。`opt:"port,p"` なら `PORT` です。既定名を確認するには、prefix、nested key、snake case を組み合わせてください。

### `--config-path` を指定したら起動できない

明示 path は排他的で、user / system config directory へ fallback しません。その path が存在するか、読めるか、そして directory ではなく file を指しているかを確認してください。

### test ごとに target が増える

`Bind` target は process 内に登録されます。package 内 test で複数回登録する場合は、test 専用の `configbind.ResetTargets()` で状態を初期化できます。通常のアプリケーションコードでは起動時に一度だけ `Bind` / `Load` してください。
