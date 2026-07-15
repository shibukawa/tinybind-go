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

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		input, err := httpbinder.Bind[CreateUserRequest](r)
		if err != nil {
			httpbinder.WriteError(w, r, err)
			return
		}
		_ = input
		_ = httpbinder.Write[CreateUserResponse](w, r, CreateUserResponse{ID: "1"})
	})
}
