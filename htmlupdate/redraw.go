package htmlupdate

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
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
	// Head is what this component contributes to a document head: the merged,
	// ready-to-write tags of the component and everything it calls.
	//
	// A redraw rewrites a region of a page this endpoint never rendered, so
	// unlike a navigation it cannot merge into a head it owns. Publishing the
	// contribution is what lets a caller put it in the document shell before
	// anything is swapped, which is the only way nothing is fetched mid-swap.
	Head []string
	// Assets names the static files this component requires. It is Head read as
	// identity rather than as markup, which is what a caller needs to decide
	// whether the page already carries them.
	Assets []htmlbind.Asset
}

// Registry holds the components a deployment publishes for redraw.
//
// Nothing is registered implicitly. Being exported, single-rooted, and
// renderable is not enough, because publishing an endpoint must be deliberate.
type Registry struct {
	kinds map[string]Reloadable
	// order keeps registration order, so the required set a caller reads back is
	// the same on every run. A map walk would make a document shell's head
	// change between two starts of the same binary.
	order []string
}

// Register adds a component to the redraw surface.
//
// A repeated kind is refused rather than overwritten. The kind covers a
// component's name, parameters, and compiled markup but not its package, so two
// identical templates in different packages produce the same one; silently
// keeping the last registration would then serve a component that looks the
// same but calls its own package's external functions.
//
// This must fail at startup, and failing at startup is not the same as
// panicking: a caller running its own startup validation pass collects what is
// wrong and reports all of it, rather than aborting the process on the first
// one. So the failure is returned, and a caller that wants the abort still has
// it one line away.
func (reg *Registry) Register(component Reloadable) error {
	if component.KindID == "" {
		return errors.New("htmlupdate: reloadable component has no kind")
	}
	if reg.kinds == nil {
		reg.kinds = map[string]Reloadable{}
	}
	if _, taken := reg.kinds[component.KindID]; taken {
		return errors.New("htmlupdate: two components registered as " + component.KindID +
			"; the kind covers name, parameters, and markup but not the package, so rename one or change its markup")
	}
	reg.kinds[component.KindID] = component
	reg.order = append(reg.order, component.KindID)
	return nil
}

// RequiredHead is every registered component's head contribution, deduplicated
// and in registration order.
//
// It is what a caller puts in its document shell. A redraw addresses a region on
// a page this endpoint did not render, so it can only swap markup into a head
// somebody else already wrote; a component whose stylesheet is not there renders
// unstyled, and nothing about the swap can repair that afterwards.
//
// Reading it needs no request and no render, so a document shell built once at
// startup covers every redraw the deployment will ever serve.
func (reg *Registry) RequiredHead() []string {
	var merged []string
	seen := map[string]bool{}
	for _, kind := range reg.order {
		for _, tag := range reg.kinds[kind].Head {
			if tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			merged = append(merged, tag)
		}
	}
	return merged
}

// RequiredAssets is every registered component's required files, deduplicated by
// identity and in registration order. It is RequiredHead read as identity rather
// than as markup, for a caller deciding where each file is served.
func (reg *Registry) RequiredAssets() []htmlbind.Asset {
	var merged []htmlbind.Asset
	seen := map[string]bool{}
	for _, kind := range reg.order {
		for _, asset := range reg.kinds[kind].Assets {
			if asset.ID == "" || seen[asset.ID] {
				continue
			}
			seen[asset.ID] = true
			merged = append(merged, asset)
		}
	}
	return merged
}

// A redraw's head used to travel in a header, packed as base64 of JSON, and was
// bounded at startup so a proxy could not drop it. Both are gone: the body is
// the shared JSON shape now, and a head field in a body needs no packing and no
// bound. Registry.RequiredHead is unaffected and is still what a deployment puts
// in its document shell, which is the only way nothing is fetched mid-swap.

// MustRegister is Register for a caller with nowhere to return an error, such
// as a package-level registry value.
func (reg *Registry) MustRegister(component Reloadable) {
	if err := reg.Register(component); err != nil {
		panic(err.Error())
	}
}

// DefaultMaxQueryBytes bounds the arguments a redraw may carry, since a GET
// puts every one of them in the URL. Options.MaxQueryBytes overrides it, for a
// deployment behind a proxy with its own URL limit.
const DefaultMaxQueryBytes = 4 << 10

// notFoundMessage is what http.NotFound writes, kept as a value so a caller
// that takes over the response can still reproduce the default body.
const notFoundMessage = "404 page not found"

// Redraw answers a redraw request at whatever URL the caller serves it from,
// and reports whether it did.
//
// It is the entry a caller branches on inside its own page handler:
//
//	func page(w http.ResponseWriter, r *http.Request) {
//		if options.Redraw(w, r, registry) {
//			return
//		}
//		// ordinary page render
//	}
//
// Addressing it at the page's own URL is the point. Path protection is
// configured by path pattern, so a redraw on a reserved path needs its own
// pattern maintained in parallel with the one protecting the page the component
// sits on — two rules that must agree and that nothing forces to agree. At the
// page URL the redraw inherits that protection automatically, and placed after
// the handler's own checks it inherits those too, not merely the middleware's.
//
// A request that is not a redraw returns false with nothing written, including a
// request from a page another build rendered: at a page URL the right answer to
// a stale redraw is that page, which the caller is about to render anyway, and
// that costs a reload rather than a refusal and then a reload.
//
// options reach the component's render, so a redraw sees the same cache store,
// URL scheme policy, and CSRF token the page render was given. Without them a
// component renders one way inside its page and another in the response that
// replaces it — and one containing an unsafe form does not render at all, since
// [htmlbind.Builder.CSRFField] needs a token. The boundary prefix and the build
// identity are supplied from these Options and do not need passing.
func (o Options) Redraw(w http.ResponseWriter, r *http.Request, reg *Registry, options ...htmlbind.Option) bool {
	// The page response and the redraw response share a URL, so the cache keys
	// that tell them apart must be declared whichever one this turns out to be.
	// Without the kind and instance here, two components redrawing on one page
	// would be one cache entry and either could be answered with the other's
	// markup.
	w.Header().Add("Vary", o.renderHeader())
	w.Header().Add("Vary", o.buildHeader())
	w.Header().Add("Vary", o.kindHeader())
	w.Header().Add("Vary", o.instanceHeader())
	if o.Negotiate(r).Mode != ModeRedraw {
		return false
	}
	kind := r.Header.Get(o.kindHeader())
	instance := r.Header.Get(o.instanceHeader())
	if kind == "" || instance == "" {
		o.fail(w, r, Failure{
			Kind:       FailureMalformedRequest,
			Status:     http.StatusBadRequest,
			Message:    "redraw names no component",
			KindID:     kind,
			InstanceID: instance,
		})
		return true
	}
	component, known := reg.kinds[kind]
	if !known {
		o.fail(w, r, Failure{
			Kind:       FailureUnknownComponent,
			Status:     http.StatusNotFound,
			Message:    notFoundMessage,
			KindID:     kind,
			InstanceID: instance,
		})
		return true
	}
	o.writeRedraw(w, r, component, kind, instance, options)
	return true
}

// writeRedraw renders one instance and writes the response. Both entries reach
// it with the target resolved and the build already settled their own way.
func (o Options) writeRedraw(w http.ResponseWriter, r *http.Request, component Reloadable, kind, instance string, options []htmlbind.Option) {
	if len(r.URL.RawQuery) > o.maxQueryBytes() {
		o.fail(w, r, Failure{
			Kind:       FailureArgumentsTooLarge,
			Status:     http.StatusRequestURITooLong,
			Message:    "redraw arguments too large",
			KindID:     kind,
			InstanceID: instance,
		})
		return
	}
	fragment, err := component.Render(r, instance, r.URL.Query())
	if err != nil {
		o.fail(w, r, Failure{
			Kind:       FailureInvalidArguments,
			Status:     http.StatusBadRequest,
			Message:    "invalid redraw arguments",
			Err:        err,
			KindID:     kind,
			InstanceID: instance,
		})
		return
	}
	// A redraw answers with the region the request named, so the component has to
	// be addressable at that id. Generated code guarantees it — a reloadable
	// component is an update boundary and takes its id from a declared parameter
	// — and a registration assembled by hand can get it wrong, where the failure
	// would otherwise be a response with no operations in it.
	if got := fragment.InstanceID(); got != instance {
		o.fail(w, r, Failure{
			Kind:       FailureRenderFailed,
			Status:     http.StatusInternalServerError,
			Message:    "component is not addressable at the requested instance",
			Err:        errors.New("htmlupdate: " + kind + " rendered as instance " + strconv.Quote(got) + ", want " + strconv.Quote(instance)),
			KindID:     kind,
			InstanceID: instance,
		})
		return
	}
	// The request's context goes in ahead of the caller's options, so a shared
	// cache store and a context-taking external see this request's cancellation.
	// The caller may still override it, since its own options come last.
	render := o.renderOptions(append([]htmlbind.Option{htmlbind.WithContext(r.Context())}, options...))
	// A redraw returns the same decomposition every other update path returns:
	// the component's own fragment, a hole where each nested boundary sits, and
	// a fragment for each of those. An empty known manifest is what makes every
	// one of them travel, which is right — the caller asked for this region and
	// holds nothing of what is about to replace it.
	diff, err := delta.RenderDelta(o.Key, delta.Manifest{}, nil, fragment, render...)
	if err != nil {
		o.fail(w, r, Failure{
			Kind:       FailureRenderFailed,
			Status:     http.StatusInternalServerError,
			Message:    "render failed",
			Err:        err,
			KindID:     kind,
			InstanceID: instance,
		})
		return
	}
	// The manifest is what a redraw used to leave stale. A reloadable component
	// is an update boundary, so the client held a validator the page render gave
	// it and this replacement made it wrong; returning the new one is what keeps
	// the next navigation delta from re-sending a region that is already right.
	// The head is the component's published contribution rather than the render's
	// merged one, which is what Reloadable.Head documents and what
	// Registry.RequiredHead aggregates: a redraw rewrites a region of a page this
	// endpoint never rendered, so it can only name what its component needs.
	response := deltaResponse{Head: component.Head}
	for _, operation := range diff.Operations {
		response.Operations = append(response.Operations, deltaOperation{
			Kind: operation.Kind, ID: operation.InstanceID, HTML: operation.HTML,
			Boundaries: operation.Boundaries,
		})
	}
	for _, entry := range diff.Manifest.Instances {
		response.Manifest = append(response.Manifest, deltaInstance{
			ID: entry.ID, Frame: entry.FrameValidator,
			Children: entry.ChildrenValidator, Parent: entry.ParentID,
		})
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		o.fail(w, r, Failure{
			Kind:       FailureRenderFailed,
			Status:     http.StatusInternalServerError,
			Message:    "render failed",
			Err:        err,
			KindID:     kind,
			InstanceID: instance,
		})
		return
	}
	// A redraw response is identified by its URL and its bytes, so it can be
	// revalidated like any other resource. Sending the digest is what lets an
	// unchanged region cost a 304 instead of its whole markup.
	//
	// The digest covers the whole body, head and manifest included, so a
	// component that changed only what it contributes to the document head does
	// not answer 304 with the old contribution.
	etag := `"` + o.redrawETag(string(encoded)) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", o.redrawCacheControl())
	w.Header().Set(o.renderHeader(), modeRedraw)
	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(encoded)
}

const modeRedraw = "redraw"

// DefaultRedrawCacheControl keeps a redraw out of every shared cache and makes
// a private one revalidate.
//
// It is private rather than public because a redraw usually renders per-user
// content, and no-cache rather than no-store because no-store would forbid the
// conditional request the ETag exists for: a browser that may not keep the
// bytes can never ask whether they changed.
const DefaultRedrawCacheControl = "private, no-cache"

func (o Options) redrawCacheControl() string {
	if o.RedrawCacheControl == "" {
		return DefaultRedrawCacheControl
	}
	return o.RedrawCacheControl
}

// redrawETag identifies the rendered bytes. It is a content digest rather than
// a version, because the point is to detect that nothing changed.
//
// It is keyed for the same reason a frame validator is: a conditional request
// answered 304 confirms a guess, and a redraw usually renders low-entropy
// per-user content that is cheap to guess. Without a key, anyone able to reach
// the endpoint could enumerate what a region says by digesting candidates. An
// unkeyed digest is the fallback for a deployment that set no key, which is
// only supportable for public pages either way.
func (o Options) redrawETag(body string) string {
	if len(o.Key) == 0 {
		sum := sha256.Sum256([]byte(body))
		return base64.RawURLEncoding.EncodeToString(sum[:16])
	}
	mac := hmac.New(sha256.New, o.Key)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// matchesETag reads an If-None-Match header, which is a comma-separated list of
// tags or the wildcard, and reports whether the response's tag is in it.
//
// A weak comparison is the right one here: the two representations differ only
// if the bytes differ, so the weak prefix carries no extra information.
func matchesETag(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}
