package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type CreateUserRequest struct {
	Name string
}
type CreateUserResponse struct {
	ID string
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		input, err := httpbind.Bind[CreateUserRequest](r)
		if err != nil {
			httpbind.WriteError(w, r, err)
			return
		}
		_ = input
		_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{ID: "1"})
	})
}
