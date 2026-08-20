package fasthttpbind

import (
	"testing"

	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Runtime-primitive benchmarks. There is no committed fasthttp binder fixture,
// so the request-side helpers generated binders are built from are measured
// here directly: the raw-body reads the inline JSON pass uses, the Object
// split it replaced, and the query lookup every bound field pays.

var benchBody = []byte(`{"name":"Alice","email":"a@example.com","note":"leave at the front desk"}`)

func newBenchCtx() *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.SetBody(benchBody)
	ctx.Request.SetRequestURI("/users?name=Alice&email=a%40example.com&page=3")
	return ctx
}

func BenchmarkReadJSONBody(b *testing.B) {
	ctx := newBenchCtx()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ReadJSONBody(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadJSONBodyOwned(b *testing.B) {
	ctx := newBenchCtx()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ReadJSONBodyOwned(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadJSONObject(b *testing.B) {
	ctx := newBenchCtx()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ReadJSONObject(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueryLookupHit(b *testing.B) {
	ctx := newBenchCtx()
	q := Queries(ctx)
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := QueryLookup(q, "email"); !ok {
			b.Fatal("missing key")
		}
	}
}

func BenchmarkQueryLookupMiss(b *testing.B) {
	ctx := newBenchCtx()
	q := Queries(ctx)
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := QueryLookup(q, "absent"); ok {
			b.Fatal("unexpected hit")
		}
	}
}

func BenchmarkReadBodyDispatch(b *testing.B) {
	ctx := newBenchCtx()
	b.ReportAllocs()
	for b.Loop() {
		if _, _, _, err := ReadBody(ctx, true, false); err != nil {
			b.Fatal(err)
		}
	}
}
