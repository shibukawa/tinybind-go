package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type CreateUserRequest struct {
	Name string
}

type CreateUserResponse struct {
	ID string
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbinder.Bind[CreateUserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = input
	output := CreateUserResponse{ID: "1"}
	_ = httpbinder.Write[CreateUserResponse](w, r, output)
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /users/{id}", createUserHandler)
}
