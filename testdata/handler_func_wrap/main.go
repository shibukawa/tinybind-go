package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type UserRequest struct {
	Name string
}
type UserResponse struct {
	Name string
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[UserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[UserResponse](w, r, UserResponse{Name: in.Name})
}

func register(mux *http.ServeMux) {
	mux.Handle("POST /users/{id}", http.HandlerFunc(createUserHandler))
}
