// Package archive is the route that needs the request itself rather than the
// address it was sent to.
//
// Its reader is not in the path and not in the query: it is put on the context
// before the handler runs, the way an authenticated session or a database pool
// is. Both shapes that can reach it are exercised here — one read inline where
// it is written, and one bound to a name — so neither has to drop to the
// handler rung to read one value.
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

// LatestMemo is the same value bound to a name rather than read in place. It
// declares the context for the same reason CurrentReader does, and the template
// binds it with {val}, which is what the typed entry point used to be for.
func LatestMemo(ctx context.Context) string {
	return reader(ctx) + "'s latest memo"
}
