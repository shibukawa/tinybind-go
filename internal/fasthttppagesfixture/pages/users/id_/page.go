package id_

import (
	"context"
	"strings"

	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// ReaderName is the value the page renders, bound by the template. It declares
// a leading context.Context and receives the render context; on this transport
// the request value is itself a context, so the generated handler passes it as
// it stands rather than calling a method that does not exist here.
func ReaderName(ctx context.Context, id string) string {
	name := strings.ToUpper(id)
	if reader, ok := ctx.Value(readerKey{}).(string); ok {
		name = name + " for " + reader
	}
	return name
}

type readerKey struct{}

// WithReader is what a caller's middleware would do: put a request-scoped value
// on the context the generated handler passes down.
func WithReader(ctx *fasthttp.RequestCtx, name string) {
	ctx.SetUserValue(readerKey{}, name)
}

// Rename is a server function: a handler of this transport's shape that a
// template names instead of a URL. It owns its whole response.
//
// Discovery recognizes it by the same signature a rung 3 page declares, which
// is why the shape is configuration rather than a constant.
func Rename(ctx *fasthttp.RequestCtx) {
	name, _ := fasthttpbind.QueryValue(ctx, "name")
	_, _ = ctx.Write([]byte("renamed to " + strings.ToUpper(name)))
}

// unexported handlers stay private, because generated code in another package
// cannot reach them.
func internalOnly(ctx *fasthttp.RequestCtx) {
	_, _ = ctx.Write([]byte("unreachable"))
}

var _ = internalOnly
