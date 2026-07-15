package app

import (
	"net/http"
	"time"

	"github.com/shibukawa/httpbind-go"
)

type PingRequest struct{}
type PingResponse struct{}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	_, err := httpbinder.Bind[PingRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = httpbinder.Write[PingResponse](w, r, PingResponse{})
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
