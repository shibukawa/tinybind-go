# tinybind-go

[日本語](README.ja.md)

Reflection-free, code-generation-first binding for TinyGo and standard Go. Runtime dependencies are isolated into HTTP, JSON, SQL, and DynamoDB packages.

User guides: [httpbind](docs/httpbind.md) · [jsonbind](docs/jsonbind.md) · [configbind](docs/configbind.md) · [htmlbind](docs/htmlbind.md) · [sqlbind](docs/sqlbind.md) · [dynamobind](docs/dynamobind.md) · [firestorebind](docs/firestorebind.md) · [cachekeybind](docs/cachekeybind.md) · [reloadable components](docs/httpbind_reloadable_componet.md) · [fasthttp backend](docs/httpbind_fasthttp.md)

Building a framework on top of this? Start with [framework facilities](docs/httpbind_framework_facilities.md), the index of what is available to you and what is not, then [htmlbind for framework owners](docs/htmlbind_frameworkowner.md) and, if your users will build against fasthttp, [the fasthttp backend for framework owners](docs/httpbind_fasthttp_frameworkowner.md). Owning a browser runtime as well? [Client behaviour](docs/httpbind_client_behavior.md) covers server actions, client handlers, and component parameters in one place.

Define request/response structs once. The generator emits type-specific binders and writers, so the same model covers **JSON, form, multipart, and query** (plus path / header / cookie via tags). Responses adapt to the client **`Accept`** (and streaming negotiation where used). From the same analysis it also **generates OpenAPI 3.1** (JSON), kept in sync with binders and writers, with **godoc comments carried into `summary` / `description`**. Route registration is discovered by **static analysis of real `net/http` styles** (`HandleFunc`, `Handle`, method values, wrappers, and so on)—not by a separate DSL.

```go
type CreateUserRequest struct {
	// input = query + payload (JSON / form / multipart). Tag may be omitted.
	Name  string `input:"name"`  // same as untagged: Name string
	Email string `input:"email"` // same as untagged: Email string
	OrgID string `path:"org_id"`
	Token string `header:"Authorization"`
}

type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	OrgID string `json:"org_id"`
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	// Name/Email: query and/or JSON/form/multipart body (input).
	// OrgID from path, Token from Authorization header.
	out := CreateUserResponse{
		ID:    "u_1",
		Name:  input.Name,
		Email: input.Email,
		OrgID: input.OrgID,
	}
	_ = httpbind.Write[CreateUserResponse](w, r, out)
}
```

Run the generator on the package (binders + OpenAPI embed):

```bash
go run ./cmd/tinybind-gen generate -dir . -openapi
```

The same generation pass also supports CLI-only application subcommands through
`configbind.SubCommand[T]`, including required, optional, and rest positional
arguments. See the [configbind subcommand guide](docs/configbind.md#cli-subcommands).

### Struct tag reference

Wire name defaults to the lower-camel field name when a tag value is omitted (e.g. untagged `Name` → `"name"`).

| Tag | Source | Notes |
|-----|--------|--------|
| *(none)* or `input:"name"` | **query + payload** | Default. Payload covers JSON, `application/x-www-form-urlencoded`, and `multipart/form-data`. Tag is optional when the field is plain user input. |
| `query:"page"` | query only | Not read from the body. |
| `payload:"name"` | body only | JSON / form / multipart by `Content-Type`. Not read from the query string. |
| `payload:"image"` on `httpbind.File` | multipart file part | Binds filename, content type, size, and bytes from the named part. Payload-only (not query). Multipart bodies are capped at **1 MiB** by default; override with `httpbind.SetMaxMultipartBodyBytes`. |
| `path:"org_id"` | path parameter | Matches `{org_id}` (or equivalent) in the route pattern. |
| `header:"Authorization"` | request header | Header name is the tag value. |
| `cookie:"session"` | cookie | Cookie name is the tag value. |

**`input` vs `payload` vs `query`**

- Prefer **`input`** (or no tag) for normal fields that may arrive as query *or* body.
- Use **`query`** / **`payload`** only when you must restrict the origin (e.g. search filters in the query string, body-only JSON fields).
- `payload` is not the same as `input`: it does **not** accept query parameters.

Example that mixes restrictions:

```go
type SearchRequest struct {
	Keyword string `query:"keyword"`   // query only
	Page    int    `query:"page"`
	Filter  string `payload:"filter"`  // body only (JSON/form/multipart)
}
```

Response structs commonly use standard `json:"..."` names for encoding; request binding still uses the source tags above.

### Streaming (ideal API)

```go
httpbind.WriteStream(w, r, func(s *httpbind.Stream[ChatEvent]) error {
    if err := s.Write(ChatEvent{Type: "delta", Delta: "hi"}); err != nil {
        return err
    }
    return s.Write(ChatEvent{Type: "done"})
})
```

- **`Write` can be called many times** (incremental events).
- Format is chosen once when the stream opens, from `?stream=`, `Accept`, `User-Agent`, then default **NDJSON**.
- The entry closes the stream, so the trailing `]` of the JSON array framing is written even when the callback fails halfway.
- Formats:
  - **SSE** — `text/event-stream`
  - **NDJSON / JSONL** — `application/x-ndjson` (one object per line; *not* a JSON array)
  - **JSON array** — `application/json` as `[obj1,obj2,...]` (`Close` writes the trailing `]`)
- Do **not** use removed helpers `WriteNDJSON` / `WriteSSE`.

## Packages

| Path | Role |
|------|------|
| `.` (`package httpbind`) | Runtime: Bind / Write / WriteError / WriteStream / OpenAPI serve / SwaggerUI |
| `jsonbind/` | Standalone DecodeJSON / EncodeJSON runtime; does not import `net/http` or `database/sql` |
| `sqlbind/` | ScanRows runtime and row helpers; does not import `net/http` |
| `dynamobind/` | DynamoDB item runtime over `tinygodriver/nosql/dynamodb`; does not import `net/http` or `database/sql` |
| `firestorebind/` | Firestore Datastore-mode entity runtime over `tinygodriver/nosql/datastore`; does not import `net/http` or `database/sql` |
| `cachekeybind/` | Cache key framing runtime; stdlib only |
| `generator/` | Field-plan binders/writers + OpenAPI 3.1 + template generation |
| `parser/` | Route/handler discovery (`Bind`, `Write`, `WriteStream`, errors) |
| `templates/htmlbind/` | Typed, context-safe HTML template compiler |
| `templates/sqlbind/` | Typed, parameterized SQL template compiler |
| `templates/firestorebind/` | Typed Firestore access-pattern declarations (`.tb.firestore`) |
| `cmd/tinybind-gen` | CLI: binders + OpenAPI + templates from a package dir |
| `examples/demo` | End-to-end sample app |
| `internal/*` | Test fixtures |
| `testdata/cmd/*` | Dev-only helpers (not for distribution; under `testdata` so `go get` / `./...` skip them) |

```bash
go run ./cmd/tinybind-gen generate -dir ./path/to/package
```

Every generated file records a `// tinybind:generated` comment holding the
SHA-256 of the inputs that produced it, so a run whose package sources,
templates, `go.mod`, options, and generator binary all hash to the recorded
value exits without regenerating. `-force` regenerates regardless. See
[docs/httpbind.md](docs/httpbind.md#skipping-unchanged-packages).

The CLI automatically discovers `.tb.html` and `.tb.sql` files in the target
package and writes `tinybind_templates_gen.go`. A package containing SQL
templates must name its database with `-sql-dialect postgresql`, `mysql`, or
`sqlite`; there is no default. SQL value expressions become driver arguments,
and placeholders are generated in encounter order in the style that dialect
requires — `$1`, `$2`, … for PostgreSQL and `?` for MySQL and SQLite:

```text
package store

type User { id: int, name: string }

export statement FindUser(id: int): sql.optional<User> {
SELECT id, name FROM users WHERE id = {id}
}
```

This generates both `BuildFindUser(id) (Statement, error)` and the
`FindUser(ctx, db, id) (*User, error)` convenience API. The SQL compiler also
supports `sql.exec`, `sql.one<T>`, `sql.many<T>`, private `sql.predicate`
composition, private `sql.relation<T>` subqueries, conditional clauses, and
array value-list expansion. Hand-authored placeholders and unguarded
`UPDATE`/`DELETE` statements are rejected.

`sql.many<T>` streams rows as `iter.Seq2[T, error]` without first allocating a
result slice. Query, scan, and iteration errors are yielded as the error value,
and stopping the range early closes the underlying `sql.Rows`:

```go
for user, err := range FindUsers(ctx, db, filter) {
    if err != nil {
        return err
    }
    // consume user
}
```

Web frameworks can opt in to executor-from-Context wrappers while the explicit
`db` APIs remain available:

```bash
go run ./cmd/tinybind-gen generate -dir ./path/to/package -sql-context-api
```

The generated `FindUsersContext(ctx, filter)` resolves the `*sql.DB`,
`*sql.Conn`, or `*sql.Tx` installed with `sqlbind.WithSQLExecutor`. This allows
transaction middleware to keep the executor inside its callback Context:

```go
web.Transaction(func(ctx context.Context) error {
    for user, err := range FindUsersContext(ctx, filter) {
        if err != nil {
            return err
        }
        // consume user within the transaction
    }
    return nil
})
```

Custom generator commands may use a framework-owned Context key by setting a
resolver with the signature
`func(context.Context) (SQLExecutor, error)`; setting it also enables the
Context wrappers:

```go
options.SQLExecutorResolver = &generator.SymbolPattern{
    PackagePath: "example.com/web/dbctx",
    Name:        "Executor",
}
```

Frameworks may wrap the runtime functions and still make those calls visible to
the generator. Register the wrapper's package identity, semantic operation, and
the zero-based positions of only the type/value roles that the generator needs:

```go
package main

import "github.com/shibukawa/tinybind-go/generator"

func main() {
    calls := generator.NewCallRegistry()
    if err := calls.Register(
        // func RegisterConfig[T any](ctx context.Context, name string) *T
        generator.ConfigBindCall(
            generator.Function("example.com/framework", "RegisterConfig"),
            generator.GenericType("config", 0),
            generator.Argument("prefix", 1),
        ),
        // func Created(ctx context.Context, w http.ResponseWriter, value any) error
        generator.ResponseWriteStatusCall(
            generator.Function("example.com/framework", "Created"),
            generator.ArgumentType("response", 2),
            generator.Constant("status", 201),
        ),
    ); err != nil {
        panic(err)
    }
    options, err := calls.Options(generator.DefaultOptions())
    if err != nil {
        panic(err)
    }
    generator.Main(generator.MustCommandSet(generator.GenerateCommand(options)))
}
```

Extra wrapper arguments do not need to be described. Use `GenericType` when the
model comes from a generic type argument, `ArgumentType` when it comes from a
value argument's static type, `Argument` for a runtime value such as a config
prefix or route pattern, and `Constant` when the wrapper hides a fixed value
such as status 201. The available operations have matching constructors:
`RequestBindCall`, `ResponseWriteCall`, `ResponseWriteStatusCall`,
`StreamCreateCall`, `JSONDecodeCall`, `JSONEncodeCall`, `RowsScanCall`,
`ConfigBindCall`, `ConfigSubCommandCall`, `RouteRegisterCall`, and
`ErrorResponseCall`. `Function`
targets package functions; `Method` targets named-receiver methods.

The required role names, in the same order, are `request`; `response`;
`response` + `status`; `stream`; `decode`; `encode`; `row`; `config` +
`prefix`; `config` + `name` + `help`; `pattern` + `handler`; and `status`.

`RuntimePackages` remains a shorthand for functions with the standard tinybind
names and signatures. Use explicit call patterns for renamed wrappers, reordered
arguments, extra arguments, or hidden constants. `generator.Options{}`
deliberately has no discovery identities. Add a feature to `DisableFeatures` to
prevent discovery even under `-generate-all`.

A framework can combine the built-in `generate` command with its own lifecycle
commands:

```go
commands := generator.MustCommandSet(
    generator.GenerateCommand(options),
    generator.Command{Name: "init", Summary: "initialize a project", Run: runInit},
    generator.Command{Name: "build", Summary: "generate and build", Run: runBuild},
    generator.Command{Name: "watch", Summary: "watch, generate, and build", Run: runWatch},
)
generator.Main(commands)
```

Each command receives a `context.Context`, arguments, and injected `CommandIO`
containing stdin/stdout/stderr, working directory, and environment. A `build` or
`watch` implementation can generate in-process without invoking a CLI:

```go
result, err := generator.New(options).GeneratePackage(ctx, generator.GenerateRequest{
    Dir: dir, OpenAPI: true,
})
```

`GeneratePackage` runs template, mapping, configbind, and optional OpenAPI
generation and returns the written paths. `generator.Main` is only the outer
process boundary; tests and composed commands should call `CommandSet.Run` or
`GeneratePackage` directly.

Generation is usage-aware: a package that only calls `DecodeJSON[T]` gets only
its JSON decoder, imports `jsonbind`, and does not import the root HTTP runtime
or `net/http`. Set `Options.GenerateAll` for
the legacy all-enabled-mappings mode. Compatible multipart file aliases can be
listed in `Options.FileTypes.Set`.

Standalone JSON uses the dependency-isolated package:

```go
value, err := jsonbind.DecodeJSON[Document](reader)
err = jsonbind.EncodeJSON(writer, value)
```

JSON reads are capped at 1 MiB by default. Use
`jsonbind.SetMaxJSONBodyBytes` globally or `jsonbind.DecodeJSONLimit` per call.
`jsonbind` returns transport-neutral errors; `httpbind.Bind` maps an oversized
HTTP request to status 413.

Joined SQL rows can be grouped into an object tree with generated, reflection-free `ScanRows[T]` code:

```go
type Organization struct {
    ID    int    `db:"organization_id" groupkey:""`
    Name  string `db:"organization_name"`
    Users []User
}
type User struct {
    ID   int    `db:"user_id" groupkey:""`
    Name string `db:"user_name"`
}

organizations, err := sqlbind.ScanRows[Organization](rows)
```

Every grouped struct level has one `groupkey` field. Repeated keys merge into
the same object; a NULL child key represents an absent outer-join child.

## Formatting templates

`.tb.html`, `.tb.sql`, and `.tb.dynamo` are file formats this module invented, so
no editor knows how to format them. The generator ships the formatter:

```bash
go run ./cmd/tinybind-gen fmt -w -dir ./store
```

`-l` lists the files that would change and exits non-zero, which is the CI form.
`-as sql` (or `html`, `dynamo`) filters one source from stdin to stdout, which is
what an editor "format on save" hook needs.

What it does per format:

- **SQL** — one clause per line, CTE bodies and subqueries indented under their
  own `SELECT`, `JOIN` and its `ON` split, `AND`/`OR` aligned when a condition
  list is long. Keyword case, literals, and comments are left exactly as written.
- **HTML** — one tag per line inside `head`, `table`, and the other positions
  where the HTML parser discards whitespace anyway. Elsewhere a line break only
  replaces whitespace that was already there, so `<b>a</b><i>b</i>` stays glued
  and rendering never changes. `pre`, `textarea`, `script`, `style`, and any
  `preserve-whitespace` subtree are copied byte for byte.
- **DynamoDB** — `table` then `key`, one clause per line.

A source that does not parse is reported and left untouched. Everything the
command does is available as a library:

```go
import "github.com/shibukawa/tinybind-go/templates/templatefmt"

formatted, err := templatefmt.Source("users.tb.sql", source, templatefmt.Options{})
results, err := templatefmt.Dir("./store", templatefmt.Options{Width: 120})
```

`templatefmt.Dir` reads but never writes; each `Result` reports whether the file
would change and carries `Write()` for when you want it applied.

## Demo

```bash
go generate ./examples/demo
go run ./examples/demo
# http://localhost:8080/       index + browser stream demo
# http://localhost:8080/docs/  Swagger UI
# http://localhost:8080/chat   WriteStream (SSE / NDJSON / JSON array auto)
```

See [`examples/demo/README.md`](examples/demo/README.md) for full curl recipes.

## Benchmarks

Generated code has no reflection to drive and no intermediate `map[string]any`
to build, which is where the difference comes from. Measured on an Apple M3,
Go 1.26.5, `darwin/arm64`, best of 10 runs.

Each pair produces the same output: the JSON codecs are checked against
`encoding/json` by differential fuzzing, and the handler and template pairs are
asserted equivalent by the tests sitting beside the benchmarks. Reproduce with:

```bash
go test ./internal/benchfixture -run xxx -bench . -benchmem
```

### Throughput

The document is a 312-byte order with a nested object, a three-element array of
objects, and a string array. The page is a five-row user list.

| Path | Standard library | Generated |
|------|------------------|-----------|
| JSON decode (`io.Reader`) | 3447 ns · 1688 B · 30 allocs | **777 ns · 856 B · 15 allocs** |
| JSON decode (`json.Unmarshal`, bytes in hand) | 3287 ns · 888 B · 25 allocs | — |
| JSON encode | 579 ns · 144 B · 1 alloc | **272 ns · 0 B · 0 allocs** |
| `Bind` + `Write` (request reused) | 850 ns · 1584 B · 17 allocs | **584 ns · 1021 B · 16 allocs** |
| `Bind` + `Write` (incl. request construction) | 1695 ns · 7445 B · 31 allocs | **1422 ns · 6883 B · 30 allocs** |
| HTML render (`html/template` vs `htmlbind`) | 7346 ns · 2705 B · 107 allocs | **930 ns · 464 B · 4 allocs** |

The JSON comparisons are against `encoding/json`; the handler row against a
hand-written `net/http` handler doing the same decode, path, and header reads;
the HTML row against an `html/template` template rendering the same document.

Encoding allocates nothing: generated encoders append into a pooled buffer, so
a response costs no garbage at all. Decoding's 15 allocations are the 13 strings
and slices that end up in the result, plus the body buffer and its reader —
there is nothing left to remove without changing what the caller gets back.
The HTML render's four allocations are per render rather than per row — the
bound fragment, the options, the renderer, and its conversion buffer — so a
longer page costs the same four.

### Binary size

The same small JSON program built two ways, once over `encoding/json` and once
over generated `jsonbind` codecs. `jsonbind` does not import `encoding/json` at
all, so the reflection-based codec never enters the binary. Built with Go
1.26.5 and TinyGo 0.41.1; the native rows are `darwin/arm64`.

| Build | `encoding/json` | `jsonbind` | Saved |
|-------|-----------------|------------|-------|
| `go build` | 3,075,666 | **2,565,106** | −511 KB (−16.6%) |
| `go build -ldflags="-s -w"` | 2,061,138 | **1,708,034** | −353 KB (−17.1%) |
| `tinygo build` (native) | 474,256 | **293,632** | −181 KB (−38.1%) |
| `tinygo build` (native) + `strip` | 287,856 | **187,968** | −100 KB (−34.7%) |
| `tinygo build -target wasi` | 1,264,464 | **738,966** | −525 KB (−41.6%) |
| `tinygo build -target wasi -no-debug` | 488,762 | **222,564** | −266 KB (−54.5%) |

Stripping makes the gap matter more, not less: once debug information is gone,
the reflection machinery is a larger share of what is left. On a stripped TinyGo
wasm build it is about half the binary.

The wasm and native rows strip differently because the debug information lives
in different places. A wasm binary embeds its DWARF, which is what `-no-debug`
removes; a Mach-O binary never carries it (macOS keeps DWARF in a separate
dSYM), so `-no-debug` changes nothing there and `strip`, which drops the symbol
table, is the flag's native equivalent.

### encoding/json/v2

`encoding/json/v2` is still behind `GOEXPERIMENT=jsonv2` on Go 1.26, so a
library cannot import it unconditionally. It was measured anyway, because the
obvious question is whether generated codecs should target it instead.

```bash
GOEXPERIMENT=jsonv2 go test ./internal/benchfixture -run xxx -bench JSON -benchmem
```

| Path | v1, flag off | v1, flag on | v2 API | Generated |
|------|--------------|-------------|--------|-----------|
| decode (`io.Reader`) | 3543 ns · 1688 B · 30 | 2536 ns · 1889 B · 18 | 1650 ns · 544 B · 11 | **799 ns · 856 B · 15** |
| decode (bytes in hand) | 3352 ns · 888 B · 25 | 1871 ns · 496 B · 10 | 1525 ns · 496 B · 10 | — |
| encode | 587 ns · 144 B · 1 | 1330 ns · 1824 B · 11 | 943 ns · 288 B · 2 | **274 ns · 0 B · 0** |

Turning the flag on and changing nothing else is a real improvement for
decoding, because the v1 API is reimplemented over v2 — but watch the encode
row, which gets 2.3× slower and allocates 12× more. The flag is not free either
way.

Generating onto `jsontext`, the v2 tokenizer, was the interesting option: the
same key-switch shape driven by `ReadToken` lands on 13 allocations with a
reused decoder — exactly what `jsonbind.Parser` allocates — but takes 1320 ns to
do it, and 1804 ns · 1600 B · 38 allocs when the decoder is constructed per
call, as a codec entry point would have to.

Size settles it. On the same small program, the experiment costs:

| Build | Flag off | Flag on |
|-------|----------|---------|
| `go build` | 3,075,522 | 3,887,730 (+26%) |
| `go build -ldflags="-s -w"` | 2,061,010 | 2,598,722 (+26%) |
| `tinygo build -target wasi` | 1,345,144 | 2,217,774 (+65%) |
| `tinygo build -target wasi -no-debug` | 496,869 | 881,891 (+78%) |

A stripped wasm build with the experiment on is 3.5× the size of the same
program on `jsonbind`. For a library whose first-class target is TinyGo, that
rules v2 out as a dependency, and nothing in the speed columns argues for
carrying a second implementation behind a build tag to get it.

## TinyGo

TinyGo is a first-class target for generated binding code. The JSON runtime is
kept independent of `net/http` so it can be used on js/wasm toolchains where
TinyGo's standard-library HTTP path is unavailable.

Verified with **TinyGo 0.41.1 + Go 1.26.x**.

```bash
./scripts/tinygo-check.sh
```

### Runtime notes relevant to TinyGo

- `AsHTTPError` avoids `errors.As` (unimplemented `AssignableTo` on some TinyGo builds).
- `WriteError` hand-builds problem JSON (avoids fragile nested `encoding/json` + RawMessage interactions).
- Registry uses `reflect.Type` only as a **type identity key**, not for field walking.
- Generated bind/write code does not import `reflect`.
- `jsonbind` parses and writes JSON itself and does not import `encoding/json`, so a JSON-only binary carries no reflection-based codec — around 40% of a `tinygo build -target wasi` binary, and about half of a `-no-debug` one. See [Benchmarks](#binary-size).
- Do not build with `GOEXPERIMENT=jsonv2`. `encoding/json/v2` is still behind the experiment on Go 1.26, and on TinyGo it grows the same wasi binary by about 60% while `jsonbind` never calls it.

### Known limitations

| Topic | Limitation |
|-------|------------|
| Toolchain | Project baseline is TinyGo 0.41.1 + Go 1.26.x |
| js/wasm HTTP | TinyGo 0.41.1 + Go 1.26.x fails inside `net/http/roundtrip_js.go`; use `jsonbind` for HTTP-free WASM code |
| Streaming | Prefer host `go test` for `WriteStream`; not fully TinyGo-matrixed |
| ServeMux | `DefaultOptions` discovers both `net/http.ServeMux` and `tinygodriver/httpmux.ServeMux`; use `httpmux` for Go 1.22 method and wildcard routing under TinyGo |
| Multipart `File` | Supported via `httpbind.File` (`payload`); size/MIME `check` rules deferred. Body cap defaults to **1 MiB** (`SetMaxMultipartBodyBytes`) |
| SQL mapping | `ScanRows` and generated SQL scanners target host Go and are excluded from TinyGo builds |
| Generator | Host-side only (`go run` / `go test`) |

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
