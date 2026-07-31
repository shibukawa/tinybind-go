package dynamobind

import (
	"context"
	"errors"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// ErrNoClient reports that a Context does not carry a DynamoDB client. It is
// returned rather than panicking, so generated Context wrappers stay ordinary
// error-returning functions.
var ErrNoClient = errors.New("dynamobind: no DynamoDB client in context")

// ErrNoTablePrefix reports that a Context carries a client but was never told
// what to prepend to a declared table name.
//
// There is no empty-prefix default. A missing client cannot issue a request at
// all, but a missing prefix would read the unprefixed table and answer with a
// normal empty page, which is indistinguishable from a table that holds
// nothing. A deployment with no prefix says so with WithTablePrefix("").
var ErrNoTablePrefix = errors.New("dynamobind: no table prefix in context")

type clientContextKey struct{}

// clientEntry is the client together with the deployment table prefix. Both are
// facts of one deployment, fixed for the process, so they travel together: a
// caller that set one and not the other would have a Context that resolves a
// client and still cannot name a table.
type clientEntry struct {
	client *dynamodb.Client
	prefix string
	// hasPrefix separates "the prefix is empty" from "no prefix was set". The
	// string alone cannot: both are "".
	hasPrefix bool
}

// ClientOption configures what a stored client is used with.
type ClientOption func(*clientEntry)

// WithTablePrefix records what a resolved table name begins with, so one
// deployment can name its tables apart from another in the same account.
// Pass "" to declare that this deployment uses the declared names unchanged.
func WithTablePrefix(prefix string) ClientOption {
	return func(entry *clientEntry) {
		entry.prefix = prefix
		entry.hasPrefix = true
	}
}

// WithClient returns a child Context carrying a DynamoDB client, and the table
// prefix when one is given. Framework middleware installs it once, and
// generated Context wrappers resolve it.
func WithClient(ctx context.Context, c *dynamodb.Client, options ...ClientOption) context.Context {
	entry := clientEntry{client: c}
	for _, option := range options {
		option(&entry)
	}
	return context.WithValue(ctx, clientContextKey{}, entry)
}

// ClientFromContext returns the client installed by WithClient.
//
// It is what an item operation uses: Load, Store and the rest keep their client
// and table parameters, and a caller resolving from a Context spends one line
// rather than a doubled API. Use TableFromContext when the table name is a
// declared one that the prefix applies to.
func ClientFromContext(ctx context.Context) (*dynamodb.Client, error) {
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok || entry.client == nil {
		return nil, ErrNoClient
	}
	return entry.client, nil
}

// TableFromContext resolves a declared table name into the client and the name
// to send. It is the resolver generated Context wrappers use, and it has the
// signature a framework resolver must have to replace it.
func TableFromContext(ctx context.Context, table string) (*dynamodb.Client, string, error) {
	entry, ok := ctx.Value(clientContextKey{}).(clientEntry)
	if !ok || entry.client == nil {
		return nil, "", ErrNoClient
	}
	if !entry.hasPrefix {
		return nil, "", ErrNoTablePrefix
	}
	return entry.client, entry.prefix + table, nil
}
