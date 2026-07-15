package app

import (
	"net/http"
	"time"

	"github.com/shibukawa/httpbind-go"
)

type UploadRequest struct{}
type UploadResponse struct{}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[UploadRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[UploadResponse](w, r, UploadResponse{})
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"POST /upload",
		http.TimeoutHandler(
			http.MaxBytesHandler(
				http.HandlerFunc(uploadHandler),
				10<<20,
			),
			30*time.Second,
			"timeout",
		),
	)
}
