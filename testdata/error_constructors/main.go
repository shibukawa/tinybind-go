package app

import (
	"errors"
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type UserRequest struct {
	Email string
}
type UserResponse struct {
	ID string
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[UserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}

	_ = httpbinder.BadRequest(httpbinder.Problem{Code: "invalid_email", Message: "bad"})
	_ = httpbinder.Unauthorized(httpbinder.Problem{Code: "auth", Message: "no"})
	_ = httpbinder.Forbidden(httpbinder.Problem{Code: "forbid", Message: "no"})
	_ = httpbinder.NotFound(httpbinder.Problem{Code: "missing", Message: "no"})
	_ = httpbinder.Conflict(httpbinder.Problem{Code: "dup", Message: "no"})
	_ = httpbinder.Internal(errors.New("boom"))
	_ = httpbinder.Validation(httpbinder.Field("email", "payload", "invalid"))

	_ = httpbinder.Write[UserResponse](w, r, UserResponse{ID: "1"})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /users", userHandler)
}
