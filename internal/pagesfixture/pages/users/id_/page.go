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

// RenameRequest is what the rename form submits. It is bound by the generated
// binder of this package, which exists because the generator was run over the
// route packages the tree reports.
type RenameRequest struct {
	Name string `input:"name" check:"required"`
}

// Rename is a server function: an ordinary http.HandlerFunc that a template
// names instead of a URL. It owns its whole response, so it reads its own input
// and writes whatever it wants.
func Rename(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[RenameRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_, _ = w.Write([]byte("renamed to " + strings.ToUpper(in.Name)))
}

// Retire is the same shape reached the other way: a form names it, so it is
// submitted natively to the page's own pattern rather than to its hash address.
// It writes nothing, which is what asks for the post-redirect-get default.
func Retire(w http.ResponseWriter, r *http.Request) {
	Retired = r.FormValue("reason") + " for " + r.PathValue("id")
}

// Retired records the last native submit. A handler answering with the default
// redirect writes nothing, so this is how a test tells that it ran at all — and
// it carries the path value, which is the thing the hash address cannot supply.
var Retired string

// unexported handlers stay private, because generated code in another package
// cannot reach them. This is the opt-out, and it needs no declaration.
func internalOnly(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("unreachable"))
}

var _ = internalOnly
