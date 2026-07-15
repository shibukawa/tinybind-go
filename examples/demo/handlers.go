package main

import (
	"net/http"
	"strings"

	httpbinder "github.com/shibukawa/httpbind-go"
)

// in-memory toy store for the demo
var users = map[string]UserGetResponse{
	"user_123": {ID: "user_123", Name: "Alice"},
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	_ = httpbinder.Write[HealthResponse](w, r, HealthResponse{Status: "ok"})
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[CreateUserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	if input.Name == "" || input.Email == "" {
		httpbinder.WriteError(w, r, httpbinder.Validation(
			httpbinder.Field("name", "payload", "required"),
			httpbinder.Field("email", "payload", "required"),
		))
		return
	}
	if !strings.Contains(input.Email, "@") {
		httpbinder.WriteError(w, r, httpbinder.BadRequest(httpbinder.Problem{
			Code:    "invalid_email",
			Message: "email is invalid",
		}))
		return
	}
	if input.Name == "taken" {
		httpbinder.WriteError(w, r, httpbinder.Conflict(httpbinder.Problem{
			Code:    "duplicate_name",
			Message: "name already exists",
		}))
		return
	}

	id := "user_123"
	users[id] = UserGetResponse{ID: id, Name: input.Name}
	out := CreateUserResponse{
		ID:    id,
		Name:  input.Name,
		Email: input.Email,
		OrgID: input.OrgID,
	}
	if err := httpbinder.Write[CreateUserResponse](w, r, out); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[SearchRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	if input.Keyword == "" {
		httpbinder.WriteError(w, r, httpbinder.BadRequest(httpbinder.Problem{
			Code:    "missing_keyword",
			Message: "keyword is required",
		}))
		return
	}
	page := input.Page
	if page <= 0 {
		page = 1
	}
	out := SearchResponse{
		Keyword: input.Keyword,
		Page:    page,
		Filter:  input.Filter,
		Hits:    1,
	}
	if err := httpbinder.Write[SearchResponse](w, r, out); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[EchoRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	if input.Message == "" {
		httpbinder.WriteError(w, r, httpbinder.Validation(
			httpbinder.Field("message", "payload", "required"),
		))
		return
	}
	out := EchoResponse{Message: input.Message, N: input.N}
	if err := httpbinder.Write[EchoResponse](w, r, out); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}

func sessionHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[SessionRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	if input.Session == "" {
		httpbinder.WriteError(w, r, httpbinder.Unauthorized(httpbinder.Problem{
			Code:    "no_session",
			Message: "session cookie missing",
		}))
		return
	}
	out := SessionResponse{Session: input.Session, OK: true}
	if err := httpbinder.Write[SessionResponse](w, r, out); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[UserGetRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	u, ok := users[input.ID]
	if !ok {
		httpbinder.WriteError(w, r, httpbinder.NotFound(httpbinder.Problem{
			Code:    "user_not_found",
			Message: "user not found",
		}))
		return
	}
	if err := httpbinder.Write[UserGetResponse](w, r, u); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}

// chatStreamHandler demos NewStream + multiple Write calls.
// Transport (SSE / NDJSON-JSONL / JSON array) is selected from Accept / ?stream= / User-Agent.
func chatStreamHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[ChatRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}

	stream, err := httpbinder.NewStream[ChatEvent](w, r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	defer stream.Close()

	msg := input.Message
	if msg == "" {
		msg = "hello"
	}

	// Incremental event emission (ideal streaming path).
	events := []ChatEvent{
		{Type: "start"},
		{Type: "delta", Delta: msg + " "},
		{Type: "delta", Delta: "from "},
		{Type: "delta", Delta: "httpbinder"},
		{Type: "meta", Delta: string(stream.Format())},
		{Type: "done"},
	}
	for _, e := range events {
		if err := stream.Write(e); err != nil {
			return
		}
	}
}

func forbiddenDemoHandler(w http.ResponseWriter, r *http.Request) {
	httpbinder.WriteError(w, r, httpbinder.Forbidden(httpbinder.Problem{
		Code:    "forbidden",
		Message: "you shall not pass",
	}))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

// RegisterDemoRoutes mounts all demo routes on mux (also used for OpenAPI discovery).
func RegisterDemoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", indexHandler)
	mux.HandleFunc("GET /health", healthHandler)

	mux.HandleFunc("POST /orgs/{org_id}/users", createUserHandler)
	mux.HandleFunc("GET /users/{id}", getUserHandler)

	mux.HandleFunc("POST /search", searchHandler)
	mux.HandleFunc("POST /echo", echoHandler)
	mux.HandleFunc("GET /session", sessionHandler)

	// Streaming: one endpoint, format auto-selected.
	mux.HandleFunc("POST /chat", chatStreamHandler)
	mux.HandleFunc("GET /forbidden", forbiddenDemoHandler)

	mux.HandleFunc("GET /openapi.json", httpbinder.OpenAPIJSON)
	mux.HandleFunc("GET /openapi.yaml", httpbinder.OpenAPIYAML)
	mux.Handle("GET /docs/{$}", httpbinder.SwaggerUI("/openapi.json"))
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusFound)
	})
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>httpbinder demo</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 54rem; margin: 2rem auto; padding: 0 1rem; line-height: 1.45; }
    code, pre { background: #f4f4f5; border-radius: 4px; }
    pre { padding: 0.75rem 1rem; overflow-x: auto; }
    h1 { margin-bottom: 0.25rem; }
    .muted { color: #666; }
    a { color: #0b57d0; }
    #stream-out { white-space: pre-wrap; min-height: 4rem; border: 1px solid #ddd; padding: 0.75rem; border-radius: 6px; }
  </style>
</head>
<body>
  <h1>httpbinder demo</h1>
  <p class="muted">Bind / Write / WriteError / OpenAPI / <code>NewStream[T]</code> (SSE / NDJSON-JSONL / JSON array).</p>
  <ul>
    <li><a href="/docs/">GET /docs/</a> — Swagger UI</li>
    <li><a href="/health">GET /health</a></li>
    <li><a href="/openapi.json">GET /openapi.json</a></li>
    <li><a href="/openapi.yaml">GET /openapi.yaml</a></li>
  </ul>

  <h2>Browser stream (fetch → format via Accept)</h2>
  <p>
    <input id="msg" value="hello" />
    <button type="button" id="btn-sse">Stream as SSE</button>
    <button type="button" id="btn-nd">Stream as NDJSON/JSONL</button>
    <button type="button" id="btn-ja">Stream as JSON array</button>
  </p>
  <div id="stream-out" class="muted">output…</div>

  <h2>curl recipes</h2>
  <pre># create user (JSON + path + header)
curl -sS -X POST 'http://localhost:8080/orgs/acme/users' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer secret' \
  -d '{"name":"Alice","email":"a@example.com"}'

# create user (query input)
curl -sS -X POST 'http://localhost:8080/orgs/acme/users?name=Bob&email=b@example.com' \
  -H 'Authorization: Bearer secret'

# create user (form)
curl -sS -X POST 'http://localhost:8080/orgs/acme/users' \
  -H 'Authorization: Bearer secret' \
  -d 'name=Carol&email=c@example.com'

# validation error
curl -sS -X POST 'http://localhost:8080/orgs/acme/users' \
  -H 'Content-Type: application/json' -H 'Authorization: Bearer x' -d '{}'

# search: query + JSON payload field
curl -sS -X POST 'http://localhost:8080/search?keyword=go&page=2' \
  -H 'Content-Type: application/json' -d '{"filter":"active"}'

# echo
curl -sS -X POST 'http://localhost:8080/echo?n=3' \
  -H 'Content-Type: application/json' -d '{"message":"hi"}'

# session cookie
curl -sS 'http://localhost:8080/session' -H 'Cookie: session=abc'

# not found
curl -sS 'http://localhost:8080/users/missing'

# stream — curl UA defaults to NDJSON; use -N to not buffer
curl -sSN -X POST 'http://localhost:8080/chat' \
  -H 'Content-Type: application/json' -d '{"message":"hello"}'

# stream — force SSE via query
curl -sSN -X POST 'http://localhost:8080/chat?stream=sse' \
  -H 'Content-Type: application/json' -d '{"message":"hello"}'

# stream — force SSE via Accept
curl -sSN -X POST 'http://localhost:8080/chat' \
  -H 'Content-Type: application/json' -H 'Accept: text/event-stream' \
  -d '{"message":"hello"}'

# stream — force NDJSON/JSONL via query (even with browser-like Accept)
curl -sSN -X POST 'http://localhost:8080/chat?stream=ndjson' \
  -H 'Content-Type: application/json' -d '{"message":"hello"}'

# stream — JSON array document (not JSONL): [obj1,obj2,...]
curl -sSN -X POST 'http://localhost:8080/chat' \
  -H 'Content-Type: application/json' -H 'Accept: application/json' \
  -d '{"message":"hello"}'
</pre>
  <script>
    async function runStream(accept) {
      const out = document.getElementById('stream-out');
      out.textContent = '…';
      const msg = document.getElementById('msg').value || 'hello';
      const res = await fetch('/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Accept': accept,
        },
        body: JSON.stringify({ message: msg }),
      });
      out.textContent = 'HTTP ' + res.status + '  Content-Type: ' + res.headers.get('content-type') + '\n\n';
      const reader = res.body.getReader();
      const dec = new TextDecoder();
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        out.textContent += dec.decode(value, { stream: true });
      }
    }
    document.getElementById('btn-sse').onclick = () => runStream('text/event-stream');
    document.getElementById('btn-nd').onclick = () => runStream('application/x-ndjson');
    document.getElementById('btn-ja').onclick = () => runStream('application/json');
  </script>
</body>
</html>
`
