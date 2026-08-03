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
	// A component's head is a static declaration, so an oversized one is a fact
	// about the templates rather than about any request. Discovering it at
	// startup is the point: the alternative is a proxy dropping the header in
	// production and a region rendering unstyled with nothing to look at.
	if encoded := encodeHead(component.Head); len(encoded) > DefaultMaxHeadBytes {
		return errors.New("htmlupdate: " + component.KindID + " contributes " + strconv.Itoa(len(encoded)) +
			" bytes of head, past the " + strconv.Itoa(DefaultMaxHeadBytes) + " a redraw response may carry;" +
			" put its contribution in the document shell from Registry.RequiredHead instead")
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

// DefaultMaxHeadBytes bounds the head one redraw response may carry. It is a
// header, so it lives inside whatever the deployment's proxies allow, and the
// value it holds is a static template declaration rather than anything a request
// influenced.
const DefaultMaxHeadBytes = 2 << 10

// encodeHead packs head tags into one header value.
//
// Base64 of JSON rather than the markup itself: a head tag holds quotes and may
// hold any character an attribute value may, and a header is not a place to
// discover which of those a proxy will pass through. The runtime unpacks it with
// the two calls every browser has.
func encodeHead(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(encoded)
}

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

// RedrawHandler serves the registered components.
//
// The path is <prefix>/redraw/<kind>/<instance>. The instance id travels so the
// returned root element arrives already addressable; the render itself depends
// only on the kind and the query values.
func (o Options) RedrawHandler(reg *Registry) http.Handler {
	base := o.pathPrefix() + "/redraw/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind, instance, ok := splitRedrawPath(strings.TrimPrefix(r.URL.Path, base))
		if !ok {
			o.fail(w, r, Failure{
				Kind:    FailureMalformedPath,
				Status:  http.StatusNotFound,
				Message: notFoundMessage,
			})
			return
		}
		component, known := reg.kinds[kind]
		if !known {
			// This deployment does not publish that component at all.
			o.fail(w, r, Failure{
				Kind:       FailureUnknownComponent,
				Status:     http.StatusNotFound,
				Message:    notFoundMessage,
				KindID:     kind,
				InstanceID: instance,
			})
			return
		}
		// A kind is stable across builds on purpose, so it cannot say whether
		// the page asking is current. The build identity does, and it covers
		// every change a kind cannot see: a component this one calls, an
		// external function, the render runtime itself.
		if r.Header.Get(o.buildHeader()) != o.buildID() {
			o.fail(w, r, Failure{
				Kind:       FailureStalePage,
				Status:     http.StatusConflict,
				Message:    "stale page",
				KindID:     kind,
				InstanceID: instance,
			})
			return
		}
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
		var out strings.Builder
		if err := htmlbind.Render(&out, fragment); err != nil {
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
		body := out.String()
		// The body stays the bare subtree — no envelope, so the endpoint is still
		// testable with curl and a client parses what it already parsed — and the
		// contribution travels beside it.
		//
		// It is sent so a component whose stylesheet is not on the page installs
		// it before its markup lands, which is the flash of unstyled content the
		// navigation delta added its own head field to prevent. A well-configured
		// deployment has already put every one of these in its shell from
		// Registry.RequiredHead, and then this changes nothing: the runtime finds
		// each tag present and installs none.
		//
		// A component contributing no head sends no header, so its response is
		// byte-identical to what it was before this existed.
		if encoded := encodeHead(component.Head); encoded != "" {
			w.Header().Set(o.headHeader(), encoded)
		}
		// A redraw response is identified by its URL and its bytes, so it can
		// be revalidated like any other resource. Sending the digest is what
		// lets an unchanged region cost a 304 instead of its whole markup.
		etag := `"` + o.redrawETag(body) + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", o.redrawCacheControl())
		// The response depends on the build the page was rendered by, so a
		// cache holding it must key on that too.
		w.Header().Add("Vary", o.buildHeader())
		w.Header().Set(o.renderHeader(), modeRedraw+";v="+versionText)
		if matchesETag(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	})
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

// splitRedrawPath reads "<kind>/<instance>" and rejects anything else, so a
// missing or extra segment cannot be read as a valid target.
func splitRedrawPath(rest string) (kind, instance string, ok bool) {
	kind, instance, found := strings.Cut(rest, "/")
	if !found || kind == "" || instance == "" || strings.Contains(instance, "/") {
		return "", "", false
	}
	return kind, instance, true
}

// RedrawPath is the URL for one instance of a registered component, exposed so
// a test and a non-browser client can build exactly what the runtime does.
func (o Options) RedrawPath(kindID, instanceID string, values url.Values) string {
	path := o.pathPrefix() + "/redraw/" + url.PathEscape(kindID) + "/" + url.PathEscape(instanceID)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}
