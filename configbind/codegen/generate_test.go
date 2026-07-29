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

func TestGenerateEmitsDurationField(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		PackagePath: "example.test/fixture",
		TypeName:    "ServerConfig",
		Prefix:      "server",
		Fields: []Field{
			{GoName: "ReadTimeout", Key: "read_timeout", Kind: FieldDuration, Default: "1h30m"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"time"`,
		`{Key: "read_timeout", Kind: configbind.ScaffoldDuration, Default: "1h30m"}`,
		`d, err := time.ParseDuration(v)`,
		`p.ReadTimeout = 5400000000000 // 1h30m0s`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated duration field %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateRejectsUnparsableDurationDefault(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields:   []Field{{GoName: "ReadTimeout", Key: "read_timeout", Kind: FieldDuration, Default: "5"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "invalid duration default") {
		t.Fatalf("err=%v want a rejected bare-number default", err)
	}
}

func TestGenerateEmitsDependsOnMap(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "RDBConfig",
		Prefix:   "rdb",
		Fields: []Field{
			{GoName: "DSN", Key: "dsn", Kind: FieldString},
			{GoName: "PoolSize", Key: "pool_size", Kind: FieldInt, Default: "10", DependsOn: "rdb.dsn"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"DependsOn: map[string]string{",
		`"rdb.pool_size": "rdb.dsn"`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated dependon %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateAcceptsDependsOnFromAnotherPackage(t *testing.T) {
	// The parent lives in a package generated elsewhere, so it cannot be checked
	// here; the tag passes through unvalidated rather than failing the build.
	src, err := Generate("fixture", []Spec{{
		TypeName: "AppConfig",
		Prefix:   "app",
		Fields:   []Field{{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "middleware.rdb.dsn"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `"app.pool": "middleware.rdb.dsn"`) {
		t.Fatalf("cross-package parent missing:\n%s", src)
	}
}

func TestGenerateRejectsBadDependsOn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields []Field
		want   string
	}{
		{
			name: "non-scalar parent",
			fields: []Field{
				{GoName: "Port", Key: "port", Kind: FieldInt},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.port"},
			},
			want: "must be a string or bool field",
		},
		{
			name:   "self reference",
			fields: []Field{{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.pool"}},
			want:   "refers to itself",
		},
		{
			name: "several parents",
			fields: []Field{
				{GoName: "DSN", Key: "dsn", Kind: FieldString},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.dsn,rdb.other"},
			},
			want: "one parent key",
		},
		{
			name: "cycle",
			fields: []Field{
				{GoName: "A", Key: "a", Kind: FieldString, DependsOn: "rdb.b"},
				{GoName: "B", Key: "b", Kind: FieldString, DependsOn: "rdb.a"},
			},
			want: "dependon cycle",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Generate("fixture", []Spec{{TypeName: "RDBConfig", Prefix: "rdb", Fields: tc.fields}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}

func TestGenerateRejectsDependsOnInSubCommand(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName:   "MigrateOptions",
		SubCommand: true,
		Name:       "migrate",
		Help:       "run migrations",
		Fields:     []Field{{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.dsn"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "cannot use dependon") {
		t.Fatalf("err=%v want a subcommand rejection", err)
	}
}

func TestGenerateEmitsFalsyMap(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "obs",
		Fields: []Field{
			{GoName: "Tracing", Key: "tracing", Kind: FieldString, Falsy: "off"},
			{GoName: "URL", Key: "url", Kind: FieldString, DependsOn: "obs.tracing"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Falsy: map[string]string{",
		`"obs.tracing": "off"`,
		`"obs.url": "obs.tracing"`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated falsy %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateRejectsFalsyOnNonStringField(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "obs",
		Fields:   []Field{{GoName: "Level", Key: "level", Kind: FieldInt, Falsy: "0"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "falsy applies to string fields only") {
		t.Fatalf("err=%v want a kind rejection", err)
	}
}

func TestGenerateRejectsFalsyInSubCommand(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName:   "MigrateOptions",
		SubCommand: true,
		Name:       "migrate",
		Help:       "run migrations",
		Fields:     []Field{{GoName: "Mode", Key: "mode", Kind: FieldString, Falsy: "off"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "cannot use falsy") {
		t.Fatalf("err=%v want a subcommand rejection", err)
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

func TestGenerateDurationInsideTableArrayElement(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{{
			GoName:   "Routes",
			Key:      "routes",
			Kind:     FieldStructSlice,
			ElemType: "RouteConfig",
			Nested: []Field{
				{GoName: "MaxAge", Key: "max_age", Kind: FieldDuration, Default: "1h"},
			},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	// The element loop reads its own overlay, and diagnostics name the full path.
	for _, want := range []string{
		`ta1.Tables[i1].GetString("max_age")`,
		`"configbind: server.routes.max_age: %w"`,
		`p.Routes[i1].MaxAge = 3600000000000 // 1h0m0s`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated element duration %q missing:\n%s", want, text)
		}
	}
	if strings.Contains(text, `o.GetString("max_age")`) {
		t.Fatalf("element field must not read the top-level overlay:\n%s", text)
	}
}

func TestGenerateRejectsDependsOnInsideTableArrayElement(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{{
			GoName:   "Routes",
			Key:      "routes",
			Kind:     FieldStructSlice,
			ElemType: "RouteConfig",
			Nested: []Field{
				{GoName: "Dir", Key: "dir", Kind: FieldString, DependsOn: "server.root"},
			},
		}},
	}})
	if err == nil || !strings.Contains(err.Error(), "no provenance key for dependon or falsy") {
		t.Fatalf("err=%v want an element-field rejection", err)
	}
}
