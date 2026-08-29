// Package app exercises every branch of rule:transform-eligibility in one
// package, so the analyzer is tested against real type information rather than
// against synthesized AST.
package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type CreateUserRequest struct {
	Name string
}

type CreateUserResponse struct {
	ID string
}

// --- admitted ---------------------------------------------------------------

// plainHandler touches the transport only through recognized calls. WriteError
// is one of them: it names no model, so it needs a transport-only pattern or
// every handler that reports an error would be refused.
func plainHandler(w http.ResponseWriter, r *http.Request) {
	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{ID: input.Name})
}

// contextHandler reads r.Context(), which the rewrite table covers because a
// RequestCtx satisfies context.Context.
func contextHandler(w http.ResponseWriter, r *http.Request) {
	if r.Context().Err() != nil {
		return
	}
	_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{})
}

// callsAdmittedHelper hands the transport to a helper that is itself admitted,
// so the closure over the call graph keeps both. A shared render or error
// helper is the normal shape of a handler package, and refusing it would refuse
// everything that calls it.
func callsAdmittedHelper(w http.ResponseWriter, r *http.Request) {
	renderOK(w, r, CreateUserResponse{ID: "u_1"})
}

func renderOK(w http.ResponseWriter, r *http.Request, out CreateUserResponse) {
	_ = httpbind.Write[CreateUserResponse](w, r, out)
}

// discardHandler writes the transport to the blank identifier, which is common
// enough in real handlers that refusing it would refuse most of them.
func discardHandler(w http.ResponseWriter, r *http.Request) {
	_ = r
	_ = w
}

// --- refused ----------------------------------------------------------------

// unknownCallHandler hands the writer to a function outside the package. This
// is the shape of every tracing, metrics and session library.
func unknownCallHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "hello")
	_ = r
}

// refusedHelper reads a selector no rewrite covers, and so refuses every
// caller that passes it the transport.
func refusedHelper(w http.ResponseWriter, r *http.Request) {
	_ = r.URL
	_ = w
}

func inheritsRefusal(w http.ResponseWriter, r *http.Request) {
	refusedHelper(w, r)
}

func unknownSelectorHandler(w http.ResponseWriter, r *http.Request) {
	_ = r.RemoteAddr
	_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{})
}

func typeAssertionHandler(w http.ResponseWriter, r *http.Request) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	_ = r
}

func closureHandler(w http.ResponseWriter, r *http.Request) {
	go func() {
		_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{})
	}()
}

func escapeHandler(w http.ResponseWriter, r *http.Request) {
	saved := r
	_ = saved
	_ = w
}

// The rewrite turns r.Context() into the pooled *RequestCtx on fasthttp, so
// carrying it out of the handler — to a goroutine, a channel, a field, or a
// global — is a refusal even though the selector on its own is rewritable.

var ctxSink context.Context
var ctxChan = make(chan context.Context, 1)

type ctxHolder struct{ ctx context.Context }

var heldCtx ctxHolder

func goEscapeHandler(w http.ResponseWriter, r *http.Request) {
	go consumeContext(r.Context())
	_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{})
}

func chanEscapeHandler(w http.ResponseWriter, r *http.Request) {
	ctxChan <- r.Context()
	_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{})
}

func fieldEscapeHandler(w http.ResponseWriter, r *http.Request) {
	heldCtx.ctx = r.Context()
	_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{})
}

func globalEscapeHandler(w http.ResponseWriter, r *http.Request) {
	ctxSink = r.Context()
	_ = httpbind.Write[CreateUserResponse](w, r, CreateUserResponse{})
}

func consumeContext(ctx context.Context) { _ = ctx }

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /users", plainHandler)
	mux.HandleFunc("GET /ctx", contextHandler)
	mux.HandleFunc("GET /helper", callsAdmittedHelper)
	mux.HandleFunc("GET /discard", discardHandler)
	mux.HandleFunc("GET /trace", unknownCallHandler)
	mux.HandleFunc("GET /inherit", inheritsRefusal)
	mux.HandleFunc("GET /selector", unknownSelectorHandler)
	mux.HandleFunc("GET /assert", typeAssertionHandler)
	mux.HandleFunc("GET /closure", closureHandler)
	mux.HandleFunc("GET /escape", escapeHandler)
	mux.HandleFunc("GET /go-escape", goEscapeHandler)
	mux.HandleFunc("GET /chan-escape", chanEscapeHandler)
	mux.HandleFunc("GET /field-escape", fieldEscapeHandler)
	mux.HandleFunc("GET /global-escape", globalEscapeHandler)
}
