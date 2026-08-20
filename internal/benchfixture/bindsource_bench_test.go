package benchfixture

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Benchmarks for the non-JSON binding sources. The handler benchmarks in
// bench_test.go send a JSON body, so the query path is only ever exercised as
// a miss and the form path not at all; these fill that gap.

func newQueryRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/orgs/acme/users?name=Alice&email=a%40example.com", nil)
	r.Header.Set("Authorization", "Bearer secret")
	r.SetPathValue("org_id", "acme")
	return r
}

// BenchmarkHandlerGeneratedQueryBind serves the same handler with both input
// fields answered by the query string, so no body is read at all.
func BenchmarkHandlerGeneratedQueryBind(b *testing.B) {
	w := devNull{}
	r := newQueryRequest()
	b.ReportAllocs()
	for b.Loop() {
		CreateUser(w, r)
	}
}

var formBody = []byte("name=Alice&email=a%40example.com")

func newFormRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", bytes.NewReader(formBody))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Authorization", "Bearer secret")
	r.SetPathValue("org_id", "acme")
	return r
}

// BenchmarkHandlerGeneratedFormBind serves the handler from an urlencoded
// form. ParseForm caches into the request, so the cached state is cleared with
// the body each iteration to keep every loop a full parse.
func BenchmarkHandlerGeneratedFormBind(b *testing.B) {
	w := devNull{}
	r := newFormRequest()
	b.ReportAllocs()
	for b.Loop() {
		r.Body = io.NopCloser(bytes.NewReader(formBody))
		r.Form = nil
		r.PostForm = nil
		CreateUser(w, r)
	}
}

func TestQueryAndFormBindsAgreeWithJSON(t *testing.T) {
	viaJSON := httptest.NewRecorder()
	CreateUser(viaJSON, newUserRequest())
	viaQuery := httptest.NewRecorder()
	CreateUser(viaQuery, newQueryRequest())
	viaForm := httptest.NewRecorder()
	CreateUser(viaForm, newFormRequest())
	if viaQuery.Code != viaJSON.Code || viaForm.Code != viaJSON.Code {
		t.Fatalf("status differs: json=%d query=%d form=%d", viaJSON.Code, viaQuery.Code, viaForm.Code)
	}
	// The JSON request carries "Alice"/"a@example.com" too, so all three
	// responses must be byte-identical.
	if viaQuery.Body.String() != viaJSON.Body.String() {
		t.Fatalf("query bind differs:\n json %s\nquery %s", viaJSON.Body.String(), viaQuery.Body.String())
	}
	if viaForm.Body.String() != viaJSON.Body.String() {
		t.Fatalf("form bind differs:\n json %s\n form %s", viaJSON.Body.String(), viaForm.Body.String())
	}
}
