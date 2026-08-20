package configbind_test

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
)

type migrateOptions struct {
	Path   string
	Label  string
	DryRun bool
	Limit  int
	Tags   []string
	Extra  []string
}

type statusOptions struct {
	Verbose bool
}

type dummyConfig struct{}

func registerSubcommands(t *testing.T) {
	t.Helper()
	configbind.ResetDefinitions()
	configbind.ResetTargets()
	configbind.RegisterSubCommand[migrateOptions](configbind.SubCommandDefinition{
		TypeName: "configbind_test.migrateOptions",
		Name:     "migrate",
		Help:     "run migrations",
		Defaults: map[string]string{"dry_run": "false", "limit": "5"},
		FlagMetas: []cliparser.FieldMeta{
			{Key: "dry_run", Env: "-", Help: "print only", Kind: cliparser.KindBool},
			{Key: "limit", Env: "-", Opt: "limit,l", Help: "migration limit"},
			{Key: "tags", Env: "-", Opt: "tag", Help: "migration tag", Kind: cliparser.KindArray},
		},
		Positionals: []configbind.Positional{
			{ConfigKey: "path", Name: "path", Help: "migration path", Role: configbind.PositionalRequired},
			{ConfigKey: "label", Name: "label", Help: "optional label", Role: configbind.PositionalOptional},
			{ConfigKey: "extra", Name: "extra", Help: "extra inputs", Role: configbind.PositionalRest},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			value := dst.(*migrateOptions)
			if raw, ok := overlay.GetString("path"); ok {
				value.Path = raw
			}
			if raw, ok := overlay.GetString("label"); ok {
				value.Label = raw
			}
			if raw, ok := overlay.GetString("dry_run"); ok {
				parsed, err := strconv.ParseBool(raw)
				if err != nil {
					return err
				}
				value.DryRun = parsed
			}
			if raw, ok := overlay.GetString("limit"); ok {
				parsed, err := strconv.Atoi(raw)
				if err != nil {
					return err
				}
				value.Limit = parsed
			}
			if raw, ok := overlay.GetMulti("tags"); ok {
				value.Tags = raw
			}
			if raw, ok := overlay.GetMulti("extra"); ok {
				value.Extra = raw
			}
			return nil
		},
	})
	configbind.RegisterSubCommand[statusOptions](configbind.SubCommandDefinition{
		TypeName: "configbind_test.statusOptions",
		Name:     "status",
		Help:     "show status",
		FlagMetas: []cliparser.FieldMeta{
			{Key: "verbose", Env: "-", Help: "verbose output", Kind: cliparser.KindBool},
		},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			value := dst.(*statusOptions)
			if raw, ok := overlay.GetString("verbose"); ok {
				value.Verbose = raw == "true"
			}
			return nil
		},
	})
}

func useProcessArgs(t *testing.T, args ...string) {
	t.Helper()
	previous := os.Args
	os.Args = append([]string{"demo"}, args...)
	t.Cleanup(func() { os.Args = previous })
}

func TestSubCommandSelectsAndAppliesFlagsAndPositionals(t *testing.T) {
	registerSubcommands(t)
	useProcessArgs(t, "migrate", "./db", "--dry_run", "release", "-l", "9", "--tag", "a", "--tag=b", "one", "two")

	migrate := configbind.SubCommand[migrateOptions]("migrate", "run migrations")
	status := configbind.SubCommand[statusOptions]("status", "show status")
	if migrate == nil {
		t.Fatal("selected migrate command is nil")
	}
	if status != nil {
		t.Fatalf("unselected status command=%+v", status)
	}
	if migrate.Path != "./db" || migrate.Label != "release" || !migrate.DryRun || migrate.Limit != 9 {
		t.Fatalf("SubCommand returned before parsing options: %+v", migrate)
	}

	_, err := configbind.Load(configbind.LoadOptions{
		Args:    os.Args[1:],
		Environ: []string{"DRY_RUN=false", "LIMIT=100"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if migrate.Path != "./db" || migrate.Label != "release" || !migrate.DryRun || migrate.Limit != 9 {
		t.Fatalf("migrate=%+v", migrate)
	}
	if got := migrate.Extra; len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("Extra=%v", got)
	}
	if got := migrate.Tags; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Tags=%v", got)
	}
}

func TestSubCommandUsesDefaultsButNeverEnvironment(t *testing.T) {
	registerSubcommands(t)
	configbind.Register[dummyConfig](configbind.Definition{
		TypeName: "configbind_test.dummyConfig",
		Prefix:   "server",
		Apply:    func(any, *configbind.Overlay) error { return nil },
	})
	_ = configbind.Bind[dummyConfig]("server")
	configPath := t.TempDir() + "/config.toml"
	if err := os.WriteFile(configPath, []byte("dry_run = true\nlimit = 100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	useProcessArgs(t, "migrate", "./db")
	migrate := configbind.SubCommand[migrateOptions]("migrate", "run migrations")

	_, err := configbind.Load(configbind.LoadOptions{
		Args:               os.Args[1:],
		Environ:            []string{"DRY_RUN=true", "LIMIT=100"},
		ExplicitConfigPath: configPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if migrate.DryRun || migrate.Limit != 5 || migrate.Label != "" {
		t.Fatalf("subcommand read TOML/environment instead of generated defaults: %+v", migrate)
	}
}

func TestSubCommandMissingRequiredIncludesUsage(t *testing.T) {
	registerSubcommands(t)
	useProcessArgs(t, "migrate", "--dry_run")
	if command := configbind.SubCommand[migrateOptions]("migrate", "run migrations"); command != nil {
		t.Fatalf("command with invalid arguments=%+v", command)
	}

	_, err := configbind.Load(configbind.LoadOptions{Args: os.Args[1:], Environ: []string{}})
	var usageErr *configbind.UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("error=%T %v", err, err)
	}
	for _, want := range []string{"missing required argument <path>", "Usage: demo migrate", "<path>", "run migrations"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("usage error missing %q:\n%s", want, err)
		}
	}
}

func TestSubCommandTopLevelHelpListsCommands(t *testing.T) {
	registerSubcommands(t)
	useProcessArgs(t, "--help")
	if command := configbind.SubCommand[migrateOptions]("migrate", "run migrations"); command != nil {
		t.Fatalf("help selected command=%+v", command)
	}
	if command := configbind.SubCommand[statusOptions]("status", "show status"); command != nil {
		t.Fatalf("help selected command=%+v", command)
	}

	_, err := configbind.Load(configbind.LoadOptions{Args: os.Args[1:], Environ: []string{}})
	var usageErr *configbind.UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("error=%T %v", err, err)
	}
	for _, want := range []string{"Commands:", "migrate", "run migrations", "status", "show status"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("top-level usage missing %q:\n%s", want, err)
		}
	}
}

func TestSubCommandSelectionDoesNotDependOnBindCallOrder(t *testing.T) {
	registerSubcommands(t)
	configbind.Register[dummyConfig](configbind.Definition{
		TypeName: "configbind_test.dummyConfig",
		Prefix:   "server",
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "server", Key: "port", Opt: "port"},
		},
		Apply: func(any, *configbind.Overlay) error { return nil },
	})
	useProcessArgs(t, "--port", "8080", "migrate", "./db")

	migrate := configbind.SubCommand[migrateOptions]("migrate", "run migrations")
	if migrate == nil || migrate.Path != "./db" {
		t.Fatalf("SubCommand before Bind=%+v", migrate)
	}
	_ = configbind.Bind[dummyConfig]("server")
	configPath := t.TempDir() + "/config.toml"
	if err := os.WriteFile(configPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := configbind.Load(configbind.LoadOptions{
		Args:               os.Args[1:],
		Environ:            []string{},
		ExplicitConfigPath: configPath,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestEmptyArgvSelectsNoSubcommandAndLoads covers the wasm component case,
// where the program is entered through an exported function and
// wasi:cli/environment.get-arguments answers with an empty list. os.Args is
// then empty rather than holding a program name, and every layer that reaches
// for os.Args[0] or os.Args[1:] panicked on it — inside package init, where
// there is nothing left to report the panic.
func TestEmptyArgvSelectsNoSubcommandAndLoads(t *testing.T) {
	registerSubcommands(t)
	// A Bind target beside the subcommands is what an application has, and it
	// is what makes a bare invocation legal rather than a usage error.
	configbind.Register[emptyArgvConfig](configbind.Definition{
		TypeName:  "configbind_test.emptyArgvConfig",
		Prefix:    "webserver",
		KnownKeys: []string{"webserver.port"},
		Defaults:  map[string]string{"webserver.port": "8080"},
		FlagMetas: []cliparser.FieldMeta{{Prefix: "webserver", Key: "port"}},
		Apply: func(dst any, overlay *configbind.Overlay) error {
			value := dst.(*emptyArgvConfig)
			value.Port, _ = overlay.GetString("webserver.port")
			return nil
		},
	})
	bound := configbind.Bind[emptyArgvConfig]("webserver")

	previous := os.Args
	os.Args = nil
	t.Cleanup(func() { os.Args = previous })

	if selected := configbind.SubCommand[migrateOptions]("migrate", "run migrations"); selected != nil {
		t.Fatalf("empty argv selected a subcommand: %+v", selected)
	}
	if _, err := configbind.Load(configbind.LoadOptions{Vendor: "tinybind", Tool: "empty-argv", Environ: []string{}}); err != nil {
		t.Fatal(err)
	}
	if bound.Port != "8080" {
		t.Fatalf("Port=%q, want the default", bound.Port)
	}
}

type emptyArgvConfig struct{ Port string }
