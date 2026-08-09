//go:build !fasthttp

package app

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// The partial-update entries in a handler, which is the shape the whole
// read-only half exists for: branch on what the request asked for, compute an
// answer, send it.
//
// The Options value is built here rather than read from a package-level var on
// purpose. A var is a declaration and not a function, so the transform does not
// rewrite it and the tag excludes the file it lives in; a deployment wanting one
// declares it in a tagged file pair, one per backend. Everything else on this
// page is the same value on both backends, because only Options and Response are
// redeclared per transport and the rest are shared.
func updateAction(w http.ResponseWriter, r *http.Request) {
	options := htmlupdate.Options{Key: []byte("k"), CallerOwnsRuntime: true}
	if !options.WantsUpdate(r) {
		// A submission that cannot apply an update response is answered with a
		// redirect, which is what keeps the page working without JavaScript.
		htmlupdate.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}
	if err := options.VerifyCSRF(r, "session-token"); err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	answer, err := options.WriteUpdate(r, []htmlupdate.Update{
		htmlupdate.Replace("cart", htmlbind.Fragment{}),
	})
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_, _ = answer.WriteTo(w)
}

// updatePage is the branching page handler the guides show: the redraw first,
// the sequence beside it. It also exercises the half a caller has to write
// itself — the conditional answer, and a header set applied to a response it is
// sending some other way.
//
// The document render behind the branches is not here, because it has no
// counterpart yet: the render and stream entries write through the response as
// they go, so a handler calling one is refused with its occurrence named.
func updatePage(w http.ResponseWriter, r *http.Request) {
	options := htmlupdate.Options{Key: []byte("k"), CallerOwnsRuntime: true}
	// The registry is the same value on both backends, so a helper building one
	// needs no tag and no second copy.
	registry := redrawRegistry()

	htmlupdate.ApplyTo(options.RedrawHeaders(r), w)
	if answer, ok := options.Redraw(r, registry); ok {
		if answer.NotModified(r) {
			htmlupdate.ApplyTo(answer.Header, w)
			return
		}
		_, _ = answer.WriteTo(w)
		return
	}
	if answer, ok := options.Sequence(r); ok {
		_, _ = answer.WriteTo(w)
		return
	}
	if options.Negotiate(r).Mode == htmlupdate.ModeDocument {
		htmlupdate.ApplyTo(options.Headers(r, nil, htmlbind.Fragment{}), w)
	}
}

// updateFeed is the streaming half. Both entries take a callback rather than
// handing a stream back, which is what makes this body the same text on either
// transport: only the signature line moves.
func updateFeed(w http.ResponseWriter, r *http.Request) {
	options := htmlupdate.Options{Key: []byte("k"), CallerOwnsRuntime: true}
	htmlupdate.ApplyTo(options.StreamHeaders(r, nil, htmlbind.Fragment{}), w)
	options.WriteStream(w, r, []string{"<title>Feed</title>"}, func(stream *htmlupdate.DeltaStream) error {
		stream.Replace("feed", `<main id="feed">one</main>`, htmlupdate.ManifestEntry{Frame: "f1"})
		return nil
	})
}

// updateLive holds the response open. The cancellation context is the caller's
// and is not a transport value, so it survives the rewrite — on fasthttp it is
// the only thing that can bound a delivery, because the handler has returned by
// the time records are written.
func updateLive(w http.ResponseWriter, r *http.Request) {
	options := htmlupdate.Options{Key: []byte("k"), CallerOwnsRuntime: true}
	htmlupdate.ApplyTo(options.LiveHeaders(r, nil, htmlbind.Fragment{}), w)
	if err := options.RenderLiveStream(r.Context(), w, r, nil, htmlbind.Fragment{}); err != nil {
		httpbind.WriteError(w, r, err)
	}
}
