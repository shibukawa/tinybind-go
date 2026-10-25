package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type ListRequest struct{}
type ListResponse struct{}
type CreateRequest struct{}
type CreateResponse struct{}

func listHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[ListRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[ListResponse](w, r, ListResponse{})
}

func createHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[CreateRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, httpbind.BadRequest(httpbind.Problem{Code: "bad", Message: "x"}))
		return
	}
	_ = httpbind.Write[CreateResponse](w, r, CreateResponse{})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("GET /items", listHandler)
	mux.HandleFunc("POST /items", createHandler)
}
