package configbind_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
)

func writeEnvFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func placeOf(t *testing.T, result *configbind.LoadResult, key string) configbind.Place {
	t.Helper()
	entry, ok := result.Overlay.Get(key)
	if !ok {
		t.Fatalf("%s is absent from the overlay", key)
	}
	return entry.Place
}

func TestEnvFilesLaterFileWinsAndEnvironWinsOverEveryFile(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	dir := t.TempDir()
	first := writeEnvFile(t, dir, ".env", "PORT=1\nWEBSERVER_HOST=from-first\n")
	second := writeEnvFile(t, dir, ".env.stg", "PORT=2\n")

	cfg := configbind.Bind[testServerConfig]("webserver")
	result, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		Args:     []string{},
		Environ:  []string{"WEBSERVER_HOST=from-process"},
		EnvFiles: []string{first, second},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 2 {
		t.Fatalf("Port=%d want 2 (later file wins)", cfg.Port)
	}
	if cfg.Host != "from-process" {
		t.Fatalf("Host=%q want from-process (Environ wins over every file)", cfg.Host)
	}
	if got := placeOf(t, result, "webserver.port"); got != configbind.PlaceEnvFile+configbind.Place(second) {
		t.Fatalf("port place=%q want the later file", got)
	}
	if got := placeOf(t, result, "webserver.host"); got != configbind.PlaceEnv {
		t.Fatalf("host place=%q want env", got)
	}
}

func TestEnvFilesPlaceNamesTheFile(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	dir := t.TempDir()
	stg := writeEnvFile(t, dir, ".env.stg", "PORT=9090\n")

	configbind.Bind[testServerConfig]("webserver")
	result, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		Args:     []string{},
		Environ:  []string{},
		EnvFiles: []string{stg},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	place := placeOf(t, result, "webserver.port")
	if place != configbind.PlaceEnvFile+configbind.Place(stg) {
		t.Fatalf("place=%q", place)
	}
	file, ok := configbind.EnvFileOf(place)
	if !ok || file != stg {
		t.Fatalf("EnvFileOf(%q)=%q,%v", place, file, ok)
	}
	if _, ok := configbind.EnvFileOf(configbind.PlaceEnv); ok {
		t.Fatalf("PlaceEnv must not decode as a file")
	}
	if got := placeOf(t, result, "webserver.host"); got != configbind.PlaceDefault {
		t.Fatalf("untouched key place=%q want default", got)
	}
}

func TestEnvFilesFeedTOMLInterpolation(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	dir := t.TempDir()
	envFile := writeEnvFile(t, dir, ".env", "MY_HOST=from-dotenv\n")
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[webserver]\nhost = \"${MY_HOST}\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := configbind.Bind[testServerConfig]("webserver")
	result, err := configbind.Load(configbind.LoadOptions{
		ExplicitConfigPath: tomlPath,
		Args:               []string{},
		Environ:            []string{},
		EnvFiles:           []string{envFile},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host != "from-dotenv" {
		t.Fatalf("Host=%q want the dotenv value through ${MY_HOST}", cfg.Host)
	}
	if got := placeOf(t, result, "webserver.host"); got != configbind.PlaceFile {
		t.Fatalf("expanded key place=%q want file_toml", got)
	}
}

func TestEnvFilesSkipMissingAndReportRead(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	dir := t.TempDir()
	present := writeEnvFile(t, dir, ".env", "PORT=7\n")
	missing := filepath.Join(dir, ".env.nope")

	cfg := configbind.Bind[testServerConfig]("webserver")
	result, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		Args:     []string{},
		Environ:  []string{},
		EnvFiles: []string{missing, present},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 7 {
		t.Fatalf("Port=%d", cfg.Port)
	}
	if len(result.EnvFiles) != 1 || result.EnvFiles[0] != present {
		t.Fatalf("EnvFiles=%v want only the present file", result.EnvFiles)
	}
}

func TestEnvFilesUnreadablePresentFileIsAnError(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	dir := t.TempDir()
	configbind.Bind[testServerConfig]("webserver")
	_, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		Args:     []string{},
		Environ:  []string{},
		EnvFiles: []string{dir}, // exists, is a directory
	})
	if err == nil || !strings.Contains(err.Error(), "read env file") {
		t.Fatalf("err=%v want a read error", err)
	}
}

func TestEnvFilesParseErrorNamesFileAndLine(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	dir := t.TempDir()
	bad := writeEnvFile(t, dir, ".env", "PORT=1\nNOT AN ASSIGNMENT\n")
	configbind.Bind[testServerConfig]("webserver")
	_, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		Args:     []string{},
		Environ:  []string{},
		EnvFiles: []string{bad},
	})
	if err == nil {
		t.Fatal("want a parse error")
	}
	want := "configbind: read env file " + `"` + bad + `"` + " line 2: "
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("err=%q want prefix %q", err, want)
	}
}

func TestEnvFilesEmptyAssignmentCountsAsSet(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	dir := t.TempDir()
	envFile := writeEnvFile(t, dir, ".env", "WEBSERVER_HOST=\n")
	cfg := configbind.Bind[testServerConfig]("webserver")
	result, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		Args:     []string{},
		Environ:  []string{},
		EnvFiles: []string{envFile},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Host != "" {
		t.Fatalf("Host=%q want empty", cfg.Host)
	}
	if got := placeOf(t, result, "webserver.host"); got != configbind.PlaceEnvFile+configbind.Place(envFile) {
		t.Fatalf("place=%q", got)
	}
}

func TestEnvFilesNilBehavesAsBefore(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	cfg := configbind.Bind[testServerConfig]("webserver")
	result, err := configbind.Load(configbind.LoadOptions{
		Vendor:  "acme-missing-vendor-xyz",
		Tool:    "tool-missing-xyz",
		Args:    []string{},
		Environ: []string{"PORT=5"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 5 || placeOf(t, result, "webserver.port") != configbind.PlaceEnv {
		t.Fatalf("cfg=%+v place=%q", cfg, placeOf(t, result, "webserver.port"))
	}
	if result.EnvFiles != nil {
		t.Fatalf("EnvFiles=%v want nil", result.EnvFiles)
	}
}

func TestEnvFilesNeverReachSubcommands(t *testing.T) {
	registerSubcommands(t)
	configbind.Register[dummyConfig](configbind.Definition{
		TypeName: "configbind_test.dummyConfig",
		Prefix:   "server",
		Apply:    func(any, *configbind.Overlay) error { return nil },
	})
	_ = configbind.Bind[dummyConfig]("server")
	dir := t.TempDir()
	envFile := writeEnvFile(t, dir, ".env", "DRY_RUN=true\nLIMIT=100\n")
	useProcessArgs(t, "migrate", "./db")
	migrate := configbind.SubCommand[migrateOptions]("migrate", "run migrations")

	_, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		Args:     os.Args[1:],
		Environ:  []string{},
		EnvFiles: []string{envFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	if migrate.DryRun || migrate.Limit != 5 {
		t.Fatalf("subcommand read a dotenv file instead of generated defaults: %+v", migrate)
	}
}

func TestEnvVariableReportsRegisteredName(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	configbind.Register[struct{ Off, On string }](configbind.Definition{
		TypeName:  "envVariableProbe",
		Prefix:    "probe",
		KnownKeys: []string{"probe.off", "probe.on"},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "probe", Key: "off", Env: "-"},
			{Prefix: "probe", Key: "on", Env: "PROBE_ON_EXPLICIT"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	cases := map[string]string{
		"webserver.port": "PORT",
		"webserver.host": "WEBSERVER_HOST",
		"probe.on":       "PROBE_ON_EXPLICIT",
		"probe.off":      "",
		"unknown.key":    "",
	}
	for key, want := range cases {
		if got := configbind.EnvVariable(key); got != want {
			t.Errorf("EnvVariable(%q)=%q want %q", key, got, want)
		}
	}
}
