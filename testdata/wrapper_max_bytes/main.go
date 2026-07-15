package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type UploadRequest struct {
	Name string
}
type UploadResponse struct {
	OK bool
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[UploadRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[UploadResponse](w, r, UploadResponse{OK: true})
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"POST /upload",
		http.MaxBytesHandler(
			http.HandlerFunc(uploadHandler),
			10<<20,
		),
	)
}
