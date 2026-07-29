# configbind User Guide

`configbind` loads application configuration into Go structs. Define the struct once, then overlay defaults, TOML, environment variables, and CLI options onto the same fields.

Nothing about that layering is configurable. The precedence is fixed, and sources farther to the right win:

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

Application code never implements any of the generated internals. It obtains a pointer with `Bind` and calls `Load` once during startup.

## What you provide

1. A Go struct representing configuration
2. A `configbind.Bind[T]("prefix")` call with a literal prefix
3. A startup call to `configbind.Load`
4. Optional TOML files, environment variables, and CLI options
5. A code-generation command

## Setup and generation

```go
package main

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
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

When config targets are present, the default output is `configbind_gen.go`. Generation reads the type argument and prefix statically, which is why the prefix has to be a string literal rather than a computed value.

## Generating configuration scaffolds

Each generated package registers one definition for each `Bind` call. Public `configbind` functions combine their scaffold fields across the framework and every imported application package:

```go
func ScaffoldTOML() (string, error)
func ScaffoldEnv() (string, error)
func WriteScaffoldTOML(w io.Writer) error
func WriteScaffoldEnv(w io.Writer) error
```

The TOML output uses the supported restricted subset. Both formats use `default` values when present, type-appropriate zero values otherwise, and comments from `help` tags. The environment scaffold also respects `opt`, `env:"NAME"`, and `env:"-"`.

Within a `[prefix]` table the keys follow the declaration order of the struct. The tables themselves are ordered by prefix and type name, so scaffold output never depends on package initialization order. The environment scaffold stays sorted by variable name, since it has no table grouping to hang declaration order on.

For example, this definition:

```go
// ServerConfig configures the public listener.
type ServerConfig struct {
	Port     int    `default:"8080" opt:"port,p" help:"HTTP listen port"`
	Host     string `default:"localhost" help:"listen host"`
	Internal string `env:"-"`
}

func serverConfig() *ServerConfig {
	return configbind.Bind[ServerConfig]("server")
}
```

contributes text equivalent to the following in the combined output:

```toml
# ServerConfig configures the public listener.
[server]
# HTTP listen port
port = 8080
# listen host
host = "localhost"
internal = ""
```

The struct's godoc becomes the table comment. The `.env` scaffold is sorted globally by variable name, so it carries field comments only.

```dotenv
# HTTP listen port
PORT=8080
# listen host
SERVER_HOST="localhost"
```

Generation may run separately in a server framework package and in each modular-monolith package. Importing those generated packages registers all definitions; the final application does not need to rescan their source. Output order is deterministic, and duplicate keys or environment names return an error.

The generator does not create files at runtime or add a scaffold subcommand. Add the command shape that fits your application and call the public output functions:

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

Redirect it when you want a file:

```bash
./myserver scaffold-config toml > config.toml
./myserver scaffold-config env > .env
```

One gap is easy to miss. `configbind.Load` reads process environment variables and does not parse `.env` files at all, so a scaffolded `.env` needs your preferred dotenv loader or shell mechanism to reach the process before `Load` runs.

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
| `falsy:"value"` | The choice that means "off" for a string option | `falsy:"off"` |
| `dependon:"key"` | Hide this field from provenance while that key is empty | `dependon:"webserver.tls.enabled"` |

### Godoc as the help source

A field without a `help` tag takes its description from its godoc comment, and the generator writes that text back into the struct tag:

```go
type ServerConfig struct {
	// Port is the HTTP listen port.
	Port int `default:"8080"`
}
```

After one generator run the source reads:

```go
type ServerConfig struct {
	// Port is the HTTP listen port.
	Port int `default:"8080" help:"Port is the HTTP listen port"`
}
```

The tag is the single source of truth from then on: an existing `help` tag always wins over the comment, and re-running the generator changes nothing. Only the first paragraph is used, `//go:` and lint directives are dropped, and one trailing period is removed. A trailing line comment (`Host string // listen address`) works too.

The same text feeds generated CLI usage. A `SubCommand` registered with an empty help string falls back to its struct godoc.

To keep the generator from editing hand-written sources, disable the feature — godoc still seeds help in the generated output:

```go
options := generator.DefaultOptions()
options.DisableFeatures = append(options.DisableFeatures, generator.FeatureHelpBackfill)
```

Tags combine, and the combination is what fixes a field's name on every surface at once:

```go
type ServerConfig struct {
	Port int `key:"listen_port" default:"8080" opt:"port,p" help:"HTTP listen port"`
}
```

For the `server` prefix, this one field appears under four names:

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

Go field names become snake-case keys. In CLI names, nested dots turn into hyphens; environment names go further still, replacing both hyphens and dots with underscores and uppercasing the result.

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

Quoted keys, inline tables, arrays of tables, and nested arrays are not supported. There are really two limits here rather than one — what the parser accepts, and what a struct field can receive — and the second is the narrower of the two. A TOML float parses, yet it cannot be bound directly to a float field.

## Configuration file discovery

```go
result, err := configbind.Load(configbind.LoadOptions{
	Vendor:   "acme",
	Tool:     "myserver",
	FileName: "settings.toml",
})
```

`FileName` defaults to `config.toml`. configbind selects the first readable file
in this order and reads only that file:

1. `ExplicitConfigPath`, or `--config-path` when the field is empty
2. `ExtraConfigReadPaths`, in slice order
3. the OS user configuration directory under `Vendor` / `Tool`
4. the OS system configuration directory under `Vendor` / `Tool`

Files are never merged. This allows a local test configuration to replace,
rather than combine with, a production system configuration. Missing or
unreadable entries in `ExtraConfigReadPaths` are skipped. When no candidate is
found, defaults, environment, and CLI values still load.

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

Use `ExtraConfigReadPaths` for optional local or deployment-specific files:

```go
result, err := configbind.Load(configbind.LoadOptions{
	Vendor:               "acme",
	Tool:                 "myserver",
	ExtraConfigReadPaths: []string{"./config.test.toml", "/run/secrets/app.toml"},
})
```

If `./config.test.toml` exists, it is the only TOML file read. Otherwise
`/run/secrets/app.toml` is tried, followed by user and system config.

### `LoadOptions` reference

| Field | Meaning | Default |
| --- | --- | --- |
| `Vendor` | Vendor name below OS configuration directories | Required when resolution reaches configdir |
| `Tool` | Application or tool name | Required when resolution reaches configdir |
| `FileName` | TOML basename to discover | `config.toml` |
| `Args` | CLI arguments without the program name | `os.Args[1:]` when nil |
| `Environ` | Environment as `KEY=value` entries | `os.Environ()` when nil |
| `ExplicitConfigPath` | File path that must be used | Empty uses `--config-path`, extras, or directory discovery |
| `ExtraConfigReadPaths` | Optional file paths searched in slice order | Missing entries are skipped |

The distinction between nil and empty matters in tests, because nil means "fall back to the process." Pass an empty slice to shut CLI or environment input off entirely:

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

## CLI subcommands

`SubCommand[T]` declares a generated, CLI-only command branch. Its fields never
read TOML or environment values. Fields without `arg` are options; positional
fields use `arg:"required"`, `arg:"optional"`, or `arg:"*"`.

```go
type MigrateOptions struct {
	Path   string   `arg:"required" help:"migration directory"`
	Label  string   `arg:"optional" help:"migration label"`
	DryRun bool     `default:"false" help:"print changes without applying"`
	Extra  []string `arg:"*" help:"additional migration inputs"`
}

server := configbind.Bind[ServerConfig]("server")
migrate := configbind.SubCommand[MigrateOptions]("migrate", "run database migrations")

if _, err := configbind.Load(configbind.LoadOptions{
	Vendor: "acme",
	Tool:   "myserver",
}); err != nil {
	// *configbind.UsageError includes generated top-level or command usage.
	log.Fatal(err)
}
if migrate != nil {
	runMigrations(*migrate)
	return
}
runServer(*server)
```

After running `go generate`, these forms select and fill `MigrateOptions`:

```bash
./myserver migrate ./migrations
./myserver migrate ./migrations --dry_run release extra-a extra-b
```

Only the selected `SubCommand` call returns non-nil. A missing required
argument, an unknown command or option, or `--help` returns
`*configbind.UsageError` carrying generated usage text, and options may appear
before or after positional arguments.

Selection and parsing read the same argument list, which is worth remembering in
tests. Leave `LoadOptions.Args` nil in production so both use `os.Args[1:]`; a
test that overrides `Args` must set the matching `os.Args` before calling
`SubCommand`.

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

TOML is the asymmetric case. An unknown key parses, matches no struct field, and is silently not applied — so a misspelled key in a config file fails quietly where a misspelled CLI option fails loudly. Applications that must reject configuration typos can compare `LoadResult.Overlay.Keys()` against their expected keys during startup.

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

`LoadResult.Provenance()` returns the effective configuration ready to log:

```go
result, err := configbind.Load(options)
if err != nil {
	return err
}

for _, entry := range result.Provenance() {
	log.Printf("%s = %s (%s)", entry.Key, entry.Value, entry.Place)
}
```

The slice is ordered rather than sorted: bindings appear in `Bind` call order, and the keys of one binding in the declaration order of its struct, nested structs expanded where they are declared. Keys that belong to no registered binding — a stray entry in someone's TOML file — trail the known ones in alphabetical order.

Two filters run before you see the slice. A key whose path contains `password`, `secret`, `token`, `apikey`, `api_key`, `credential`, or `access_key` reports `*****` instead of its value. And a field with a `dependon` tag disappears while its parent is empty, which the next section covers.

### Hiding settings of a disabled feature

An unconfigured subsystem otherwise prints its whole block of defaults, burying the settings actually in use. `dependon` names the parent that decides whether this field matters:

```go
type WebServerConfig struct {
	Tracing    string `enum:"off,otlp,jaeger" falsy:"off" help:"tracing exporter"`
	TracingURL string `dependon:"webserver.tracing" help:"collector URL"`
}
```

The parent is a full config key including its prefix, so a field can depend on one bound by another package. While `webserver.tracing` reads as empty, `webserver.tracing_url` is absent from the provenance slice; `webserver.tracing` itself still appears, since an empty parent is the reason its dependents vanished. A hidden parent hides its own dependents in turn.

"Empty" means the empty string or `false` — an `int` of 0, an empty list, and a zero duration are deliberate settings, not absent ones. An enum-style option needs a third form, which is what `falsy` supplies: it names the choice that means "off". That choice then counts as empty for anything depending on the field, and it also fills the field in when nothing sets it:

- No `default` tag and no source sets the key: the field resolves to `off`.
- A source sets the key to `""`: it resolves to `off`, keeping that source as its `Place`.
- A `default` tag is present: the default wins and `falsy` never substitutes.

None of this reaches the bound struct. `TracingURL` is still populated from its sources, CLI flags and help are unchanged, and scaffolds still list every field so the options stay discoverable before a first load.

### The raw overlay

`LoadResult.Overlay` holds the merged values and each winning source, unfiltered:

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

`Overlay.All()` iterates every entry in sorted key order when you want the whole table.

`LoadResult.ConfigPath` is the selected file path, and `FoundFile` reports whether a TOML file was found at all. Nothing in the overlay is masked, so logging raw overlay values wholesale will log your credentials with them — use `Provenance()` for anything that reaches a log.

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
- `time.Duration`
- `[]string`
- Named nested structs containing those types

Floats, maps, arbitrary slices, and pointers cannot be bound directly. Receive them in a supported representation and convert after `Load`.

### Durations

A `time.Duration` field accepts the Go duration syntax and nothing else, in every source:

```go
type ServerConfig struct {
	ReadTimeout time.Duration `default:"5s" help:"request read timeout"`
}
```

```toml
[webserver]
read_timeout = "1h30m"
```

A bare number is rejected, because `5` cannot say whether it means seconds or nanoseconds. That applies to the `default` tag as well, where an unparsable value fails `go generate` rather than `Load`. Scaffolds emit durations as quoted strings, and a `default`-less field starts at `"0s"`.

Only `time.Duration` itself is treated this way. A named type of your own whose underlying type is `time.Duration` binds as an integer.

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
