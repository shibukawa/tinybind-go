package firestorebind

import (
	"context"
	"errors"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// ErrNoClient reports that a Context does not carry a Datastore client, or that
// a zero Handle was passed to an entry taking one. It is returned rather than
// panicking, so every entry stays an ordinary error-returning function.
var ErrNoClient = errors.New("firestorebind: no Datastore client in context")

// NamespaceResolver picks the namespace for one request.
//
// A namespace is a tenancy dimension, not a property of a type, so it lives here
// rather than in a struct tag: putting it on the type would make one struct
// unusable for a second tenant. The driver's own datastore.WithNamespace already
// covers a namespace fixed for the process; this covers one that varies per
// request, which is the case that option cannot serve:
//
//	firestorebind.WithClient(ctx, client, firestorebind.WithNamespace(
//		func(ctx context.Context) string { return tenantOf(ctx) }))
//
// A generated key carries no namespace. This resolver supplies it at the runtime
// entry, so a key value stays portable across tenants.
//
// It keeps its Context parameter in every form, including the entries suffixed
// On: a per-request tenant is read from a Context even when the client is not.
type NamespaceResolver func(ctx context.Context) string

type clientContextKey struct{}

// Handle is a Datastore client together with the tenancy of one deployment.
//
// It is what WithClient stores in a Context, and it is what the entries suffixed
// On take directly. The two forms are the same value reached two ways: a Context
// keeps it off every call site, and a parameter keeps it out of every lookup.
// Which one a program uses is a call-site preference, not a behaviour
// difference.
//
// The zero Handle carries no client, so an entry given one returns ErrNoClient
// exactly as a Context carrying none does.
//
// Fields are unexported so a field added later is not a breaking change; build
// one with NewHandle.
type Handle struct {
	client    *datastore.Client
	namespace NamespaceResolver
}

// NewHandle binds a client to the tenancy of one deployment, for the entries
// suffixed On.
//
// It takes the same ClientOption list as WithClient, so a program moving between
// the two forms rewrites the call and not the configuration:
//
//	h := firestorebind.NewHandle(client, firestorebind.WithNamespace(tenantOf))
//	r, err := firestorebind.LoadOn[Reading](ctx, h, key)
func NewHandle(c *datastore.Client, options ...ClientOption) Handle {
	h := Handle{client: c}
	for _, option := range options {
		option(&h)
	}
	return h
}

// Client returns the driver client this Handle carries, or nil for the zero
// Handle. It is the escape hatch for reaching the driver directly, and it
// applies no namespace; KeyForOn is what places a key.
func (h Handle) Client() *datastore.Client { return h.client }

// resolve returns the client and the namespace resolver together. Every entry
// suffixed On calls it, and so does clientFor once it has read the Handle out of
// a Context.
func (h Handle) resolve() (*datastore.Client, NamespaceResolver, error) {
	if h.client == nil {
		return nil, nil, ErrNoClient
	}
	return h.client, h.namespace, nil
}

// ClientOption configures what a stored client is used with.
type ClientOption func(*Handle)

// WithNamespace records how a request picks its namespace. Without it, keys are
// sent as built and the client's own namespace applies. A nil resolver is
// ignored, so a mistaken nil behaves as no resolver rather than panicking on the
// first call.
func WithNamespace(resolve NamespaceResolver) ClientOption {
	return func(h *Handle) {
		if resolve != nil {
			h.namespace = resolve
		}
	}
}

// WithClient returns a child Context carrying a Datastore client and, with
// WithNamespace, how its tenancy is resolved. Framework middleware installs it
// once, and every entry of this package resolves it.
//
// A second client is a second Context, not a second signature. That is what a
// test, a second database or a second project uses.
func WithClient(ctx context.Context, c *datastore.Client, options ...ClientOption) context.Context {
	return WithHandle(ctx, NewHandle(c, options...))
}

// WithHandle returns a child Context carrying an already-built Handle.
//
// It is WithClient for a caller that holds a Handle: a framework resolving one
// out of its own Context value, or a setup path that builds the Handle once and
// installs it in several Contexts.
func WithHandle(ctx context.Context, h Handle) context.Context {
	return context.WithValue(ctx, clientContextKey{}, h)
}

// HandleFromContext returns the Handle installed by WithClient or WithHandle.
//
// It is the one lookup a caller needs: a framework reading it once in middleware
// has the client and the tenancy in hand, and can then call the entries suffixed
// On with no further Context lookup on the operation path.
func HandleFromContext(ctx context.Context) (Handle, error) {
	h, ok := ctx.Value(clientContextKey{}).(Handle)
	if !ok || h.client == nil {
		return Handle{}, ErrNoClient
	}
	return h, nil
}

// ClientFromContext returns the client installed by WithClient.
//
// It is the escape hatch for reaching the driver directly, for an operation this
// package does not wrap. Note that it applies no namespace: a key passed to the
// driver through it is sent as built. Pass keys through KeyFor or KeysFor first
// to place them where every wrapped entry of this package would.
func ClientFromContext(ctx context.Context) (*datastore.Client, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return h.client, nil
}

// KeyFor stamps the Context's namespace onto a key, the way every wrapped entry
// of this package does before it sends one.
//
// It exists for the ClientFromContext escape hatch. A key reaching the driver
// any other way is sent as built, which places it in the default namespace: for
// a multi-tenant caller that is a data-placement bug no test running in the
// default namespace can see, and for a test isolating itself in a namespace of
// its own it is a teardown that deletes nothing and reports success.
//
// The key is returned unchanged when the Context carries no client, when no
// WithNamespace resolver was installed, when the resolver answers the empty
// string, and when the key already names a namespace. That last one is the
// point: an explicitly placed key is not silently moved.
//
// There is no error to return, so there is none in the signature. A Context
// with no client meets ErrNoClient at the operation, not here.
func KeyFor(ctx context.Context, key datastore.Key) datastore.Key {
	h, ok := ctx.Value(clientContextKey{}).(Handle)
	if !ok {
		return key
	}
	return KeyForOn(ctx, h, key)
}

// KeysFor is KeyFor over a slice. It allocates only when the resolver would
// change something, which a caller looping over KeyFor cannot avoid.
func KeysFor(ctx context.Context, keys []datastore.Key) []datastore.Key {
	h, ok := ctx.Value(clientContextKey{}).(Handle)
	if !ok {
		return keys
	}
	return KeysForOn(ctx, h, keys)
}

// KeyForOn is KeyFor taking its Handle as an argument. A zero Handle returns the
// key unchanged, as a Context carrying no client does.
func KeyForOn(ctx context.Context, h Handle, key datastore.Key) datastore.Key {
	return applyNamespace(ctx, h.namespace, key)
}

// KeysForOn is KeysFor taking its Handle as an argument.
func KeysForOn(ctx context.Context, h Handle, keys []datastore.Key) []datastore.Key {
	return applyNamespaceAll(ctx, h.namespace, keys)
}

// applyNamespace stamps the resolved namespace onto a key. A key that already
// names one keeps it, so an explicitly placed key is not silently moved.
func applyNamespace(ctx context.Context, resolve NamespaceResolver, key datastore.Key) datastore.Key {
	if resolve == nil || key.Namespace != "" {
		return key
	}
	ns := resolve(ctx)
	if ns == "" {
		return key
	}
	return key.WithNamespace(ns)
}

// applyNamespaceAll is applyNamespace over a slice, allocating only when the
// resolver would change something.
func applyNamespaceAll(ctx context.Context, resolve NamespaceResolver, keys []datastore.Key) []datastore.Key {
	if resolve == nil || len(keys) == 0 {
		return keys
	}
	out := make([]datastore.Key, len(keys))
	for i, key := range keys {
		out[i] = applyNamespace(ctx, resolve, key)
	}
	return out
}
