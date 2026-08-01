package htmlupdate

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Reloadable is one component published as a redraw endpoint.
//
// Registering a component publishes an HTTP endpoint whose parameters anyone
// can supply, so the component authorizes its own inputs exactly as an
// ordinary handler does. Registration is the review point: a component that
// only formats values handed to it is safe, while one that loads a record by
// identifier must check ownership itself.
type Reloadable struct {
	// KindID is the generated component identity, name plus a hash of its
	// parameters and compiled plan. Editing the template changes it, so a page
	// loaded before a deploy requests a kind that no longer exists.
	KindID string
	// Render decodes the query values and returns the bound component. It is
	// generated code: the decoder is typed, and an unknown name or an
	// undecodable value is an error rather than a zero value.
	Render func(r *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error)
}

// Registry holds the components a deployment publishes for redraw.
//
// Nothing is registered implicitly. Being exported, single-rooted, and
// renderable is not enough, because publishing an endpoint must be deliberate.
type Registry struct {
	kinds map[string]Reloadable
}

// Register adds a component to the redraw surface.
//
// A repeated kind panics rather than overwriting. The kind covers a component's
// name, parameters, and compiled markup but not its package, so two identical
// templates in different packages produce the same one; silently keeping the
// last registration would then serve a component that looks the same but calls
// its own package's external functions. Registration happens at startup, so
// failing there is the cheapest place to find it.
func (reg *Registry) Register(component Reloadable) {
	if component.KindID == "" {
		panic("htmlupdate: reloadable component has no kind")
	}
	if reg.kinds == nil {
		reg.kinds = map[string]Reloadable{}
	}
	if _, taken := reg.kinds[component.KindID]; taken {
		panic("htmlupdate: two components registered as " + component.KindID +
			"; the kind covers name, parameters, and markup but not the package, so rename one or change its markup")
	}
	reg.kinds[component.KindID] = component
}

// MaxQueryBytes bounds the arguments a redraw may carry, since a GET puts every
// one of them in the URL.
const MaxQueryBytes = 4 << 10

// RedrawHandler serves the registered components.
//
// The path is <prefix>/redraw/<kind>/<instance>. The instance id travels so the
// returned root element arrives already addressable; the render itself depends
// only on the kind and the query values.
func (o Options) RedrawHandler(reg *Registry) http.Handler {
	base := o.pathPrefix() + "/redraw/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind, instance, ok := splitRedrawPath(strings.TrimPrefix(r.URL.Path, base))
		if !ok {
			http.NotFound(w, r)
			return
		}
		component, known := reg.kinds[kind]
		if !known {
			// This deployment does not publish that component at all.
			http.NotFound(w, r)
			return
		}
		// A kind is stable across builds on purpose, so it cannot say whether
		// the page asking is current. The build identity does, and it covers
		// every change a kind cannot see: a component this one calls, an
		// external function, the render runtime itself.
		if r.Header.Get(o.buildHeader()) != o.buildID() {
			http.Error(w, "stale page", http.StatusConflict)
			return
		}
		if len(r.URL.RawQuery) > MaxQueryBytes {
			http.Error(w, "redraw arguments too large", http.StatusRequestURITooLong)
			return
		}
		fragment, err := component.Render(r, instance, r.URL.Query())
		if err != nil {
			http.Error(w, "invalid redraw arguments", http.StatusBadRequest)
			return
		}
		var out strings.Builder
		if err := htmlbind.Render(&out, fragment); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The URL alone identifies the response and the content is usually
		// per-user, so a shared cache must never hold it.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set(o.renderHeader(), modeRedraw+";v="+versionText)
		_, _ = w.Write([]byte(out.String()))
	})
}

const modeRedraw = "redraw"

// splitRedrawPath reads "<kind>/<instance>" and rejects anything else, so a
// missing or extra segment cannot be read as a valid target.
func splitRedrawPath(rest string) (kind, instance string, ok bool) {
	kind, instance, found := strings.Cut(rest, "/")
	if !found || kind == "" || instance == "" || strings.Contains(instance, "/") {
		return "", "", false
	}
	return kind, instance, true
}

// RedrawPath is the URL for one instance of a registered component, exposed so
// a test and a non-browser client can build exactly what the runtime does.
func (o Options) RedrawPath(kindID, instanceID string, values url.Values) string {
	path := o.pathPrefix() + "/redraw/" + url.PathEscape(kindID) + "/" + url.PathEscape(instanceID)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
