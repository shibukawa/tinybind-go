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

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbinder.Bind[UserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[UserResponse](w, r, UserResponse{Name: in.Name})
}

func register(mux *http.ServeMux) {
	mux.Handle("POST /users/{id}", http.HandlerFunc(createUserHandler))
}
