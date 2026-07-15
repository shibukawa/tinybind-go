package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/shibukawa/httpbind-go"
	"github.com/shibukawa/httpbind-go/generator"
	_ "github.com/shibukawa/httpbind-go/internal/openapifixture"
)

func main() {
	outJSON, outYAML := "openapi-sample.json", "openapi-sample.yaml"
	if len(os.Args) > 1 {
		outJSON = os.Args[1]
	}
	if len(os.Args) > 2 {
		outYAML = os.Args[2]
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
	ya, err := doc.YAML()
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(outYAML, ya, 0o644); err != nil {
		panic(err)
	}

	rec := httptest.NewRecorder()
	httpbinder.OpenAPIJSON(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	fmt.Printf("serve_status=%d registered_len=%d body_has_31=%v\n",
		rec.Code, len(httpbinder.OpenAPIDocumentJSON()),
		len(rec.Body.Bytes()) > 0 && rec.Code == 200)
}
