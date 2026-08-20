package mappingfixture_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/internal/mappingfixture"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

// skipWithoutToolchain skips a test that shells out to the Go toolchain. Those
// tests tidy a temp module against this one, which costs seconds apiece. Short
// mode is the fast loop; a full run skips nothing.
func skipWithoutToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("short mode: this test runs the Go toolchain against a temp module")
	}
}

func TestBind_JSONAndMetadata(t *testing.T) {
	body := `{"name":"Alice","email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	req.SetPathValue("org_id", "acme")

	got, err := httpbind.Bind[mappingfixture.CreateUserRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Alice" || got.Email != "a@example.com" {
		t.Fatalf("input fields: %+v", got)
	}
	if got.OrgID != "acme" {
		t.Fatalf("path org_id: %q", got.OrgID)
	}
	if got.Token != "Bearer secret" {
		t.Fatalf("header token: %q", got.Token)
	}
}

func TestBind_ProblemPlusJSONContentType(t *testing.T) {
	// RFC 6839 +json suffix (e.g. application/problem+json) must bind as JSON.
	body := `{"name":"Eve","email":"e@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/problem+json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer secret")
	req.SetPathValue("org_id", "acme")

	got, err := httpbind.Bind[mappingfixture.CreateUserRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Eve" || got.Email != "e@example.com" {
		t.Fatalf("plus-json bind: %+v", got)
	}
}

func TestBind_PayloadRestJSON(t *testing.T) {
	body := `{
		"name": "Ada",
		"email": "ada@example.com",
		"role": "admin",
		"meta": { "source": "import" }
	}`
	req := httptest.NewRequest(http.MethodPost, "/patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got, err := httpbind.Bind[mappingfixture.PatchWithExtrasRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Ada" || got.Email != "ada@example.com" {
		t.Fatalf("named fields: %+v", got)
	}
	if got.Extra == nil {
		t.Fatal("Extra should be non-nil empty-or-filled map")
	}
	if _, ok := got.Extra["name"]; ok {
		t.Fatalf("named key name must not appear in rest: %#v", got.Extra)
	}
	if _, ok := got.Extra["email"]; ok {
		t.Fatalf("named key email must not appear in rest: %#v", got.Extra)
	}
	if got.Extra["role"] != "admin" {
		t.Fatalf("role in rest: %#v", got.Extra["role"])
	}
	meta, ok := got.Extra["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta should be nested object, got %T %#v", got.Extra["meta"], got.Extra["meta"])
	}
	if meta["source"] != "import" {
		t.Fatalf("meta.source: %#v", meta["source"])
	}
}

func TestBind_PayloadRestJSON_EmptyExtras(t *testing.T) {
	body := `{"name":"Bob","email":"b@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got, err := httpbind.Bind[mappingfixture.PatchWithExtrasRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Extra == nil {
		t.Fatal("prefer non-nil empty rest map")
	}
	if len(got.Extra) != 0 {
		t.Fatalf("expected empty rest, got %#v", got.Extra)
	}
}

func TestBind_PayloadRestForm(t *testing.T) {
	form := "name=Cara&email=c@example.com&role=editor&note=hi"
	req := httptest.NewRequest(http.MethodPost, "/patch", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got, err := httpbind.Bind[mappingfixture.PatchWithExtrasRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Cara" || got.Email != "c@example.com" {
		t.Fatalf("named: %+v", got)
	}
	if got.Extra["role"] != "editor" || got.Extra["note"] != "hi" {
		t.Fatalf("form rest: %#v", got.Extra)
	}
	if _, ok := got.Extra["name"]; ok {
		t.Fatalf("name leaked into rest: %#v", got.Extra)
	}
}

func TestBind_PayloadRestRawJSON(t *testing.T) {
	body := `{"name":"Dan","flag":true,"n":3}`
	req := httptest.NewRequest(http.MethodPost, "/patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got, err := httpbind.Bind[mappingfixture.PatchWithExtrasRawRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Dan" {
		t.Fatalf("name: %q", got.Name)
	}
	if _, ok := got.Extra["name"]; ok {
		t.Fatalf("name in raw rest: %#v", got.Extra)
	}
	if string(got.Extra["flag"]) != "true" {
		t.Fatalf("flag raw: %s", got.Extra["flag"])
	}
	if string(got.Extra["n"]) != "3" {
		t.Fatalf("n raw: %s", got.Extra["n"])
	}
}

func TestBind_PayloadRest_NonObjectJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/patch", strings.NewReader(`[1,2,3]`))
	req.Header.Set("Content-Type", "application/json")
	_, err := httpbind.Bind[mappingfixture.PatchWithExtrasRequest](req)
	if err == nil {
		t.Fatal("expected error for non-object JSON with rest field")
	}
	he, ok := httpbind.AsHTTPError(err)
	if !ok || he.Status != http.StatusBadRequest {
		t.Fatalf("want 400 HTTPError, got %#v", err)
	}
}

func TestBind_PayloadRest_NullJSON(t *testing.T) {
	// JSON null unmarshals to a nil map; must not succeed as empty rest.
	req := httptest.NewRequest(http.MethodPost, "/patch", strings.NewReader(`null`))
	req.Header.Set("Content-Type", "application/json")
	_, err := httpbind.Bind[mappingfixture.PatchWithExtrasRequest](req)
	if err == nil {
		t.Fatal("expected error for JSON null body with rest field")
	}
	he, ok := httpbind.AsHTTPError(err)
	if !ok || he.Status != http.StatusBadRequest {
		t.Fatalf("want 400 HTTPError, got %#v", err)
	}
}

func TestBind_NestedOrderJSON(t *testing.T) {
	body := `{
		"customer": {"id": "c1", "name": "Ada"},
		"items": [
			{"sku": "A-1", "qty": 2},
			{"sku": "B-9", "qty": 1}
		],
		"labels": {"channel": "web", "priority": "high"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got, err := httpbind.Bind[mappingfixture.NestedOrderRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Customer.ID != "c1" || got.Customer.Name != "Ada" {
		t.Fatalf("customer: %+v", got.Customer)
	}
	if len(got.Items) != 2 || got.Items[0].SKU != "A-1" || got.Items[0].Qty != 2 || got.Items[1].SKU != "B-9" {
		t.Fatalf("items: %+v", got.Items)
	}
	if got.Labels["channel"] != "web" || got.Labels["priority"] != "high" {
		t.Fatalf("labels: %#v", got.Labels)
	}
}

func TestDecodeEncodeJSON_RoundTrip(t *testing.T) {
	in := mappingfixture.NestedOrderRequest{
		Customer: mappingfixture.NestedCustomer{ID: "c2", Name: "Bob"},
		Items: []mappingfixture.NestedLineItem{
			{SKU: "Z", Qty: 3},
		},
		Labels: map[string]string{"k": "v"},
	}
	var buf bytes.Buffer
	if err := jsonbind.EncodeJSON(&buf, in); err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	raw := buf.String()
	if !strings.Contains(raw, `"customer"`) || !strings.Contains(raw, `"Bob"`) {
		t.Fatalf("encoded JSON: %s", raw)
	}
	got, err := jsonbind.DecodeJSON[mappingfixture.NestedOrderRequest](strings.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got.Customer.Name != "Bob" || len(got.Items) != 1 || got.Items[0].Qty != 3 || got.Labels["k"] != "v" {
		t.Fatalf("round-trip: %+v", got)
	}
}

// TestEncodeJSON_NilSliceAndMapAreEmpty pins the encoder to what
// encoding/json/v2 writes: a nil slice is an empty array and a nil map an empty
// object, not the nulls encoding/json produced. Nothing in the Go type
// separates "no items" from "an empty list", so a client should not have to
// read that distinction off the wire.
func TestEncodeJSON_NilSliceAndMapAreEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := jsonbind.EncodeJSON(&buf, mappingfixture.NestedOrderRequest{}); err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	want := `{"customer":{"id":"","name":""},"items":[],"labels":{}}`
	if got := strings.TrimSpace(buf.String()); got != want {
		t.Errorf("zero value encodes as %s, want %s", got, want)
	}
}

// TestEncodeJSON_OmitTags covers the json tag options the encoder reads.
// omitempty follows encoding/json/v2 — it drops what would encode as an empty
// JSON value and leaves 0 and false alone — while omitzero drops the Go zero
// value. The pair pulls apart on a non-nil empty map: it is empty but not zero.
func TestEncodeJSON_OmitTags(t *testing.T) {
	full := mappingfixture.OmitTagged{
		Lead:   "l",
		Live:   3,
		Tags:   []string{"a"},
		Meta:   map[string]string{"k": "v"},
		Sub:    mappingfixture.OmitInner{A: "x"},
		Type:   "t",
		After:  "z",
		Secret: "not on the wire",
	}
	for name, tc := range map[string]struct {
		in   mappingfixture.OmitTagged
		want string
	}{
		"zero value keeps only the untagged member": {
			in:   mappingfixture.OmitTagged{},
			want: `{"type":""}`,
		},
		"every member set": {
			in:   full,
			want: `{"lead":"l","live":3,"tags":["a"],"meta":{"k":"v"},"sub":{"a":"x","b":[]},"type":"t","after":"z"}`,
		},
		"empty slice is empty, empty map is not zero": {
			in:   mappingfixture.OmitTagged{Tags: []string{}, Meta: map[string]string{}},
			want: `{"meta":{},"type":""}`,
		},
		"omitzero keeps a false-y number out but not a set one": {
			in:   mappingfixture.OmitTagged{Live: 0, After: "z"},
			want: `{"type":"","after":"z"}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := jsonbind.EncodeJSON(&buf, tc.in); err != nil {
				t.Fatalf("EncodeJSON: %v", err)
			}
			if got := strings.TrimSpace(buf.String()); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestDecodeJSON_DashSkipsBothWays checks the other half of json:"-": the
// member is not written, and one arriving under that name is not read either.
func TestDecodeJSON_DashSkipsBothWays(t *testing.T) {
	got, err := jsonbind.DecodeJSON[mappingfixture.OmitTagged](
		strings.NewReader(`{"type":"t","secret":"leaked"}`))
	if err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got.Type != "t" {
		t.Errorf("type: got %q", got.Type)
	}
	if got.Secret != "" {
		t.Errorf(`json:"-" field was decoded: %q`, got.Secret)
	}
}

// TestEncodeJSON_OmitBeforeRestMap covers the separator a rest map has to place
// when the only member in front of it may not be there.
func TestEncodeJSON_OmitBeforeRestMap(t *testing.T) {
	for name, tc := range map[string]struct {
		in   mappingfixture.OmitRest
		want string
	}{
		"member omitted": {
			in:   mappingfixture.OmitRest{Extra: map[string]any{"a": 1.0, "b": "two"}},
			want: `{"a":1,"b":"two"}`,
		},
		"member written": {
			in:   mappingfixture.OmitRest{Note: "n", Extra: map[string]any{"a": 1.0}},
			want: `{"note":"n","a":1}`,
		},
		"nothing at all": {
			in:   mappingfixture.OmitRest{},
			want: `{}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := jsonbind.EncodeJSON(&buf, tc.in); err != nil {
				t.Fatalf("EncodeJSON: %v", err)
			}
			if got := strings.TrimSpace(buf.String()); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestDecodeEncodeJSON_CodecOnlyType(t *testing.T) {
	// Type is registered via generated codecs; exercise Decode/Encode entry points.
	_ = mappingfixture.CodecOnlyNote{} // keep type linked
	note := mappingfixture.CodecOnlyNote{Text: "hello", N: 7}
	var buf bytes.Buffer
	if err := jsonbind.EncodeJSON(&buf, note); err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	got, err := jsonbind.DecodeJSON[mappingfixture.CodecOnlyNote](strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got.Text != "hello" || got.N != 7 {
		t.Fatalf("got %+v", got)
	}
}

func TestDecodeJSON_MissingCodec(t *testing.T) {
	type unregistered struct{ X string }
	_, err := jsonbind.DecodeJSON[unregistered](strings.NewReader(`{"X":"a"}`))
	if err == nil {
		t.Fatal("expected missing codec error")
	}
	if !strings.Contains(err.Error(), "no JSON decoder") && !strings.Contains(err.Error(), "decoder") {
		t.Fatalf("want missing-codec error, got %#v", err)
	}
}

func writeTempModule(t *testing.T, dir string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	mod := "module tempmod\n\ngo 1.25\n\nrequire github.com/shibukawa/tinybind-go v0.0.0\n\nreplace github.com/shibukawa/tinybind-go => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerator_DiscoversDecodeEncode(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import "github.com/shibukawa/tinybind-go/jsonbind"

type Note struct {
	Text string ` + "`payload:\"text\"`" + `
}

func use() {
	_, _ = jsonbind.DecodeJSON[Note](nil)
	_ = jsonbind.EncodeJSON[Note](nil, Note{})
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	skipWithoutToolchain(t)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	plan, err := generator.AnalyzePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range plan.Discovered {
		if n == "Note" {
			found = true
		}
	}
	if !found {
		t.Fatalf("DecodeJSON/EncodeJSON discovery missing Note: %v", plan.Discovered)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	for _, n := range []string{"RegisterDecode[Note]", "RegisterEncode[Note]", "decodeNoteJSON", "encodeNote"} {
		if !strings.Contains(s, n) {
			t.Fatalf("missing %q in generated code", n)
		}
	}
}

func TestBind_QueryInput(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users?name=Bob&email=b@example.com", nil)
	req.SetPathValue("org_id", "org1")
	req.Header.Set("Authorization", "t")

	got, err := httpbind.Bind[mappingfixture.CreateUserRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Bob" || got.Email != "b@example.com" {
		t.Fatalf("query input: %+v", got)
	}
	if got.OrgID != "org1" || got.Token != "t" {
		t.Fatalf("meta: %+v", got)
	}
}

func TestBind_SearchQueryAndPayload(t *testing.T) {
	body := `{"filter":"active"}`
	req := httptest.NewRequest(http.MethodPost, "/search?keyword=go&page=2", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got, err := httpbind.Bind[mappingfixture.SearchRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Keyword != "go" || got.Page != 2 {
		t.Fatalf("query fields: %+v", got)
	}
	if got.Filter != "active" {
		t.Fatalf("payload filter: %q", got.Filter)
	}
}

func TestBind_FormPayload(t *testing.T) {
	form := "name=Carol&email=c@example.com"
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("org_id", "o")
	req.Header.Set("Authorization", "tok")

	got, err := httpbind.Bind[mappingfixture.CreateUserRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Carol" || got.Email != "c@example.com" {
		t.Fatalf("form bind: %+v", got)
	}
}

func TestWrite_JSONResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	err := httpbind.Write[mappingfixture.CreateUserResponse](rec, req, mappingfixture.CreateUserResponse{
		ID:    "user_123",
		Name:  "Alice",
		Email: "a@example.com",
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type: %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v\n%s", err, rec.Body.String())
	}
	if body["id"] != "user_123" || body["name"] != "Alice" || body["email"] != "a@example.com" {
		t.Fatalf("body: %#v", body)
	}
}

func TestWriteError_ValidationProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	err := httpbind.Validation(
		httpbind.Field("email", "payload", "must be a valid email"),
	)
	httpbind.WriteError(rec, req, err)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/problem+json") {
		t.Fatalf("content-type: %q", ct)
	}
	// Avoid map[string]any + interface type asserts here: TinyGo's encoding/json
	// can panic with reflect.AssignableTo when that path is linked with RawMessage bind.
	raw := rec.Body.String()
	if !strings.Contains(raw, `"status":400`) {
		t.Fatalf("status missing in %s", raw)
	}
	if !strings.Contains(raw, `"field":"email"`) || !strings.Contains(raw, `"location":"payload"`) {
		t.Fatalf("field error missing in %s", raw)
	}
}

func TestWriteError_HidesInternalCause(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	httpbind.WriteError(rec, req, httpbind.Internal(io.ErrUnexpectedEOF))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "unexpected EOF") {
		t.Fatalf("internal cause leaked: %s", raw)
	}
}

func TestBind_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("org_id", "o")
	req.Header.Set("Authorization", "t")
	_, err := httpbind.Bind[mappingfixture.CreateUserRequest](req)
	if err == nil {
		t.Fatal("expected error")
	}
	he, ok := httpbind.AsHTTPError(err)
	if !ok || he.Status != http.StatusBadRequest {
		t.Fatalf("want 400 HTTPError, got %#v", err)
	}
}

func TestBind_InvalidJSONFieldMapsToHTTPValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":1,"email":"a@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("org_id", "o")
	req.Header.Set("Authorization", "t")
	_, err := httpbind.Bind[mappingfixture.CreateUserRequest](req)
	he, ok := httpbind.AsHTTPError(err)
	if !ok || he.Status != http.StatusBadRequest || len(he.Fields) != 1 {
		t.Fatalf("want one 400 validation field, got %#v", err)
	}
	if he.Fields[0].Field != "name" || he.Fields[0].Location != "payload" {
		t.Fatalf("unexpected field error: %+v", he.Fields[0])
	}
}

func buildMultipartRequest(t *testing.T, path string, fields map[string]string, fileField, filename, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if fileField != "" {
		part, err := w.CreateFormFile(fileField, filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestBind_MultipartFileAndScalars(t *testing.T) {
	const (
		filename = "avatar.png"
		content  = "fake-png-bytes"
		title    = "profile"
	)
	req := buildMultipartRequest(t, "/users/u42/avatar", map[string]string{
		"title": title,
	}, "image", filename, content)
	req.SetPathValue("user_id", "u42")

	got, err := httpbind.Bind[mappingfixture.UploadAvatarRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.UserID != "u42" {
		t.Fatalf("path user_id: %q", got.UserID)
	}
	if got.Title != title {
		t.Fatalf("title: %q", got.Title)
	}
	if got.Image.Filename != filename {
		t.Fatalf("filename: %q", got.Image.Filename)
	}
	if string(got.Image.Content) != content {
		t.Fatalf("content: %q", got.Image.Content)
	}
	if got.Image.Empty() {
		t.Fatal("Image should not be empty")
	}
	if got.Image.Size <= 0 && len(got.Image.Content) == 0 {
		t.Fatalf("size: %d", got.Image.Size)
	}
}

func TestBind_MultipartMissingFile(t *testing.T) {
	req := buildMultipartRequest(t, "/users/u1/avatar", map[string]string{
		"title": "only-title",
	}, "", "", "")
	req.SetPathValue("user_id", "u1")

	_, err := httpbind.Bind[mappingfixture.UploadAvatarRequest](req)
	if err == nil {
		t.Fatal("expected missing file error")
	}
	he, ok := httpbind.AsHTTPError(err)
	if !ok || he.Status != http.StatusBadRequest {
		t.Fatalf("want 400 HTTPError, got %#v", err)
	}
	found := false
	for _, f := range he.Fields {
		if f.Field == "image" && f.Location == "payload" && strings.Contains(f.Message, "missing") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected image missing field error, got %+v", he.Fields)
	}
}

func TestBind_MultipartTooLarge(t *testing.T) {
	req := buildMultipartRequest(t, "/users/u1/avatar", map[string]string{
		"title": "x",
	}, "image", "big.bin", strings.Repeat("z", 200))
	req.SetPathValue("user_id", "u1")
	// Re-wrap body with MaxBytesReader so parse hits size limit.
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = req.Body.Close()
	req.Body = http.MaxBytesReader(nil, io.NopCloser(bytes.NewReader(body)), 40)
	req.ContentLength = int64(len(body))

	_, err = httpbind.Bind[mappingfixture.UploadAvatarRequest](req)
	if err == nil {
		t.Fatal("expected size error")
	}
	he, ok := httpbind.AsHTTPError(err)
	if !ok || he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413 HTTPError, got %#v status=%v", err, func() any {
			if ok {
				return he.Status
			}
			return nil
		}())
	}
}

func TestBind_CreateUser_MultipartScalars(t *testing.T) {
	// Non-File request models still bind scalar fields from multipart form values.
	req := buildMultipartRequest(t, "/orgs/o/users", map[string]string{
		"name":  "Dana",
		"email": "d@example.com",
	}, "", "", "")
	req.SetPathValue("org_id", "o")
	req.Header.Set("Authorization", "tok")

	got, err := httpbind.Bind[mappingfixture.CreateUserRequest](req)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.Name != "Dana" || got.Email != "d@example.com" {
		t.Fatalf("multipart scalars: %+v", got)
	}
}

func TestGenerator_EmitsTypeSpecificNoReflect(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	// copy types into temp package
	src, err := os.ReadFile("types.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "types.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	skipWithoutToolchain(t)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	// package name in types.go is mappingfixture — keep it
	opts := generator.DefaultOptions()
	opts.GenerateAll = true
	out, err := generator.New(opts).Generate(dir, dir, "tinybind_gen.go")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	code := string(data)
	if !strings.Contains(code, "func bindCreateUserRequest") {
		t.Fatalf("missing bindCreateUserRequest in:\n%s", code)
	}
	if !strings.Contains(code, "func writeCreateUserResponse") {
		t.Fatalf("missing writeCreateUserResponse in:\n%s", code)
	}
	if !strings.Contains(code, "RegisterBind[CreateUserRequest]") {
		t.Fatalf("missing registration in:\n%s", code)
	}
	if !strings.Contains(code, "func bindUploadAvatarRequest") {
		t.Fatalf("missing bindUploadAvatarRequest in:\n%s", code)
	}
	if !strings.Contains(code, "httpbind.ReadJSONBody(r)") && !strings.Contains(code, "httpbind.ReadJSONBodyOwned(r)") {
		t.Fatalf("missing httpbind.ReadJSONBody in:\n%s", code)
	}
	if !strings.Contains(code, "httpbind.ReadFormBody(r,") {
		t.Fatalf("missing httpbind.ReadFormBody in:\n%s", code)
	}
	if strings.Contains(code, "\"reflect\"") || strings.Contains(code, "reflect.") {
		t.Fatalf("generated code must not use reflect:\n%s", code)
	}
	// field sources present as literals / calls
	for _, needle := range []string{
		`PathValue(r, "org_id")`,
		`HeaderValue(r, "Authorization")`,
		`QueryLookup(queryVals, "name")`,
		`QueryLookup(queryVals, "keyword")`,
		`fileBody["image"]`,
	} {
		if !strings.Contains(code, needle) {
			t.Fatalf("missing %s in generated code", needle)
		}
	}
}

func TestGeneratedFile_NoReflectImport(t *testing.T) {
	data, err := os.ReadFile("tinybind_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("\"reflect\"")) || bytes.Contains(data, []byte("reflect.")) {
		t.Fatal("committed generated file imports/uses reflect")
	}
}

func TestRoundTrip_HandlerStyle(t *testing.T) {
	// Real user path: Bind → service value → Write.
	// Call the handler directly (not via Go 1.22 method-path ServeMux patterns),
	// so TinyGo's net/http (without full pattern routing) can exercise the same I/O.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input, err := httpbind.Bind[mappingfixture.CreateUserRequest](r)
		if err != nil {
			httpbind.WriteError(w, r, err)
			return
		}
		out := mappingfixture.CreateUserResponse{
			ID:    "user_123",
			Name:  input.Name,
			Email: input.Email,
		}
		if err := httpbind.Write[mappingfixture.CreateUserResponse](w, r, out); err != nil {
			httpbind.WriteError(w, r, err)
		}
	})

	body := `{"name":"Alice","email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer x")
	req.SetPathValue("org_id", "acme")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var m map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["id"] != "user_123" || m["name"] != "Alice" || m["email"] != "a@example.com" {
		t.Fatalf("response: %#v", m)
	}
}
