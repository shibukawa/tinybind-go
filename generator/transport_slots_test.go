package generator_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

const httpbindPath = "github.com/shibukawa/tinybind-go"

func defaultPatterns(t *testing.T) map[string]generator.CallPattern {
	t.Helper()
	options, err := generator.NewCallRegistry().Options(generator.DefaultOptions())
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	byKey := map[string]generator.CallPattern{}
	for _, pattern := range options.Calls.Set {
		if pattern.Target.Function == nil {
			continue
		}
		byKey[pattern.Target.Function.PackagePath+"."+pattern.Target.Function.Name] = pattern
	}
	return byKey
}

func slot(t *testing.T, name string, p *int) int {
	t.Helper()
	if p == nil {
		t.Fatalf("%s slot is undeclared", name)
	}
	return *p
}

// Without these the transform cannot know which arguments vanish when both
// halves of the transport collapse into one value.
func TestDefaultsDeclareTransportSlots(t *testing.T) {
	patterns := defaultPatterns(t)

	bind, ok := patterns[httpbindPath+".Bind"]
	if !ok {
		t.Fatal("no default pattern for Bind")
	}
	if got := slot(t, "Bind request", bind.Transport.Request); got != 0 {
		t.Errorf("Bind request slot = %d, want 0", got)
	}
	if bind.Transport.Writer != nil {
		t.Errorf("Bind declares a writer slot; it takes no writer")
	}

	for _, name := range []string{"Write", "WriteStatus", "NewStream", "WriteStream"} {
		pattern, ok := patterns[httpbindPath+"."+name]
		if !ok {
			t.Fatalf("no default pattern for %s", name)
		}
		if got := slot(t, name+" writer", pattern.Transport.Writer); got != 0 {
			t.Errorf("%s writer slot = %d, want 0", name, got)
		}
		if got := slot(t, name+" request", pattern.Transport.Request); got != 1 {
			t.Errorf("%s request slot = %d, want 1", name, got)
		}
		if !pattern.Transport.Drops(0) || !pattern.Transport.Drops(1) || pattern.Transport.Drops(2) {
			t.Errorf("%s Drops is wrong: 0=%v 1=%v 2=%v", name,
				pattern.Transport.Drops(0), pattern.Transport.Drops(1), pattern.Transport.Drops(2))
		}
	}
}

// The canonical names are spelled once per runtime package, so a same-named
// function in a runtime with no transport must not inherit the claim.
func TestNonHTTPRuntimesDeclareNoTransportSlots(t *testing.T) {
	for _, pattern := range mustOptions(t).Calls.Set {
		if pattern.Target.Function == nil || pattern.Target.Function.PackagePath == httpbindPath {
			continue
		}
		if pattern.Transport.Declared() {
			t.Errorf("%s.%s declares a transport slot",
				pattern.Target.Function.PackagePath, pattern.Target.Function.Name)
		}
	}
}

func mustOptions(t *testing.T) generator.Options {
	t.Helper()
	options, err := generator.NewCallRegistry().Options(generator.DefaultOptions())
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	return options
}

func TestTransportSlotValidation(t *testing.T) {
	target := generator.Function("example.com/fw", "Respond")
	for _, tc := range []struct {
		name    string
		options []generator.CallPatternOption
		wantErr string
	}{
		{
			name:    "negative writer",
			options: []generator.CallPatternOption{generator.GenericType("response", 0), generator.WriterArgument(-1)},
			wantErr: "negative transport slot",
		},
		{
			name:    "same argument twice",
			options: []generator.CallPatternOption{generator.GenericType("response", 0), generator.WriterArgument(1), generator.RequestArgument(1)},
			wantErr: "both writer and request",
		},
		{
			name: "slot also supplies a value role",
			options: []generator.CallPatternOption{
				generator.GenericType("response", 0), generator.WriterArgument(0),
				generator.RequestArgument(1), generator.Argument("status", 1),
			},
			wantErr: "cannot also supply role",
		},
		{
			name: "slot also supplies a type role",
			options: []generator.CallPatternOption{
				generator.ArgumentType("response", 0), generator.WriterArgument(0),
			},
			wantErr: "cannot also supply type role",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := generator.NewCallRegistry().Register(generator.ResponseWriteCall(target, tc.options...))
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestFrameworkWrapperDeclaresItsOwnSlots(t *testing.T) {
	// func Created(ctx context.Context, w http.ResponseWriter, r *http.Request, value any) error
	registry := generator.NewCallRegistry()
	err := registry.Register(generator.ResponseWriteStatusCall(
		generator.Function("example.com/fw", "Created"),
		generator.ArgumentType("response", 3),
		generator.Constant("status", 201),
		generator.WriterArgument(1),
		generator.RequestArgument(2),
	))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	options, err := registry.Options(generator.DefaultOptions())
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	for _, pattern := range options.Calls.Set {
		if pattern.Target.Function == nil || pattern.Target.Function.Name != "Created" {
			continue
		}
		if got := slot(t, "Created writer", pattern.Transport.Writer); got != 1 {
			t.Errorf("writer slot = %d, want 1", got)
		}
		if got := slot(t, "Created request", pattern.Transport.Request); got != 2 {
			t.Errorf("request slot = %d, want 2", got)
		}
		// The context leads and the value trails; neither is a transport slot.
		if pattern.Transport.Drops(0) || pattern.Transport.Drops(3) {
			t.Error("a non-transport argument was marked as dropped")
		}
		return
	}
	t.Fatal("registered pattern did not survive into the options snapshot")
}
