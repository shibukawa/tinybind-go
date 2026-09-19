package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// tomlBindSource names one type from a reader call, one from a declaration,
// and leaves one struct unnamed, so a fixture covers discovery, the
// declaration, nested tables, an array of tables, a float and a default.
const tomlBindSource = `package main

import (
	"os"
	"time"

	"github.com/shibukawa/tinybind-go/tomlbind"
)

type TLS struct {
	Enabled  bool
	CertPath string ` + "`key:\"cert\"`" + `
}

type Listener struct {
	Addr string
	Port int ` + "`default:\"80\"`" + `
}

type Settings struct {
	Name      string        ` + "`default:\"svc\"`" + `
	Ratio     float64
	Scale     float32
	Timeout   time.Duration ` + "`default:\"5s\"`" + `
	Origins   []string
	TLS       TLS
	Listeners []Listener
}

type Manifest struct {
	Version uint16
}

var _ = tomlbind.GenerateDecoder[Manifest]()

type Unrelated struct {
	A int
}

func main() {
	f, _ := os.Open("settings.toml")
	if s, err := tomlbind.DecodeTOML[Settings](f); err == nil {
		_ = s
	}
}
`

func emitTOMLBind(t *testing.T, src string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	result, err := generator.New(generator.DefaultOptions()).GeneratePackage(t.Context(), generator.GenerateRequest{Dir: dir})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.TOMLBindPath == "" {
		return dir, ""
	}
	code, err := os.ReadFile(result.TOMLBindPath)
	if err != nil {
		t.Fatal(err)
	}
	return dir, string(code)
}

// A call and a declaration each name a type; a struct nothing names gets
// nothing, and nested structs are inlined rather than given decoders.
func TestTOMLBindDiscoversCallsAndDeclarations(t *testing.T) {
	_, source := emitTOMLBind(t, tomlBindSource)
	for _, want := range []string{
		"func decodeSettingsTOML(dst *Settings, doc minitoml.Document) error",
		"func (v *Settings) DecodeTOMLFrom(data []byte) error",
		"func (v *Manifest) DecodeTOMLFrom(data []byte) error",
		`doc.Get("tls.cert")`,
		`doc.Get("ratio")`,
		"x, err := v.AsFloat()",
		"dst.Scale = float32(x)",
		`"tomlbind: listeners: %w"`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated source lacks %q\n%s", want, source)
		}
	}
	mustContainNone(t, source, "Unrelated", "func (v *TLS)", "func (v *Listener)", "minitoml: ", "configbind")
}

// A project calling no entry point writes no file at all.
func TestTOMLBindOffEmitsNothing(t *testing.T) {
	src := `package main

type Unused struct {
	A int
}

func main() {}
`
	dir, source := emitTOMLBind(t, src)
	if source != "" {
		t.Fatalf("a package calling nothing emitted a decoder:\n%s", source)
	}
	if _, err := os.Stat(filepath.Join(dir, "tomlbind_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("tomlbind_gen.go exists: %v", err)
	}
}

// A declaration naming a type from another package is refused with the
// remedy, since the method has to land beside the type.
func TestTOMLBindDeclarationRefusesAForeignType(t *testing.T) {
	src := `package main

import (
	"net/url"

	"github.com/shibukawa/tinybind-go/tomlbind"
)

var _ = tomlbind.GenerateDecoder[url.URL]()

func main() {}
`
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	_, err := generator.New(generator.DefaultOptions()).GeneratePackage(t.Context(), generator.GenerateRequest{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "net/url") || !strings.Contains(err.Error(), "beside the type") {
		t.Fatalf("a foreign declaration was not refused with the remedy: %v", err)
	}
}

// The feature switch removes the pass and leaves everything else standing.
func TestTOMLBindFeatureDisable(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(tomlBindSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	options := generator.DefaultOptions()
	options.DisableFeatures = []generator.Feature{generator.FeatureDecodeTOML}
	result, err := generator.New(options).GeneratePackage(t.Context(), generator.GenerateRequest{Dir: dir})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.TOMLBindPath != "" {
		t.Fatalf("the disabled feature still wrote %s", result.TOMLBindPath)
	}
}

// The end of the line: the generated method makes the entry point compile,
// and the document lands in the struct the way configbind would bind it.
func TestTOMLBindDecodesCompiled(t *testing.T) {
	dir, _ := emitTOMLBind(t, tomlBindSource)
	probe := `package main

import (
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/tomlbind"
)

const doc = ` + "`" + `
name = "edge"
ratio = 0.25
scale = 3
timeout = "250ms"
origins = ["https://a.example", "https://b.example"]
tls.enabled = true

[tls]
cert = "/etc/edge.pem"

[[listeners]]
addr = ":8080"

[[listeners]]
addr = ":8443"
port = 8443
` + "`" + `

func TestDecodesTheDocument(t *testing.T) {
	got, err := tomlbind.DecodeTOML[Settings](strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "edge" || got.Ratio != 0.25 || got.Scale != 3 || got.Timeout != 250*time.Millisecond {
		t.Fatalf("scalars: %+v", got)
	}
	if len(got.Origins) != 2 || got.Origins[1] != "https://b.example" {
		t.Fatalf("origins: %+v", got.Origins)
	}
	if !got.TLS.Enabled || got.TLS.CertPath != "/etc/edge.pem" {
		t.Fatalf("tls: %+v", got.TLS)
	}
	if len(got.Listeners) != 2 || got.Listeners[0].Port != 80 || got.Listeners[1].Port != 8443 {
		t.Fatalf("listeners: %+v", got.Listeners)
	}
}

func TestDefaultsFillAnEmptyDocument(t *testing.T) {
	got, err := tomlbind.DecodeTOMLBytes[Settings](nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "svc" || got.Timeout != 5*time.Second || got.Ratio != 0 {
		t.Fatalf("defaults: %+v", got)
	}
}

func TestADecodeReplacesTheValue(t *testing.T) {
	v := Settings{Name: "stale", Ratio: 9}
	if err := v.DecodeTOMLFrom([]byte("name = \"fresh\"")); err != nil {
		t.Fatal(err)
	}
	if v.Name != "fresh" || v.Ratio != 0 {
		t.Fatalf("a decode kept the old value: %+v", v)
	}
}

func TestParseErrorsCarryLineAndColumn(t *testing.T) {
	_, err := tomlbind.DecodeTOMLBytes[Settings]([]byte("name = \"a\"\nname = \"b\"\n"))
	if err == nil || !strings.Contains(err.Error(), "2:") {
		t.Fatalf("a duplicate key was not refused with its line: %v", err)
	}
	_, err = tomlbind.DecodeTOMLBytes[Settings]([]byte("tls = { enabled = true }"))
	if err == nil {
		t.Fatal("an inline table decoded")
	}
}

func TestAWrongKindNamesTheKey(t *testing.T) {
	_, err := tomlbind.DecodeTOMLBytes[Manifest]([]byte("version = 1.5"))
	if err == nil || !strings.Contains(err.Error(), "tomlbind: version") {
		t.Fatalf("a float in an integer field was not refused by key: %v", err)
	}
	_, err = tomlbind.DecodeTOMLBytes[Manifest]([]byte("version = 70000"))
	if err == nil || !strings.Contains(err.Error(), "out of range for uint16") {
		t.Fatalf("an overflowing integer was not refused by width: %v", err)
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
		t.Fatalf("generated decoders do not decode: %v\n%s", err, output)
	}
}
