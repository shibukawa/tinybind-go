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
func registerProvenanceFixture[T any](prefix string, keys []string, dependsOn map[string]string) {
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
		[]string{"host", "password", "api_key", "access_key", "dsn"}, nil)
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
	configbind.Bind[firstProvenanceConfig]("db")

	values := map[string]string{}
	for _, entry := range loadProvenanceFixture(t, nil, "") {
		values[entry.Key] = entry.Value
	}
	for _, key := range []string{"db.password", "db.api_key", "db.access_key"} {
		if values[key] != "*****" {
			t.Fatalf("%s=%q want a mask", key, values[key])
		}
	}
	if values["db.host"] != "v-host" || values["db.dsn"] != "v-dsn" {
		t.Fatalf("non-sensitive keys were masked: %v", values)
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
		DependsOn: map[string]string{
			"rdb.pool_size": "rdb.dsn",
			"rdb.pool_idle": "rdb.pool_size",
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
		DependsOn: map[string]string{"limits.window": "limits.max"},
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
