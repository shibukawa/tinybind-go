package generator_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// cacheKeyModule writes a temp module whose single source is src.
func cacheKeyModule(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// cacheKeySource composes a package that declares body. Nothing in this module
// consumes a cache key, so generate-all plus the tag gate is what discovers a
// key type here; the usage-directed path is exercised separately below.
func cacheKeySource(body string) string {
	return "package sample\n\n" + body + "\n"
}

func analyzeCacheKeys(t *testing.T, src string) (*generator.CacheKeyPackagePlan, error) {
	t.Helper()
	return generator.AnalyzeCacheKeys(cacheKeyModule(t, cacheKeySource(src)))
}

func generateCacheKeys(t *testing.T, src string) string {
	t.Helper()
	plan, err := analyzeCacheKeys(t, src)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	code, err := generator.EmitCacheKeys(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return string(code)
}

func cacheKeyError(t *testing.T, src string) string {
	t.Helper()
	_, err := analyzeCacheKeys(t, src)
	if err == nil {
		t.Fatal("expected an error, got none")
	}
	return err.Error()
}

func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("missing %q in:\n%s", want, got)
	}
}

func requireAbsent(t *testing.T, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Fatalf("unexpected %q in:\n%s", unwanted, got)
	}
}

func TestCacheKeyEmitsMethodAndAssertion(t *testing.T) {
	code := generateCacheKeys(t, `type UserSummary struct {
	UserID string `+"`cache:\"key\"`"+`
	Page   int    `+"`cache:\"key\"`"+`
}`)
	requireContains(t, code, "var _ cachekeybind.CacheKey = UserSummary{}")
	requireContains(t, code, "func (v UserSummary) CacheKey() string {")
	requireContains(t, code, `cachekeybind.KeyString("tempmod.UserSummary")`)
	requireContains(t, code, "cachekeybind.KeyString(v.UserID)")
	requireContains(t, code, "cachekeybind.KeyInt(v.Page)")
}

// Marking is opt-in, so an entity's payload stays out of the key. Keying on it
// would mean building the key from the value the lookup exists to avoid.
func TestCacheKeyExcludesUnmarkedFields(t *testing.T) {
	code := generateCacheKeys(t, `type UserSummary struct {
	UserID string `+"`cache:\"key\"`"+`
	Name   string
	Total  int
}`)
	requireContains(t, code, "cachekeybind.KeyString(v.UserID)")
	requireAbsent(t, code, "v.Name")
	requireAbsent(t, code, "v.Total")
}

// An entity already tagged for storage is a key source as-is.
func TestCacheKeyIgnoresStorageTagsOnTheSameField(t *testing.T) {
	code := generateCacheKeys(t, `type Reading struct {
	Sensor  string  `+"`dynamo:\"sensor,partitionkey\" cache:\"key\"`"+`
	Celsius float64 `+"`dynamo:\"celsius\"`"+`
}`)
	requireContains(t, code, "cachekeybind.KeyString(v.Sensor)")
	requireAbsent(t, code, "v.Celsius")
}

// The identity is derived from the type name, so two key types holding equal
// field values never reach one entry.
func TestCacheKeyIdentityDiffersByTypeName(t *testing.T) {
	code := generateCacheKeys(t, `type A struct {
	ID string `+"`cache:\"key\"`"+`
}

type B struct {
	ID string `+"`cache:\"key\"`"+`
}`)
	requireContains(t, code, `"tempmod.A"`)
	requireContains(t, code, `"tempmod.B"`)
}

func TestCacheKeyFramesEveryScalarKind(t *testing.T) {
	code := generateCacheKeys(t, `import "time"

type Every struct {
	S  string    `+"`cache:\"key\"`"+`
	B  bool      `+"`cache:\"key\"`"+`
	I  int64     `+"`cache:\"key\"`"+`
	U  uint32    `+"`cache:\"key\"`"+`
	F  float64   `+"`cache:\"key\"`"+`
	By []byte    `+"`cache:\"key\"`"+`
	T  time.Time `+"`cache:\"key\"`"+`
}`)
	for _, want := range []string{
		"cachekeybind.KeyString(v.S)",
		"cachekeybind.KeyBool(v.B)",
		"cachekeybind.KeyInt(v.I)",
		"cachekeybind.KeyUint(v.U)",
		"cachekeybind.KeyFloat(v.F)",
		"cachekeybind.KeyBytes(v.By)",
		"cachekeybind.KeyTime(v.T)",
	} {
		requireContains(t, code, want)
	}
}

func TestCacheKeyFramesPointerAndSliceThroughClosures(t *testing.T) {
	code := generateCacheKeys(t, `type Query struct {
	After *string  `+"`cache:\"key\"`"+`
	Tags  []string `+"`cache:\"key\"`"+`
}`)
	requireContains(t, code, "cachekeybind.KeyOptional(v.After, func(e1 string) string { return cachekeybind.KeyString(e1) })")
	requireContains(t, code, "cachekeybind.KeyArray(v.Tags, func(e1 string) string { return cachekeybind.KeyString(e1) })")
}

// A nested closure must not shadow the one it is written inside.
func TestCacheKeyNestsClosuresWithoutShadowing(t *testing.T) {
	code := generateCacheKeys(t, `type Query struct {
	Groups [][]int `+"`cache:\"key\"`"+`
}`)
	requireContains(t, code, "func(e1 []int) string")
	requireContains(t, code, "func(e2 int) string")
}

// A named field type must reach a helper without a conversion, which is what
// generic framing over the underlying kind buys.
func TestCacheKeyFramesNamedScalarTypes(t *testing.T) {
	code := generateCacheKeys(t, `type Page int

type Query struct {
	P Page `+"`cache:\"key\"`"+`
}`)
	requireContains(t, code, "cachekeybind.KeyInt(v.P)")
}

func TestCacheKeyOrdersFieldsByDeclaration(t *testing.T) {
	code := generateCacheKeys(t, `type Query struct {
	B string `+"`cache:\"key\"`"+`
	A string `+"`cache:\"key\"`"+`
}`)
	b := strings.Index(code, "v.B")
	a := strings.Index(code, "v.A")
	if b < 0 || a < 0 || b > a {
		t.Fatalf("fields not in declaration order:\n%s", code)
	}
}

// Generate-all is gated on the tag, so an unrelated struct acquires nothing.
func TestCacheKeyGenerateAllSkipsUntaggedStructs(t *testing.T) {
	plan, err := analyzeCacheKeys(t, `type Request struct {
	Name string
}`)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(plan.Keys) != 0 {
		t.Fatalf("expected no keys, got %d", len(plan.Keys))
	}
}

// An unframeable field is an error, not a skip: a skipped field is the silent
// wrong-answer failure this package exists to remove.
func TestCacheKeyRejectsUnframeableFieldType(t *testing.T) {
	got := cacheKeyError(t, `type Inner struct{ N int }

type Query struct {
	I Inner `+"`cache:\"key\"`"+`
}`)
	requireContains(t, got, "has no cache key framing")
}

func TestCacheKeyRejectsMapField(t *testing.T) {
	got := cacheKeyError(t, `type Query struct {
	M map[string]int `+"`cache:\"key\"`"+`
}`)
	requireContains(t, got, "has no cache key framing")
}

func TestCacheKeyRejectsUnknownTagOption(t *testing.T) {
	got := cacheKeyError(t, `type Query struct {
	Name string `+"`cache:\"-\"`"+`
}`)
	requireContains(t, got, "unknown cache tag option")
}

// There is no key version. A blank marker field was the spelling a version
// would have taken, so it is refused with the reason rather than ignored — an
// ignored one would read as a version that silently did nothing.
func TestCacheKeyRejectsATaggedBlankField(t *testing.T) {
	got := cacheKeyError(t, `type Query struct {
	_    struct{} `+"`cache:\"version=2\"`"+`
	Name string   `+"`cache:\"key\"`"+`
}`)
	requireContains(t, got, "a blank field carries no value")
}

// The identity is wholly derived, so nothing an author writes appears in it
// beyond the type's own name.
func TestCacheKeyIdentityCarriesNoDeclaredComponent(t *testing.T) {
	code := generateCacheKeys(t, `type Query struct {
	Name string `+"`cache:\"key\"`"+`
}`)
	requireContains(t, code, `cachekeybind.KeyString("tempmod.Query")`)
	requireAbsent(t, code, "/v")
}

func TestCacheKeyRejectsMarkedUnexportedField(t *testing.T) {
	got := cacheKeyError(t, `type Query struct {
	name string `+"`cache:\"key\"`"+`
}`)
	requireContains(t, got, "cannot be marked")
}

// A hand-written key is never silently replaced.
func TestCacheKeyRejectsExistingMethod(t *testing.T) {
	got := cacheKeyError(t, `type Query struct {
	Name string `+"`cache:\"key\"`"+`
}

func (q Query) CacheKey() string { return "" }`)
	requireContains(t, got, "already declares CacheKey")
}

// The usage-directed path: a downstream framework registers the call that reads
// a key, and the key is an argument type rather than a type parameter.
func cacheKeyUsageOptions(t *testing.T) generator.Options {
	t.Helper()
	calls := generator.NewCallRegistry()
	if err := calls.Register(generator.CacheKeyCall(
		generator.Function("tempmod/frameworkpkg", "Memo"),
		generator.ArgumentType("key", 1),
	)); err != nil {
		t.Fatalf("register: %v", err)
	}
	base := generator.DefaultOptions()
	base.GenerateAll = false
	opts, err := calls.Options(base)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	return opts
}

// usageModule writes a two-package temp module: a stand-in framework declaring
// the memo call, and a sample package that calls it.
func usageModule(t *testing.T, body, calls string) string {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	pkgDir := filepath.Join(dir, "frameworkpkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The key parameter is the runtime interface, which is the shape a framework
	// actually writes and the one the framework-owner guide documents. Discovery
	// must read the static type of the argument expression, not the declared
	// parameter type, or every key type would arrive as the interface.
	framework := `package frameworkpkg

import (
	"context"

	"github.com/shibukawa/tinybind-go/cachekeybind"
)

// Memo is generic over the cached result, and the key is the value beside it.
func Memo[T any](ctx context.Context, key cachekeybind.CacheKey, fetch func(context.Context) (T, error)) (T, error) {
	return fetch(ctx)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "framework.go"), []byte(framework), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `package sample

import (
	"context"

	"tempmod/frameworkpkg"
)

` + body + `

` + calls + `
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// The stand-in framework imports the runtime for its key parameter, so the
	// temp module has to resolve it before packages.Load will type-check.
	tidyTempModule(t, dir)
	return dir
}

func TestCacheKeyDiscoversTypePassedToARegisteredCall(t *testing.T) {
	dir := usageModule(t, `type UserSummary struct {
	UserID string `+"`cache:\"key\"`"+`
	Name   string
}`, `func load(ctx context.Context) (string, error) {
	return frameworkpkg.Memo(ctx, UserSummary{}, func(ctx context.Context) (string, error) { return "", nil })
}

var _ = load`)
	plan, err := generator.AnalyzeCacheKeysWithOptions(dir, cacheKeyUsageOptions(t))
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(plan.Keys) != 1 || plan.Keys[0].Name != "UserSummary" {
		t.Fatalf("expected UserSummary, got %+v", plan.Keys)
	}
}

// The error fires on reaching, which is the case default-include could never
// produce and the reason this check exists at all.
func TestCacheKeyRejectsUnmarkedStructReachedAsAKey(t *testing.T) {
	dir := usageModule(t, `type UserSummary struct {
	UserID string
	Name   string
}`, `func load(ctx context.Context) (string, error) {
	return frameworkpkg.Memo(ctx, UserSummary{}, func(ctx context.Context) (string, error) { return "", nil })
}

var _ = load`)
	_, err := generator.AnalyzeCacheKeysWithOptions(dir, cacheKeyUsageOptions(t))
	if err == nil {
		t.Fatal("expected an error for a struct reached as a key with no marked field")
	}
	requireContains(t, err.Error(), "marks no field")
}

// An untagged struct that is never passed as a key is not a key type, and not
// an error.
func TestCacheKeyIgnoresUnmarkedStructNeverReached(t *testing.T) {
	dir := usageModule(t, `type UserSummary struct {
	UserID string
}`, `var _ = frameworkpkg.Memo[string]`)
	plan, err := generator.AnalyzeCacheKeysWithOptions(dir, cacheKeyUsageOptions(t))
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(plan.Keys) != 0 {
		t.Fatalf("expected no keys, got %+v", plan.Keys)
	}
}

func TestCacheKeyFeatureDisableSuppressesGeneration(t *testing.T) {
	dir := cacheKeyModule(t, cacheKeySource(`type Query struct {
	Name string `+"`cache:\"key\"`"+`
}`))
	opts := generator.DefaultOptions()
	opts.GenerateAll = true
	opts.DisableFeatures = []generator.Feature{generator.FeatureCacheKey}
	g := &generator.Generator{Options: opts}
	out, err := g.GenerateCacheKeys(dir, t.TempDir(), "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out != "" {
		t.Fatalf("expected no output, got %q", out)
	}
}

func TestCacheKeyGeneratesFileOnDisk(t *testing.T) {
	dir := cacheKeyModule(t, cacheKeySource(`type Query struct {
	Name string `+"`cache:\"key\"`"+`
}`))
	opts := generator.DefaultOptions()
	opts.GenerateAll = true
	g := &generator.Generator{Options: opts}
	outDir := t.TempDir()
	out, err := g.GenerateCacheKeys(dir, outDir, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out == "" {
		t.Fatal("expected a generated file")
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, string(body), "func (v Query) CacheKey() string {")
	requireContains(t, string(body), `"github.com/shibukawa/tinybind-go/cachekeybind"`)
}

// Emitting well-formed Go is not the same as emitting well-typed Go. Every
// framing path here goes through a generic helper whose constraint the emitter
// picked from go/types, so compiling the output is the only check that the two
// agree.
func TestCacheKeyGeneratedCodeCompiles(t *testing.T) {
	skipWithoutToolchain(t)
	dir := cacheKeyModule(t, cacheKeySource(`import "time"

type Page int

type Label string

type Every struct {
	S      string      `+"`cache:\"key\"`"+`
	Named  Label       `+"`cache:\"key\"`"+`
	B      bool        `+"`cache:\"key\"`"+`
	I      int         `+"`cache:\"key\"`"+`
	I64    int64       `+"`cache:\"key\"`"+`
	P      Page        `+"`cache:\"key\"`"+`
	U      uint32      `+"`cache:\"key\"`"+`
	F32    float32     `+"`cache:\"key\"`"+`
	F64    float64     `+"`cache:\"key\"`"+`
	By     []byte      `+"`cache:\"key\"`"+`
	T      time.Time   `+"`cache:\"key\"`"+`
	Opt    *string     `+"`cache:\"key\"`"+`
	OptT   *time.Time  `+"`cache:\"key\"`"+`
	Tags   []string    `+"`cache:\"key\"`"+`
	Times  []time.Time `+"`cache:\"key\"`"+`
	Groups [][]int     `+"`cache:\"key\"`"+`
	Ignore string
}`))
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.GenerateAll = true
	g := &generator.Generator{Options: opts}
	out, err := g.GenerateCacheKeys(dir, dir, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out == "" {
		t.Fatal("expected a generated file")
	}
	generated, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated keys do not compile: %v\n%s\n%s", err, output, generated)
	}
}

// Regeneration must not read its own output as a hand-written collision.
func TestCacheKeyRegenerationOverItsOwnOutput(t *testing.T) {
	skipWithoutToolchain(t)
	dir := cacheKeyModule(t, cacheKeySource(`type Query struct {
	Name string `+"`cache:\"key\"`"+`
}`))
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.GenerateAll = true
	g := &generator.Generator{Options: opts}
	if _, err := g.GenerateCacheKeys(dir, dir, ""); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if _, err := g.GenerateCacheKeys(dir, dir, ""); err != nil {
		t.Fatalf("second generate: %v", err)
	}
}

// EmitCacheKeys is exported, so a hand-built plan can reach the emitter without
// passing the collector's checks. Every fallback it could take is a key missing
// a field, so it must fail instead.
func TestCacheKeyEmitterRejectsAnIdentityOnlyPlan(t *testing.T) {
	_, err := generator.EmitCacheKeys(&generator.CacheKeyPackagePlan{
		Package:     "sample",
		PackagePath: "tempmod",
		Keys:        []generator.CacheKeyPlan{{Name: "Query"}},
	})
	if err == nil {
		t.Fatal("expected an error for a plan with no fields")
	}
	requireContains(t, err.Error(), "identity-only key")
}

func TestCacheKeyEmitterRejectsAnUnknownKind(t *testing.T) {
	_, err := generator.EmitCacheKeys(&generator.CacheKeyPackagePlan{
		Package:     "sample",
		PackagePath: "tempmod",
		Keys: []generator.CacheKeyPlan{{
			Name:   "Query",
			Fields: []generator.CacheKeyFieldPlan{{Name: "N", Type: generator.CacheKeyType{Kind: "nonesuch", Go: "int"}}},
		}},
	})
	if err == nil {
		t.Fatal("expected an error for an unplannable kind")
	}
	requireContains(t, err.Error(), "no framing helper")
}

func TestCacheKeyEmitterRejectsAnElementlessCollection(t *testing.T) {
	_, err := generator.EmitCacheKeys(&generator.CacheKeyPackagePlan{
		Package:     "sample",
		PackagePath: "tempmod",
		Keys: []generator.CacheKeyPlan{{
			Name:   "Query",
			Fields: []generator.CacheKeyFieldPlan{{Name: "T", Type: generator.CacheKeyType{Kind: generator.CacheKeyArray, Go: "[]string"}}},
		}},
	})
	if err == nil {
		t.Fatal("expected an error for an array with no element type")
	}
	requireContains(t, err.Error(), "plans no element type")
}

// The package pipeline is what tinybind-gen runs, so a key type has to reach a
// file through GeneratePackage and not only through GenerateCacheKeys.
func TestCacheKeyReachesTheGeneratePackagePipeline(t *testing.T) {
	skipWithoutToolchain(t)
	dir := cacheKeyModule(t, cacheKeySource(`type UserSummary struct {
	UserID string `+"`cache:\"key\"`"+`
	Name   string
}`))
	tidyTempModule(t, dir)
	opts := generator.DefaultOptions()
	opts.GenerateAll = true
	g := &generator.Generator{Options: opts}
	result, err := g.GeneratePackage(context.Background(), generator.GenerateRequest{Dir: dir, Out: dir})
	if err != nil {
		t.Fatalf("generate package: %v", err)
	}
	if result.CacheKeyPath == "" {
		t.Fatal("expected a cache key file from the package pipeline")
	}
	body, err := os.ReadFile(result.CacheKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	requireContains(t, string(body), "func (v UserSummary) CacheKey() string {")
	if filepath.Base(result.CacheKeyPath) != "cachekeybind_gen.go" {
		t.Fatalf("unexpected output name %q", filepath.Base(result.CacheKeyPath))
	}
}
