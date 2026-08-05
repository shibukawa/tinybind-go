package dynamobind

import (
	"context"
	"errors"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// ErrNoClient reports that a Context does not carry a DynamoDB client, or that a
// zero Handle was passed to an entry taking one. It is returned rather than
// panicking, so every entry stays an ordinary error-returning function.
var ErrNoClient = errors.New("dynamobind: no DynamoDB client in context")

// TableResolver maps the name a declaration or a call site writes onto the name
// this deployment uses.
//
// It takes the Context so the mapping can depend on the request as well as on
// the process: a per-tenant table, or a name read from configuration bound to
// the Context, is the same one function. Nothing here composes the name, so a
// prefix, a suffix, a lookup table and a wholly unrelated name are equally
// expressible:
//
//	dynamobind.WithTableNames(func(ctx context.Context, declared string) string {
//		return "staging-" + declared
//	})
//
//	dynamobind.WithTableNames(func(ctx context.Context, declared string) string {
//		return config.Tables[declared] // whatever the IaC named it
//	})
type TableResolver func(ctx context.Context, declared string) string

type clientContextKey struct{}

// Handle is a DynamoDB client together with the table naming of one deployment.
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
	client *dynamodb.Client
	names  TableResolver
}

// NewHandle binds a client to the table naming of one deployment, for the entries
// suffixed On.
//
// It takes the same ClientOption list as WithClient, so a program moving between
// the two forms rewrites the call and not the configuration:
//
//	h := dynamobind.NewHandle(client, dynamobind.WithTableNames(names))
//	r, err := dynamobind.LoadOn[Reading](ctx, h, "readings", key)
func NewHandle(c *dynamodb.Client, options ...ClientOption) Handle {
	h := Handle{client: c}
	for _, option := range options {
		option(&h)
	}
	return h
}

// Client returns the driver client this Handle carries, or nil for the zero
// Handle. It is the escape hatch for reaching the driver directly, and it
// applies no table naming; Table is what applies that.
func (h Handle) Client() *dynamodb.Client { return h.client }

// Table resolves a declared table name into the client and the name to send.
// Every entry suffixed On calls it, and so does TableFromContext once it has
// read the Handle out of a Context.
func (h Handle) Table(ctx context.Context, table string) (*dynamodb.Client, string, error) {
	if h.client == nil {
		return nil, "", ErrNoClient
	}
	if h.names == nil {
		return h.client, table, nil
	}
	return h.client, h.names(ctx, table), nil
}

// ClientOption configures what a stored client is used with.
type ClientOption func(*Handle)

// WithTableNames records how declared table names map onto this deployment's.
// Without it the declared name is sent unchanged, which is what a deployment
// whose tables are named as declared wants. A nil resolver is ignored, so a
// mistaken nil behaves as no resolver rather than panicking on the first call.
func WithTableNames(resolve TableResolver) ClientOption {
	return func(h *Handle) {
		if resolve != nil {
			h.names = resolve
		}
	}
}

// WithClient returns a child Context carrying a DynamoDB client and, with
// WithTableNames, how its tables are named. Framework middleware installs it
// once, and every entry of this package resolves it.
func WithClient(ctx context.Context, c *dynamodb.Client, options ...ClientOption) context.Context {
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
// has the client and the table naming in hand, and can then call the entries
// suffixed On with no further Context lookup on the operation path.
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
// package does not wrap. Everything this package does wrap resolves through
// TableFromContext instead, so the name mapping is applied.
func ClientFromContext(ctx context.Context) (*dynamodb.Client, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return h.client, nil
}

// TableFromContext resolves a declared table name into the client and the name
// to send. Every Context-resolving entry of this package calls it.
func TableFromContext(ctx context.Context, table string) (*dynamodb.Client, string, error) {
	h, err := HandleFromContext(ctx)
	if err != nil {
		return nil, "", err
	}
	return h.Table(ctx, table)
}
