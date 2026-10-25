package app

import (
	"net/http"
	"time"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type JobRequest struct{}
type JobResponse struct{}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[JobRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[JobResponse](w, r, JobResponse{})
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
