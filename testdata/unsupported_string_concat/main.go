package app

import (
	"net/http"

	"github.com/shibukawa/httpbind-go"
)

type Req struct{}
type Resp struct{}

func handler(w http.ResponseWriter, r *http.Request) {
	_, _ = httpbinder.Bind[Req](r)
	_ = httpbinder.Write[Resp](w, r, Resp{})
}

func register(mux *http.ServeMux) {
	path := "/users"
	// Dynamic pattern: must not yield a discovered route.
	mux.HandleFunc("GET "+path, handler)
}
