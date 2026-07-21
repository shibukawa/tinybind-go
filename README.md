# tinybind-go

[日本語](README.ja.md)

Reflection-free, code-generation-first binding for TinyGo and standard Go. Runtime dependencies are isolated into HTTP, JSON, and SQL packages.

Define request/response structs once. The generator emits type-specific binders and writers, so the same model covers **JSON, form, multipart, and query** (plus path / header / cookie via tags). Responses adapt to the client **`Accept`** (and streaming negotiation where used). From the same analysis it also **generates OpenAPI 3.1**, kept in sync with binders and writers. Route registration is discovered by **static analysis of real `net/http` styles** (`HandleFunc`, `Handle`, method values, wrappers, and so on)—not by a separate DSL.

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
go run ./cmd/tinybind-gen -dir . -openapi
```

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
stream, err := httpbind.NewStream[ChatEvent](w, r)
if err != nil {
    httpbind.WriteError(w, r, err)
    return
}
defer stream.Close()

_ = stream.Write(ChatEvent{Type: "delta", Delta: "hi"})
_ = stream.Write(ChatEvent{Type: "done"})
```

- **`Write` can be called many times** (incremental events).
- Format is chosen once in `NewStream` from `?stream=`, `Accept`, `User-Agent`, then default **NDJSON**.
- Formats:
  - **SSE** — `text/event-stream`
  - **NDJSON / JSONL** — `application/x-ndjson` (one object per line; *not* a JSON array)
  - **JSON array** — `application/json` as `[obj1,obj2,...]` (`Close` writes the trailing `]`)
- Do **not** use removed helpers `WriteNDJSON` / `WriteSSE`.

## Packages

| Path | Role |
|------|------|
| `.` (`package httpbind`) | Runtime: Bind / Write / WriteError / NewStream / OpenAPI serve / SwaggerUI |
| `jsonbind/` | Standalone DecodeJSON / EncodeJSON runtime; does not import `net/http` or `database/sql` |
| `sqlbind/` | ScanRows runtime and row helpers; does not import `net/http` |
| `generator/` | Field-plan binders/writers + OpenAPI 3.1 + template generation |
| `parser/` | Route/handler discovery (`Bind`, `Write`, `NewStream`, errors) |
| `templates/htmlbind/` | Typed, context-safe HTML template compiler |
| `templates/sqlbind/` | Typed, parameterized SQL template compiler |
| `cmd/tinybind-gen` | CLI: binders + OpenAPI + templates from a package dir |
| `examples/demo` | End-to-end sample app |
| `internal/*` | Test fixtures |
| `testdata/cmd/*` | Dev-only helpers (not for distribution; under `testdata` so `go get` / `./...` skip them) |

```bash
go run ./cmd/tinybind-gen -dir ./path/to/package
```

The CLI automatically discovers `.tb.html` and `.tb.sql` files in the target
package and writes `tinybind_templates_gen.go`. SQL value expressions become
driver arguments; PostgreSQL-style `$1`, `$2`, … placeholders are generated in
encounter order:

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

Custom generator commands only need to call `generator.Main`. Start with
`DefaultOptions`, then replace each authoritative `Set` with every identity the
project accepts:

```go
package main

import "github.com/shibukawa/tinybind-go/generator"

func main() {
    options := generator.DefaultOptions()
    options.ServeMuxes.Set = []generator.TypePattern{
        {PackagePath: "net/http", Name: "ServeMux"},
        {PackagePath: "github.com/shibukawa/petitweb-go/handler", Name: "ServeMux"},
    }
    options.RuntimePackages.Set = []string{
        "github.com/shibukawa/tinybind-go",
        "github.com/shibukawa/tinybind-go/jsonbind",
        "github.com/shibukawa/tinybind-go/sqlbind",
        "github.com/shibukawa/petitweb-go/handler",
    }
    generator.Main(options)
}
```

`RuntimePackages` expands the same-named `Bind`, `Write`, `WriteStatus`,
`DecodeJSON`, `EncodeJSON`, `NewStream`, and `ScanRows` functions. An
operation-specific set such as `options.DecodeJSON.Set` replaces that expansion.
`Set` always replaces defaults; include both the standard and compatibility
identity when both should be explored. `generator.Options{}` deliberately has
no discovery identities. Set a pattern's `Disabled` field, or add its feature to
`DisableFeatures`, to prevent discovery even under `-generate-all`.

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

## Demo

```bash
go generate ./examples/demo
go run ./examples/demo
# http://localhost:8080/       index + browser stream demo
# http://localhost:8080/docs/  Swagger UI
# http://localhost:8080/chat   NewStream (SSE / NDJSON / JSON array auto)
```

See [`examples/demo/README.md`](examples/demo/README.md) for full curl recipes.

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
- JSON-only generated code imports `jsonbind` only; the test matrix builds it with `tinygo build -target wasm`.

### Known limitations

| Topic | Limitation |
|-------|------------|
| Toolchain | Project baseline is TinyGo 0.41.1 + Go 1.26.x |
| js/wasm HTTP | TinyGo 0.41.1 + Go 1.26.x fails inside `net/http/roundtrip_js.go`; use `jsonbind` for HTTP-free WASM code |
| Streaming | Prefer host `go test` for `NewStream`; not fully TinyGo-matrixed |
| ServeMux | Prefer testing handlers with `ServeHTTP` + `SetPathValue` under TinyGo |
| Multipart `File` | Supported via `httpbind.File` (`payload`); size/MIME `check` rules deferred. Body cap defaults to **1 MiB** (`SetMaxMultipartBodyBytes`) |
| SQL mapping | `ScanRows` and generated SQL scanners target host Go and are excluded from TinyGo builds |
| Generator | Host-side only (`go run` / `go test`) |

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
