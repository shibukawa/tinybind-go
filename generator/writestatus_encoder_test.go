package generator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// httpbind.WriteStatus serializes through jsonbind.EncodeJSON rather than the
// registered writer, so a response type reached only through WriteStatus needs
// its encoder registered. The OpenAPI test next door asserts the document and
// never calls the function, which is how a missing registration stayed
// invisible: the encoder body is emitted either way, only the init entry is
// absent, and the failure surfaces as missing_codec at the first request.
func TestGenerate_WriteStatusRegistersEncoder(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
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

	out, err := generator.Generate(dir, dir, "tinybind_gen.go")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	gen := string(data)
	for _, want := range []string{
		"httpbind.RegisterWrite[CreateResp](writeCreateResp)",
		"jsonbind.RegisterEncode[CreateResp](encodeCreateResp)",
	} {
		if !strings.Contains(gen, want) {
			t.Errorf("generated init missing %q:\n%s", want, gen)
		}
	}
}

// The fasthttp backend emits the same registration text against a different
// runtime import, and fasthttpbind.WriteStatus reaches the same shared jsonbind
// registry, so the missing encoder was missing on both transports. The usage
// mapping is keyed on the operation rather than the transport, which is why one
// mapping entry covers both; this pins that the fasthttp half really follows.
func TestGenerate_WriteStatusRegistersEncoderForFasthttpBackend(t *testing.T) {
	transform := generator.DefaultTransformOptions()
	options := generator.DefaultOptions()
	options.Transform = &transform
	result, err := generator.New(options).GeneratePackage(context.Background(), generator.GenerateRequest{
		Dir: filepath.Join("..", "testdata", "transform_rewrite"),
		Out: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.FastBindersPath == "" {
		t.Fatal("no fasthttp mapping file was written")
	}
	data, err := os.ReadFile(result.FastBindersPath)
	if err != nil {
		t.Fatal(err)
	}
	gen := string(data)
	for _, want := range []string{
		`httpbind "github.com/shibukawa/tinybind-go/fasthttpbind"`,
		"httpbind.RegisterWrite[CreateUserResponse](writeCreateUserResponse)",
		"jsonbind.RegisterEncode[CreateUserResponse](encodeCreateUserResponse)",
	} {
		if !strings.Contains(gen, want) {
			t.Errorf("fasthttp mapping missing %q:\n%s", want, gen)
		}
	}
}
