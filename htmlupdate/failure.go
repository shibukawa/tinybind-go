package htmlupdate

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// FailureKind names why an update endpoint could not answer.
//
// The kind is what a caller branches on. The status and the message are
// defaults it may keep or replace, but the kind is the fact: a stale page and a
// failed render are the same 4xx-or-5xx to a proxy and completely different
// events to whoever is on call.
type FailureKind int

const (
	// FailureMalformedRequest is a redraw that named no component: the mode said
	// redraw and the kind or instance header was missing or empty.
	FailureMalformedRequest FailureKind = iota
	// FailureUnknownComponent is a kind this deployment does not publish.
	//
	// It is the ordinary version-skew signal: a page loaded before a deploy
	// asks for a component whose markup has since changed, gets a 404, and
	// reloads. A sustained rate of it after a deploy has settled means
	// something else.
	FailureUnknownComponent
	// FailureArgumentsTooLarge is a query past the configured bound.
	FailureArgumentsTooLarge
	// FailureInvalidArguments is a query the generated decoder refused.
	FailureInvalidArguments
	// FailureRenderFailed is a component that could not render.
	FailureRenderFailed
)

// String names the kind for a log line or a span attribute.
func (k FailureKind) String() string {
	switch k {
	case FailureMalformedRequest:
		return "malformed_request"
	case FailureUnknownComponent:
		return "unknown_component"
	case FailureArgumentsTooLarge:
		return "arguments_too_large"
	case FailureInvalidArguments:
		return "invalid_arguments"
	case FailureRenderFailed:
		return "render_failed"
	}
	return "unknown"
}

// Failure is one request an update endpoint could not answer.
//
// This package has to write a response, because it owns the endpoint. It does
// not have to decide what a failure looks like, which is why the whole value
// reaches the caller instead of a status and a line of plain text reaching the
// client. A caller with problem responses, its own error pages, a
// request-scoped logger, or a tracer sees every one of these.
type Failure struct {
	// Kind is why the request was refused.
	Kind FailureKind
	// Status is the response status this package would have written.
	Status int
	// Message is the plain-text body this package would have written. It never
	// contains anything the request supplied, so it is safe to send as is.
	Message string
	// Err is the underlying cause, when there is one. A decoder rejection and a
	// render failure carry theirs; a stale page has none, because nothing
	// failed.
	//
	// It may name internal detail, so it belongs in a log rather than in a
	// response body.
	Err error
	// KindID and InstanceID name what was asked for, when the path was
	// well-formed enough to say. Both are attacker-supplied, so treat them as
	// untrusted when they reach a log.
	KindID     string
	InstanceID string
}

// Error makes a Failure usable wherever an error is, so a caller can hand it
// straight to a logger or a span.
func (f Failure) Error() string {
	if f.Err != nil {
		return "htmlupdate: " + f.Kind.String() + ": " + f.Err.Error()
	}
	return "htmlupdate: " + f.Kind.String()
}

// Unwrap exposes the cause, so errors.Is and errors.As reach it.
func (f Failure) Unwrap() error { return f.Err }

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
func FailureResponse(failure Failure) Response {
	return Response{
		Status:  failure.Status,
		Header:  http.Header{"Content-Type": []string{"application/problem+json"}},
		Body:    failure.problemJSON(),
		Failure: &failure,
	}
}

// failure is FailureResponse for the entries, which also report the refusal
// through the observation hook when a caller installed one.
func (o Options) failure(r *http.Request, f Failure) Response {
	if o.OnFailure != nil {
		o.OnFailure(r, f)
	}
	return FailureResponse(f)
}

// problemBody is the RFC 9457 shape, matching what httpbind.WriteError emits so
// one error format reaches a client whichever entry refused the request.
type problemBody struct {
	Type   string        `json:"type"`
	Title  string        `json:"title"`
	Status int           `json:"status"`
	Detail string        `json:"detail"`
	Code   string        `json:"code"`
	Errors []problemItem `json:"errors,omitempty"`
}

type problemItem struct {
	Field    string `json:"field"`
	Location string `json:"location"`
	Message  string `json:"message"`
}

func (f Failure) problemJSON() []byte {
	body := problemBody{
		Type:   "about:blank",
		Title:  http.StatusText(f.Status),
		Status: f.Status,
		// Message is a constant this package chose, never anything the request
		// supplied, so it needs no blanking at 5xx the way a wrapped cause would.
		// Err stays out entirely: it may name internal detail and belongs in the
		// caller's log through the failure value, not in a response.
		Detail: f.Message,
		// The kind is the fact a caller branches on, so it travels as the code
		// rather than being left for someone to infer from the status.
		Code: f.Kind.String(),
	}
	var refused *QueryError
	if errors.As(f.Err, &refused) {
		body.Errors = []problemItem{{
			Field:    refused.Parameter,
			Location: "query",
			Message:  refused.Reason,
		}}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		// Every field is a string or an int, so this cannot happen; falling back
		// to the status alone keeps a refusal a refusal if it ever does.
		return []byte(`{"type":"about:blank","status":` + strconv.Itoa(f.Status) + `}`)
	}
	return encoded
}

// Validate reports every option this package cannot use, so a caller running a
// startup validation pass hears about all of them at once rather than finding
// the first one in a browser.
//
// Nothing calls it automatically: an Options value is a struct literal, so
// there is no constructor to fail in, and a handler is the wrong place to
// discover a configuration mistake.
func (o Options) Validate() error {
	var problems []error
	if err := validateNamePrefix("data attribute prefix", o.dataAttributePrefix()); err != nil {
		problems = append(problems, err)
	}
	if o.HeaderPrefix != "" && strings.ContainsAny(o.HeaderPrefix, " \t\r\n:") {
		problems = append(problems, errors.New(
			"htmlupdate: header prefix "+strconv.Quote(o.HeaderPrefix)+" must not contain whitespace or a colon"))
	}
	if o.MaxManifestBytes < 0 {
		problems = append(problems, errors.New("htmlupdate: MaxManifestBytes must not be negative"))
	}
	if o.MaxQueryBytes < 0 {
		problems = append(problems, errors.New("htmlupdate: MaxQueryBytes must not be negative"))
	}
	// Who serves the browser runtime has to be answered, because the wrong
	// answer is invisible at run time. A deployment that serves none and owns
	// none compiles, starts, renders every page correctly, and then does nothing
	// when a boundary should update: no error, no log line, no failed request.
	// Every other setting here is wrong in a way somebody notices.
	switch {
	case o.ServeRuntime && o.CallerOwnsRuntime:
		problems = append(problems, errors.New(
			"htmlupdate: ServeRuntime and CallerOwnsRuntime are both set; a document carrying two runtimes has"+
				" two boundary id spaces and two build identities, so set exactly one"))
	case !o.ServeRuntime && !o.CallerOwnsRuntime:
		problems = append(problems, errors.New(
			"htmlupdate: neither ServeRuntime nor CallerOwnsRuntime is set, so no browser runtime reaches the page"+
				" and every update silently does nothing; set ServeRuntime to serve the reference client, or"+
				" CallerOwnsRuntime if you ship your own (RuntimeSource returns the bytes to merge)"))
	}
	return errors.Join(problems...)
}

// validateNamePrefix rejects a prefix that cannot name both an attribute and an
// element.
//
// It is stricter than the generator's attribute-only rule in one place: a
// custom element name must start with a lowercase letter, while a data
// attribute may start with a digit. The prefix now names the placeholder
// element too, so the stricter rule is the one that applies.
func validateNamePrefix(what, prefix string) error {
	if prefix == "" {
		return errors.New("htmlupdate: " + what + " must not be empty")
	}
	if prefix[0] < 'a' || prefix[0] > 'z' {
		return errors.New("htmlupdate: " + what + " " + strconv.Quote(prefix) +
			" must start with a lowercase letter, because it also names the placeholder element")
	}
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return errors.New("htmlupdate: " + what + " " + strconv.Quote(prefix) +
				" must use lowercase letters, digits, and hyphens")
		}
	}
	if strings.HasSuffix(prefix, "-") {
		return errors.New("htmlupdate: " + what + " " + strconv.Quote(prefix) +
			" must not end with a hyphen")
	}
	return nil
}

// fail reports one failure through the caller's hook, or writes the default.
