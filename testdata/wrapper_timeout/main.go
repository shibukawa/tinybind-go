package app

import (
	"net/http"
	"time"

	"github.com/shibukawa/httpbind-go"
)

type JobRequest struct{}
type JobResponse struct{}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[JobRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[JobResponse](w, r, JobResponse{})
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"POST /jobs",
		http.TimeoutHandler(
			http.HandlerFunc(jobHandler),
			30*time.Second,
			"timeout",
		),
	)
}
