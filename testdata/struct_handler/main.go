package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type UserRequest struct {
	Name string
}
type UserResponse struct {
	Name string
}

// UserHandler serves one user by name.
type UserHandler struct{}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[UserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[UserResponse](w, r, UserResponse{Name: input.Name})
}

func register(mux *http.ServeMux) {
	mux.Handle("POST /users", &UserHandler{})
}
