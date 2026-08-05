package firestorebind

import (
	"context"
	"errors"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// ErrNoClient reports that a Context does not carry a Datastore client. It is
// returned rather than panicking, so every entry stays an ordinary
// error-returning function.
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
type NamespaceResolver func(ctx context.Context) string

type clientContextKey struct{}

// clientEntry is the client and, optionally, the tenancy of one deployment.
type clientEntry struct {
	client    *datastore.Client
	namespace NamespaceResolver
}

// ClientOption configures what a stored client is used with.
type ClientOption func(*clientEntry)

// WithNamespace records how a request picks its namespace. Without it, keys are
// sent as built and the client's own namespace applies. A nil resolver is
// ignored, so a mistaken nil behaves as no resolver rather than panicking on the
// first call.
func WithNamespace(resolve NamespaceResolver) ClientOption {
	return func(entry *clientEntry) {
		if resolve != nil {
			entry.namespace = resolve
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
	entry := clientEntry{client: c}
	for _, option := range options {
		option(&entry)
	}
	return context.WithValue(ctx, clientContextKey{}, entry)
}

// ClientFromContext returns the client installed by WithClient.
//
// It is the escape hatch for reaching the driver directly, for an operation this
// package does not wrap. Note that it applies no namespace: a key passed to the
// driver through it is sent as built. Pass keys through KeyFor or KeysFor first
// to place them where every wrapped entry of this package would.
func ClientFromContext(ctx context.Context) (*datastore.Client, error) {
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok || entry.client == nil {
		return nil, ErrNoClient
	}
	return entry.client, nil
}

// clientFor resolves the client and the namespace resolver together. Every entry
// of this package calls it.
func clientFor(ctx context.Context) (*datastore.Client, NamespaceResolver, error) {
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok || entry.client == nil {
		return nil, nil, ErrNoClient
	}
	return entry.client, entry.namespace, nil
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
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok {
		return key
	}
	return applyNamespace(ctx, entry.namespace, key)
}

// KeysFor is KeyFor over a slice. It allocates only when the resolver would
// change something, which a caller looping over KeyFor cannot avoid.
func KeysFor(ctx context.Context, keys []datastore.Key) []datastore.Key {
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok {
		return keys
	}
	return applyNamespaceAll(ctx, entry.namespace, keys)
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
