package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type ListRequest struct{}
type ListResponse struct{}
type CreateRequest struct{}
type CreateResponse struct{}

func listHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[ListRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[ListResponse](w, r, ListResponse{})
}

func createHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[CreateRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, httpbinder.BadRequest(httpbinder.Problem{Code: "bad", Message: "x"}))
		return
	}
	_ = httpbinder.Write[CreateResponse](w, r, CreateResponse{})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /items", listHandler)
	mux.HandleFunc("POST /items", createHandler)
}
