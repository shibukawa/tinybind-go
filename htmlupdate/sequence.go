package htmlupdate

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// A sequence is the static half of a fragment, and the only thing on this wire
// that is not per user: it derives from the template rather than from a request.
// That is why it travels as its own response — it is the one thing here that can
// be public, immutable, and held by a shared cache, and riding it inside a
// private delta would forfeit exactly that.

const modeSequence = "sequence"

// sequenceAddressHeader names the address a sequence request asks for.
//
// It spells out Address rather than sitting one letter from the capability
// header beside it: a pair reading -Sequences and -Sequence is two headers a
// reader tells apart by counting characters, which is the kind of naming that
// produces a bug nobody can see in a diff.
func (o Options) sequenceAddressHeader() string { return o.prefix() + "-Sequence-Address" }

// DefaultSequenceCacheControl keeps a sequence forever. It is addressed by a
// digest of its own content, so a deploy that changes a template produces a new
// address rather than a new body at the old one, and nothing needs invalidating.
const DefaultSequenceCacheControl = "public, max-age=31536000, immutable"

// Sequence answers a request for one sequence tree, and reports whether it did.
//
// It is the entry a caller branches on inside its own handler, exactly as Redraw
// is, so the address a client asks at is the caller's to choose and this package
// mounts nothing:
//
//	func page(w http.ResponseWriter, r *http.Request) {
//		if options.Sequence(w, r) {
//			return
//		}
//		// ordinary page render
//	}
//
// An address this process has never rendered is answered 404, and a client falls
// back to asking for the assembled form it can always be sent instead. That is
// the whole recovery path: a sequence is an optimisation over markup that is
// still available, never a thing a screen depends on.
func (o Options) Sequence(w http.ResponseWriter, r *http.Request) bool {
	if o.Negotiate(r).Mode != ModeSequence {
		return false
	}
	address := r.Header.Get(o.sequenceAddressHeader())
	if address == "" {
		o.fail(w, r, Failure{
			Kind:    FailureMalformedRequest,
			Status:  http.StatusBadRequest,
			Message: "sequence request names no address",
		})
		return true
	}
	sequence, known := htmlbind.LookupSequence(address)
	if !known {
		// A sequence is registered when the plan behind it renders, so an
		// address this process has not rendered is one this process cannot
		// describe. Answering not-found rather than guessing is what keeps the
		// client's fallback — ask for markup instead — the only recovery needed.
		o.fail(w, r, Failure{
			Kind:    FailureUnknownComponent,
			Status:  http.StatusNotFound,
			Message: notFoundMessage,
		})
		return true
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", o.sequenceCacheControl())
	w.Header().Set(o.renderHeader(), modeSequence)
	_, _ = w.Write(sequence.AppendJSON(nil))
	return true
}

func (o Options) sequenceCacheControl() string {
	if o.SequenceCacheControl == "" {
		return DefaultSequenceCacheControl
	}
	return o.SequenceCacheControl
}
