package app

import (
	"net/http"
	"time"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type PingRequest struct{}
type PingResponse struct{}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbind.Bind[PingRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[PingResponse](w, r, PingResponse{})
}

func register(mux *http.ServeMux) {
	mux.Handle(
		"GET /ping",
		http.TimeoutHandler(
			http.HandlerFunc(pingHandler),
			time.Second,
			"slow",
		),
	)
}
