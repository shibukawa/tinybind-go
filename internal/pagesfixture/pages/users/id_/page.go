package id_

import (
	"net/http"
	"strings"

	httpbind "github.com/shibukawa/tinybind-go"
)

// Load loads the display name for one user. It is the typed rung: the request
// reaches Go first and the component parameters are this function's results.
func Load(id string) (string, error) {
	if id == "" {
		return "", nil
	}
	return strings.ToUpper(id), nil
}

// Rename is a server function: an ordinary http.HandlerFunc that a template
// names instead of a URL. It owns its whole response, so it reads its own input
// and writes whatever it wants.
//
// It reads the form directly rather than through httpbind.Bind because binder
// generation is driven by user-written route registrations, and a server
// function is registered by generated code. Until that discovery covers a route
// package, Bind inside a server function finds no registered binder.
func Rename(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpbind.WriteError(w, r, httpbind.BadRequest(
			httpbind.Problem{Code: "form_parse", Message: "invalid form body"}, err))
		return
	}
	_, _ = w.Write([]byte("renamed to " + strings.ToUpper(r.PostFormValue("name"))))
}

// unexported handlers stay private, because generated code in another package
// cannot reach them. This is the opt-out, and it needs no declaration.
func internalOnly(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("unreachable"))
}

var _ = internalOnly
