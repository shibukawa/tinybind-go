package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// generateConfigBindSource writes one package into a temp module and returns
// the generated configbind source.
func generateConfigBindSource(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "config.go"), source)
	tidyTempModule(t, dir)
	g := generator.New(generator.DefaultOptions())
	path, err := g.GenerateConfigBind(dir, t.TempDir(), "configbind_gen.go")
	if err != nil {
		t.Fatalf("GenerateConfigBind: %v", err)
	}
	if path == "" {
		t.Fatal("expected a generated path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A type alias is transparent: it must bind as the type it names. Since Go 1.24
// go/types reports one as *types.Alias, so a plain *types.Named test misses it.
func TestConfigBindResolvesTypeAliases(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import (
 "time"
 "github.com/shibukawa/tinybind-go/configbind"
)
type TLSConfig struct {
 Enabled bool
 CertPath string
}
type TLS = TLSConfig
type Timeout = time.Duration
type ServerConfig struct {
 ReadTimeout Timeout `+"`default:\"5s\"`"+`
 TLS TLS
}
var config = configbind.Bind[ServerConfig]("server")
`)
	for _, want := range []string{
		// The aliased struct expands into its own table instead of failing.
		`"server.tls.enabled"`,
		`"server.tls.cert_path"`,
		// The aliased duration keeps duration handling; int binding would
		// silently reject "5s".
		"time.ParseDuration",
		"configbind.ScaffoldDuration",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
}

// A config type reached through an alias is the same type, so Bind on the alias
// generates the definition of the type it names.
func TestConfigBindResolvesAliasedBindTypeArgument(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type ServerConfig struct {
 Host string
}
type Server = ServerConfig
var config = configbind.Bind[Server]("server")
`)
	if !strings.Contains(text, "Register[ServerConfig]") {
		t.Fatalf("expected the aliased type's own definition in\n%s", text)
	}
}

// A generic helper passing its own type parameter names no concrete config
// type, so that call site is skipped. It used to abort the package scan, which
// also lost the config types declared beside the helper.
func TestConfigBindSkipsGenericWrapperCall(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type ServerConfig struct {
 Host string
}
func RegisterConfig[T any](prefix string) *T {
 return configbind.Bind[T](prefix)
}
var config = configbind.Bind[ServerConfig]("server")
`)
	if !strings.Contains(text, "Register[ServerConfig]") {
		t.Fatalf("the concrete config beside the generic helper was lost:\n%s", text)
	}
}

// A sized integer keeps its width: parsing at 64 bits and assigning int() does
// not compile for int64 and truncates on a 32-bit target.
func TestConfigBindKeepsIntegerWidths(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type LimitConfig struct {
 MaxRequestBody int64 `+"`default:\"1048576\"`"+`
 Workers uint32
 Port int
}
var config = configbind.Bind[LimitConfig]("limit")
`)
	for _, want := range []string{
		"strconv.ParseInt(v, 10, 64)",
		"p.MaxRequestBody = int64(n)",
		"strconv.ParseUint(v, 10, 32)",
		"p.Workers = uint32(n)",
		"strconv.ParseInt(v, 10, 0)",
		"p.Port = int(n)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
}

// A defined integer type needs both its own name for the assignment and its
// underlying width for the parse; v1 carries only one, so it fails generation
// instead of emitting an assignment that does not compile.
func TestConfigBindRejectsDefinedIntegerType(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "config.go"), `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type Level int
type LogConfig struct {
 Level Level
}
var config = configbind.Bind[LogConfig]("log")
`)
	tidyTempModule(t, dir)
	g := generator.New(generator.DefaultOptions())
	_, err := g.GenerateConfigBind(dir, t.TempDir(), "configbind_gen.go")
	if err == nil {
		t.Fatal("expected a generation error for a defined integer type")
	}
	if !strings.Contains(err.Error(), "Level") {
		t.Fatalf("error should name the field type: %v", err)
	}
}

// A dependon or secret tag on a nested struct describes the whole subtree, and
// a leading dot resolves against the struct the tag is written in, so one
// shared type serves every prefix it is embedded at.
func TestConfigBindReadsSubtreeAndRelativeTags(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type EndpointConfig struct {
 Enabled bool
 Path string `+"`dependon:\".enabled\"`"+`
}
type CredentialsConfig struct {
 User string
 Password string
}
type ServerConfig struct {
 Health EndpointConfig
 Readiness EndpointConfig
 Upstream CredentialsConfig `+"`secret:\"hide\"`"+`
}
var config = configbind.Bind[ServerConfig]("server")
`)
	for _, want := range []string{
		`"server.health.path": {{Key: "server.health.enabled"}}`,
		`"server.readiness.path": {{Key: "server.readiness.enabled"}}`,
		`"server.upstream.user": "hide"`,
		`"server.upstream.password": "hide"`,
	} {
		if !containsNormalizedSource(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
}

func TestConfigBindReadsSecretTag(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type ServerConfig struct {
 Token string `+"`secret:\"show\"`"+`
 License string `+"`secret:\"mask\"`"+`
}
var config = configbind.Bind[ServerConfig]("server")
`)
	for _, want := range []string{
		"Secrets: map[string]string{",
		`"server.token": "show"`,
		`"server.license": "mask"`,
	} {
		if !containsNormalizedSource(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
}

// A duration parent needs its own falsy choice before anything may depend on
// it, because zero is otherwise a real setting.
func TestConfigBindAllowsDurationParentWithFalsy(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import (
 "time"
 "github.com/shibukawa/tinybind-go/configbind"
)
type SQLConfig struct {
 SlowThreshold time.Duration `+"`falsy:\"0s\"`"+`
 Explain bool `+"`dependon:\"sql.slow_threshold\"`"+`
}
var config = configbind.Bind[SQLConfig]("sql")
`)
	for _, want := range []string{
		`"sql.slow_threshold": "0s"`,
		`"sql.explain": {{Key: "sql.slow_threshold"}}`,
	} {
		if !containsNormalizedSource(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
}

// A mode key holds a non-empty value in every mode, so only a value condition
// can say which of its sibling subtrees the current mode actually uses. The
// relative form resolves against the struct the tag is written in, exactly as
// the emptiness form does.
func TestConfigBindReadsValueConditionTags(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type PasskeyConfig struct {
 Path string
}
type JWTConfig struct {
 Leeway string
}
type AuthConfig struct {
 Enabled bool
 Mode string `+"`default:\"oidc_only\" enum:\"oidc_only,oidc_passkey,jwt_only\" dependon:\".enabled\"`"+`
 Passkey PasskeyConfig `+"`dependon:\".mode=oidc_only,oidc_passkey\"`"+`
 JWT JWTConfig `+"`dependon:\".mode!=jwt_only\"`"+`
}
var config = configbind.Bind[AuthConfig]("auth")
`)
	for _, want := range []string{
		`"auth.mode": {{Key: "auth.enabled"}}`,
		`"auth.passkey.path": {{Key: "auth.mode", Op: "=", Values: []string{"oidc_only", "oidc_passkey"}}}`,
		`"auth.jwt.leeway": {{Key: "auth.mode", Op: "!=", Values: []string{"jwt_only"}}}`,
	} {
		if !containsNormalizedSource(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
}

// A summary tag on a nested struct rates every key below it, which is what makes
// rating a deployment's unremarkable defaults affordable at all.
func TestConfigBindReadsSummaryTag(t *testing.T) {
	text := generateConfigBindSource(t, `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type QueryConfig struct {
 Explain bool
 MaxSQLLength int
}
type ObservabilityConfig struct {
 MinimumLevel string `+"`default:\"info\"`"+`
 Query QueryConfig `+"`summary:\"omit\"`"+`
 Jitter int `+"`default:\"20\" summary:\"omit\"`"+`
}
var config = configbind.Bind[ObservabilityConfig]("obs")
`)
	for _, want := range []string{
		"Summary: map[string]string{",
		`"obs.query.explain": "omit"`,
		`"obs.query.max_sql_length": "omit"`,
		`"obs.jitter": "omit"`,
	} {
		if !containsNormalizedSource(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if containsNormalizedSource(text, `"obs.minimum_level": "omit"`) {
		t.Fatalf("an untagged key was rated:\n%s", text)
	}
}

func TestConfigBindRejectsBadSummaryMode(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "config.go"), `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type ObservabilityConfig struct {
 Jitter int `+"`default:\"20\" summary:\"verbose\"`"+`
}
var config = configbind.Bind[ObservabilityConfig]("obs")
`)
	tidyTempModule(t, dir)
	g := generator.New(generator.DefaultOptions())
	_, err := g.GenerateConfigBind(dir, t.TempDir(), "configbind_gen.go")
	if err == nil || !strings.Contains(err.Error(), "summary must be omit") {
		t.Fatalf("err=%v want a rejected summary mode", err)
	}
}

// A mistyped value would hide its whole subtree silently and for good, so the
// enum tag of the parent is read to catch it while the author is still here.
func TestConfigBindRejectsValueOutsideParentEnum(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "config.go"), `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type AuthConfig struct {
 Mode string `+"`default:\"oidc_only\" enum:\"oidc_only,jwt_only\"`"+`
 Passkey string `+"`dependon:\".mode=oidc_passkey\"`"+`
}
var config = configbind.Bind[AuthConfig]("auth")
`)
	tidyTempModule(t, dir)
	g := generator.New(generator.DefaultOptions())
	_, err := g.GenerateConfigBind(dir, t.TempDir(), "configbind_gen.go")
	if err == nil || !strings.Contains(err.Error(), "is not one of the enum choices") {
		t.Fatalf("err=%v want a rejected dependon value", err)
	}
}

// An enum names one value, which a struct has none of. The tag became readable
// only for the sake of a dependon condition, so its placement is decided here
// rather than accepted and dropped.
func TestConfigBindRejectsEnumOnNestedStruct(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "config.go"), `package sample
import "github.com/shibukawa/tinybind-go/configbind"
type TLSConfig struct {
 CertPath string
}
type ServerConfig struct {
 TLS TLSConfig `+"`enum:\"a,b\"`"+`
}
var config = configbind.Bind[ServerConfig]("server")
`)
	tidyTempModule(t, dir)
	g := generator.New(generator.DefaultOptions())
	_, err := g.GenerateConfigBind(dir, t.TempDir(), "configbind_gen.go")
	if err == nil || !strings.Contains(err.Error(), "enum applies to a leaf field") {
		t.Fatalf("err=%v want an enum placement rejection", err)
	}
}

func TestConfigBindRejectsDurationParentWithoutFalsy(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "config.go"), `package sample
import (
 "time"
 "github.com/shibukawa/tinybind-go/configbind"
)
type SQLConfig struct {
 SlowThreshold time.Duration
 Explain bool `+"`dependon:\"sql.slow_threshold\"`"+`
}
var config = configbind.Bind[SQLConfig]("sql")
`)
	tidyTempModule(t, dir)
	g := generator.New(generator.DefaultOptions())
	_, err := g.GenerateConfigBind(dir, t.TempDir(), "configbind_gen.go")
	if err == nil || !strings.Contains(err.Error(), "needs its own falsy tag") {
		t.Fatalf("err=%v want a parent-kind rejection", err)
	}
}

// containsNormalizedSource collapses whitespace on both sides so gofmt's
// map-key alignment does not decide whether an assertion matches.
func containsNormalizedSource(text, want string) bool {
	collapse := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	return strings.Contains(collapse(text), collapse(want))
}
