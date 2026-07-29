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

// registerProvenanceFixture registers one definition whose keys are deliberately
// not in alphabetical order, so a key sort would be visible in the output.
func registerProvenanceFixture[T any](prefix string, keys []string, dependsOn map[string][]string) {
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
		DependsOn: map[string][]string{
			"rdb.pool_size": {"rdb.dsn"},
			"rdb.pool_idle": {"rdb.pool_size"},
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
		DependsOn: map[string][]string{"limits.window": {"limits.max"}},
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
			DependsOn: map[string][]string{"sql.explain": {"sql.slow_threshold"}},
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
		DependsOn: map[string][]string{"sql.explain": {"sql.slow_threshold"}},
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
		DependsOn: map[string][]string{
			"security.hsts.max_age": {"security.enabled", "security.mode"},
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
func registerConnectionsFixture(secrets map[string]string, dependsOn map[string][]string, extraKeys []string) {
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
		map[string][]string{"rdb.connections": {"rdb.enabled"}},
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
