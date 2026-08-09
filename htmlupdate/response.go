package htmlupdate

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/internal/updatecore"
)

// This package writes bytes and nothing else. It sets no header and no status on
// a caller's response.
//
// What it knows and a caller cannot derive — which request headers a response
// depends on, what its body is, which mode was served, what the body digests to
// — it computes and hands over. What a deployment decides — the cache policy,
// whether to answer a conditional request, what a failure looks like — it never
// had any business writing.
//
// Splitting it that way is what makes a wrong header traceable to one place. It
// also costs something real, and the cost is stated rather than hidden: Vary is
// a correctness control, not a preference, and a response that loses it can be
// handed by a shared cache to a browser asking for a page. Headers and Response
// both compute it; ApplyTo and WriteTo both write it; a caller that does neither
// has decided to, rather than forgotten.

// Response is a complete answer this package computed and did not send.
//
// It exists for the entries whose headers depend on the body — a redraw digests
// what it rendered — because such an entry cannot both write the body and leave
// the headers alone. The buffered entries return one; the streaming entries take
// their headers from Headers instead, since a stream commits before its first
// record.
type Response struct {
	// Status is what this answer means. A caller may serve another, and an
	// update response is applied by a client whatever the status says, which is
	// how a rejected form returns 422 and still rewrites its own region.
	Status int
	// Header is what the response has to carry: the Vary axes it depends on, its
	// content type, the served mode echoed back, and an entity tag where the
	// answer has one. It carries no cache policy, which is the caller's.
	Header http.Header
	// Body is the bytes to write.
	Body []byte
	// Failure is why this answer is a refusal, or nil for an ordinary one.
	//
	// It travels here rather than through a hook that writes: a caller that only
	// wants to observe logs it and sends the response as it stands, and one that
	// wants its own error page reads the kind and writes that instead. Either way
	// nothing is written until the caller writes it, which is the whole of this
	// package's position on responses.
	Failure *Failure
}

// WriteTo sends the response: its headers, its status, and its body.
//
// It is here so the common case is one call and an omission is a decision. A
// caller wanting its own policy sets it on w before calling, or copies Header
// itself and writes Body.
func (resp Response) WriteTo(w http.ResponseWriter) (int64, error) {
	header := w.Header()
	for name, values := range resp.Header {
		for _, value := range values {
			header.Add(name, value)
		}
	}
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	written, err := w.Write(resp.Body)
	return int64(written), err
}

// ApplyTo copies a computed header set onto a response, adding rather than
// replacing so a caller's own values survive.
func ApplyTo(header http.Header, w http.ResponseWriter) {
	target := w.Header()
	for name, values := range header {
		for _, value := range values {
			target.Add(name, value)
		}
	}
}

// NotModified reports whether the request already holds this answer, by
// comparing its If-None-Match against the entity tag this response carries.
//
// Answering it is the caller's: a 304 is a cache policy decision, and this
// package no longer makes one. A response with no entity tag is never a match.
func (resp Response) NotModified(r *http.Request) bool {
	return updatecore.Response(resp).NotModified(reader(r))
}

// Headers is what a response to this request must carry, for the entries that
// write their body directly and therefore need their headers set first.
//
// It is computable before anything renders: the Vary axes come from which
// request headers this package reads, the content type from the mode negotiated,
// and the live marker from the composition. Pass the wrappers and leaf a render
// entry will be given; pass none and the live marker is omitted.
func (o Options) Headers(r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.core().Headers(reader(r), wrappers, leaf)
}

// RedrawHeaders names the Vary axes a URL that answers redraws depends on,
// whichever way this request turns out.
//
// A page handler declares them before it branches: a page and the redraws of the
// components on it share one URL, so a cache that learned only the page would
// answer a redraw from it, and one that learned only one component's redraw
// would answer another component with it. The axes have to be on the response
// whether or not this request was a redraw, which is why they are computable
// without deciding.
func (o Options) RedrawHeaders(r *http.Request) http.Header {
	return o.core().RedrawHeaders(reader(r))
}

// StreamHeaders is Headers for a streamed navigation, whose body is a record
// stream rather than one of the buffered shapes.
//
// A live request reaching an entry that does not hold subscriptions open is
// answered as a navigation and terminated, so the echo says navigation: what a
// response claims to be has to be what it is, or a proxy substitution stops
// being detectable.
func (o Options) StreamHeaders(r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.core().StreamHeaders(reader(r), wrappers, leaf)
}

// LiveHeaders is StreamHeaders for the entry that does hold subscriptions open,
// so a live request keeps the live mode rather than being downgraded.
func (o Options) LiveHeaders(r *http.Request, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.core().LiveHeaders(reader(r), wrappers, leaf)
}
