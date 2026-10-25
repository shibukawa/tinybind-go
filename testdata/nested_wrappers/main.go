package app

import (
	"net/http"
	"time"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type UploadRequest struct{}
type UploadResponse struct{}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[UploadRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[UploadResponse](w, r, UploadResponse{})
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
