//go:build !fasthttp

package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

// createUser is the ordinary shape: bind, work, write, and report errors.
func createUser(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	if input.Name == "" {
		httpbind.WriteError(w, r, httpbind.Validation(httpbind.Field("name", "payload", "required")))
		return
	}
	renderUser(w, r, CreateUserResponse{ID: "u_1", Name: input.Name})
}

// renderUser is the shared helper the closure over the call graph has to carry.
func renderUser(w http.ResponseWriter, r *http.Request, out CreateUserResponse) {
	_ = httpbind.Write[CreateUserResponse](w, r, out)
}

// cancelAware reads the request context, the one selector the rewrite covers.
func cancelAware(w http.ResponseWriter, r *http.Request) {
	if err := r.Context().Err(); err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	httpbind.WriteStatus[CreateUserResponse](w, r, http.StatusAccepted, CreateUserResponse{})
}
