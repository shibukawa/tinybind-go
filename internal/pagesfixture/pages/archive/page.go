// Package archive is the route that needs the request itself rather than the
// address it was sent to.
//
// Its reader is not in the path and not in the query: it is put on the context
// before the handler runs, the way an authenticated session or a database pool
// is. Both shapes that can reach it are exercised here — a synchronous external
// called from the template, and the typed entry point — so neither has to drop
// to the handler rung to read one value.
package archive

import "context"

type readerKey struct{}

// WithReader is what a caller's middleware would do: put the request-scoped
// value on the context before the generated handler runs.
func WithReader(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, readerKey{}, name)
}

func reader(ctx context.Context) string {
	name, _ := ctx.Value(readerKey{}).(string)
	if name == "" {
		return "guest"
	}
	return name
}

// CurrentReader is a synchronous external. It declares the context and so
// receives the render context, which is what lets the value render inline
// without travelling through this page's parameters.
func CurrentReader(ctx context.Context) string { return reader(ctx) }

// Load is the typed rung. The leading context is not a URL input, so it is not
// one of the page's decoded parameters; everything after it would be.
func Load(ctx context.Context) (string, error) {
	return reader(ctx) + "'s latest memo", nil
}
