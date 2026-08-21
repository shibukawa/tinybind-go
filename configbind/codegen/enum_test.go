package codegen

import (
	"strings"
	"testing"
)

// rule:enum-value-validation is enforced where the value is applied, so the
// check covers every source at once: a default, a TOML key, an environment
// variable, and a flag all reach the same assignment.
func TestGenerateChecksEnumValuesAtLoad(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "RunConfig",
		Prefix:   "run",
		Fields: []Field{
			{GoName: "Topology", Key: "topology", Kind: FieldString, Default: "standalone", Enum: "standalone, listen, p2p"},
			{GoName: "Port", Key: "port", Kind: FieldInt, Enum: "80,443"},
			{GoName: "Beat", Key: "beat", Kind: FieldDuration, Enum: "1s,1m"},
			{GoName: "Enable", Key: "enable", Kind: FieldStringSlice, Enum: "websocket,webrtc"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		// The tag's spaces are trimmed, as rule:enum-tag-semantics has them trimmed
		// for a request model.
		`case "standalone", "listen", "p2p":`,
		`return fmt.Errorf("configbind: run.topology: %q must be one of: standalone, listen, p2p", v)`,
		// An int and a duration are matched on the parsed value, so a choice is
		// one number rather than one spelling of it.
		"switch n {\n\t\tcase 80, 443:",
		"switch d {\n\t\tcase 1000000000, 60000000000: // 1s, 1m",
		// A list is a vocabulary its elements are drawn from, so the element that
		// failed is the one named.
		"for _, item := range v {",
		`return fmt.Errorf("configbind: run.enable: %q must be one of: websocket, webrtc", item)`,
	} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("generated enum check %q missing:\n%s", want, src)
		}
	}
}

// An enum needs no stable config key, only the value in hand, which an element
// overlay holds exactly as a leaf does. The rejection that used to cover it is
// left naming dependon and falsy, which do need a key.
func TestGenerateChecksEnumInsideTableArrayElement(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "BuildConfig",
		Prefix:   "build",
		Fields: []Field{{
			GoName:   "Targets",
			Key:      "target",
			Kind:     FieldStructSlice,
			ElemType: "TargetConfig",
			Nested: []Field{
				{GoName: "Kind", Key: "kind", Kind: FieldString, Enum: "wasm,native"},
			},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// The element's only identifier is its position, so the rejection names it.
	want := `return fmt.Errorf("configbind: build.target[%d].kind: %q must be one of: wasm, native", i1, v)`
	if !strings.Contains(string(src), want) {
		t.Fatalf("generated element enum check %q missing:\n%s", want, src)
	}
}

// A default and a falsy value each name one value of the field they sit on, so
// an allowlist on that field has to contain them. Neither typo has a runtime
// symptom: an unlisted default is simply applied, and an unlisted falsy quietly
// disables the emptiness test that dependon rides on.
func TestGenerateRejectsEnumTagsItCannotHonor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields []Field
		want   string
	}{
		{
			name:   "default outside the enum",
			fields: []Field{{GoName: "Mode", Key: "mode", Kind: FieldString, Enum: "fast,slow", Default: "quick"}},
			want:   `field Mode: default "quick" is not one of the enum choices "fast,slow"`,
		},
		{
			name:   "falsy outside the enum",
			fields: []Field{{GoName: "Tracing", Key: "tracing", Kind: FieldString, Enum: "otlp,jaeger", Falsy: "off"}},
			want:   `field Tracing: falsy "off" is not one of the enum choices "otlp,jaeger"`,
		},
		{
			name:   "bool field",
			fields: []Field{{GoName: "Enabled", Key: "enabled", Kind: FieldBool, Enum: "true,false"}},
			want:   "a bool already holds only true and false",
		},
		{
			name:   "empty choice",
			fields: []Field{{GoName: "Mode", Key: "mode", Kind: FieldString, Enum: "fast,,slow"}},
			want:   `enum "fast,,slow" names an empty choice`,
		},
		{
			name:   "repeated choice",
			fields: []Field{{GoName: "Beat", Key: "beat", Kind: FieldDuration, Enum: "60s,1m"}},
			want:   `field Beat: enum names "60s" twice`,
		},
		{
			name:   "unparsable int choice",
			fields: []Field{{GoName: "Port", Key: "port", Kind: FieldInt, Enum: "80,https"}},
			want:   `field Port: enum value "https" is not a valid int`,
		},
		{
			name:   "unparsable duration choice",
			fields: []Field{{GoName: "Beat", Key: "beat", Kind: FieldDuration, Enum: "1s,soon"}},
			want:   `field Beat: enum value "soon" is not a duration`,
		},
		{
			name: "element field of an array of tables",
			fields: []Field{{GoName: "Targets", Key: "target", Kind: FieldStructSlice, ElemType: "TargetConfig", Nested: []Field{
				{GoName: "Kind", Key: "kind", Kind: FieldString, Enum: "wasm,,native"},
			}}},
			want: `field Targets.Kind: enum "wasm,,native" names an empty choice`,
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

// The allowlist reaches the surfaces a developer reads before the one that
// rejects them, which is where the typo would otherwise be made.
func TestGenerateCarriesEnumIntoScaffoldAndFlagMetas(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName: "RunConfig",
		Prefix:   "run",
		Fields: []Field{
			{GoName: "Topology", Key: "topology", Kind: FieldString, Default: "standalone", Enum: "standalone,p2p", Help: "execution topology"},
			{GoName: "Targets", Key: "target", Kind: FieldStructSlice, ElemType: "TargetConfig", Nested: []Field{
				{GoName: "Kind", Key: "kind", Kind: FieldString, Enum: "wasm,native"},
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{Prefix: "run", Key: "topology", Help: "execution topology", Enum: []string{"standalone", "p2p"}}`,
		`{Key: "topology", Kind: configbind.ScaffoldString, Default: "standalone", Help: "execution topology", Enum: []string{"standalone", "p2p"}}`,
		// An element field has no flag form, so the scaffold is the only surface
		// that can carry its choices.
		`{Key: "kind", Kind: configbind.ScaffoldString, Enum: []string{"wasm", "native"}}`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("generated enum metadata %q missing:\n%s", want, src)
		}
	}
}

// A subcommand field is CLI-only, and rule:enum-value-validation covers it too.
func TestGenerateCarriesEnumThroughSubCommandOptionsAndArguments(t *testing.T) {
	src, err := Generate("fixture", []Spec{{
		TypeName:   "MigrateOptions",
		SubCommand: true,
		Name:       "migrate",
		Help:       "run database migrations",
		Fields: []Field{
			{GoName: "Direction", Key: "direction", Kind: FieldString, Arg: "required", Enum: "up,down"},
			{GoName: "Mode", Key: "mode", Kind: FieldString, Enum: "dry,apply", Help: "how far to go"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{Key: "mode", Env: "-", Help: "how far to go", Enum: []string{"dry", "apply"}}`,
		`{ConfigKey: "direction", Name: "direction", Role: configbind.PositionalRequired, Enum: []string{"up", "down"}}`,
		`return fmt.Errorf("configbind: mode: %q must be one of: dry, apply", v)`,
	} {
		if !containsNormalized(src, want) {
			t.Fatalf("generated subcommand enum %q missing:\n%s", want, src)
		}
	}
}
