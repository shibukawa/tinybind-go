package httpbind

import "net/http"

// DefaultActionSelectorField is the hidden field a generated form carries to say
// which server function a native submit is for. It matches the template
// compiler's own default, and a project renaming one renames both.
const DefaultActionSelectorField = "_action"

// ActionSelector returns the server function selector a native form submit
// carried, or the empty string when it carried none.
//
// The query is read before the body because a submit button's formaction is what
// carries the selector when one form dispatches to several handlers, and that
// channel has to win over the form's own hidden field rather than merely coexist
// with it.
//
// The value is compared as one opaque key by the caller, so no mismatch between
// the hash half and the name half is representable.
func ActionSelector(r *http.Request, field string) string {
	if field == "" {
		field = DefaultActionSelectorField
	}
	if value := r.URL.Query().Get(field); value != "" {
		return value
	}
	return r.PostFormValue(field)
}

// DispatchAction runs one server function on the page's own POST route and
// applies the post-redirect-get default.
//
// A handler that writes nothing gets a 303 back to the page it was submitted
// from, so a reload does not resubmit and the address bar keeps showing the
// page. A handler that writes a status, a header, or a body keeps exactly that
// response, which is what lets it redirect elsewhere, render the page inline
// with validation errors, or stream.
//
// The direct entry point of a server function adds no redirect: there the
// handler's output is the response verbatim. The two entry points therefore
// differ in what a silent handler means, which is deliberate and documented,
// because only the form entry point has a page to go back to.
func DispatchAction(w http.ResponseWriter, r *http.Request, handler http.HandlerFunc) {
	observer := &actionResponse{ResponseWriter: w, headers: len(w.Header())}
	handler(observer, r)
	if observer.wrote() {
		return
	}
	http.Redirect(w, r, r.URL.RequestURI(), http.StatusSeeOther)
}

// actionResponse observes whether a handler produced a response of its own. It
// exists so a handler needs no flag and no framework type to choose between
// answering and letting the default redirect stand.
type actionResponse struct {
	http.ResponseWriter
	// headers is how many header keys were set before the handler ran, so a
	// header a middleware installed is not mistaken for one the handler wrote.
	headers   int
	status    bool
	bodyBytes bool
}

// wrote reports whether the handler produced anything. A header counts, because
// a handler setting one and returning has chosen its response as surely as one
// writing a body.
func (w *actionResponse) wrote() bool {
	return w.status || w.bodyBytes || len(w.ResponseWriter.Header()) != w.headers
}

func (w *actionResponse) WriteHeader(status int) {
	w.status = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *actionResponse) Write(b []byte) (int, error) {
	w.bodyBytes = true
	return w.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to [http.ResponseController], so a
// handler that flushes or sets a deadline reaches the real response rather than
// this observer.
func (w *actionResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush forwards to the underlying writer for a handler that streams through the
// older interface rather than through [http.ResponseController]. Flushing is a
// response, so it counts as one.
func (w *actionResponse) Flush() {
	w.bodyBytes = true
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
