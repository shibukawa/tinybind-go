package benchfixture

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

func (devNull) Header() http.Header { return http.Header{} }

func (devNull) WriteHeader(int) {}

var (
	orderJSON = []byte(`{"id":"ord-10293","customer":{"name":"Ada Lovelace","email":"ada@example.com","tier":"gold"},"items":[{"sku":"SKU-1","qty":2,"price":19.99},{"sku":"SKU-2","qty":1,"price":249.5},{"sku":"SKU-3","qty":7,"price":3.25}],"total":312.23,"paid":true,"tags":["priority","gift","repeat"],"note":"leave at the front desk"}`)

	order = Order{
		ID:       "ord-10293",
		Customer: Customer{Name: "Ada Lovelace", Email: "ada@example.com", Tier: "gold"},
		Items: []LineItem{
			{SKU: "SKU-1", Qty: 2, Price: 19.99},
			{SKU: "SKU-2", Qty: 1, Price: 249.5},
			{SKU: "SKU-3", Qty: 7, Price: 3.25},
		},
		Total: 312.23,
		Paid:  true,
		Tags:  []string{"priority", "gift", "repeat"},
		Note:  "leave at the front desk",
	}

	userBody = []byte(`{"name":"Alice","email":"a@example.com"}`)

	pageParams = UserPageParams{
		Title: "Team & Co <staff>",
		Rows: []Row{
			{Name: "Ada Lovelace", Email: "ada@example.com", Active: true},
			{Name: "Grace Hopper", Email: "grace@example.com", Active: false},
			{Name: "Alan <Turing>", Email: "alan@example.com", Active: true},
			{Name: "Katherine Johnson", Email: "kj@example.com", Active: true},
			{Name: "Barbara Liskov", Email: "liskov@example.com", Active: false},
		},
	}
)

var sinkOrder Order

// ---------- JSON document codec ----------

// DecodeJSON takes an io.Reader, so json.NewDecoder is the like-for-like
// comparison; json.Unmarshal over a ready []byte is the lower bound.
func BenchmarkJSONDecodeStdlib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v Order
		if err := json.NewDecoder(bytes.NewReader(orderJSON)).Decode(&v); err != nil {
			b.Fatal(err)
		}
		sinkOrder = v
	}
}

func BenchmarkJSONDecodeStdlibUnmarshal(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v Order
		if err := json.Unmarshal(orderJSON, &v); err != nil {
			b.Fatal(err)
		}
		sinkOrder = v
	}
}

func BenchmarkJSONDecodeGenerated(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		v, err := DecodeOrder(bytes.NewReader(orderJSON))
		if err != nil {
			b.Fatal(err)
		}
		sinkOrder = v
	}
}

func BenchmarkJSONEncodeStdlib(b *testing.B) {
	var w io.Writer = devNull{}
	b.ReportAllocs()
	for b.Loop() {
		if err := json.NewEncoder(w).Encode(order); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONEncodeGenerated(b *testing.B) {
	var w io.Writer = devNull{}
	b.ReportAllocs()
	for b.Loop() {
		if err := EncodeOrder(w, order); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------- whole HTTP handler ----------

func newUserRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", bytes.NewReader(userBody))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	r.SetPathValue("org_id", "acme")
	return r
}

// The handler benchmarks reuse one request and only reset its body, because
// httptest.NewRequest costs ~7 KB per call and would bury what bind and write
// actually do.
func resetBody(r *http.Request) {
	r.Body = io.NopCloser(bytes.NewReader(userBody))
}

func BenchmarkHandlerStdlib(b *testing.B) {
	w := devNull{}
	r := newUserRequest()
	b.ReportAllocs()
	for b.Loop() {
		resetBody(r)
		stdlibCreateUser(w, r)
	}
}

func BenchmarkHandlerGenerated(b *testing.B) {
	w := devNull{}
	r := newUserRequest()
	b.ReportAllocs()
	for b.Loop() {
		resetBody(r)
		CreateUser(w, r)
	}
}

// Same handlers with request construction included, for a whole-request view.
func BenchmarkHandlerStdlibWithRequest(b *testing.B) {
	w := devNull{}
	b.ReportAllocs()
	for b.Loop() {
		stdlibCreateUser(w, newUserRequest())
	}
}

func BenchmarkHandlerGeneratedWithRequest(b *testing.B) {
	w := devNull{}
	b.ReportAllocs()
	for b.Loop() {
		CreateUser(w, newUserRequest())
	}
}

// ---------- HTML template ----------

func BenchmarkHTMLStdlibTemplate(b *testing.B) {
	var w io.Writer = devNull{}
	b.ReportAllocs()
	for b.Loop() {
		if err := stdlibPage.Execute(w, pageParams); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHTMLGenerated(b *testing.B) {
	var w io.Writer = devNull{}
	b.ReportAllocs()
	for b.Loop() {
		if err := htmlbind.Render(w, UserPage(pageParams)); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------- equivalence, so the numbers compare like for like ----------

var wsRun = regexp.MustCompile(`\s+`)

func normalize(s string) string {
	s = wsRun.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "> <", "><")
	return strings.TrimSpace(s)
}

func TestHandlersProduceTheSameResponse(t *testing.T) {
	a, b := httptest.NewRecorder(), httptest.NewRecorder()
	stdlibCreateUser(a, newUserRequest())
	CreateUser(b, newUserRequest())

	var want, got map[string]any
	if err := json.Unmarshal(a.Body.Bytes(), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatalf("field count differs\n got %v\nwant %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %v want %v", k, got[k], v)
		}
	}
	if a.Code != b.Code {
		t.Fatalf("status: got %d want %d", b.Code, a.Code)
	}
}

func TestTemplatesProduceTheSameDocument(t *testing.T) {
	var want, got bytes.Buffer
	if err := stdlibPage.Execute(&want, pageParams); err != nil {
		t.Fatal(err)
	}
	if err := htmlbind.Render(&got, UserPage(pageParams)); err != nil {
		t.Fatal(err)
	}
	if normalize(got.String()) != normalize(want.String()) {
		t.Fatalf("documents differ\n got %s\nwant %s", normalize(got.String()), normalize(want.String()))
	}
}

func TestJSONCodecsAgree(t *testing.T) {
	var want Order
	if err := json.Unmarshal(orderJSON, &want); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeOrder(bytes.NewReader(orderJSON))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.Total != want.Total || len(got.Items) != len(want.Items) {
		t.Fatalf("decode differs\n got %+v\nwant %+v", got, want)
	}
	var a, b bytes.Buffer
	if err := json.NewEncoder(&a).Encode(order); err != nil {
		t.Fatal(err)
	}
	if err := EncodeOrder(&b, order); err != nil {
		t.Fatal(err)
	}
	var av, bv map[string]any
	if err := json.Unmarshal(a.Bytes(), &av); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b.Bytes(), &bv); err != nil {
		t.Fatal(err)
	}
	if len(av) != len(bv) {
		t.Fatalf("encoded field count differs")
	}
}
