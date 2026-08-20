package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// cborHTTPFixtureSource is one route binding a payload and answering a typed
// response, which is the whole surface EnableCBORHTTP touches.
const cborHTTPFixtureSource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Tag struct {
	Label string ` + "`json:\"label\"`" + `
}

type CreateUserRequest struct {
	Name string  ` + "`payload:\"name\" check:\"required\"`" + `
	Age  int     ` + "`payload:\"age\"`" + `
	Tags []Tag   ` + "`payload:\"tags\"`" + `
	Bio  string  ` + "`query:\"bio\"`" + `
}

type User struct {
	ID   string  ` + "`json:\"id\"`" + `
	Name string  ` + "`json:\"name\"`" + `
	Age  int     ` + "`json:\"age\"`" + `
	Tags []Tag   ` + "`json:\"tags\"`" + `
}

func Handler(w http.ResponseWriter, r *http.Request) {
	req, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[User](w, r, User{ID: "1", Name: req.Name, Age: req.Age, Tags: req.Tags})
}

func main() {
	http.HandleFunc("POST /users", Handler)
}
`

// emitCBORHTTP analyzes and emits the fixture with the option on.
func emitCBORHTTP(t *testing.T, src string, opts generator.Options) (string, string) {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.AnalyzePackageWithOptions(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return dir, string(code)
}

// The option is the switch: a run leaving it off emits no CBOR spelling at
// all, which is the size guarantee the option exists to give.
func TestCBORHTTPOffEmitsNoCBOR(t *testing.T) {
	_, source := emitCBORHTTP(t, cborHTTPFixtureSource, generator.DefaultOptions())
	mustContainNone(t, source, "CBORHTTP", "cbor.", "AcceptsCBOR", "IsCBORRequest")
}

func TestCBORHTTPEmitsNegotiationArms(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	_, source := emitCBORHTTP(t, cborHTTPFixtureSource, opts)
	mustContainAll(t, source,
		"github.com/shibukawa/tinygodriver/encoding/cbor",
		"httpbind.IsCBORRequest(r)",
		"cborBody, err = httpbind.ReadCBORBody(r)",
		"httpbind.AcceptsCBOR(r)",
		"func appendUserCBORHTTP(dst []byte, v User) []byte",
		"func appendTagCBORHTTP(dst []byte, v Tag) []byte",
		"httpbind.WriteCBORBytes(w, http.StatusOK, *buf)",
		"func cborHTTPReadOptions(n int) cbor.DecoderOptions",
	)
}

// The generated pieces have to answer over a real request in both formats,
// which is what proves the negotiation and the codecs name each other
// correctly rather than merely appearing.
func TestCBORHTTPRoundTripsOverHTTP(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	dir, code := emitCBORHTTP(t, cborHTTPFixtureSource, opts)
	if err := os.WriteFile(filepath.Join(dir, "tinybind_gen.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := `package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

func cborRequestBody() []byte {
	body := cbor.AppendMapHeader(nil, 3)
	body = cbor.AppendText(body, "name")
	body = cbor.AppendText(body, "ada")
	body = cbor.AppendText(body, "age")
	body = cbor.AppendInt(body, 30)
	body = cbor.AppendText(body, "tags")
	body = cbor.AppendArrayHeader(body, 1)
	body = cbor.AppendMapHeader(body, 1)
	body = cbor.AppendText(body, "label")
	body = cbor.AppendText(body, "admin")
	return body
}

func TestCBORInCBOROut(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(cborRequestBody()))
	r.Header.Set("Content-Type", "application/cbor")
	r.Header.Set("Accept", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %x", w.Code, w.Body.Bytes())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("content type %q", ct)
	}
	// The response is a map in struct field order: id, name, age, tags.
	want := cbor.AppendMapHeader(nil, 4)
	want = cbor.AppendText(want, "id")
	want = cbor.AppendText(want, "1")
	want = cbor.AppendText(want, "name")
	want = cbor.AppendText(want, "ada")
	want = cbor.AppendText(want, "age")
	want = cbor.AppendInt(want, 30)
	want = cbor.AppendText(want, "tags")
	want = cbor.AppendArrayHeader(want, 1)
	want = cbor.AppendMapHeader(want, 1)
	want = cbor.AppendText(want, "label")
	want = cbor.AppendText(want, "admin")
	if !bytes.Equal(w.Body.Bytes(), want) {
		t.Fatalf("body %x, want %x", w.Body.Bytes(), want)
	}
}

// A JSON client is untouched by the option: same request, same bytes.
func TestJSONClientStaysJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(` + "`" + `{"name":"ada","age":30,"tags":[{"label":"admin"}]}` + "`" + `))
	r.Header.Set("Content-Type", "application/json")
	Handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
	const want = ` + "`" + `{"id":"1","name":"ada","age":30,"tags":[{"label":"admin"}]}` + "`" + `
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
}

// A CBOR body may answer JSON when the client did not ask for CBOR back, and
// an unknown member is skipped the way a JSON one is collected or ignored.
func TestCBORInJSONOutAndUnknownKeySkipped(t *testing.T) {
	body := cbor.AppendMapHeader(nil, 2)
	body = cbor.AppendText(body, "unknown")
	body = cbor.AppendText(body, "ignored")
	body = cbor.AppendText(body, "name")
	body = cbor.AppendText(body, "ada")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
}

// The required check reads the same presence the JSON path records.
func TestCBORMissingRequiredMemberIs400(t *testing.T) {
	body := cbor.AppendMapHeader(nil, 1)
	body = cbor.AppendText(body, "age")
	body = cbor.AppendInt(body, 30)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}

// A body that is not a CBOR map is a 400, not a panic and not a 500.
func TestCBORMalformedBodyIs400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("not cbor"))
	r.Header.Set("Content-Type", "application/cbor")
	Handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(probe), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated negotiation does not answer: %v\n%s\n%s", err, output, code)
	}
}

// A payload rest map has no CBOR mapping, and silence would drop members, so
// the run is refused with the field named.
func TestCBORHTTPRefusesARestMap(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type PatchRequest struct {
	Name   string         ` + "`payload:\"name\"`" + `
	Extras map[string]any ` + "`payload:\"*\"`" + `
}

func main() {
	http.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {
		_, _ = httpbind.Bind[PatchRequest](r)
	})
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	plan, err := generator.AnalyzePackageWithOptions(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	_, err = generator.Emit(plan)
	if err == nil {
		t.Fatal("want an error for a payload rest map under EnableCBORHTTP")
	}
	if !strings.Contains(err.Error(), "PatchRequest.Extras") || !strings.Contains(err.Error(), "rest map") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// RejectFloats moves the float refusal to generation time, where the field can
// be named, rather than to the first request that carries one.
func TestCBORHTTPProfileRejectsAFloatField(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type Reading struct {
	Value float64 ` + "`payload:\"value\"`" + `
}

func main() {
	http.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {
		_, _ = httpbind.Bind[Reading](r)
	})
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	opts.CBORHTTPProfile.RejectFloats = true
	plan, err := generator.AnalyzePackageWithOptions(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	_, err = generator.Emit(plan)
	if err == nil {
		t.Fatal("want an error for a float64 field under RejectFloats")
	}
	if !strings.Contains(err.Error(), "Reading.Value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// RequireSortedKeys settles member order at generation time: the encoded keys
// come out in RFC 8949 bytewise order rather than declaration order.
func TestCBORHTTPSortedKeysReorderMembers(t *testing.T) {
	opts := generator.DefaultOptions()
	opts.EnableCBORHTTP = true
	opts.CBORHTTPProfile.RequireSortedKeys = true
	_, source := emitCBORHTTP(t, cborHTTPFixtureSource, opts)
	// User declares id, name, age, tags; bytewise order of the encoded text
	// keys sorts shorter keys first: id, age, name, tags.
	fn := source[strings.Index(source, "func appendUserCBORHTTP"):]
	fn = fn[:strings.Index(fn, "\n}\n")]
	order := []string{`"id"`, `"age"`, `"name"`, `"tags"`}
	last := -1
	for _, key := range order {
		at := strings.Index(fn, "cbor.AppendText(dst, "+key+")")
		if at < 0 {
			t.Fatalf("key %s not emitted:\n%s", key, fn)
		}
		if at < last {
			t.Fatalf("key %s out of order:\n%s", key, fn)
		}
		last = at
	}
}
