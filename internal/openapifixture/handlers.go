package openapifixture

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

// createUserHandler creates one organization user.
//
// Input is accepted from the JSON body, form body, or query string; the
// organization comes from the path and the caller from the Authorization header.
func createUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	if input.Email == "" {
		httpbind.WriteError(w, r, httpbind.Validation(
			httpbind.Field("email", "payload", "required"),
		))
		return
	}
	if input.Name == "conflict" {
		httpbind.WriteError(w, r, httpbind.Conflict(httpbind.Problem{
			Code: "duplicate", Message: "name taken",
		}))
		return
	}
	out := CreateUserResponse{
		ID:    "user_123",
		Name:  input.Name,
		Email: input.Email,
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
	if input.Keyword == "" {
		httpbind.WriteError(w, r, httpbind.BadRequest(httpbind.Problem{
			Code: "missing_keyword", Message: "keyword required",
		}))
		return
	}
	out := SearchResponse{
		Keyword: input.Keyword,
		Page:    input.Page,
		Filter:  input.Filter,
	}
	if err := httpbind.Write[SearchResponse](w, r, out); err != nil {
		httpbind.WriteError(w, r, err)
	}
}

// getMissingHandler always reports the user as missing.
//
// Deprecated: kept only to exercise NotFound discovery.
func getMissingHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	httpbind.WriteError(w, r, httpbind.NotFound(httpbind.Problem{
		Code: "user_not_found", Message: "missing",
	}))
}

// RegisterRoutes mounts static routes for OpenAPI discovery (same package).
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orgs/{org_id}/users", createUserHandler)
	mux.HandleFunc("GET /search", searchHandler)
	mux.HandleFunc("GET /users/{org_id}", getMissingHandler)
}
