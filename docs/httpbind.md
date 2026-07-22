# httpbind User Guide

`httpbind` maps HTTP requests to Go structs and writes structs as JSON responses in `net/http` handlers. It also generates an OpenAPI 3.1 document from the same analysis.

## What is automated

- Binding query, JSON, form, multipart, path, header, and cookie values to structs
- Converting strings to `int`, `int64`, `bool`, and `float64`
- Input validation and defaults from `check` tags
- Encoding structs as JSON responses
- Converting binding and validation failures to RFC 9457 Problem Details
- Statically discovering `net/http` routes and the types used by handlers
- Generating an OpenAPI 3.1 document from routes, inputs, outputs, and errors
- Selecting SSE, NDJSON, or JSON array for streaming responses

You do not need a separate routing DSL or schema file. Ordinary Go types, `net/http` registrations, and calls to `Bind` and `Write` are the inputs to generation.

## What you provide

1. Request and response structs
2. `net/http` handlers that call `httpbind.Bind`, `Write`, or related APIs
3. Route registrations on `http.ServeMux` or another configured mux
4. A code-generation command
5. Authentication, database access, and application logic

## Setup and generation

Place a generation directive in the target package:

```go
package api

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

```bash
go generate ./...
```

The default output files are:

- `tinybind_gen.go` — HTTP and JSON bindings for types actually used
- `tinybind_openapi_gen.go` — embedded OpenAPI JSON and YAML
- `tinybind_templates_gen.go` — generated templates, when the package contains templates

Use `-check` in CI to fail when route candidates cannot be analyzed:

```bash
go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir . -check
```

One invocation analyzes one Go package. It does not follow handler implementations into another package, so route registration and the analyzed handler should normally live in the same package.

## Minimal API

```go
package api

import (
	"net/http"

	httpbind "github.com/shibukawa/tinybind-go"
)

type HelloRequest struct {
	Name string `query:"name" check:"required"`
}

type HelloResponse struct {
	Message string `json:"message"`
}

func hello(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[HelloRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}

	out := HelloResponse{Message: "Hello, " + in.Name}
	if err := httpbind.Write(w, r, out); err != nil {
		httpbind.WriteError(w, r, httpbind.Internal(err))
	}
}

func Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", hello)
	return mux
}
```

```bash
curl 'http://localhost:8080/hello?name=Ada'
# {"message":"Hello, Ada"}
```

## Request field sources

An untagged field uses the `input` source. Without an explicit wire name, the field name becomes lower camel case: `DisplayName` becomes `displayName`.

| Tag | Source | Typical use |
| --- | --- | --- |
| no tag / `input:"name"` | query or body | General input; query wins when both are present |
| `query:"page"` | query only | Search filters and pagination |
| `payload:"name"` | body only | Values restricted to JSON, form, or multipart |
| `path:"id"` | path value | Values from a route such as `GET /users/{id}` |
| `header:"Authorization"` | HTTP header | Authentication and metadata |
| `cookie:"session"` | cookie | Session identifiers |
| `method:"method"` | HTTP method | `GET`, `POST`, and so on |

For scalar `input` fields, binding checks the query first and reads the body only when the query value is absent. Nested structs, slices, and maps are read from the body. Use explicit `query` and `payload` tags when the source must not be ambiguous.

```go
type SearchRequest struct {
	Keyword string `query:"keyword" check:"required"`
	Page    int    `query:"page" check:"min=1,default=1"`
	Filter  string `payload:"filter"`
}
```

```bash
curl 'http://localhost:8080/search?keyword=go&page=2' \
  -H 'Content-Type: application/json' \
  -d '{"filter":"active"}'
```

### Supported request bodies

- `application/json`
- `application/x-www-form-urlencoded`
- `multipart/form-data`

```go
type CreateUserRequest struct {
	Name  string `payload:"name"`
	Email string `payload:"email"`
}
```

The same model accepts JSON or form data:

```bash
curl http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada","email":"ada@example.com"}'

curl http://localhost:8080/users \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'name=Ada' \
  --data-urlencode 'email=ada@example.com'
```

### Path, header, and cookie values

```go
type GetUserRequest struct {
	ID      string `path:"id" check:"required,uuid"`
	Token   string `header:"Authorization" check:"required"`
	Session string `cookie:"session"`
}

func getUser(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[GetUserRequest](r)
	// ...
}

mux.HandleFunc("GET /users/{id}", getUser)
```

### Multipart files

Use `httpbind.File` with a `payload` tag:

```go
type UploadRequest struct {
	Title string        `payload:"title" check:"required"`
	Image httpbind.File `payload:"image"`
}

func upload(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[UploadRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}

	_ = in.Image.Filename
	_ = in.Image.ContentType
	_ = in.Image.Size
	_ = in.Image.Content
}
```

```bash
curl http://localhost:8080/uploads \
  -F 'title=avatar' \
  -F 'image=@avatar.png'
```

The default multipart body limit is 1 MiB. Change it during application startup when needed:

```go
httpbind.SetMaxMultipartBodyBytes(8 << 20) // 8 MiB
```

### Collecting undeclared fields

Use `payload:"*"` to collect JSON or form fields that were not explicitly declared:

```go
type EventRequest struct {
	Type   string         `payload:"type"`
	Extras map[string]any `payload:"*"`
}
```

Use `map[string]json.RawMessage` to retain raw JSON values:

```go
type EventRequest struct {
	Type   string                     `payload:"type"`
	Extras map[string]json.RawMessage `payload:"*"`
}
```

## Input validation

`check` tags are interpreted during generation. The application runs generated validation code rather than parsing tags at runtime.

| Rule | Applies to | Example |
| --- | --- | --- |
| `required` | Input presence; also rejects empty strings and empty files | `check:"required"` |
| `default=value` | Scalars | `check:"default=1"` |
| `min` / `max` | Numbers | `check:"min=1,max=100"` |
| `minlen` / `maxlen` / `len` | Strings | `check:"minlen=3,maxlen=64"` |
| `enum=a\|b` | Scalars | `check:"enum=asc\|desc"` |
| `pattern=...` | Strings | `check:"pattern=^[A-Z]{3}$"` |
| `email` | Strings | `check:"email"` |
| `uuid` | Strings | `check:"uuid"` |
| `date` | Strings | `YYYY-MM-DD` |
| `time` | Strings | `HH:MM:SS` |
| `datetime` | Strings | RFC 3339 |

Put `pattern` last when it contains commas, because commas otherwise separate rules.

```go
type CreateAccountRequest struct {
	Name     string `check:"required,minlen=1,maxlen=64"`
	Email    string `check:"required,email,maxlen=254"`
	Age      int    `check:"min=0,max=150"`
	Plan     string `check:"enum=free|pro,default=free"`
	PostCode string `check:"pattern=^[0-9]{3}-[0-9]{4}$"`
}
```

Defaults are applied after validation when a value was absent. This makes `check:"min=1,default=-1"` useful as a sentinel: an absent value becomes `-1`, while an explicitly supplied `-1` fails validation.

For non-pointer numbers and booleans, Go's zero value can make an omitted value indistinguishable from an explicit `0` or `false` in some situations. Account for that limitation when presence itself is part of the API contract.

## Responses

### 200 JSON

```go
type UserResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

err := httpbind.Write(w, r, UserResponse{ID: "u_1", Name: "Ada"})
```

`Write` writes `200 OK` with `application/json`.

The generated encoder uses the name portion of `json` tags, but currently does not apply `omitempty` or exclusion via `json:"-"`. Design response models assuming every field is emitted.

### Other success statuses

```go
err := httpbind.WriteStatus(
	w,
	r,
	http.StatusCreated,
	UserResponse{ID: "u_1", Name: "Ada"},
)
```

For `204 No Content`, `WriteStatus` does not write a body.

## Error responses

Return HTTP-aware errors from application logic and pass them to `WriteError` at the handler boundary:

```go
func findUser(id string) (UserResponse, error) {
	if id == "" {
		return UserResponse{}, httpbind.BadRequest(httpbind.Problem{
			Code:    "missing_id",
			Message: "id is required",
		})
	}
	return UserResponse{}, httpbind.NotFound(httpbind.Problem{
		Code:    "user_not_found",
		Message: "user was not found",
	})
}
```

Available constructors:

- `BadRequest` — 400
- `Unauthorized` — 401
- `Forbidden` — 403
- `NotFound` — 404
- `Conflict` — 409
- `PayloadTooLarge` — 413
- `Internal` — 500
- `Validation` — 400 with field details

```go
err := httpbind.Validation(
	httpbind.Field("email", "payload", "already registered"),
)
httpbind.WriteError(w, r, err)
```

Clients receive `application/problem+json`. For 5xx responses, internal causes and implementation details are not exposed in the body.

## Streaming

```go
type ChatEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
}

func chat(w http.ResponseWriter, r *http.Request) {
	stream, err := httpbind.NewStream[ChatEvent](w, r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	defer stream.Close()

	if err := stream.Write(ChatEvent{Type: "delta", Delta: "hello"}); err != nil {
		return
	}
	_ = stream.Write(ChatEvent{Type: "done"})
}
```

The format is selected once by `NewStream`, in this order:

1. `?stream=`
2. `Accept`
3. User-Agent heuristics
4. NDJSON by default

```bash
# SSE
curl -N 'http://localhost:8080/chat?stream=sse'

# NDJSON
curl -N -H 'Accept: application/x-ndjson' http://localhost:8080/chat

# One JSON array document
curl -H 'Accept: application/json' http://localhost:8080/chat
```

Always use `defer stream.Close()`: for the JSON array format, `Close` writes the closing `]`.

## OpenAPI and Swagger UI

The generator reflects discovered routes, `Bind` types, `Write` / `WriteStatus` / `NewStream` types, and HTTP errors in OpenAPI.

```go
mux.HandleFunc("GET /openapi.json", httpbind.OpenAPIJSON)
mux.HandleFunc("GET /openapi.yaml", httpbind.OpenAPIYAML)
mux.Handle("GET /docs/{$}", httpbind.SwaggerUI("/openapi.json"))
```

Swagger UI assets are loaded from a CDN. In offline environments, serve only the OpenAPI JSON/YAML or host a UI separately.

## Missing generated bindings

The generator normally discovers concrete types from generic calls in source code:

```go
httpbind.Bind[CreateUserRequest](r)
httpbind.Write[CreateUserResponse](w, r, out)
```

Discovery may fail when the call is hidden in another package, exists only behind a custom wrapper, or does not statically identify a type. Check `-check` diagnostics first. `-generate-all` can generate every enabled mapping for every struct, but direct concrete calls normally produce smaller output.
