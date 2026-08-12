package configbind_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
)

type firstProvenanceConfig struct{}
type secondProvenanceConfig struct{}

// on builds the emptiness conditions a dependon tag with no operator generates,
// which is what most of these fixtures declare.
func on(parents ...string) []configbind.Dependency {
	out := make([]configbind.Dependency, 0, len(parents))
	for _, parent := range parents {
		out = append(out, configbind.Dependency{Key: parent})
	}
	return out
}

// is builds one membership condition, as 'dependon:"parent=a,b"' generates.
func is(parent string, values ...string) []configbind.Dependency {
	return []configbind.Dependency{{Key: parent, Op: configbind.DependOpEqual, Values: values}}
}

// isNot builds one exclusion condition, as 'dependon:"parent!=a,b"' generates.
func isNot(parent string, values ...string) []configbind.Dependency {
	return []configbind.Dependency{{Key: parent, Op: configbind.DependOpNotEqual, Values: values}}
}

// registerProvenanceFixture registers one definition whose keys are deliberately
// not in alphabetical order, so a key sort would be visible in the output.
func registerProvenanceFixture[T any](prefix string, keys []string, dependsOn map[string][]configbind.Dependency) {
	full := make([]string, 0, len(keys))
	defaults := map[string]string{}
	metas := make([]cliparser.FieldMeta, 0, len(keys))
	for _, key := range keys {
		configKey := prefix + "." + key
		full = append(full, configKey)
		defaults[configKey] = "v-" + key
		metas = append(metas, cliparser.FieldMeta{Prefix: prefix, Key: key})
	}
	configbind.Register[T](configbind.Definition{
		TypeName:  "example.test." + prefix,
		Prefix:    prefix,
		KnownKeys: full,
		Defaults:  defaults,
		DependsOn: dependsOn,
		FlagMetas: metas,
		Apply:     func(any, *configbind.Overlay) error { return nil },
	})
}

func loadProvenanceFixture(t *testing.T, environ []string, tomlBody string) []configbind.ProvenanceEntry {
	t.Helper()
	args := []string{}
	if tomlBody != "" {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(tomlBody), 0o644); err != nil {
			t.Fatal(err)
		}
		args = []string{"--config-path", path}
	}
	if environ == nil {
		environ = []string{}
	}
	res, err := configbind.Load(configbind.LoadOptions{
		Vendor: "tinybind-test", Tool: "provenance", Args: args, Environ: environ,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return res.Provenance()
}

func provenanceKeys(entries []configbind.ProvenanceEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func TestProvenanceFollowsRegistrationThenDeclarationOrder(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	// "zulu" is registered first and declares its keys out of alphabetical
	// order: a sorted output would put alpha.* first and reorder both bodies.
	registerProvenanceFixture[firstProvenanceConfig]("zulu", []string{"port", "host"}, nil)
	registerProvenanceFixture[secondProvenanceConfig]("alpha", []string{"zeta", "beta"}, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("zulu")
	configbind.Bind[secondProvenanceConfig]("alpha")

	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	want := "zulu.port,zulu.host,alpha.zeta,alpha.beta"
	if got != want {
		t.Fatalf("order=%s want %s", got, want)
	}
}

func TestProvenanceSortsUnknownKeysAfterKnownOnes(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	registerProvenanceFixture[firstProvenanceConfig]("zulu", []string{"port", "host"}, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("zulu")

	body := "[stray]\nsecond = \"2\"\nfirst = \"1\"\n"
	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, body)), ",")
	want := "zulu.port,zulu.host,stray.first,stray.second"
	if got != want {
		t.Fatalf("order=%s want %s", got, want)
	}
}

func TestProvenanceMasksSensitiveKeys(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	registerProvenanceFixture[firstProvenanceConfig]("db",
		[]string{"host", "password", "api_key", "access_key", "dsn", "private_key"}, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("db")

	values := map[string]string{}
	for _, entry := range loadProvenanceFixture(t, nil, "") {
		values[entry.Key] = entry.Value
	}
	// A DSN embeds its password, and a private key is the credential itself.
	for _, key := range []string{"db.password", "db.api_key", "db.access_key", "db.dsn", "db.private_key"} {
		if values[key] != "*****" {
			t.Fatalf("%s=%q want a mask", key, values[key])
		}
	}
	if values["db.host"] != "v-host" {
		t.Fatalf("non-sensitive key was masked: %v", values)
	}
}

func TestProvenanceHidesDependentsTransitively(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.rdb",
		Prefix:    "rdb",
		KnownKeys: []string{"rdb.dsn", "rdb.pool_size", "rdb.pool_idle"},
		Defaults: map[string]string{
			"rdb.dsn":       "",
			"rdb.pool_size": "10",
			"rdb.pool_idle": "2",
		},
		DependsOn: map[string][]configbind.Dependency{
			"rdb.pool_size": on("rdb.dsn"),
			"rdb.pool_idle": on("rdb.pool_size"),
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "rdb", Key: "dsn"},
			{Prefix: "rdb", Key: "pool_size"},
			{Prefix: "rdb", Key: "pool_idle"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("rdb")

	// An empty dsn hides pool_size, and a hidden pool_size hides pool_idle.
	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "rdb.dsn" {
		t.Fatalf("keys=%s want only the parent", got)
	}

	// Setting dsn brings the whole chain back.
	got = strings.Join(provenanceKeys(loadProvenanceFixture(t, []string{"RDB_DSN=postgres://x"}, "")), ",")
	if got != "rdb.dsn,rdb.pool_size,rdb.pool_idle" {
		t.Fatalf("keys=%s want the full chain", got)
	}
}

func TestProvenanceTreatsZeroAsConfigured(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.limits",
		Prefix:    "limits",
		KnownKeys: []string{"limits.max", "limits.window"},
		Defaults:  map[string]string{"limits.max": "0", "limits.window": "30s"},
		DependsOn: map[string][]configbind.Dependency{"limits.window": on("limits.max")},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "limits", Key: "max"},
			{Prefix: "limits", Key: "window"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("limits")

	// An int 0 is a deliberate setting, not an absent parent.
	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "limits.max,limits.window" {
		t.Fatalf("keys=%s; 0 must not hide dependents", got)
	}
}

// registerModeFixture builds one mode key and three subtrees, each selected by a
// value condition. It is the shape a login-method or storage-backend key has: the
// mode holds a non-empty value in every mode, so only its value can tell the
// applicable subtree from the two that are inert.
func registerModeFixture(t *testing.T, dependsOn map[string][]configbind.Dependency) {
	t.Helper()
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.auth",
		Prefix:    "auth",
		KnownKeys: []string{"auth.mode", "auth.oidc.issuer", "auth.passkey.path", "auth.jwt.leeway"},
		Defaults: map[string]string{
			"auth.mode":         "oidc_only",
			"auth.oidc.issuer":  "https://issuer",
			"auth.passkey.path": "/auth/passkey",
			"auth.jwt.leeway":   "30s",
		},
		DependsOn: dependsOn,
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "auth", Key: "mode"},
			{Prefix: "auth", Key: "oidc.issuer"},
			{Prefix: "auth", Key: "passkey.path"},
			{Prefix: "auth", Key: "jwt.leeway"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("auth")
}

func TestProvenanceKeepsOnlyTheSelectedMode(t *testing.T) {
	registerModeFixture(t, map[string][]configbind.Dependency{
		"auth.oidc.issuer":  is("auth.mode", "oidc_only", "oidc_passkey"),
		"auth.passkey.path": is("auth.mode", "oidc_passkey"),
		"auth.jwt.leeway":   is("auth.mode", "jwt_only"),
	})

	// oidc_only is in the oidc list and in neither of the others.
	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "auth.mode,auth.oidc.issuer" {
		t.Fatalf("keys=%s want only the selected subtree", got)
	}

	// oidc_passkey is in two lists at once, which is what the comma is for.
	got = strings.Join(provenanceKeys(loadProvenanceFixture(t, []string{"AUTH_MODE=oidc_passkey"}, "")), ",")
	if got != "auth.mode,auth.oidc.issuer,auth.passkey.path" {
		t.Fatalf("keys=%s want both subtrees the value selects", got)
	}

	got = strings.Join(provenanceKeys(loadProvenanceFixture(t, []string{"AUTH_MODE=jwt_only"}, "")), ",")
	if got != "auth.mode,auth.jwt.leeway" {
		t.Fatalf("keys=%s want the bearer-token subtree alone", got)
	}
}

func TestProvenanceExcludesOnNotEqual(t *testing.T) {
	registerModeFixture(t, map[string][]configbind.Dependency{
		"auth.passkey.path": isNot("auth.mode", "jwt_only"),
	})

	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	want := "auth.mode,auth.oidc.issuer,auth.passkey.path,auth.jwt.leeway"
	if got != want {
		t.Fatalf("keys=%s want %s", got, want)
	}
	got = strings.Join(provenanceKeys(loadProvenanceFixture(t, []string{"AUTH_MODE=jwt_only"}, "")), ",")
	want = "auth.mode,auth.oidc.issuer,auth.jwt.leeway"
	if got != want {
		t.Fatalf("keys=%s want %s", got, want)
	}
}

// A parent nothing set compares as "", which hides under "=" and shows under
// "!=": over-showing is the safe direction for a parent whose value is unknown.
func TestProvenanceReadsAnAbsentConditionParentAsEmpty(t *testing.T) {
	register := func(dependsOn map[string][]configbind.Dependency) {
		configbind.ResetDefinitions()
		t.Cleanup(configbind.ResetDefinitions)
		configbind.Register[firstProvenanceConfig](configbind.Definition{
			TypeName:  "example.test.app",
			Prefix:    "app",
			KnownKeys: []string{"app.mode", "app.detail"},
			// mode carries no default, so nothing puts it in the overlay.
			Defaults:  map[string]string{"app.detail": "on"},
			DependsOn: dependsOn,
			FlagMetas: []cliparser.FieldMeta{{Prefix: "app", Key: "mode"}, {Prefix: "app", Key: "detail"}},
			Apply:     func(any, *configbind.Overlay) error { return nil },
		})
		configbind.ResetTargets()
		t.Cleanup(configbind.ResetTargets)
		configbind.Bind[firstProvenanceConfig]("app")
	}

	register(map[string][]configbind.Dependency{"app.detail": is("app.mode", "fast")})
	if got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ","); got != "" {
		t.Fatalf("keys=%s; an absent parent matches no named value", got)
	}
	register(map[string][]configbind.Dependency{"app.detail": isNot("app.mode", "fast")})
	if got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ","); got != "app.detail" {
		t.Fatalf("keys=%s; an absent parent is not the excluded value", got)
	}
}

// A condition names its own off value, so the parent's falsy choice is not also
// consulted: a tag reading =off has to keep its dependent at "off".
func TestProvenanceIgnoresFalsyUnderAValueCondition(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.obs",
		Prefix:    "obs",
		KnownKeys: []string{"obs.tracing", "obs.off_note"},
		Defaults:  map[string]string{"obs.tracing": "off", "obs.off_note": "why tracing is off"},
		Falsy:     map[string]string{"obs.tracing": "off"},
		DependsOn: map[string][]configbind.Dependency{"obs.off_note": is("obs.tracing", "off")},
		FlagMetas: []cliparser.FieldMeta{{Prefix: "obs", Key: "tracing"}, {Prefix: "obs", Key: "off_note"}},
		Apply:     func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("obs")

	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "obs.tracing,obs.off_note" {
		t.Fatalf("keys=%s; the falsy value is the one the condition selects", got)
	}
}

// A value is compared in the terms of the parent's kind, so a duration condition
// matches every spelling of the same span.
func TestProvenanceComparesConditionValuesByKind(t *testing.T) {
	for _, threshold := range []string{"0", "0s", "0ms"} {
		configbind.ResetDefinitions()
		configbind.Register[firstProvenanceConfig](configbind.Definition{
			TypeName:  "example.test.sql",
			Prefix:    "sql",
			KnownKeys: []string{"sql.slow_threshold", "sql.slow_level"},
			Defaults:  map[string]string{"sql.slow_threshold": threshold, "sql.slow_level": "warn"},
			DependsOn: map[string][]configbind.Dependency{"sql.slow_level": isNot("sql.slow_threshold", "0s")},
			FlagMetas: []cliparser.FieldMeta{
				{Prefix: "sql", Key: "slow_threshold"},
				{Prefix: "sql", Key: "slow_level"},
			},
			Scaffold: []configbind.ScaffoldField{
				{Key: "slow_threshold", Kind: configbind.ScaffoldDuration},
				{Key: "slow_level", Kind: configbind.ScaffoldString},
			},
			Apply: func(any, *configbind.Overlay) error { return nil },
		})
		configbind.ResetTargets()
		configbind.Bind[firstProvenanceConfig]("sql")
		got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
		if got != "sql.slow_threshold" {
			t.Fatalf("threshold=%s keys=%s; every spelling of zero is the excluded value", threshold, got)
		}
	}
	configbind.ResetDefinitions()
	configbind.ResetTargets()
}

// A mode key normally carries its own dependon on the feature switch, so a value
// condition on that mode inherits the switch and needs no "enabled and mode=x".
func TestProvenanceValueConditionInheritsTheParentsOwnGate(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.auth",
		Prefix:    "auth",
		KnownKeys: []string{"auth.enabled", "auth.mode", "auth.oidc.issuer"},
		Defaults: map[string]string{
			"auth.enabled":     "false",
			"auth.mode":        "oidc_only",
			"auth.oidc.issuer": "https://issuer",
		},
		DependsOn: map[string][]configbind.Dependency{
			"auth.mode":        on("auth.enabled"),
			"auth.oidc.issuer": is("auth.mode", "oidc_only"),
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "auth", Key: "enabled"},
			{Prefix: "auth", Key: "mode"},
			{Prefix: "auth", Key: "oidc.issuer"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("auth")

	// mode still holds oidc_only, but it is hidden, so its dependents go too.
	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "auth.enabled" {
		t.Fatalf("keys=%s; a hidden mode hides what its value selected", got)
	}
	got = strings.Join(provenanceKeys(loadProvenanceFixture(t, []string{"AUTH_ENABLED=true"}, "")), ",")
	if got != "auth.enabled,auth.mode,auth.oidc.issuer" {
		t.Fatalf("keys=%s want the chain back once the switch is on", got)
	}
}

// registerSummaryFixture rates two of three keys as detail. jitter and note are
// tagged, level is not, and every key carries a default so the Place half of the
// rule is what the environment decides.
func registerSummaryFixture(t *testing.T) {
	t.Helper()
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.html",
		Prefix:    "html",
		KnownKeys: []string{"html.level", "html.jitter", "html.note"},
		Defaults: map[string]string{
			"html.level":  "info",
			"html.jitter": "20",
			"html.note":   "detail",
		},
		Summary: map[string]string{
			"html.jitter": configbind.SummaryOmit,
			"html.note":   configbind.SummaryOmit,
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "html", Key: "level"},
			{Prefix: "html", Key: "jitter"},
			{Prefix: "html", Key: "note"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("html")
}

// omittableKeys is the short surface: what a caller renders when it skips the
// entries the library marked.
func omittableKeys(entries []configbind.ProvenanceEntry) []string {
	var out []string
	for _, entry := range entries {
		if entry.Omittable {
			out = append(out, entry.Key)
		}
	}
	return out
}

func TestProvenanceMarksRatedDefaultsOmittable(t *testing.T) {
	registerSummaryFixture(t)

	entries := loadProvenanceFixture(t, nil, "")
	// Nothing is dropped: the caller, not the library, decides by surface.
	if got := strings.Join(provenanceKeys(entries), ","); got != "html.level,html.jitter,html.note" {
		t.Fatalf("keys=%s; a rating must remove nothing", got)
	}
	if got := strings.Join(omittableKeys(entries), ","); got != "html.jitter,html.note" {
		t.Fatalf("omittable=%s want the two rated keys", got)
	}
}

// The conjunction is the safety property: a rated key someone set is a decision
// this deployment made, and hiding a decision would be a bug.
func TestProvenanceKeepsRatedKeySetBySource(t *testing.T) {
	registerSummaryFixture(t)

	entries := loadProvenanceFixture(t, []string{"HTML_JITTER=35"}, "")
	if got := strings.Join(omittableKeys(entries), ","); got != "html.note" {
		t.Fatalf("omittable=%s; a source-set key must stay notable", got)
	}
	values := map[string]configbind.ProvenanceEntry{}
	for _, entry := range entries {
		values[entry.Key] = entry
	}
	if got := values["html.jitter"]; got.Place != configbind.PlaceEnv || got.Omittable {
		t.Fatalf("jitter=%+v want env and not omittable", got)
	}
}

func TestProvenanceNeverMarksAnUntaggedKey(t *testing.T) {
	registerSummaryFixture(t)

	for _, entry := range loadProvenanceFixture(t, nil, "") {
		if entry.Key == "html.level" && entry.Omittable {
			t.Fatalf("level=%+v; an untagged key at its default is still notable", entry)
		}
	}
}

// A rating reaches an array-of-tables element by its stable path under the array,
// the same way a secret mode does, and the per-element Place still decides.
//
// Today that makes it inert: Defaults is keyed by stable key and an element has
// none, so mergeDocument builds an element overlay from the file alone and every
// element field that appears was set by a source. The rating is still resolved,
// which is what keeps this correct rather than special-cased if element defaults
// are ever seeded; this test pins both halves so the coupling is visible.
func TestProvenanceMarksTableArrayElementFields(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.rdb",
		Prefix:    "rdb",
		KnownKeys: []string{"rdb.connections"},
		Summary:   map[string]string{"rdb.connections.max_idle_conns": configbind.SummaryOmit},
		Scaffold: []configbind.ScaffoldField{{
			Key:  "connections",
			Kind: configbind.ScaffoldTableArray,
			Nested: []configbind.ScaffoldField{
				{Key: "dsn", Kind: configbind.ScaffoldString},
				{Key: "max_idle_conns", Kind: configbind.ScaffoldInt, Default: "2"},
			},
		}},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("rdb")

	body := "[[rdb.connections]]\ndsn = \"sqlite://a.db\"\n[[rdb.connections]]\ndsn = \"sqlite://b.db\"\nmax_idle_conns = 8\n"
	entries := loadProvenanceFixture(t, nil, body)
	got := map[string]configbind.ProvenanceEntry{}
	for _, entry := range entries {
		got[entry.Key] = entry
	}
	// Element 0 never set max_idle_conns, and no default reaches an element, so
	// the key is absent rather than present-and-omittable.
	if _, ok := got["rdb.connections[0].max_idle_conns"]; ok {
		t.Fatalf("keys=%v; an element field has no default layer to fall back to", provenanceKeys(entries))
	}
	// Element 1 set it, so the rating is overruled by the Place half of the rule.
	e, ok := got["rdb.connections[1].max_idle_conns"]
	if !ok {
		t.Fatalf("keys=%v want the file-set element field", provenanceKeys(entries))
	}
	if e.Place != configbind.PlaceFile || e.Omittable {
		t.Fatalf("element 1=%+v want a notable file value", e)
	}
	if e := got["rdb.connections[0].dsn"]; e.Omittable {
		t.Fatalf("dsn=%+v; an untagged element field is not rated", e)
	}
}

// A falsy fill-in decides omittability through its Place and nothing else, which
// is what keeps the two features from needing to know about each other: an absent
// key filled in reports default and is droppable, while a source that set the key
// to "" keeps its own Place and is not.
func TestProvenanceRatesFalsyFillInByItsPlace(t *testing.T) {
	register := func() {
		configbind.ResetDefinitions()
		configbind.Register[firstProvenanceConfig](configbind.Definition{
			TypeName:  "example.test.obs",
			Prefix:    "obs",
			KnownKeys: []string{"obs.tracing"},
			Falsy:     map[string]string{"obs.tracing": "off"},
			Summary:   map[string]string{"obs.tracing": configbind.SummaryOmit},
			FlagMetas: []cliparser.FieldMeta{{Prefix: "obs", Key: "tracing"}},
			Apply:     func(any, *configbind.Overlay) error { return nil },
		})
		configbind.ResetTargets()
		configbind.Bind[firstProvenanceConfig]("obs")
	}
	t.Cleanup(configbind.ResetDefinitions)
	t.Cleanup(configbind.ResetTargets)

	register()
	entries := loadProvenanceFixture(t, nil, "")
	if len(entries) != 1 || entries[0].Place != configbind.PlaceDefault || !entries[0].Omittable {
		t.Fatalf("entries=%+v want an omittable default fill-in", entries)
	}

	register()
	entries = loadProvenanceFixture(t, []string{"OBS_TRACING="}, "")
	if len(entries) != 1 || entries[0].Place != configbind.PlaceEnv || entries[0].Omittable {
		t.Fatalf("entries=%+v; env decided the value, so it stays notable", entries)
	}
}

// A dropped entry is never marked, because it is not there to mark: the two
// policies compose by one removing what the other would have rated.
func TestProvenanceDoesNotMarkKeysDependencyDropped(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.session",
		Prefix:    "session",
		KnownKeys: []string{"session.backend", "session.dynamo.table"},
		Defaults: map[string]string{
			"session.backend":      "redis",
			"session.dynamo.table": "pw_session",
		},
		DependsOn: map[string][]configbind.Dependency{
			"session.dynamo.table": is("session.backend", "dynamo"),
		},
		Summary:   map[string]string{"session.dynamo.table": configbind.SummaryOmit},
		FlagMetas: []cliparser.FieldMeta{{Prefix: "session", Key: "backend"}, {Prefix: "session", Key: "dynamo.table"}},
		Apply:     func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("session")

	entries := loadProvenanceFixture(t, nil, "")
	if got := strings.Join(provenanceKeys(entries), ","); got != "session.backend" {
		t.Fatalf("keys=%s; the inert key is dropped, not merely rated", got)
	}
}

func TestFalsyOnlyFillsWhenNoDefaultExists(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.obs",
		Prefix:    "obs",
		KnownKeys: []string{"obs.tracing", "obs.metrics"},
		// tracing has no default, so its falsy choice fills in; metrics has one,
		// so the default wins and the falsy choice never applies.
		Defaults:  map[string]string{"obs.metrics": "prometheus"},
		Falsy:     map[string]string{"obs.tracing": "off", "obs.metrics": "off"},
		FlagMetas: []cliparser.FieldMeta{{Prefix: "obs", Key: "tracing"}, {Prefix: "obs", Key: "metrics"}},
		Apply:     func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("obs")

	values := map[string]configbind.ProvenanceEntry{}
	for _, entry := range loadProvenanceFixture(t, nil, "") {
		values[entry.Key] = entry
	}
	if got := values["obs.tracing"]; got.Value != "off" || got.Place != configbind.PlaceDefault {
		t.Fatalf("tracing=%+v want the falsy choice", got)
	}
	if got := values["obs.metrics"]; got.Value != "prometheus" {
		t.Fatalf("metrics=%+v; a default outranks falsy", got)
	}
}

func TestFalsyKeepsWinningPlaceWhenSourceIsEmpty(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.obs",
		Prefix:    "obs",
		KnownKeys: []string{"obs.tracing"},
		Falsy:     map[string]string{"obs.tracing": "off"},
		FlagMetas: []cliparser.FieldMeta{{Prefix: "obs", Key: "tracing"}},
		Apply:     func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("obs")

	// An env var set to "" still reports env: the source did decide the value.
	entries := loadProvenanceFixture(t, []string{"OBS_TRACING="}, "")
	if len(entries) != 1 {
		t.Fatalf("entries=%+v", entries)
	}
	if entries[0].Value != "off" || entries[0].Place != configbind.PlaceEnv {
		t.Fatalf("entry=%+v want off from env", entries[0])
	}
}

// A zero duration is a deliberate setting on its own, so it hides dependents
// only where a falsy tag says that zero is what "off" means for this key.
func TestProvenanceHidesDependentsOfZeroDurationParent(t *testing.T) {
	register := func(threshold string) {
		configbind.ResetDefinitions()
		configbind.Register[firstProvenanceConfig](configbind.Definition{
			TypeName:  "example.test.sql",
			Prefix:    "sql",
			KnownKeys: []string{"sql.slow_threshold", "sql.explain"},
			Defaults:  map[string]string{"sql.slow_threshold": threshold, "sql.explain": "true"},
			Falsy:     map[string]string{"sql.slow_threshold": "0s"},
			DependsOn: map[string][]configbind.Dependency{"sql.explain": on("sql.slow_threshold")},
			Scaffold: []configbind.ScaffoldField{
				{Key: "slow_threshold", Kind: configbind.ScaffoldDuration},
				{Key: "explain", Kind: configbind.ScaffoldBool},
			},
			FlagMetas: []cliparser.FieldMeta{
				{Prefix: "sql", Key: "slow_threshold"},
				{Prefix: "sql", Key: "explain"},
			},
			Apply: func(any, *configbind.Overlay) error { return nil },
		})
		configbind.ResetTargets()
		configbind.Bind[firstProvenanceConfig]("sql")
	}
	t.Cleanup(configbind.ResetDefinitions)
	t.Cleanup(configbind.ResetTargets)

	// "0" and "0ms" are the same duration as the declared "0s", which raw
	// string equality could not see.
	for _, zero := range []string{"0s", "0", "0ms"} {
		register(zero)
		got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
		if got != "sql.slow_threshold" {
			t.Fatalf("threshold=%s keys=%s want only the parent", zero, got)
		}
	}

	register("500ms")
	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "sql.slow_threshold,sql.explain" {
		t.Fatalf("keys=%s want the dependent back", got)
	}
}

// Without a falsy tag a zero duration stays a real setting, so its dependents
// keep printing.
func TestProvenanceKeepsDependentsOfZeroDurationWithoutFalsy(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.sql",
		Prefix:    "sql",
		KnownKeys: []string{"sql.slow_threshold", "sql.explain"},
		Defaults:  map[string]string{"sql.slow_threshold": "0s", "sql.explain": "true"},
		DependsOn: map[string][]configbind.Dependency{"sql.explain": on("sql.slow_threshold")},
		Scaffold: []configbind.ScaffoldField{
			{Key: "slow_threshold", Kind: configbind.ScaffoldDuration},
			{Key: "explain", Kind: configbind.ScaffoldBool},
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "sql", Key: "slow_threshold"},
			{Prefix: "sql", Key: "explain"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("sql")

	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "sql.slow_threshold,sql.explain" {
		t.Fatalf("keys=%s; a zero duration without falsy is a setting", got)
	}
}

// Every parent has to be non-empty: a key under a dependent struct answers to
// the struct's parent as well as its own.
func TestProvenanceHidesKeyWhenAnyParentIsEmpty(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.security",
		Prefix:    "security",
		KnownKeys: []string{"security.enabled", "security.mode", "security.hsts.max_age"},
		Defaults: map[string]string{
			"security.enabled":      "true",
			"security.mode":         "",
			"security.hsts.max_age": "31536000",
		},
		DependsOn: map[string][]configbind.Dependency{
			"security.hsts.max_age": on("security.enabled", "security.mode"),
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "security", Key: "enabled"},
			{Prefix: "security", Key: "mode"},
			{Prefix: "security", Key: "hsts.max_age"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("security")

	// enabled is set but mode is empty, so the leaf still goes.
	got := strings.Join(provenanceKeys(loadProvenanceFixture(t, nil, "")), ",")
	if got != "security.enabled,security.mode" {
		t.Fatalf("keys=%s want the leaf hidden by its second parent", got)
	}
	got = strings.Join(provenanceKeys(loadProvenanceFixture(t, []string{"SECURITY_MODE=strict"}, "")), ",")
	if got != "security.enabled,security.mode,security.hsts.max_age" {
		t.Fatalf("keys=%s want the leaf back once both parents are set", got)
	}
}

func TestProvenanceAppliesSecretTag(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	configbind.Register[firstProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.app",
		Prefix:    "app",
		KnownKeys: []string{"app.license", "app.owner", "app.identity_token", "app.plain"},
		Defaults: map[string]string{
			"app.license":        "L-1",
			"app.owner":          "ops",
			"app.identity_token": "public-claim",
			"app.plain":          "visible",
		},
		Secrets: map[string]string{
			"app.license":        "mask",
			"app.owner":          "hide",
			"app.identity_token": "show",
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "app", Key: "license"},
			{Prefix: "app", Key: "owner"},
			{Prefix: "app", Key: "identity_token"},
			{Prefix: "app", Key: "plain"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("app")

	entries := loadProvenanceFixture(t, nil, "")
	byKey := map[string]configbind.ProvenanceEntry{}
	for _, entry := range entries {
		byKey[entry.Key] = entry
	}
	if _, ok := byKey["app.owner"]; ok {
		t.Fatalf("hide must drop the entry: %v", entries)
	}
	if got := byKey["app.license"]; got.Value != "*****" || !got.Masked {
		t.Fatalf("license=%+v want a masked entry", got)
	}
	// show wins over the key-name policy, which would have masked this one.
	if got := byKey["app.identity_token"]; got.Value != "public-claim" || got.Masked {
		t.Fatalf("identity_token=%+v want the raw value", got)
	}
	if got := byKey["app.plain"]; got.Value != "visible" || got.Masked {
		t.Fatalf("plain=%+v want the raw value", got)
	}
}

func TestProvenanceMarksAutoMaskedEntries(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	registerProvenanceFixture[firstProvenanceConfig]("db", []string{"host", "password"}, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("db")

	for _, entry := range loadProvenanceFixture(t, nil, "") {
		wantMasked := entry.Key == "db.password"
		if entry.Masked != wantMasked {
			t.Fatalf("%s Masked=%v want %v", entry.Key, entry.Masked, wantMasked)
		}
	}
}

type tableArrayProvenanceConfig struct{}

// registerConnectionsFixture registers one array of tables whose element fields
// are declared group-then-dsn, which is the reverse of their alphabetical order.
func registerConnectionsFixture(secrets map[string]string, dependsOn map[string][]configbind.Dependency, extraKeys []string) {
	keys := append([]string{"rdb.connections"}, extraKeys...)
	configbind.Register[tableArrayProvenanceConfig](configbind.Definition{
		TypeName:  "example.test.rdb",
		Prefix:    "rdb",
		KnownKeys: keys,
		Secrets:   secrets,
		DependsOn: dependsOn,
		Apply:     func(any, *configbind.Overlay) error { return nil },
		Scaffold: []configbind.ScaffoldField{{
			Key:  "connections",
			Kind: configbind.ScaffoldTableArray,
			Nested: []configbind.ScaffoldField{
				{Key: "group", Kind: configbind.ScaffoldString},
				{Key: "dsn", Kind: configbind.ScaffoldString},
			},
		}},
	})
}

const connectionsTOML = `
[[rdb.connections]]
group = "writer"
dsn = "postgres://app:s3cret@primary/db"

[[rdb.connections]]
group = "reader"
dsn = "postgres://app:s3cret@replica/db"
`

func TestProvenanceExpandsTableArrayElements(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	registerConnectionsFixture(nil, nil, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[tableArrayProvenanceConfig]("rdb")

	entries := loadProvenanceFixture(t, nil, connectionsTOML)
	got := provenanceKeys(entries)
	want := []string{
		"rdb.connections[0].group",
		"rdb.connections[0].dsn",
		"rdb.connections[1].group",
		"rdb.connections[1].dsn",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("keys=%v want %v", got, want)
	}
	// The array key itself carries no value, so it is replaced by its elements.
	for _, entry := range entries {
		if entry.Key == "rdb.connections" {
			t.Fatalf("the bare array key must not be reported: %v", entries)
		}
	}
	if entries[0].ArrayKey != "rdb.connections" || entries[0].Index != 0 {
		t.Fatalf("entry[0]=%+v want ArrayKey rdb.connections index 0", entries[0])
	}
	if entries[3].ArrayKey != "rdb.connections" || entries[3].Index != 1 {
		t.Fatalf("entry[3]=%+v want ArrayKey rdb.connections index 1", entries[3])
	}
	if entries[0].Value != "writer" || entries[0].Place != configbind.PlaceFile {
		t.Fatalf("entry[0]=%+v want the file value", entries[0])
	}
}

// dsn is a sensitive key token, so an element value is masked with no tag at all.
func TestProvenanceMasksElementByKeyToken(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	registerConnectionsFixture(nil, nil, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[tableArrayProvenanceConfig]("rdb")

	for _, entry := range loadProvenanceFixture(t, nil, connectionsTOML) {
		wantMasked := strings.HasSuffix(entry.Key, ".dsn")
		if entry.Masked != wantMasked {
			t.Fatalf("%s Masked=%v want %v", entry.Key, entry.Masked, wantMasked)
		}
		if wantMasked && strings.Contains(entry.Value, "s3cret") {
			t.Fatalf("%s leaked the password: %q", entry.Key, entry.Value)
		}
	}
}

// A secret tag on an element field is keyed by its stable path under the array
// and applies at every index.
func TestProvenanceAppliesElementSecretAtEveryIndex(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	registerConnectionsFixture(map[string]string{"rdb.connections.group": "mask"}, nil, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[tableArrayProvenanceConfig]("rdb")

	masked := 0
	for _, entry := range loadProvenanceFixture(t, nil, connectionsTOML) {
		if strings.HasSuffix(entry.Key, ".group") {
			if !entry.Masked || entry.Value != "*****" {
				t.Fatalf("%s=%+v want a masked entry", entry.Key, entry)
			}
			masked++
		}
	}
	if masked != 2 {
		t.Fatalf("masked=%d want both elements", masked)
	}
}

// hide on an element field drops that field from every element, and hide on the
// array drops the whole set.
func TestProvenanceHidesElementFieldsAndWholeArray(t *testing.T) {
	for _, tc := range []struct {
		name    string
		secrets map[string]string
		want    []string
	}{
		{
			name:    "one field",
			secrets: map[string]string{"rdb.connections.dsn": "hide"},
			want:    []string{"rdb.connections[0].group", "rdb.connections[1].group"},
		},
		{
			name:    "whole array",
			secrets: map[string]string{"rdb.connections": "hide"},
			want:    nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configbind.ResetDefinitions()
			t.Cleanup(configbind.ResetDefinitions)
			registerConnectionsFixture(tc.secrets, nil, nil)
			configbind.ResetTargets()
			t.Cleanup(configbind.ResetTargets)
			configbind.Bind[tableArrayProvenanceConfig]("rdb")

			got := provenanceKeys(loadProvenanceFixture(t, nil, connectionsTOML))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("keys=%v want %v", got, tc.want)
			}
		})
	}
}

// An empty dependon parent folds the whole connection set away, which is the
// largest block a disabled feature owns.
func TestProvenanceHidesTableArrayUnderEmptyParent(t *testing.T) {
	configbind.ResetDefinitions()
	t.Cleanup(configbind.ResetDefinitions)
	registerConnectionsFixture(
		nil,
		map[string][]configbind.Dependency{"rdb.connections": on("rdb.enabled")},
		[]string{"rdb.enabled"},
	)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[tableArrayProvenanceConfig]("rdb")

	body := "rdb.enabled = false\n" + connectionsTOML
	got := provenanceKeys(loadProvenanceFixture(t, nil, body))
	// The parent stays: it is the reason its dependents vanished.
	if strings.Join(got, ",") != "rdb.enabled" {
		t.Fatalf("keys=%v want only rdb.enabled", got)
	}
}
