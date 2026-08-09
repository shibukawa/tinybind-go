package htmlupdate

import "net/http"

// Sequence answers a request for one sequence tree, and reports whether it did.
//
// It is the entry a caller branches on inside its own handler, exactly as Redraw
// is, so the address a client asks at is the caller's to choose and this package
// mounts nothing:
//
//	func page(w http.ResponseWriter, r *http.Request) {
//		if answer, ok := options.Sequence(r); ok {
//			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
//			_, _ = answer.WriteTo(w)
//			return
//		}
//		// ordinary page render
//	}
//
// The cache policy is the caller's, and a sequence is the one answer here that
// may be public and held forever: it is addressed by a digest of its own
// content, so a template edit produces a new address rather than a new body at
// the old one, and nothing needs invalidating.
//
// An address this process has never rendered is answered 404, and a client falls
// back to asking for the assembled form it can always be sent instead. That is
// the whole recovery path: a sequence is an optimisation over markup that is
// still available, never a thing a screen depends on.
func (o Options) Sequence(r *http.Request) (Response, bool) {
	resp, answered := o.core().Sequence(reader(r))
	return Response(resp), answered
}
