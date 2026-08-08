package htmlupdate

import (
	"net/http"
	"strings"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// Update is one region an action response rewrites.
//
// TargetID must match the id the rendered root element carries, because the
// browser locates the region by that id and the replacement has to keep it.
type Update struct {
	TargetID string
	Fragment htmlbind.Fragment
}

// Replace pairs a target element id with the fragment that replaces it.
func Replace(targetID string, fragment htmlbind.Fragment) Update {
	return Update{TargetID: targetID, Fragment: fragment}
}

// WantsUpdate reports whether the caller can apply an update response.
//
// An ordinary form submission cannot, so a handler branches on this and
// redirects instead, which is what keeps a page working without JavaScript.
func (o Options) WantsUpdate(r *http.Request) bool {
	mode, _, ok := parseRender(r.Header.Get(o.renderHeader()))
	if !ok || mode != modeAction {
		return false
	}
	// A page from another build cannot be handed regions this one rendered. It
	// is the only compatibility axis this package judges, because the client
	// belongs to the caller and so does its wire version.
	return r.Header.Get(o.buildHeader()) == o.buildID()
}

const modeAction = "action"

// WriteUpdate answers a mutating request with the regions it changed, so one
// round trip both performs the action and refreshes the page.
//
// The body is the same shape a redraw returns, so the browser applies it with
// the same code. Unlike a redraw this request is not idempotent: it carries
// ambient credentials, so it needs CSRF protection, and its response is never
// cacheable.
//
// options reach every region's render. The token one matters most here: a region
// holding an unsafe form emits a CSRF field, and without a token that render
// fails outright — which is this entry's own headline case, since rewriting a
// form with its validation errors is what it exists for.
func (o Options) WriteUpdate(w http.ResponseWriter, r *http.Request, updates []Update, options ...htmlbind.Option) error {
	return o.WriteUpdateStatus(w, r, http.StatusOK, updates, options...)
}

// WriteUpdateStatus is WriteUpdate with an explicit status, so a failed
// validation can return 422 and still rewrite the form region with its errors.
//
// The browser applies an update response whatever the status says, because
// rendering the failure is the point.
func (o Options) WriteUpdateStatus(w http.ResponseWriter, r *http.Request, status int, updates []Update, options ...htmlbind.Option) error {
	body := deltaResponse{}
	// An action can reveal a component the document never carried: a validation
	// summary, a panel that was not there before. Its stylesheet is not in the
	// live head, and markup landing before the sheet does is the flash of
	// unstyled content the navigation delta added this field to prevent.
	seen := map[string]bool{}
	// Resolved once rather than per region: the options are the same for every
	// one, and the request's context leads so a caller may still override it.
	render := o.renderOptions(append([]htmlbind.Option{htmlbind.WithContext(r.Context())}, options...))
	for _, update := range updates {
		var out strings.Builder
		if err := htmlbind.Render(&out, update.Fragment, render...); err != nil {
			// Nothing is written yet, so the caller can still choose an error
			// response.
			return err
		}
		for _, tag := range update.Fragment.Head() {
			// Two regions declaring one stylesheet emit one tag, which is the
			// htmlbind.MergeHead rule applied across the written set.
			if tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			body.Head = append(body.Head, tag)
		}
		body.Operations = append(body.Operations, deltaOperation{
			Kind: delta.OpReplace, ID: update.TargetID, HTML: out.String(),
		})
	}
	return o.writeActionBody(w, status, body)
}

// WriteNavigate tells the browser to leave the page, which is how an action
// that changed where the user belongs stays correct without guessing which
// regions to rewrite.
func (o Options) WriteNavigate(w http.ResponseWriter, url string) error {
	return o.writeActionBody(w, http.StatusOK, deltaResponse{Navigate: url})
}

func (o Options) writeActionBody(w http.ResponseWriter, status int, body deltaResponse) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(o.renderHeader(), modeAction)
	w.WriteHeader(status)
	return encodeJSON(w, body)
}
