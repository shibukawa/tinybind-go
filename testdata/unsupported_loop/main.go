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

type route struct {
	Pattern string
	Handler http.HandlerFunc
}

func register(mux *http.ServeMux) {
	routes := []route{
		{Pattern: "GET /a", Handler: handler},
		{Pattern: "GET /b", Handler: handler},
	}
	for _, route := range routes {
		mux.HandleFunc(route.Pattern, route.Handler)
	}
}
