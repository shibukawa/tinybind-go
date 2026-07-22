# configbind 利用ガイド

`configbind` は、アプリケーション設定を Go の構造体へ読み込むパッケージです。構造体を一度定義すると、default、TOML、環境変数、CLI option を同じ field へ重ね合わせます。

設定値の優先順位は常に次の順です。右側ほど優先されます。

```text
default < TOML file < environment variable < CLI option
```

> [!IMPORTANT]
> configbind の TOML parser は標準 TOML のフルセットではなく、設定用途に絞った subset です。quoted key、inline table、array of tables、nested array などは利用できません。既存の一般的な TOML file をそのまま読み込む用途ではなく、対応範囲に合わせて設定 file を用意してください。詳しくは「[TOML file](#toml-file)」を参照してください。

## 自動化されること

- 設定構造体と `configbind.Bind[T]` の利用箇所の発見
- 構造体 field から TOML key、CLI option、環境変数名の決定
- `default`、`key`、`opt`、`env`、`help` tag の反映
- nested struct と `[]string` の設定 mapping
- default → TOML → env → CLI の merge
- string、bool、int、`[]string` への型変換
- 各設定値が最終的にどの入力元から来たかの記録

生成コードの内部を利用者が実装する必要はありません。アプリケーションでは `Bind` で設定 pointer を取得し、起動時に一度 `Load` を呼びます。

## ユーザーが用意するもの

1. 設定を表す Go の構造体
2. literal prefix を指定した `configbind.Bind[T]("prefix")` 呼び出し
3. アプリケーション起動時の `configbind.Load`
4. 必要に応じた TOML file、環境変数、CLI option
5. コード生成の実行

## 導入とコード生成

```go
package main

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
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

configbind の対象がある場合、既定では `configbind_gen.go` が生成されます。`Bind` の type parameter と prefix は静的に発見できる必要があるため、prefix には文字列 literal を使ってください。

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

```go
type ServerConfig struct {
	Port int `key:"listen_port" default:"8080" opt:"port,p" help:"HTTP listen port"`
}
```

この field の名前は次のようになります。

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

Go field 名は snake case の key になります。CLI では nested key の `.` が `-` へ変わります。環境変数では `-` と `.` が `_` になり、全体が大文字になります。

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

configbind が読む TOML は意図的に限定された subset です。

- table、nested table、bare dotted key
- string、bool、integer、float の scalar
- primitive scalar の array
- comment

quoted key、inline table、array of tables、nested array は利用できません。設定構造体へ適用できる型はさらに限定されるため、float の TOML 値を float field へ直接 bind することはできません。

## 設定 file の探索

```go
result, err := configbind.Load(configbind.LoadOptions{
	Vendor:   "acme",
	Tool:     "myserver",
	FileName: "settings.toml",
})
```

`FileName` の既定は `config.toml` です。明示 path がない場合は、OS の user config directory、次に system config directory の `Vendor` / `Tool` 配下から1つを探します。file が見つからなくても error にはならず、default、env、CLI だけで load します。

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

### `LoadOptions` 一覧

| Field | 意味 | 既定 |
| --- | --- | --- |
| `Vendor` | OS config directory 内の vendor 名 | 明示 path を使わない場合は必須 |
| `Tool` | application / tool 名 | 明示 path を使わない場合は必須 |
| `FileName` | 探索する TOML basename | `config.toml` |
| `Args` | program 名を除いた CLI arguments | `nil` なら `os.Args[1:]` |
| `Environ` | `KEY=value` 形式の環境 | `nil` なら `os.Environ()` |
| `ExplicitConfigPath` | 強制的に使う file path | 空なら `--config-path` または directory 探索 |

test で CLI や環境を完全に無効にする場合は、`nil` ではなく空 slice を渡します。

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

TOML 内の未知の key は CLI の未知 option と異なり parse error にはならず、対応する struct field がないため適用されません。typo を厳密に拒否したい場合は、起動時に `LoadResult.Overlay.Keys()` と期待する key を検査してください。

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

`LoadResult.Overlay` には、merge 後の値と勝った入力元が入っています。

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

`LoadResult.ConfigPath` は選ばれた file path、`FoundFile` は TOML file が見つかったかを示します。secret を自動的に mask する機能はないため、overlay の raw value をまとめて log しないでください。

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
- `[]string`
- 上記を持つ named nested struct

float、map、任意の slice、pointer、`time.Duration` などは直接 bind できません。必要な場合は対応型で受け、`Load` 後にアプリケーション側で変換してください。

```go
type RawConfig struct {
	ReadTimeout string `default:"5s"`
}

timeout, err := time.ParseDuration(cfg.ReadTimeout)
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

明示 path は排他的です。存在しない場合に user/system config directory へ fallback しません。path、権限、file であることを確認してください。

### test ごとに target が増える

`Bind` target は process 内に登録されます。package 内 test で複数回登録する場合は、test 専用の `configbind.ResetTargets()` で状態を初期化できます。通常のアプリケーションコードでは起動時に一度だけ `Bind` / `Load` してください。
