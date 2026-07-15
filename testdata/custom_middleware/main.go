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
	_, err := httpbinder.Bind[CreateUserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[CreateUserResponse](w, r, CreateUserResponse{ID: "1"})
}

func Logging(next http.Handler) http.Handler {
	return next
}

func Auth(next http.Handler) http.Handler {
	return next
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"POST /users",
		Logging(
			Auth(
				http.HandlerFunc(createUserHandler),
			),
		),
	)
}
