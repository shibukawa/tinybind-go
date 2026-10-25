package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestBuildOpenAPI_WriteStatusCreated(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/httpbind"
)

type CreateReq struct {
	Name string ` + "`payload:\"name\"`" + `
}

type CreateResp struct {
	ID string ` + "`json:\"id\"`" + `
}

func create(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[CreateReq](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = in
	_ = httpbind.WriteStatus[CreateResp](w, r, http.StatusCreated, CreateResp{ID: "1"})
}

func register(mux *http.ServeMux) {
	mux.HandleFunc("POST /items", create)
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)

	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	paths, _ := doc["paths"].(map[string]any)
	item, _ := paths["/items"].(map[string]any)
	post, _ := item["post"].(map[string]any)
	responses, _ := post["responses"].(map[string]any)
	if _, ok := responses["201"]; !ok {
		t.Fatalf("expected 201 response, got keys %v\n%#v", keysOf(responses), responses)
	}
	if _, ok := responses["200"]; ok {
		// only WriteStatus Created — 200 should not appear unless Write also used
		t.Fatalf("unexpected 200 when only WriteStatus Created: %#v", responses)
	}
	// schema present under 201
	r201, _ := responses["201"].(map[string]any)
	content, _ := r201["content"].(map[string]any)
	if content == nil {
		t.Fatalf("201 missing content: %#v", r201)
	}
}

func keysOf(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestBuildOpenAPI_WriteStill200(t *testing.T) {
	dir := filepath.Join("..", "internal", "openapifixture")
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := doc.JSON()
	if !strings.Contains(string(raw), `"200"`) {
		t.Fatalf("expected 200 in openapi fixture output")
	}
}
