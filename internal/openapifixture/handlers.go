package openapifixture

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[CreateUserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	if input.Email == "" {
		httpbinder.WriteError(w, r, httpbinder.Validation(
			httpbinder.Field("email", "payload", "required"),
		))
		return
	}
	if input.Name == "conflict" {
		httpbinder.WriteError(w, r, httpbinder.Conflict(httpbinder.Problem{
			Code: "duplicate", Message: "name taken",
		}))
		return
	}
	out := CreateUserResponse{
		ID:    "user_123",
		Name:  input.Name,
		Email: input.Email,
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
			Code: "missing_keyword", Message: "keyword required",
		}))
		return
	}
	out := SearchResponse{
		Keyword: input.Keyword,
		Page:    input.Page,
		Filter:  input.Filter,
	}
	if err := httpbinder.Write[SearchResponse](w, r, out); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}

func getMissingHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[CreateUserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	httpbinder.WriteError(w, r, httpbinder.NotFound(httpbinder.Problem{
		Code: "user_not_found", Message: "missing",
	}))
}

// RegisterRoutes mounts static routes for OpenAPI discovery (same package).
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orgs/{org_id}/users", createUserHandler)
	mux.HandleFunc("GET /search", searchHandler)
	mux.HandleFunc("GET /users/{org_id}", getMissingHandler)
}
