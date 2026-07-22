package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinygodriver/httpmux"
)

type problemResponse struct {
	Code   string `json:"code"`
	Errors []struct {
		Field    string `json:"field"`
		Location string `json:"location"`
		Message  string `json:"message"`
	} `json:"errors"`
}

func TestDemoRoutes_CheckValidationRunsInsideBind(t *testing.T) {
	mux := httpmux.NewServeMux()
	RegisterDemoRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create validation status %d %s", rec.Code, rec.Body.String())
	}
	var problem problemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "validation_failed" || len(problem.Errors) != 2 {
		t.Fatalf("create validation %#v", problem)
	}
	wantFields := map[string]bool{"name": false, "email": false}
	for _, field := range problem.Errors {
		if field.Location != "input" {
			t.Fatalf("default input location = %q, want input: %#v", field.Location, problem.Errors)
		}
		if field.Message == "required" {
			if _, ok := wantFields[field.Field]; ok {
				wantFields[field.Field] = true
			}
		}
	}
	for field, found := range wantFields {
		if !found {
			t.Fatalf("missing required error for %s: %#v", field, problem.Errors)
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(`{"name":"Alice","email":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "valid email") {
		t.Fatalf("email validation status %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/search?keyword=go", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"page":1`) {
		t.Fatalf("search default status %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"field":"message"`) {
		t.Fatalf("echo validation status %d %s", rec.Code, rec.Body.String())
	}
}

func TestDemoRoutes_Smoke(t *testing.T) {
	mux := httpmux.NewServeMux()
	RegisterDemoRoutes(mux)

	// index page is rendered by the generated typed HTML template.
	recI := httptest.NewRecorder()
	mux.ServeHTTP(recI, httptest.NewRequest(http.MethodGet, "/", nil))
	if recI.Code != http.StatusOK {
		t.Fatalf("index %d %s", recI.Code, recI.Body.String())
	}
	indexBody := recI.Body.String()
	for _, want := range []string{
		"<title>httpbind demo</title>",
		"async function runStream",
		"JSON.stringify({ message: msg })",
		`-d '{"name":"Alice","email":"a@example.com"}'`,
	} {
		if !strings.Contains(indexBody, want) {
			t.Fatalf("index body missing %q: %s", want, indexBody)
		}
	}

	// create user
	body := `{"name":"Alice","email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/orgs/acme/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created["name"] != "Alice" || created["org_id"] != "acme" {
		t.Fatalf("created %#v", created)
	}

	// openapi
	recO := httptest.NewRecorder()
	mux.ServeHTTP(recO, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if recO.Code != http.StatusOK {
		t.Fatalf("openapi %d", recO.Code)
	}
	var doc map[string]any
	if err := json.Unmarshal(recO.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi version %#v", doc["openapi"])
	}
	paths := doc["paths"].(map[string]any)
	if paths["/orgs/{org_id}/users"] == nil {
		t.Fatalf("paths %#v", paths)
	}

	// stream: curl UA → NDJSON; multi Write; meta event includes format
	recS := httptest.NewRecorder()
	reqS := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"message":"x"}`))
	reqS.Header.Set("Content-Type", "application/json")
	reqS.Header.Set("User-Agent", "curl/8.0")
	mux.ServeHTTP(recS, reqS)
	if recS.Code != http.StatusOK {
		t.Fatalf("stream %d %s", recS.Code, recS.Body.String())
	}
	if !strings.Contains(recS.Header().Get("Content-Type"), "ndjson") {
		t.Fatalf("ctype %q", recS.Header().Get("Content-Type"))
	}
	streamBody := recS.Body.String()
	if !strings.Contains(streamBody, `"type":"done"`) || !strings.Contains(streamBody, `"delta":"ndjson"`) {
		t.Fatalf("body %s", streamBody)
	}
	if lines := strings.Split(strings.TrimSpace(streamBody), "\n"); len(lines) < 4 {
		t.Fatalf("expected multiple NDJSON lines, got %d", len(lines))
	}

	recSSE := httptest.NewRecorder()
	reqSSE := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"message":"x"}`))
	reqSSE.Header.Set("Content-Type", "application/json")
	reqSSE.Header.Set("Accept", "text/event-stream")
	mux.ServeHTTP(recSSE, reqSSE)
	if !strings.Contains(recSSE.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("sse ctype %q", recSSE.Header().Get("Content-Type"))
	}
	if !strings.Contains(recSSE.Body.String(), "data:") || strings.Count(recSSE.Body.String(), "data:") < 4 {
		t.Fatalf("sse body %s", recSSE.Body.String())
	}

	// stream: Accept application/json → single JSON array document (not JSONL)
	recJA := httptest.NewRecorder()
	reqJA := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{"message":"x"}`))
	reqJA.Header.Set("Content-Type", "application/json")
	reqJA.Header.Set("Accept", "application/json")
	mux.ServeHTTP(recJA, reqJA)
	if recJA.Code != http.StatusOK {
		t.Fatalf("json-array status %d %s", recJA.Code, recJA.Body.String())
	}
	if !strings.Contains(recJA.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("json-array ctype %q", recJA.Header().Get("Content-Type"))
	}
	var events []map[string]any
	if err := json.Unmarshal(recJA.Body.Bytes(), &events); err != nil {
		t.Fatalf("json-array body not array: %v %s", err, recJA.Body.String())
	}
	if len(events) < 4 {
		t.Fatalf("json-array expected multiple events, got %d %s", len(events), recJA.Body.String())
	}

	// Swagger UI docs page
	recD := httptest.NewRecorder()
	mux.ServeHTTP(recD, httptest.NewRequest(http.MethodGet, "/docs/", nil))
	if recD.Code != http.StatusOK {
		t.Fatalf("docs %d %s", recD.Code, recD.Body.String())
	}
	if !strings.Contains(recD.Body.String(), "SwaggerUIBundle") || !strings.Contains(recD.Body.String(), "/openapi.json") {
		t.Fatalf("docs body missing swagger bootstrap")
	}
}
