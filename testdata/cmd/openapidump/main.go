package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/generator"
	_ "github.com/shibukawa/tinybind-go/internal/openapifixture"
)

func main() {
	outJSON := "openapi-sample.json"
	if len(os.Args) > 1 {
		outJSON = os.Args[1]
	}
	dir := filepath.Join("internal", "openapifixture")
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		panic(err)
	}
	js, err := doc.JSON()
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(outJSON, js, 0o644); err != nil {
		panic(err)
	}
	rec := httptest.NewRecorder()
	httpbind.OpenAPIJSON(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	registered, _ := httpbind.AssembleOpenAPI()
	fmt.Printf("serve_status=%d registered_len=%d body_has_31=%v\n",
		rec.Code, len(registered),
		len(rec.Body.Bytes()) > 0 && rec.Code == 200)
}
