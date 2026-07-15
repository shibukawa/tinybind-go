package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type HealthRequest struct{}
type HealthResponse struct {
	OK bool
}

type GetUserRequest struct {
	ID string
}
type GetUserResponse struct {
	ID string
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[HealthRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[HealthResponse](w, r, HealthResponse{OK: true})
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[GetUserRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[GetUserResponse](w, r, GetUserResponse{ID: "u1"})
}

func register() {
	http.HandleFunc("GET /health", healthHandler)
	http.Handle("GET /users/{id}", http.HandlerFunc(getUserHandler))
}
