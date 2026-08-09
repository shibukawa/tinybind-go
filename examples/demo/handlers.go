package main

import (
	"net/http"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/httpmux"
)

// in-memory toy store for the demo
var users = map[string]UserGetResponse{
	"user_123": {ID: "user_123", Name: "Alice"},
}

// healthHandler reports service liveness.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	_ = httpbind.Write[HealthResponse](w, r, HealthResponse{Status: "ok"})
}

// createUserHandler creates a user in one organization.
//
// The organization comes from the path, the caller from the Authorization
// header, and the remaining fields from the query string or the JSON/form body.
func createUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	if input.Name == "taken" {
		httpbind.WriteError(w, r, httpbind.Conflict(httpbind.Problem{
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
	if err := httpbind.Write[CreateUserResponse](w, r, out); err != nil {
		httpbind.WriteError(w, r, err)
	}
}

// searchHandler searches users by keyword.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[SearchRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	out := SearchResponse{
		Keyword: input.Keyword,
		Page:    input.Page,
		Filter:  input.Filter,
		Hits:    1,
	}
	if err := httpbind.Write[SearchResponse](w, r, out); err != nil {
		httpbind.WriteError(w, r, err)
	}
}

// echoHandler echoes the request message back n times.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[EchoRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	out := EchoResponse{Message: input.Message, N: input.N}
	if err := httpbind.Write[EchoResponse](w, r, out); err != nil {
		httpbind.WriteError(w, r, err)
	}
}

// sessionHandler reports the session carried by the session cookie.
func sessionHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[SessionRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	if input.Session == "" {
		httpbind.WriteError(w, r, httpbind.Unauthorized(httpbind.Problem{
			Code:    "no_session",
			Message: "session cookie missing",
		}))
		return
	}
	out := SessionResponse{Session: input.Session, OK: true}
	if err := httpbind.Write[SessionResponse](w, r, out); err != nil {
		httpbind.WriteError(w, r, err)
	}
}

// getUserHandler returns one user by id.
func getUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[UserGetRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	u, ok := users[input.ID]
	if !ok {
		httpbind.WriteError(w, r, httpbind.NotFound(httpbind.Problem{
			Code:    "user_not_found",
			Message: "user not found",
		}))
		return
	}
	if err := httpbind.Write[UserGetResponse](w, r, u); err != nil {
		httpbind.WriteError(w, r, err)
	}
}

// chatStreamHandler demos WriteStream + multiple Write calls.
// Transport (SSE / NDJSON-JSONL / JSON array) is selected from Accept / ?stream= / User-Agent.
func chatStreamHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[ChatRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}

	msg := input.Message
	if msg == "" {
		msg = "hello"
	}

	// The entry closes the stream, so the JSON array framing is terminated even
	// when an event fails halfway through.
	httpbind.WriteStream(w, r, func(stream *httpbind.Stream[ChatEvent]) error {
		// Incremental event emission (ideal streaming path).
		events := []ChatEvent{
			{Type: "start"},
			{Type: "delta", Delta: msg + " "},
			{Type: "delta", Delta: "from "},
			{Type: "delta", Delta: "httpbind"},
			{Type: "meta", Delta: string(stream.Format())},
			{Type: "done"},
		}
		for _, e := range events {
			if err := stream.Write(e); err != nil {
				return err
			}
		}
		return nil
	})
}

// forbiddenDemoHandler always fails with a domain 403 error.
func forbiddenDemoHandler(w http.ResponseWriter, r *http.Request) {
	httpbind.WriteError(w, r, httpbind.Forbidden(httpbind.Problem{
		Code:    "forbidden",
		Message: "you shall not pass",
	}))
}

// indexHandler renders the demo index page from the typed HTML template.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	// Generated templates own rendering only, so the handler owns the response
	// headers.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = htmlbind.Render(w, IndexPage(IndexPageParams{Javascript: indexJavaScript}))
}

// RegisterDemoRoutes mounts all demo routes on mux (also used for OpenAPI discovery).
func RegisterDemoRoutes(mux *httpmux.ServeMux) {
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

	mux.HandleFunc("GET /openapi.json", httpbind.OpenAPIJSON)
	mux.Handle("GET /docs/{$}", httpbind.SwaggerUI("/openapi.json"))
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusFound)
	})
}
