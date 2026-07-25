// Package pw is a minimal stand-in for a framework that publishes its own
// developer-facing API on top of tinybind.
package pw

import (
	"context"
	"io"
	"net/http"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

// Parse binds an HTTP request into T.
func Parse[T any](r *http.Request) (T, error) { return httpbind.Bind[T](r) }

// WriteAPI writes a JSON API response.
func WriteAPI[T any](w http.ResponseWriter, r *http.Request, value T) error {
	return httpbind.Write(w, r, value)
}

// WriteHTML owns every HTTP concern of HTML rendering so generated templates
// stay independent of net/http.
func WriteHTML[P any](w http.ResponseWriter, r *http.Request, render func(io.Writer, P) error, params P) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return render(w, params)
}

// RegisterConfig registers one configuration section.
func RegisterConfig[T any](prefix string) *T { return configbind.Bind[T](prefix) }

// SubCommand registers one CLI subcommand.
func SubCommand[T any](name, help string) *T { return configbind.SubCommand[T](name, help) }

// SQLExecutor resolves the executor used by generated SQL functions.
func SQLExecutor(ctx context.Context) (sqlbind.SQLExecutor, error) {
	return sqlbind.SQLExecutorFromContext(ctx)
}
