package codegen

import (
	"strings"
	"testing"
)

func TestGenerateEmitsDefinitionRegistration(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		PackagePath: "example.test/fixture",
		TypeName:    "ServerConfig",
		Prefix:      "server",
		Fields:      []Field{{GoName: "Port", Key: "port", Kind: FieldInt, Default: "8080"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`configbind.Register[ServerConfig](configbind.Definition{`,
		`TypeName: "example.test/fixture.ServerConfig"`,
		`Prefix:   "server"`,
		`{Key: "port", Kind: configbind.ScaffoldInt, Default: "8080"}`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated scaffold registration %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateSupportsSameTypeAtMultiplePrefixes(t *testing.T) {
	src, err := Generate("fixture", []Spec{
		{PackagePath: "example.test/fixture", TypeName: "ServerConfig", Prefix: "primary", Fields: []Field{{GoName: "Port", Key: "port", Kind: FieldInt}}},
		{PackagePath: "example.test/fixture", TypeName: "ServerConfig", Prefix: "secondary", Fields: []Field{{GoName: "Port", Key: "port", Kind: FieldInt}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"registerServerConfigDefinition0",
		"registerServerConfigDefinition1",
		"applyServerConfigDefinition0",
		"applyServerConfigDefinition1",
		`Prefix:   "primary"`,
		`Prefix:   "secondary"`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated multi-prefix symbol %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateEmitsEnvironmentOverride(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "observability",
		Fields: []Field{
			{GoName: "ServiceName", Key: "service_name", Kind: FieldString, Env: "OTEL_SERVICE_NAME"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `Env: "OTEL_SERVICE_NAME"`) {
		t.Fatalf("generated environment override missing:\n%s", src)
	}
}

func TestGenerateRejectsDuplicateEnvironmentOverride(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "observability",
		Fields: []Field{
			{GoName: "ServiceName", Key: "service_name", Kind: FieldString, Env: "OTEL_SERVICE_NAME"},
			{GoName: "PeerName", Key: "peer_name", Kind: FieldString, Env: "OTEL_SERVICE_NAME"},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "duplicate environment variable") {
		t.Fatalf("error=%v", err)
	}
}

func TestGenerateEmitsSubCommandRegistrationAndPositionals(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		PackagePath: "example.test/fixture",
		TypeName:    "MigrateOptions",
		SubCommand:  true,
		Name:        "migrate",
		Help:        "run migrations",
		Fields: []Field{
			{GoName: "Path", Key: "path", Kind: FieldString, Arg: "required", Help: "migration path"},
			{GoName: "Label", Key: "label", Kind: FieldString, Arg: "optional"},
			{GoName: "DryRun", Key: "dry_run", Kind: FieldBool, Default: "false", Help: "print only"},
			{GoName: "Extra", Key: "extra", Kind: FieldStringSlice, Arg: "*"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`configbind.RegisterSubCommand[MigrateOptions]`,
		`Name:     "migrate"`,
		`Help:     "run migrations"`,
		`{Key: "dry_run", Env: "-", Help: "print only", Kind: cliparser.KindBool}`,
		`{ConfigKey: "path", Name: "path", Role: configbind.PositionalRequired, Help: "migration path"}`,
		`{ConfigKey: "label", Name: "label", Role: configbind.PositionalOptional}`,
		`{ConfigKey: "extra", Name: "extra", Role: configbind.PositionalRest}`,
		"applyMigrateOptionsDefinition0",
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated subcommand registration %q missing:\n%s", want, src)
		}
	}
	if strings.Contains(string(src), "Scaffold:") {
		t.Fatalf("subcommand fields must not enter scaffolds:\n%s", src)
	}
}

func TestGenerateRejectsInvalidSubCommandPositionals(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName:   "BadOptions",
		SubCommand: true,
		Name:       "bad",
		Help:       "bad command",
		Fields: []Field{
			{GoName: "Optional", Key: "optional", Kind: FieldString, Arg: "optional"},
			{GoName: "Required", Key: "required", Kind: FieldString, Arg: "required"},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "must precede") {
		t.Fatalf("error=%v", err)
	}
}

func TestGenerateTableArrayRules(t *testing.T) {
	routes := Field{
		GoName:   "Routes",
		Key:      "routes",
		Kind:     FieldStructSlice,
		ElemType: "RouteConfig",
		Nested:   []Field{{GoName: "Path", Key: "path", Kind: FieldString}},
	}

	t.Run("bind_emits_a_loop", func(t *testing.T) {
		src, err := Generate("sample", []Spec{{
			TypeName: "AppConfig", Prefix: "app", Fields: []Field{routes},
		}})
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		for _, want := range []string{
			`o.Get("app.routes")`,
			"if !ta1.IsTables {",
			"p.Routes = make([]RouteConfig, len(ta1.Tables))",
			`ta1.Tables[i1].GetString("path")`,
			`Key: "routes", Kind: configbind.ScaffoldTableArray`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in\n%s", want, text)
			}
		}
		// A repeated table has no flag or env form, so it stays out of FlagMetas.
		if strings.Contains(text, `Key: "routes", Opt`) || strings.Contains(text, `{Prefix: "app", Key: "routes"`) {
			t.Fatalf("routes must not become a flag:\n%s", text)
		}
	})

	t.Run("element_flags_rejected", func(t *testing.T) {
		flagged := routes
		flagged.Nested = []Field{{GoName: "Path", Key: "path", Kind: FieldString, Opt: "path"}}
		_, err := Generate("sample", []Spec{{
			TypeName: "AppConfig", Prefix: "app", Fields: []Field{flagged},
		}})
		if err == nil || !strings.Contains(err.Error(), "no flag or env form") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("subcommand_rejected", func(t *testing.T) {
		_, err := Generate("sample", []Spec{{
			TypeName: "Serve", SubCommand: true, Name: "serve", Help: "serve files",
			Fields: []Field{routes},
		}})
		if err == nil || !strings.Contains(err.Error(), "cannot take an array of tables") {
			t.Fatalf("err=%v", err)
		}
	})
}
