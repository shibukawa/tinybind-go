package updatecore

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
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

// NotModified reports whether the request already holds this answer, by
// comparing its If-None-Match against the entity tag this response carries.
//
// Answering it is the caller's: a 304 is a cache policy decision, and this
// package no longer makes one. A response with no entity tag is never a match.
func (resp Response) NotModified(r Reader) bool {
	etag := resp.Header.Get("ETag")
	return etag != "" && MatchesETag(r.Header("If-None-Match"), etag)
}

// Headers is what a response to this request must carry, for the entries that
// write their body directly and therefore need their headers set first.
//
// It is computable before anything renders: the Vary axes come from which
// request headers this package reads, the content type from the mode negotiated,
// and the live marker from the composition. Pass the wrappers and leaf a render
// entry will be given; pass none and the live marker is omitted.
func (o Options) Headers(r Reader, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	negotiated := o.Negotiate(r)
	header := http.Header{}
	for _, name := range o.VaryOn(negotiated.Mode) {
		header.Add("Vary", name)
	}
	header.Set("Content-Type", o.ContentTypeFor(negotiated.Mode))
	if negotiated.Mode != ModeDocument {
		header.Set(o.RenderHeader(), RenderToken(negotiated.Mode, negotiated.Version))
	}
	if leaf.Present() && htmlbind.HasLiveBlock(wrappers, leaf) {
		header.Set(o.LiveHeader(), "1")
	}
	return header
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
func (o Options) RedrawHeaders(r Reader) http.Header {
	header := http.Header{}
	for _, name := range o.VaryOn(ModeRedraw) {
		header.Add("Vary", name)
	}
	return header
}

// VaryOn names the request headers a response in this mode depends on.
//
// The render and build headers are always there: without the first a shared
// cache can hand a delta body to a browser asking for a page, and without the
// second it can hand one build's markup to another build's client. A redraw and
// a sequence read more, and a page and its redraw share a URL, so two components
// redrawing on one page would otherwise be a single cache entry.
func (o Options) VaryOn(mode Mode) []string {
	switch mode {
	case ModeRedraw:
		return []string{o.RenderHeader(), o.BuildHeader(), o.KindHeader(), o.InstanceHeader()}
	case ModeSequence:
		return []string{o.RenderHeader(), o.SequenceAddressHeader()}
	default:
		return []string{o.RenderHeader(), o.BuildHeader()}
	}
}

// ContentTypeFor names the body a buffered entry writes in each mode.
//
// A live request reaching a buffered entry is answered with the document, since
// that entry cannot hold a delivery stream open, so it takes the document type
// rather than the stream one. StreamHeaders is what an entry that can hold one
// uses instead.
func (o Options) ContentTypeFor(mode Mode) string {
	switch mode {
	case ModeNavigation, ModeRedraw, ModeSequence:
		return "application/json; charset=utf-8"
	default:
		return "text/html; charset=utf-8"
	}
}

// StreamHeaders is Headers for a streamed navigation, whose body is a record
// stream rather than one of the buffered shapes.
//
// A live request reaching an entry that does not hold subscriptions open is
// answered as a navigation and terminated, so the echo says navigation: what a
// response claims to be has to be what it is, or a proxy substitution stops
// being detectable.
func (o Options) StreamHeaders(r Reader, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.streamHeaders(r, wrappers, leaf, false)
}

// LiveHeaders is StreamHeaders for the entry that does hold subscriptions open,
// so a live request keeps the live mode rather than being downgraded.
func (o Options) LiveHeaders(r Reader, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment) http.Header {
	return o.streamHeaders(r, wrappers, leaf, true)
}

func (o Options) streamHeaders(r Reader, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, servesLive bool) http.Header {
	header := o.Headers(r, wrappers, leaf)
	negotiated := o.Negotiate(r)
	switch negotiated.Mode {
	case ModeNavigation:
		header.Set("Content-Type", o.StreamMediaType())
	case ModeLive:
		header.Set("Content-Type", o.StreamMediaType())
		if !servesLive {
			header.Set(o.RenderHeader(), RenderToken(ModeNavigation, negotiated.Version))
		}
	}
	return header
}
