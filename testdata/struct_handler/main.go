package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type UserRequest struct {
	Name string
}
type UserResponse struct {
	Name string
}

type UserHandler struct{}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[UserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[UserResponse](w, r, UserResponse{Name: input.Name})
}

func register(mux *http.ServeMux) {
	mux.Handle("POST /users", &UserHandler{})
}
