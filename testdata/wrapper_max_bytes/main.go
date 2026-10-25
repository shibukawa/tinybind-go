package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type UploadRequest struct {
	Name string
}
type UploadResponse struct {
	OK bool
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[UploadRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[UploadResponse](w, r, UploadResponse{OK: true})
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
