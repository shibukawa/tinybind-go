package main

// ServerConfig is process config loaded via configbind (TOML / env / CLI).
// With opt:"port,p", the environment variable is PORT (CLI long name based).
type ServerConfig struct {
	Port int `default:"8080" help:"HTTP listen port" opt:"port,p"`
}

// CreateUserRequest demos default input, generated validation, path, and header binding.
type CreateUserRequest struct {
	Name  string `check:"required,minlen=1,maxlen=64"`
	Email string `check:"required,email,maxlen=254"`
	OrgID string `path:"org_id"`
	Token string `header:"Authorization"`
}

// CreateUserResponse is returned after create.
type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	OrgID string `json:"org_id"`
}

// SearchRequest demos query-only and payload-only fields.
type SearchRequest struct {
	Keyword string `query:"keyword" check:"required"`
	Page    int    `query:"page" check:"min=1,default=1"`
	Filter  string `payload:"filter"`
}

// SearchResponse is search output.
type SearchResponse struct {
	Keyword string `json:"keyword"`
	Page    int    `json:"page"`
	Filter  string `json:"filter"`
	Hits    int    `json:"hits"`
}

// EchoRequest demos input from query or form/JSON body.
type EchoRequest struct {
	Message string `check:"required"`
	N       int    `query:"n"`
}

// EchoResponse echoes back.
type EchoResponse struct {
	Message string `json:"message"`
	N       int    `json:"n"`
}

// SessionRequest demos cookie binding.
type SessionRequest struct {
	Session string `cookie:"session"`
}

// SessionResponse shows cookie value.
type SessionResponse struct {
	Session string `json:"session"`
	OK      bool   `json:"ok"`
}

// UserGetRequest path-only lookup.
type UserGetRequest struct {
	ID string `path:"id"`
}

// UserGetResponse user payload.
type UserGetResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ChatRequest starts a demo stream.
type ChatRequest struct {
	Message string
}

// ChatEvent is one streamed event.
type ChatEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
}

// HealthResponse is a trivial health payload.
type HealthResponse struct {
	Status string `json:"status"`
}
