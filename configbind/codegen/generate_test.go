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
		"DependsOn: map[string][]string{",
		`"rdb.pool_size": {"rdb.dsn"}`,
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
	if !strings.Contains(string(src), `"app.pool": {"middleware.rdb.dsn"}`) {
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
			// An int parent has no empty of its own until a falsy tag names one.
			name: "number parent without falsy",
			fields: []Field{
				{GoName: "Port", Key: "port", Kind: FieldInt},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.port"},
			},
			want: "needs its own falsy tag",
		},
		{
			name: "list parent",
			fields: []Field{
				{GoName: "Origins", Key: "origins", Kind: FieldStringSlice},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.origins"},
			},
			want: "must be a string, bool, int, or duration field",
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
		`"obs.url": {"obs.tracing"}`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated falsy %q missing:\n%s", want, src)
		}
	}
}

func TestGenerateRejectsFalsyOnUnsupportedKind(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "obs",
		Fields:   []Field{{GoName: "Enabled", Key: "enabled", Kind: FieldBool, Falsy: "false"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "falsy applies to string, int, and duration fields only") {
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

// containsNormalized reports whether src contains want once both have their
// runs of whitespace collapsed, so gofmt's map-key alignment does not decide
// whether an assertion matches.
func containsNormalized(src []byte, want string) bool {
	collapse := func(text string) string { return strings.Join(strings.Fields(text), " ") }
	return strings.Contains(collapse(string(src)), collapse(want))
}

// A dependon on a nested struct covers the whole subtree, so every leaf under
// it answers to that parent without repeating the tag.
func TestGenerateSpreadsDependsOnOverNestedStruct(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "SecurityConfig",
		Prefix:   "security",
		Fields: []Field{
			{GoName: "Enabled", Key: "enabled", Kind: FieldBool},
			{
				GoName:    "HSTS",
				Key:       "hsts",
				Kind:      FieldStruct,
				DependsOn: "security.enabled",
				Nested: []Field{
					{GoName: "MaxAge", Key: "max_age", Kind: FieldInt},
					{GoName: "Preload", Key: "preload", Kind: FieldBool},
				},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"security.hsts.max_age": {"security.enabled"}`,
		`"security.hsts.preload": {"security.enabled"}`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
}

// A leaf inside a dependent subtree keeps its own parent and gains the one the
// struct declared: both have to be set for the key to be worth printing.
func TestGenerateKeepsLeafParentInsideDependentStruct(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "SecurityConfig",
		Prefix:   "security",
		Fields: []Field{
			{GoName: "Enabled", Key: "enabled", Kind: FieldBool},
			{GoName: "Mode", Key: "mode", Kind: FieldString},
			{
				GoName:    "HSTS",
				Key:       "hsts",
				Kind:      FieldStruct,
				DependsOn: "security.enabled",
				Nested: []Field{
					{GoName: "MaxAge", Key: "max_age", Kind: FieldInt, DependsOn: "security.mode"},
				},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsNormalized(src, `"security.hsts.max_age": {"security.enabled", "security.mode"}`) {
		t.Fatalf("both parents expected:\n%s", src)
	}
}

// A relative parent resolves against the struct the tag is written in, so one
// shared struct type names its own sibling under every prefix it is embedded at.
func TestGenerateResolvesRelativeDependsOn(t *testing.T) {
	endpoint := func() []Field {
		return []Field{
			{GoName: "Enabled", Key: "enabled", Kind: FieldBool},
			{GoName: "Path", Key: "path", Kind: FieldString, DependsOn: ".enabled"},
		}
	}
	src, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{
			{GoName: "Health", Key: "health", Kind: FieldStruct, Nested: endpoint()},
			{GoName: "Readiness", Key: "readiness", Kind: FieldStruct, Nested: endpoint()},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"server.health.path": {"server.health.enabled"}`,
		`"server.readiness.path": {"server.readiness.enabled"}`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
}

func TestGenerateRejectsFalsyOnNestedStruct(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "SecurityConfig",
		Prefix:   "security",
		Fields: []Field{{
			GoName: "HSTS",
			Key:    "hsts",
			Kind:   FieldStruct,
			Falsy:  "off",
			Nested: []Field{{GoName: "MaxAge", Key: "max_age", Kind: FieldInt}},
		}},
	}})
	if err == nil || !strings.Contains(err.Error(), "falsy applies to a leaf field") {
		t.Fatalf("err=%v want a nested-struct rejection", err)
	}
}

func TestGenerateParsesIntegersAtFieldWidth(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "LimitConfig",
		Prefix:   "limit",
		Fields: []Field{
			{GoName: "MaxBody", Key: "max_body", Kind: FieldInt, GoType: "int64", Default: "9223372036854775807"},
			{GoName: "Workers", Key: "workers", Kind: FieldInt, GoType: "uint32"},
			{GoName: "Port", Key: "port", Kind: FieldInt},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"strconv.ParseInt(v, 10, 64)",
		"p.MaxBody = int64(n)",
		"p.MaxBody = 9223372036854775807",
		"strconv.ParseUint(v, 10, 32)",
		"p.Workers = uint32(n)",
		// An empty GoType stays int, so a field built before widths were
		// carried keeps working.
		"strconv.ParseInt(v, 10, 0)",
		"p.Port = int(n)",
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
}

func TestGenerateRejectsOutOfRangeIntegerDefault(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "LimitConfig",
		Prefix:   "limit",
		Fields:   []Field{{GoName: "Workers", Key: "workers", Kind: FieldInt, GoType: "int32", Default: "3000000000"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "invalid int32 default") {
		t.Fatalf("err=%v want an out-of-range default rejection", err)
	}
}

func TestGenerateRejectsUnknownIntegerGoType(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "LimitConfig",
		Prefix:   "limit",
		Fields:   []Field{{GoName: "Workers", Key: "workers", Kind: FieldInt, GoType: "float64"}},
	}})
	if err == nil || !strings.Contains(err.Error(), `unsupported integer type "float64"`) {
		t.Fatalf("err=%v want an unsupported-type rejection", err)
	}
}

func TestGenerateRejectsSecretInSubCommand(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName:   "MigrateOptions",
		SubCommand: true,
		Name:       "migrate",
		Help:       "run migrations",
		Fields:     []Field{{GoName: "Token", Key: "token", Kind: FieldString, Secret: "hide"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "cannot use secret") {
		t.Fatalf("err=%v want a subcommand rejection", err)
	}
}

func TestGenerateRejectsSecretOnTableArray(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{{
			GoName:   "Routes",
			Key:      "routes",
			Kind:     FieldStructSlice,
			ElemType: "RouteConfig",
			Secret:   "hide",
			Nested:   []Field{{GoName: "Dir", Key: "dir", Kind: FieldString}},
		}},
	}})
	if err == nil || !strings.Contains(err.Error(), "secret does not apply to the array of tables") {
		t.Fatalf("err=%v want an array-of-tables rejection", err)
	}
}

func TestGenerateRejectsUnknownSecretMode(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields:   []Field{{GoName: "Token", Key: "token", Kind: FieldString, Secret: "true"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "secret must be hide, mask, or show") {
		t.Fatalf("err=%v want a mode rejection", err)
	}
}

// A secret on a nested struct covers its subtree, the way dependon does.
func TestGenerateSpreadsSecretOverNestedStruct(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{{
			GoName: "Upstream",
			Key:    "upstream",
			Kind:   FieldStruct,
			Secret: "mask",
			Nested: []Field{
				{GoName: "User", Key: "user", Kind: FieldString},
				{GoName: "Password", Key: "password", Kind: FieldString, Secret: "hide"},
			},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"server.upstream.user": "mask"`,
		// A leaf's own tag wins over the one it inherits.
		`"server.upstream.password": "hide"`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
}
