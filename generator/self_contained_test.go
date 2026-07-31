package generator_test

import (
	"context"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// TestGeneratedArtifactsAreSelfContained checks that every phase combination
// produces gofmt-clean Go source whose imports are exactly what it references,
// with no goimports-equivalent pass by the caller.
func TestGeneratedArtifactsAreSelfContained(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		openAPI bool
		options func(generator.Options) generator.Options
	}{
		{
			name: "template only html",
			files: map[string]string{
				"doc.go": "package fixture\n",
				"page.pw.html": "package fixture\n\ntype User { name: string }\n\n" +
					"export component Page(user: User): html {\n<p>{user.name}</p>\n}\n",
			},
		},
		{
			name: "template only sql",
			files: map[string]string{
				"doc.go": "package fixture\n",
				"users.pw.sql": "package fixture\n\ntype Row { id: int }\n\n" +
					"export statement Find(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}\n",
			},
		},
		{
			// Two sources of one kind used to collide: the runtime helpers each
			// file emitted were hoisted by name, and any helper missing from
			// that list redeclared itself.
			name: "two sql sources in one package",
			files: map[string]string{
				"doc.go": "package fixture\n",
				"users.pw.sql": "package fixture\n\ntype Row { id: int }\n\n" +
					"export statement Find(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}\n",
				"orders.pw.sql": "package fixture\n\ntype Order { id: int }\n\n" +
					"export statement ListOrders(ids: int[]): sql.many<Order> {SELECT id FROM orders WHERE id IN ({ids})}\n",
			},
		},
		{
			name: "two html sources in one package",
			files: map[string]string{
				"doc.go": "package fixture\n",
				"page.pw.html": "package fixture\n\ntype Payload { name: string, count: int }\n\n" +
					"export component Page(data: Payload): html {\n<script>{JsonForScript(data)}</script>\n}\n",
				"card.pw.html": "package fixture\n\ntype Card { title: string, tags: string[] }\n\n" +
					"export component CardView(card: Card): html {\n<script>{JsonForScript(card)}</script>\n}\n",
			},
		},
		{
			name: "config only",
			files: map[string]string{
				"config.go": "package fixture\n\nimport \"tempmod/pw\"\n\n" +
					"type AppConfig struct {\n\tAddr string `default:\":8080\"`\n}\n\n" +
					"func Load() *AppConfig { return pw.RegisterConfig[AppConfig](\"app\") }\n",
			},
		},
		{
			name: "binder only",
			files: map[string]string{
				"handler.go": "package fixture\n\nimport (\n\t\"net/http\"\n\n\t\"tempmod/pw\"\n)\n\n" +
					"type Request struct {\n\tName string `json:\"name\"`\n}\n\n" +
					"func handle(w http.ResponseWriter, r *http.Request) {\n" +
					"\trequest, _ := pw.Parse[Request](r)\n\t_ = request\n}\n\n" +
					"func Register(mux *http.ServeMux) { mux.HandleFunc(\"POST /x\", handle) }\n",
			},
		},
		{
			name: "writer only no openapi",
			files: map[string]string{
				"handler.go": "package fixture\n\nimport (\n\t\"net/http\"\n\n\t\"tempmod/pw\"\n)\n\n" +
					"type Response struct {\n\tID int `json:\"id\"`\n}\n\n" +
					"func handle(w http.ResponseWriter, r *http.Request) {\n" +
					"\t_ = pw.WriteAPI(w, r, Response{ID: 1})\n}\n\n" +
					"func Register(mux *http.ServeMux) { mux.HandleFunc(\"GET /x\", handle) }\n",
			},
		},
		{
			name: "custom wrapper only",
			files: map[string]string{
				"handler.go": "package fixture\n\nimport (\n\t\"net/http\"\n\n\t\"tempmod/pw\"\n)\n\n" +
					"type Request struct {\n\tName string `json:\"name\"`\n}\n\n" +
					"type Response struct {\n\tID int `json:\"id\"`\n}\n\n" +
					"func handle(w http.ResponseWriter, r *http.Request) {\n" +
					"\trequest, _ := pw.Parse[Request](r)\n" +
					"\t_ = pw.WriteAPI(w, r, Response{ID: len(request.Name)})\n}\n\n" +
					"func Register(mux *http.ServeMux) { mux.HandleFunc(\"POST /x\", handle) }\n",
			},
			options: func(options generator.Options) generator.Options {
				options.RuntimePackages = generator.PatternSet[string]{Disabled: true}
				return options
			},
		},
		{
			name:    "everything with openapi",
			openAPI: true,
			files: map[string]string{
				"handler.go": "package fixture\n\nimport (\n\t\"net/http\"\n\n\t\"tempmod/pw\"\n)\n\n" +
					"type Request struct {\n\tName string `json:\"name\"`\n}\n\n" +
					"type Response struct {\n\tID int `json:\"id\"`\n}\n\n" +
					"func handle(w http.ResponseWriter, r *http.Request) {\n" +
					"\trequest, _ := pw.Parse[Request](r)\n" +
					"\t_ = pw.WriteAPI(w, r, Response{ID: len(request.Name)})\n}\n\n" +
					"func Register(mux *http.ServeMux) { mux.HandleFunc(\"POST /x\", handle) }\n",
				"config.go": "package fixture\n\nimport \"tempmod/pw\"\n\n" +
					"type AppConfig struct {\n\tAddr string `default:\":8080\"`\n}\n\n" +
					"func Load() *AppConfig { return pw.RegisterConfig[AppConfig](\"app\") }\n",
				"page.pw.html": "package fixture\n\nexport component Page(name: string): html {\n<p>{name}</p>\n}\n",
				"users.pw.sql": "package fixture\n\ntype Row { id: int }\n\n" +
					"export statement Find(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, fixture := newFrameworkModule(t, test.files)
			options := customFrameworkOptions(t)
			if test.options != nil {
				options = test.options(options)
			}
			artifacts, err := generator.New(options).GenerateArtifacts(
				context.Background(),
				generator.GenerateRequest{Dir: fixture, OpenAPI: test.openAPI},
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(artifacts) == 0 {
				t.Fatal("no artifacts generated")
			}
			for _, artifact := range artifacts {
				formatted, err := format.Source(artifact.Content)
				if err != nil {
					t.Fatalf("%s is not valid Go: %v\n%s", artifact.OutputBase, err, artifact.Content)
				}
				if string(formatted) != string(artifact.Content) {
					t.Fatalf("%s is not gofmt-clean", artifact.OutputBase)
				}
			}
			writeArtifacts(t, fixture, artifacts)
			compileModule(t, root)
		})
	}
}

// newFrameworkModule writes a temp module holding the pw framework package and
// a fixture package built from files.
func newFrameworkModule(t *testing.T, files map[string]string) (root, fixture string) {
	t.Helper()
	root = t.TempDir()
	writeTempModule(t, root)
	source, err := os.ReadFile(filepath.Join("..", "testdata", "custom_framework", "pw", "pw.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "pw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pw", "pw.go"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	fixture = filepath.Join(root, "fixture")
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(fixture, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidyTempModule(t, root)
	return root, fixture
}

// compileModule type-checks and links the module's test binaries without
// running any test, which is what a caller gets when the generated code must
// compile with no import fixup.
func compileModule(t *testing.T, root string) {
	t.Helper()
	command := exec.Command("go", "test", "-run=^$", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go test -run=^$ ./... failed: %v\n%s", err, output)
	}
}
