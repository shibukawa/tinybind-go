package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

func TestFrameworkWrappersDriveMappingConfigBindAndOpenAPI(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "framework"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "framework", "framework.go"), `package framework
import (
 "context"
 "net/http"
 httpbind "github.com/shibukawa/tinybind-go"
 "github.com/shibukawa/tinybind-go/configbind"
)
func RegisterConfig[T any](context.Context, string) *T { return configbind.Bind[T]("") }
func Command[T any](context.Context, string, string) *T { return nil }
func BindRequest[T any](context.Context, *http.Request) (T, error) { var zero T; return zero, nil }
func OK[X, T any](context.Context, http.ResponseWriter, T) error { return nil }
func Created(context.Context, http.ResponseWriter, *http.Request, any) error { return nil }
func StreamModel[X, T any](context.Context, http.ResponseWriter) *httpbind.Stream[T] { return nil }
func BadInput(context.Context, error) error { return nil }
func TooLarge(context.Context, error) error { return nil }
func DecodeModel[X, T any](context.Context) (T, error) { var zero T; return zero, nil }
func EncodeValue(context.Context, any) error { return nil }
func ScanModel[X, T any](context.Context, any) ([]T, error) { return nil, nil }
type Router struct{}
func (Router) Route(string, string, http.HandlerFunc) {}
func (Router) Health(http.HandlerFunc) {}
var _ = httpbind.OpenAPIJSON
`)
	writeTestFile(t, filepath.Join(dir, "main.go"), `package sample
import (
 "context"
 "net/http"
 "tempmod/framework"
)
type Config struct { ServiceName string `+"`default:\"api\"`"+` }
type WorkerConfig struct { Concurrency int `+"`default:\"4\"`"+` }
type MigrateOptions struct {
 Path string `+"`arg:\"required\"`"+`
 DryRun bool
}
type Request struct { Name string `+"`json:\"name\"`"+` }
type Response struct { ID int `+"`json:\"id\"`"+` }
type PlainResponse struct { OK bool `+"`json:\"ok\"`"+` }
type Event struct { Kind string `+"`json:\"kind\"`"+` }
type Decoded struct { Value string }
type Encoded struct { Value string }
type Row struct { ID int }
const configName = "service"
var config = framework.RegisterConfig[Config](context.Background(), configName)
var workerConfig = framework.RegisterConfig[WorkerConfig](context.Background(), "worker")
var migrate = framework.Command[MigrateOptions](context.Background(), "migrate", "run migrations")
func handler(w http.ResponseWriter, r *http.Request) {
 req, _ := framework.BindRequest[Request](r.Context(), r)
 _ = req
 _ = framework.Created(r.Context(), w, r, Response{ID: 1})
 _ = framework.BadInput(r.Context(), context.Canceled)
 _ = framework.TooLarge(r.Context(), context.Canceled)
}
func plain(w http.ResponseWriter, r *http.Request) {
 _ = framework.OK[string](r.Context(), w, PlainResponse{OK: true})
}
func stream(w http.ResponseWriter, r *http.Request) {
 _ = framework.StreamModel[string, Event](r.Context(), w)
}
func register() {
 router := framework.Router{}
 router.Route("api", "POST /things", handler)
 router.Route("api", "GET /plain", plain)
 router.Route("api", "GET /events", stream)
 router.Health(plain)
 _, _ = framework.DecodeModel[string, Decoded](context.Background())
 _ = framework.EncodeValue(context.Background(), Encoded{})
 _, _ = framework.ScanModel[string, Row](context.Background(), nil)
}
`)
	tidyTempModule(t, dir)

	registry := generator.NewCallRegistry()
	err := registry.Register(
		generator.ConfigBindCall(
			generator.Function("tempmod/framework", "RegisterConfig"),
			generator.GenericType("config", 0), generator.Argument("prefix", 1),
		),
		generator.ConfigSubCommandCall(
			generator.Function("tempmod/framework", "Command"),
			generator.GenericType("config", 0), generator.Argument("name", 1), generator.Argument("help", 2),
		),
		generator.RequestBindCall(
			generator.Function("tempmod/framework", "BindRequest"),
			generator.GenericType("request", 0),
		),
		generator.ResponseWriteCall(
			generator.Function("tempmod/framework", "OK"),
			generator.GenericType("response", 1),
		),
		generator.ResponseWriteStatusCall(
			generator.Function("tempmod/framework", "Created"),
			generator.ArgumentType("response", 3), generator.Constant("status", 201),
		),
		generator.StreamCreateCall(
			generator.Function("tempmod/framework", "StreamModel"),
			generator.GenericType("stream", 1),
		),
		generator.RouteRegisterCall(
			generator.Method("tempmod/framework", "Route", "tempmod/framework", "Router"),
			generator.Argument("pattern", 1), generator.Argument("handler", 2),
		),
		generator.RouteRegisterCall(
			generator.Method("tempmod/framework", "Health", "tempmod/framework", "Router"),
			generator.Constant("pattern", "GET /health"), generator.Argument("handler", 0),
		),
		generator.ErrorResponseCall(
			generator.Function("tempmod/framework", "BadInput"), generator.Constant("status", 400),
		),
		generator.ErrorResponseCall(
			generator.Function("tempmod/framework", "TooLarge"), generator.Constant("status", 413),
		),
		generator.JSONDecodeCall(
			generator.Function("tempmod/framework", "DecodeModel"), generator.GenericType("decode", 1),
		),
		generator.JSONEncodeCall(
			generator.Function("tempmod/framework", "EncodeValue"), generator.ArgumentType("encode", 1),
		),
		generator.RowsScanCall(
			generator.Function("tempmod/framework", "ScanModel"), generator.GenericType("row", 1),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Options(generator.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	g := generator.New(options)

	plan, err := g.Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertTypeUsage(t, plan, "Request", generator.UsageBind)
	assertTypeUsage(t, plan, "Response", generator.UsageWrite)
	assertTypeUsage(t, plan, "PlainResponse", generator.UsageWrite)
	assertTypeUsage(t, plan, "Event", generator.UsageWrite|generator.UsageEncodeJSON)
	assertTypeUsage(t, plan, "Decoded", generator.UsageDecodeJSON)
	assertTypeUsage(t, plan, "Encoded", generator.UsageEncodeJSON)
	assertTypeUsage(t, plan, "Row", generator.UsageScanRows)

	out := t.TempDir()
	path, err := g.GenerateConfigBind(dir, out, "configbind_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`Register[Config]`, `Prefix:   "service"`, `"service.service_name"`,
		`Register[WorkerConfig]`, `Prefix:   "worker"`, `"worker.concurrency"`,
		`RegisterSubCommand[MigrateOptions]`, `Name:     "migrate"`, `Help:     "run migrations"`,
	} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated configbind missing %q:\n%s", want, generated)
		}
	}

	doc, err := g.BuildOpenAPI(dir)
	if err != nil {
		t.Fatal(err)
	}
	jsonDoc, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"/things"`, `"/plain"`, `"/events"`, `"/health"`, `"201"`, `"400"`, `"413"`, `"Request"`, `"Response"`, `"PlainResponse"`, `"Event"`} {
		if !strings.Contains(string(jsonDoc), want) {
			t.Fatalf("OpenAPI missing %q:\n%s", want, jsonDoc)
		}
	}

	invalid := generator.NewCallRegistry()
	if err := invalid.Register(generator.RequestBindCall(
		generator.Function("tempmod/framework", "BindRequest"),
		generator.GenericType("request", 3),
	)); err != nil {
		t.Fatal(err)
	}
	invalidOptions, err := invalid.Options(generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.New(invalidOptions).Analyze(dir); err == nil || !strings.Contains(err.Error(), "exceeds wrapper signature") {
		t.Fatalf("invalid wrapper signature error=%v", err)
	}
}

func TestCallRegistryRejectsConflictingTargetSemantics(t *testing.T) {
	registry := generator.NewCallRegistry()
	target := generator.Function("example.test/framework", "Wrap")
	if err := registry.Register(generator.RequestBindCall(target, generator.GenericType("request", 0))); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(generator.ResponseWriteCall(target, generator.GenericType("response", 0))); err == nil {
		t.Fatal("expected conflicting target error")
	}
}

func TestCallRegistryValidatesAndSnapshotsPatterns(t *testing.T) {
	target := generator.Function("example.test/framework", "Bind")
	pattern := generator.RequestBindCall(target, generator.GenericType("request", 0))
	registry := generator.NewCallRegistry()
	if err := registry.Register(pattern); err != nil {
		t.Fatal(err)
	}
	target.Function.Name = "Mutated"
	*pattern.TypeRoles["request"].GenericArgument = 7

	options, err := registry.Options(generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := options.Calls.Set[0]
	if got.Target.Function.Name != "Bind" || *got.TypeRoles["request"].GenericArgument != 0 {
		t.Fatalf("registry snapshot was mutated: %#v", got)
	}
	options.Calls.Set[0].Target.Function.Name = "ChangedAgain"
	options2, err := registry.Options(generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if options2.Calls.Set[0].Target.Function.Name != "Bind" {
		t.Fatalf("registry shared an options snapshot: %#v", options2.Calls.Set[0])
	}

	if err := registry.Register(generator.Call("unknown", generator.Function("example.test/framework", "Unknown"))); err == nil {
		t.Fatal("expected unsupported operation error")
	}
	if err := registry.Register(generator.ResponseWriteStatusCall(
		generator.Function("example.test/framework", "BadStatus"),
		generator.GenericType("response", 0), generator.Constant("status", "201"),
	)); err == nil {
		t.Fatal("expected status constant type error")
	}
	if err := registry.Register(generator.ConfigSubCommandCall(
		generator.Function("example.test/framework", "BadCommand"),
		generator.GenericType("config", 0), generator.Constant("name", "bad"), generator.Constant("help", 123),
	)); err == nil {
		t.Fatal("expected subcommand help constant type error")
	}
	if err := registry.Register(generator.ErrorResponseCall(
		generator.Function("example.test/framework", "DynamicError"),
		generator.Argument("status", 0),
	)); err == nil {
		t.Fatal("expected fixed error status validation")
	}
}
