package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type Req struct{}
type Resp struct{}

func handler(w http.ResponseWriter, r *http.Request) {
	_, _ = httpbind.Bind[Req](r)
	_ = httpbind.Write[Resp](w, r, Resp{})
}

func register(mux *http.ServeMux) {
	path := "/users"
	// Dynamic pattern: must not yield a discovered route.
	mux.HandleFunc("GET "+path, handler)
}
