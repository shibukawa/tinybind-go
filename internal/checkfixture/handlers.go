package checkfixture

import (
	"net/http"

	httpbinder "github.com/shibukawa/httpbind-go"
)

// RegisterRoutes exposes OpenAPICheck for OpenAPI generation tests.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /check", handleOpenAPICheck)
}

func handleOpenAPICheck(w http.ResponseWriter, r *http.Request) {
	in, err := httpbinder.Bind[OpenAPICheckRequest](r)
	if err != nil {
		httpbinder.WriteError(w, r, err)
		return
	}
	_ = in
	if err := httpbinder.Write[OpenAPICheckResponse](w, r, OpenAPICheckResponse{OK: true}); err != nil {
		httpbinder.WriteError(w, r, err)
	}
}
