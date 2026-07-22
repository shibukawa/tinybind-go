# httpbind demo

Sample app that exercises the main library features end-to-end.

| Feature | Where |
|---------|--------|
| Generated Bind / Write | `tinybind_gen.go` (`go generate`) |
| TinyGo-compatible Go 1.22 routing | `tinygodriver/httpmux` |
| **configbind** listen port | `ServerConfig` + `configbind_gen.go` (`PORT` / `--port` / `-p`) |
| Default `input` (query + JSON/form body) | `CreateUserRequest`, `EchoRequest` |
| `query` / `payload` | `SearchRequest` |
| `path` / `header` | create user, get user |
| `cookie` | `GET /session` |
| Generated `check` validation | request struct tags + `Bind` / `WriteError` |
| Domain 4xx / 5xx helpers | handlers + `WriteError` |
| Typed HTML template | `index.tb.html` → generated `IndexPage` |
| OpenAPI 3.1 embed | `/openapi.json`, `/openapi.yaml` |
| Swagger UI | `/docs/` |
| **Streaming ideal API** | `POST /chat` via `NewStream[T]` + multi `Write` |

## Streaming model

```go
stream, err := httpbind.NewStream[ChatEvent](w, r)
if err != nil { ... }
defer stream.Close()

_ = stream.Write(ChatEvent{Type: "delta", Delta: "hi"})
_ = stream.Write(ChatEvent{Type: "done"})
```

### Format selection (automatic)

| Priority | Source | Result |
|----------|--------|--------|
| 1 | `?stream=sse` / `ndjson` / `jsonl` / `json` / `array` | forced |
| 2 | `Accept: text/event-stream` | SSE |
| 2 | `Accept: application/x-ndjson` or `application/jsonl` | **NDJSON / JSONL** (line-delimited; not an array) |
| 2 | `Accept: application/json` | **JSON array** document `[...]` |
| 3 | Browser-like User-Agent | SSE |
| 3 | curl / wget / httpie | NDJSON |
| 4 | default | NDJSON |

`Write` is **safe to call many times**. Headers/status are sent only in `NewStream`.  
JSON array mode needs `defer stream.Close()` so the trailing `]` is written.

**NDJSON/JSONL ≠ JSON array**: JSONL is one object per line; JSON array is a single `[obj1,obj2]` body.

## Run

```bash
# from repository root
go generate ./examples/demo   # regenerate Bind/Write + OpenAPI if needed
go run ./examples/demo
```

| URL | |
|-----|--|
| http://localhost:8080/ | HTML index + browser stream buttons |
| http://localhost:8080/docs/ | Swagger UI |
| http://localhost:8080/openapi.json | OpenAPI 3.1 |

```bash
# listen port via configbind (env name follows CLI long opt: port -> PORT)
PORT=9090 go run ./examples/demo
# or:
go run ./examples/demo -- --port 9090
```

Optional TOML (configdir / `--config-path`):

```toml
[server]
port = 9090
```

## Quick checks

```bash
curl -sS http://localhost:8080/health

curl -sS -X POST 'http://localhost:8080/orgs/acme/users' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer secret' \
  -d '{"name":"Alice","email":"a@example.com"}'

# NDJSON / JSONL stream (curl default)
curl -sSN -X POST 'http://localhost:8080/chat' \
  -H 'Content-Type: application/json' \
  -d '{"message":"hello"}'

# SSE stream
curl -sSN -X POST 'http://localhost:8080/chat?stream=sse' \
  -H 'Content-Type: application/json' \
  -d '{"message":"hello"}'

# JSON array stream (single [...] document; not JSONL)
curl -sSN -X POST 'http://localhost:8080/chat' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d '{"message":"hello"}'
```

## Layout

```
examples/demo/
  main.go
  handlers.go                 # routes + NewStream chat
  index.tb.html               # typed, context-safe HTML template
  index_script.go             # JavaScript passed as trusted script content
  types.go                    # includes ServerConfig (configbind)
  generate.go                 # go:generate
  tinybind_gen.go             # generated Bind/Write
  tinybind_openapi_gen.go     # generated OpenAPI embed
  tinybind_templates_gen.go   # generated typed HTML renderer
  configbind_gen.go           # generated config apply / flags / env
  demo_test.go
  README.md
```

## Regenerate

```bash
go generate ./examples/demo
# equivalent:
# go run ./cmd/tinybind-gen -dir ./examples/demo
```
