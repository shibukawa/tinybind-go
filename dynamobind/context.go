package dynamobind

import (
	"context"
	"errors"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// ErrNoClient reports that a Context does not carry a DynamoDB client. It is
// returned rather than panicking, so every entry stays an ordinary
// error-returning function.
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

// clientEntry is the client and, optionally, the table naming of one deployment.
type clientEntry struct {
	client *dynamodb.Client
	names  TableResolver
}

// ClientOption configures what a stored client is used with.
type ClientOption func(*clientEntry)

// WithTableNames records how declared table names map onto this deployment's.
// Without it the declared name is sent unchanged, which is what a deployment
// whose tables are named as declared wants. A nil resolver is ignored, so a
// mistaken nil behaves as no resolver rather than panicking on the first call.
func WithTableNames(resolve TableResolver) ClientOption {
	return func(entry *clientEntry) {
		if resolve != nil {
			entry.names = resolve
		}
	}
}

// WithClient returns a child Context carrying a DynamoDB client and, with
// WithTableNames, how its tables are named. Framework middleware installs it
// once, and every entry of this package resolves it.
func WithClient(ctx context.Context, c *dynamodb.Client, options ...ClientOption) context.Context {
	entry := clientEntry{client: c}
	for _, option := range options {
		option(&entry)
	}
	return context.WithValue(ctx, clientContextKey{}, entry)
}

// ClientFromContext returns the client installed by WithClient.
//
// It is the escape hatch for reaching the driver directly, for an operation this
// package does not wrap. Everything this package does wrap resolves through
// TableFromContext instead, so the name mapping is applied.
func ClientFromContext(ctx context.Context) (*dynamodb.Client, error) {
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok || entry.client == nil {
		return nil, ErrNoClient
	}
	return entry.client, nil
}

// TableFromContext resolves a declared table name into the client and the name
// to send. Every entry of this package calls it.
func TableFromContext(ctx context.Context, table string) (*dynamodb.Client, string, error) {
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok || entry.client == nil {
		return nil, "", ErrNoClient
	}
	if entry.names == nil {
		return entry.client, table, nil
	}
	return entry.client, entry.names(ctx, table), nil
}
