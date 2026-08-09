package htmlupdate

import "github.com/shibukawa/tinybind-go/internal/updatecore"

// FailureResponse is the refusal this package computes, exported so a caller
// raising one of its own — a redraw it declined before this package saw it —
// answers in the same shape rather than reimplementing five status codes.
//
// The body is RFC 9457 problem details, which is this module's documented error
// format everywhere else; the update endpoints were the only paths writing
// plain text. The media type is what tells the two apart on the wire:
// application/json is an update to apply, including a non-2xx one, and
// application/problem+json is a request that produced no update at all.
//
// The status still directs. A client's rule — any non-2xx falls back to an
// ordinary navigation — is unchanged, so a client that cannot read the body
// still lands correctly; the body adds diagnosis rather than direction.
//
// Nothing is sent until a caller sends it: [Response.WriteTo] does that, and a
// caller with its own error page sends that instead.
//
// It carries no Vary, having no Options and no request to compute one from. The
// refusals this package produces get theirs added; a caller raising one of its
// own adds them from [Options.RedrawHeaders] or [Options.Headers], and a
// cacheable status makes that matter.
func FailureResponse(failure Failure) Response {
	return Response(updatecore.FailureResponse(failure))
}
