# configbind User Guide

`configbind` loads application configuration into Go structs. Define the struct once, then overlay defaults, TOML, environment variables, and CLI options onto the same fields.

The precedence is fixed; sources farther to the right win:

```text
default < TOML file < environment variable < CLI option
```

> [!IMPORTANT]
> configbind implements a configuration-focused TOML subset, not the complete TOML specification. Quoted keys, inline tables, arrays of tables, nested arrays, and some other TOML syntax are unsupported. Prepare configuration files for this supported subset rather than assuming that an arbitrary existing TOML document can be loaded. See [TOML files](#toml-files) for the complete list used by configbind.

## What is automated

- Discovering configuration structs used by `configbind.Bind[T]`
- Deriving TOML keys, CLI options, and environment names from struct fields
- Applying `default`, `key`, `opt`, `env`, and `help` tags
- Mapping nested structs and `[]string`
- Merging defaults, TOML, environment, and CLI values
- Converting values to string, bool, int, and `[]string`
- Recording the winning source for every merged setting

Application code does not implement generated internals. It obtains a pointer with `Bind` and calls `Load` once during startup.

## What you provide

1. A Go struct representing configuration
2. A `configbind.Bind[T]("prefix")` call with a literal prefix
3. A startup call to `configbind.Load`
4. Optional TOML files, environment variables, and CLI options
5. A code-generation command

## Setup and generation

```go
package main

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

Put a concrete `Bind` call in the analyzed package:

```go
func registerConfig() *ServerConfig {
	return configbind.Bind[ServerConfig]("server")
}
```

```bash
go generate ./...
```

When config targets are present, the default output is `configbind_gen.go`. The type argument and prefix must be statically discoverable, so use a string literal for the prefix.

## Minimal example

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

With no external values, this uses `localhost:8080`.

```bash
# Environment variables
SERVER_HOST=0.0.0.0 SERVER_PORT=9000 ./myserver

# CLI wins over the environment
SERVER_PORT=9000 ./myserver --server-port 10000
```

## Struct tags

| Tag | Purpose | Example |
| --- | --- | --- |
| `default:"value"` | Value used when no source supplies the setting | `default:"8080"` |
| `key:"name"` | Override the field's TOML and stable key name | `key:"listen_port"` |
| `opt:"long"` | Override the CLI long option | `opt:"port"` |
| `opt:"long,p"` | Set a long option and one-character short option | `opt:"port,p"` |
| `env:"NAME"` | Override the environment variable with an exact name | `env:"OTEL_SERVICE_NAME"` |
| `env:"-"` | Disable environment input for this field | `env:"-"` |
| `help:"text"` | Option-description metadata | `help:"HTTP listen port"` |

```go
type ServerConfig struct {
	Port int `key:"listen_port" default:"8080" opt:"port,p" help:"HTTP listen port"`
}
```

For the `server` prefix, this field has these names:

| Surface | Name |
| --- | --- |
| Stable configuration key | `server.listen_port` |
| TOML | `[server] listen_port = 8080` |
| CLI | `--port 8080` or `-p 8080` |
| Environment | `PORT=8080` |

When `opt` is present, the default `--server-listen_port` option is not registered. The environment name is also derived from the overridden long option.

## Naming rules

For the `webserver` prefix:

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

| Field | Stable key | CLI option | Environment |
| --- | --- | --- | --- |
| `Port` | `webserver.port` | `--webserver-port` | `WEBSERVER_PORT` |
| `Host` | `webserver.host` | `--webserver-host` | `WEBSERVER_HOST` |
| `TLS.Enabled` | `webserver.tls.enabled` | `--webserver-tls-enabled` | `WEBSERVER_TLS_ENABLED` |
| `TLS.CertPath` | `webserver.tls.cert_path` | `--webserver-tls-cert_path` | `WEBSERVER_TLS_CERT_PATH` |

Go field names become snake-case keys. Nested dots become hyphens in CLI names. Environment names replace hyphens and dots with underscores and uppercase the result.

The prefix itself may contain dots. Prefix and field hierarchy retain dots in stable keys and TOML, while every dot is normalized to a hyphen for CLI options.

```go
cache := configbind.Bind[CacheConfig]("middleware.cache")
```

For a `MaxEntries` field, the names are:

| Surface | Name |
| --- | --- |
| Stable key | `middleware.cache.max_entries` |
| TOML table | `[middleware.cache]` |
| CLI | `--middleware-cache-max_entries` |
| Environment | `MIDDLEWARE_CACHE_MAX_ENTRIES` |

## TOML files

```toml
[webserver]
port = 8080
host = "127.0.0.1"
cors_origins = ["https://app.example.com", "https://admin.example.com"]
tls.enabled = true
tls.cert_path = "/etc/myserver/server.crt"
```

Nested tables are also supported:

```toml
[webserver.tls]
enabled = true
cert_path = "/etc/myserver/server.crt"
```

configbind intentionally reads a restricted TOML subset:

- Tables, nested tables, and bare dotted keys
- String, bool, integer, and float scalars
- Arrays of primitive scalars
- Comments

Quoted keys, inline tables, arrays of tables, and nested arrays are not supported. Bindable struct types are more restricted than parsed TOML values; for example, a TOML float cannot be bound directly to a float field.

## Configuration file discovery

```go
result, err := configbind.Load(configbind.LoadOptions{
	Vendor:   "acme",
	Tool:     "myserver",
	FileName: "settings.toml",
})
```

`FileName` defaults to `config.toml`. Without an explicit path, configbind searches the OS user configuration directory first, then the system configuration directory, under the supplied `Vendor` and `Tool`. A missing discovered file is not an error; defaults, environment, and CLI values still load.

Use `--config-path` to select a file explicitly at runtime:

```bash
./myserver --config-path ./local.toml
```

If that file is missing, unreadable, or a directory, loading fails and does not fall back to normal configuration directories.

Tests and embedded callers may use `ExplicitConfigPath`:

```go
result, err := configbind.Load(configbind.LoadOptions{
	ExplicitConfigPath: "/tmp/test-config.toml",
	Args:               []string{},
	Environ:            []string{},
})
```

`ExplicitConfigPath` wins over `--config-path`. Production applications should normally accept `--config-path` through `Args`.

### `LoadOptions` reference

| Field | Meaning | Default |
| --- | --- | --- |
| `Vendor` | Vendor name below OS configuration directories | Required when no explicit path is used |
| `Tool` | Application or tool name | Required when no explicit path is used |
| `FileName` | TOML basename to discover | `config.toml` |
| `Args` | CLI arguments without the program name | `os.Args[1:]` when nil |
| `Environ` | Environment as `KEY=value` entries | `os.Environ()` when nil |
| `ExplicitConfigPath` | File path that must be used | Empty uses `--config-path` or directory discovery |

Pass an empty slice rather than nil to disable CLI or environment input in tests:

```go
Args:    []string{},
Environ: []string{},
```

## Environment variables

An environment name is derived from the first CLI long option:

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

The port variable is `PORT`, not `SERVER_PORT`, because `opt:"port,p"` changes the long option to `port`.

### Overriding an environment name

Use the `env` tag to follow an external standard or an established deployment convention. It changes only the environment name; the TOML key and CLI option remain unchanged.

```go
type ObservabilityConfig struct {
	ServiceName string `env:"OTEL_SERVICE_NAME"`
	Endpoint    string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

observability := configbind.Bind[ObservabilityConfig]("observability")
```

`ServiceName` then has these names:

| Surface | Name |
| --- | --- |
| TOML | `[observability] service_name = "checkout"` |
| CLI | `--observability-service_name checkout` |
| Environment | `OTEL_SERVICE_NAME=checkout` |

The `env` value is used exactly as written and must begin with a letter or `_`. Assigning the same environment name to multiple fields is a generation error. Use `env:"-"` for a field that must not accept environment input.

## CLI options

Scalar options accept separate and `=` forms:

```bash
./myserver --server-port 8080
./myserver --server-port=8080
```

A bool option without a value means true; false can be explicit:

```bash
./myserver --webserver-tls-enabled
./myserver --webserver-tls-enabled=false
```

Repeat a `[]string` option to accumulate values:

```bash
./myserver \
  --webserver-cors_origins https://app.example.com \
  --webserver-cors_origins https://admin.example.com
```

Unknown options, missing values, and invalid booleans cause `Load` to return an error.

Unlike an unknown CLI option, an unknown TOML key is accepted by the parser but has no matching struct field and is not applied. Applications that must reject configuration typos can compare `LoadResult.Overlay.Keys()` with their expected keys during startup.

## Nested settings and `[]string`

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

Here `CertPath` comes from the environment, `CorsOrigins` from CLI, `Enabled` from TOML, and `Host` from its default.

## Multiple configuration structs

Register multiple `Bind` targets and apply all of them with one `Load`:

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

Call every `Bind` before `Load`. The returned pointers contain their final values after `Load` succeeds.

## Inspecting provenance

`LoadResult.Overlay` contains the merged values and each winning source:

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

`Place` is one of:

- `configbind.PlaceDefault`
- `configbind.PlaceFile`
- `configbind.PlaceEnv`
- `configbind.PlaceCLI`

`LoadResult.ConfigPath` is the selected file path, and `FoundFile` reports whether a TOML file was found. There is no automatic secret masking, so do not log all raw overlay values.

## Public APIs

configbind does not generate a new public function for each type. Application code calls these two APIs:

```go
func Bind[T any](prefix string) *T

func Load(opts LoadOptions) (*LoadResult, error)
```

The generated file registers the type and its apply logic from `init`.

## Supported field types

The practical v1 field types are:

- `string`
- `bool`
- `int`
- `[]string`
- Named nested structs containing those types

Floats, maps, arbitrary slices, pointers, and `time.Duration` cannot be bound directly. Receive them in a supported representation and convert after `Load`:

```go
type RawConfig struct {
	ReadTimeout string `default:"5s"`
}

timeout, err := time.ParseDuration(cfg.ReadTimeout)
```

## Troubleshooting

### `type not registered; run go generate`

This occurs when generation has not run after adding or changing `configbind.Bind[Type]`:

```bash
go generate ./...
```

If it persists, verify that the call is in the analyzed package, the prefix is a string literal, and generated `configbind_gen.go` is included in the build.

### An environment variable is ignored

Environment names come from CLI long options, not directly from stable keys. `opt:"port,p"` produces `PORT`. For default names, combine the prefix, nested key, and snake-case field name using the naming table above.

### The application fails with `--config-path`

An explicit path is exclusive and does not fall back to user or system configuration directories. Check that the path exists, is readable, and points to a file.

### Bind targets accumulate across tests

Bind targets are registered in process state. Tests that register them repeatedly can call the test-only `configbind.ResetTargets()` first. Ordinary applications should call `Bind` and `Load` once during startup.
