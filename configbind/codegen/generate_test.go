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
		"DependsOn: map[string][]configbind.Dependency{",
		`"rdb.pool_size": {{Key: "rdb.dsn"}}`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated dependon %q missing:\n%s", want, src)
		}
	}
}

// A value condition is what lets a subtree belong to one choice of a mode key:
// such a key is non-empty in every mode, so an emptiness test cannot tell them
// apart. The comma separates alternative values here, not parents.
func TestGenerateEmitsValueConditions(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "AuthConfig",
		Prefix:   "auth",
		Fields: []Field{
			{GoName: "Mode", Key: "mode", Kind: FieldString, Default: "oidc_only", Enum: "oidc_only,oidc_passkey,jwt_only"},
			{GoName: "Passkey", Key: "passkey", Kind: FieldStruct, DependsOn: ".mode=oidc_only,oidc_passkey", Nested: []Field{
				{GoName: "Path", Key: "path", Kind: FieldString},
			}},
			{GoName: "Legacy", Key: "legacy", Kind: FieldStruct, DependsOn: ".mode!=jwt_only", Nested: []Field{
				{GoName: "Path", Key: "path", Kind: FieldString},
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"auth.passkey.path": {{Key: "auth.mode", Op: "=", Values: []string{"oidc_only", "oidc_passkey"}}}`,
		`"auth.legacy.path": {{Key: "auth.mode", Op: "!=", Values: []string{"jwt_only"}}}`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
}

// A number parent needs a falsy tag only for an emptiness test. A condition that
// names the value inline has already said which one means off.
func TestGenerateAcceptsValueConditionOnNumberWithoutFalsy(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "QueryConfig",
		Prefix:   "query",
		Fields: []Field{
			{GoName: "SlowThreshold", Key: "slow_threshold", Kind: FieldDuration, Default: "200ms"},
			{GoName: "SlowLevel", Key: "slow_level", Kind: FieldString, DependsOn: ".slow_threshold!=0s"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsNormalized(src, `"query.slow_level": {{Key: "query.slow_threshold", Op: "!=", Values: []string{"0s"}}}`) {
		t.Fatalf("condition missing:\n%s", src)
	}
}

// Only the first "=" is the operator, so a value may carry one of its own.
func TestGenerateKeepsEqualsInsideAValue(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "AppConfig",
		Prefix:   "app",
		Fields: []Field{
			{GoName: "Marker", Key: "marker", Kind: FieldString},
			{GoName: "Detail", Key: "detail", Kind: FieldString, DependsOn: ".marker=a=b"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsNormalized(src, `"app.detail": {{Key: "app.marker", Op: "=", Values: []string{"a=b"}}}`) {
		t.Fatalf("value lost its equals sign:\n%s", src)
	}
}

// A parent generated elsewhere has no kind and no enum here, so its values pass
// unchecked rather than failing the build — the same blind spot the parent-kind
// check already accepts.
func TestGenerateAcceptsValueConditionOnForeignParent(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "AppConfig",
		Prefix:   "app",
		Fields:   []Field{{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "middleware.rdb.driver=postgres"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsNormalized(src, `"app.pool": {{Key: "middleware.rdb.driver", Op: "=", Values: []string{"postgres"}}}`) {
		t.Fatalf("cross-package condition missing:\n%s", src)
	}
}

// A summary tag rates a key as detail. Subtree propagation is what makes it
// affordable: one tag on a struct covers every key below it, which is how a
// deployment's unremarkable defaults get rated without a tag per line.
func TestGenerateEmitsSummaryMap(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "obs",
		Fields: []Field{
			{GoName: "MinimumLevel", Key: "minimum_level", Kind: FieldString, Default: "info"},
			{GoName: "Query", Key: "query", Kind: FieldStruct, Summary: "omit", Nested: []Field{
				{GoName: "Explain", Key: "explain", Kind: FieldBool, Default: "true"},
				{GoName: "MaxSQLLength", Key: "max_sql_length", Kind: FieldInt, Default: "4096"},
			}},
			{GoName: "Jitter", Key: "jitter", Kind: FieldInt, Default: "20", Summary: "omit"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Summary: map[string]string{",
		`"obs.query.explain": "omit"`,
		`"obs.query.max_sql_length": "omit"`,
		`"obs.jitter": "omit"`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
	if containsNormalized(src, `"obs.minimum_level": "omit"`) {
		t.Fatalf("an untagged key was rated:\n%s", src)
	}
}

// Unlike dependon, falsy, and enum, this rates the key being printed rather than
// naming one to look up, so it needs no stable key of its own and reaches an
// array-of-tables element by the same path secret uses.
func TestGenerateEmitsSummaryForTableArrayElements(t *testing.T) {
	connections := func(arrayTag string, elementTag string) []Field {
		return []Field{{
			GoName:   "Connections",
			Key:      "connections",
			Kind:     FieldStructSlice,
			ElemType: "ConnectionConfig",
			Summary:  arrayTag,
			Nested: []Field{
				{GoName: "DSN", Key: "dsn", Kind: FieldString},
				{GoName: "MaxIdleConns", Key: "max_idle_conns", Kind: FieldInt, Summary: elementTag},
			},
		}}
	}
	// An element field carries its own rating.
	src, err := Generate("fixture", []Spec{{TypeName: "RDBConfig", Prefix: "rdb", Fields: connections("", "omit")}})
	if err != nil {
		t.Fatal(err)
	}
	if !containsNormalized(src, `"rdb.connections.max_idle_conns": "omit"`) {
		t.Fatalf("element rating missing:\n%s", src)
	}
	if containsNormalized(src, `"rdb.connections.dsn": "omit"`) {
		t.Fatalf("an untagged sibling was rated:\n%s", src)
	}
	// A rating on the array field reaches every element field under it.
	src, err = Generate("fixture", []Spec{{TypeName: "RDBConfig", Prefix: "rdb", Fields: connections("omit", "")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"rdb.connections.dsn": "omit"`,
		`"rdb.connections.max_idle_conns": "omit"`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
}

func TestGenerateRejectsBadSummaryMode(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ObservabilityConfig",
		Prefix:   "obs",
		Fields:   []Field{{GoName: "Jitter", Key: "jitter", Kind: FieldInt, Summary: "hide"}},
	}})
	if err == nil || !strings.Contains(err.Error(), `summary must be omit, got "hide"`) {
		t.Fatalf("err=%v want a rejected summary mode", err)
	}
}

// A subcommand field never reaches provenance, so a rating there rates it for an
// output it does not appear in. An inherited tag is caught the same way.
func TestGenerateRejectsSummaryInSubCommand(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields []Field
	}{
		{
			name:   "leaf",
			fields: []Field{{GoName: "DryRun", Key: "dry_run", Kind: FieldBool, Summary: "omit"}},
		},
		{
			name: "inherited from a struct",
			fields: []Field{{GoName: "Detail", Key: "detail", Kind: FieldStruct, Summary: "omit", Nested: []Field{
				{GoName: "Verbose", Key: "verbose", Kind: FieldBool},
			}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Generate("fixture", []Spec{{
				TypeName:   "MigrateOptions",
				SubCommand: true,
				Name:       "migrate",
				Help:       "run migrations",
				Fields:     tc.fields,
			}})
			if err == nil || !strings.Contains(err.Error(), "cannot use summary") {
				t.Fatalf("err=%v want a subcommand rejection", err)
			}
		})
	}
}

// An enum tag names a value, which neither a struct nor an array has of its own.
func TestGenerateRejectsEnumOnNonLeafFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields []Field
		want   string
	}{
		{
			name: "nested struct",
			fields: []Field{{GoName: "TLS", Key: "tls", Kind: FieldStruct, Enum: "a,b", Nested: []Field{
				{GoName: "Path", Key: "path", Kind: FieldString},
			}}},
			want: "enum applies to a leaf field, not to the nested struct tls",
		},
		{
			name: "array of tables",
			fields: []Field{{GoName: "Routes", Key: "routes", Kind: FieldStructSlice, ElemType: "RouteConfig", Enum: "a,b", Nested: []Field{
				{GoName: "Dir", Key: "dir", Kind: FieldString},
			}}},
			want: "enum applies to a leaf field, not to the array of tables routes",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Generate("fixture", []Spec{{TypeName: "ServerConfig", Prefix: "server", Fields: tc.fields}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
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
	if !strings.Contains(string(src), `"app.pool": {{Key: "middleware.rdb.dsn"}}`) {
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
		{
			name: "operator with no parent",
			fields: []Field{
				{GoName: "Mode", Key: "mode", Kind: FieldString},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "=fast"},
			},
			want: "needs a parent key before the operator",
		},
		{
			name: "empty value",
			fields: []Field{
				{GoName: "Mode", Key: "mode", Kind: FieldString},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.mode=fast,"},
			},
			want: "names an empty value",
		},
		{
			name: "repeated value",
			fields: []Field{
				{GoName: "Mode", Key: "mode", Kind: FieldString},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.mode=fast,fast"},
			},
			want: `names "fast" twice`,
		},
		{
			// The one failure this feature can cause: a typo hides a subtree for
			// good, and no reader can tell from an output the key is missing from.
			name: "value outside the parent's enum",
			fields: []Field{
				{GoName: "Mode", Key: "mode", Kind: FieldString, Enum: "fast,slow"},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.mode=faast"},
			},
			want: "is not one of the enum choices",
		},
		{
			name: "non-bool value on a bool parent",
			fields: []Field{
				{GoName: "Enabled", Key: "enabled", Kind: FieldBool},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.enabled=yes"},
			},
			want: "is a bool, so \"yes\" is not a value it can hold",
		},
		{
			name: "unparsable duration value",
			fields: []Field{
				{GoName: "Window", Key: "window", Kind: FieldDuration},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.window=soon"},
			},
			want: "is not a duration",
		},
		{
			name: "value on a list parent",
			fields: []Field{
				{GoName: "Origins", Key: "origins", Kind: FieldStringSlice},
				{GoName: "Pool", Key: "pool", Kind: FieldInt, DependsOn: "rdb.origins=any"},
			},
			want: "must be a string, bool, int, or duration field",
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
		`"obs.url": {{Key: "obs.tracing"}}`,
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
		if err == nil || !strings.Contains(err.Error(), "no flag, env, or positional form") {
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
	// The element loop reads its own overlay, and diagnostics name the full path
	// including the element index: file position is an element's only identifier.
	for _, want := range []string{
		`ta1.Tables[i1].GetString("max_age")`,
		`"configbind: server.routes[%d].max_age: %w", i1, err`,
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
		`"security.hsts.max_age": {{Key: "security.enabled"}}`,
		`"security.hsts.preload": {{Key: "security.enabled"}}`,
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
	if !containsNormalized(src, `"security.hsts.max_age": {{Key: "security.enabled"}, {Key: "security.mode"}}`) {
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
		`"server.health.path": {{Key: "server.health.enabled"}}`,
		`"server.readiness.path": {{Key: "server.readiness.enabled"}}`,
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

// A secret on the array of tables covers every field of every element. The array
// owns one stable key, so a mode written there has somewhere to land; only the
// elements' own keys carry a run-time index.
func TestGenerateSpreadsSecretOverTableArray(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{{
			GoName:   "Routes",
			Key:      "routes",
			Kind:     FieldStructSlice,
			ElemType: "RouteConfig",
			Secret:   "mask",
			Nested: []Field{
				{GoName: "Dir", Key: "dir", Kind: FieldString},
				{GoName: "Root", Key: "root", Kind: FieldString},
			},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// gofmt aligns map values, so compare with runs of spaces collapsed.
	text := collapseSpaces(string(src))
	for _, want := range []string{
		`"server.routes": "mask"`,
		`"server.routes.dir": "mask"`,
		`"server.routes.root": "mask"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
}

func collapseSpaces(s string) string {
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// An element field's own secret tag is honored and outranks the array's mode.
// It is indexed by the element's path under the array key, because the element's
// own key carries an index that exists only at run time.
func TestGenerateKeepsElementSecretByStablePath(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "RDBConfig",
		Prefix:   "rdb",
		Fields: []Field{{
			GoName:   "Connections",
			Key:      "connections",
			Kind:     FieldStructSlice,
			ElemType: "ConnectionConfig",
			Nested: []Field{
				{GoName: "Group", Key: "group", Kind: FieldString},
				{GoName: "DSN", Key: "dsn", Kind: FieldString, Secret: "mask"},
			},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := collapseSpaces(string(src))
	if !strings.Contains(text, `"rdb.connections.dsn": "mask"`) {
		t.Fatalf("element secret missing:\n%s", text)
	}
	// The untagged sibling gains no mode, and no indexed key is ever generated.
	if strings.Contains(text, `"rdb.connections.group"`) {
		t.Fatalf("untagged element field must not gain a mode:\n%s", text)
	}
	if strings.Contains(text, "connections[0]") {
		t.Fatalf("secret map must not hold a run-time index:\n%s", text)
	}
}

// falsy still has no single value to name on an array of tables.
func TestGenerateRejectsFalsyOnTableArray(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{{
			GoName:   "Routes",
			Key:      "routes",
			Kind:     FieldStructSlice,
			ElemType: "RouteConfig",
			Falsy:    "off",
			Nested:   []Field{{GoName: "Dir", Key: "dir", Kind: FieldString}},
		}},
	}})
	if err == nil || !strings.Contains(err.Error(), "falsy applies to a leaf field") {
		t.Fatalf("err=%v want a falsy rejection", err)
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

// A tag that addresses one scalar key has nowhere to land on a struct or an
// array, and the generator says so instead of dropping it. A default on a nested
// struct was accepted and silently generated nothing before this check existed.
func TestGenerateRejectsScalarOnlyTagsOnContainers(t *testing.T) {
	nested := Field{
		GoName: "Health",
		Key:    "health",
		Kind:   FieldStruct,
		Nested: []Field{{GoName: "Path", Key: "path", Kind: FieldString}},
	}
	array := Field{
		GoName:   "Routes",
		Key:      "routes",
		Kind:     FieldStructSlice,
		ElemType: "RouteConfig",
		Nested:   []Field{{GoName: "Dir", Key: "dir", Kind: FieldString}},
	}
	for _, tc := range []struct {
		name  string
		field Field
		want  string
	}{
		{"default on struct", withDefault(nested, "path=/healthz"), "default applies to a leaf field, not to the nested struct"},
		{"opt on struct", withOpt(nested, "health"), "opt applies to a leaf field, not to the nested struct"},
		{"env on struct", withEnv(nested, "HEALTH"), "env applies to a leaf field, not to the nested struct"},
		{"default on array", withDefault(array, "dir=/srv"), "default applies to a leaf field, not to the array of tables"},
		// opt and env on an array were already rejected, by an earlier check with
		// its own wording; the case is here so the coverage stays total.
		{"env on array", withEnv(array, "ROUTES"), "an array of tables has no flag or env form"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Generate("fixture", []Spec{{
				TypeName: "ServerConfig", Prefix: "server", Fields: []Field{tc.field},
			}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}

// help is exempt: godoc backfill writes help tags onto struct fields, so
// rejecting it here would fail the generation run that wrote it.
func TestGenerateAcceptsHelpOnContainers(t *testing.T) {
	_, err := Generate("fixture", []Spec{{
		TypeName: "ServerConfig",
		Prefix:   "server",
		Fields: []Field{{
			GoName: "Health",
			Key:    "health",
			Kind:   FieldStruct,
			Help:   "health endpoint settings",
			Nested: []Field{{GoName: "Path", Key: "path", Kind: FieldString}},
		}},
	}})
	if err != nil {
		t.Fatalf("help on a nested struct must be accepted: %v", err)
	}
}

func withDefault(f Field, v string) Field { f.Default = v; return f }
func withOpt(f Field, v string) Field     { f.Opt = v; return f }
func withEnv(f Field, v string) Field     { f.Env = v; return f }
