package app

import (
	"errors"
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type UserRequest struct {
	Email string
}
type UserResponse struct {
	ID string
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[UserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}

	_ = httpbind.BadRequest(httpbind.Problem{Code: "invalid_email", Message: "bad"})
	_ = httpbind.Unauthorized(httpbind.Problem{Code: "auth", Message: "no"})
	_ = httpbind.Forbidden(httpbind.Problem{Code: "forbid", Message: "no"})
	_ = httpbind.NotFound(httpbind.Problem{Code: "missing", Message: "no"})
	_ = httpbind.Conflict(httpbind.Problem{Code: "dup", Message: "no"})
	_ = httpbind.Internal(errors.New("boom"))
	_ = httpbind.Validation(httpbind.Field("email", "payload", "invalid"))

	_ = httpbind.Write[UserResponse](w, r, UserResponse{ID: "1"})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /users", userHandler)
}
