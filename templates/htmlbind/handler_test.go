package htmlbind

import (
	"strings"
	"testing"
)

const blockComponent = `export component Counter(label: string): html {
<script component>
export function setup(el) { return { increment() {}, validate() {} } }
</script>
<div class="c"><button on-click="increment" on-blur="validate" data-id="7">{label}</button></div>
}`

func generateHandlers(t *testing.T, source string, options GenerateOptions) (string, error) {
	t.Helper()
	options.Package = "id_"
	got, err := GenerateModule("page.tb.html", []byte(source), options)
	if err != nil {
		return "", err
	}
	return string(got.GoSource), nil
}

func TestClientHandlersLowerToOneMarker(t *testing.T) {
	out, err := generateHandlers(t, blockComponent, GenerateOptions{
		ClientHandlers: map[string]ClientHandlerSet{
			"Counter": {Resolved: []string{"increment", "validate"}},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// One attribute, comma between entries and colon within one, so a runtime
	// finds every bound element with a single indexed query.
	if !strings.Contains(out, `data-tb-on=\"click:increment,blur:validate\"`) {
		t.Errorf("the lowered marker is missing or misspelled:\n%s", out)
	}
	// The authored attributes are never emitted, exactly as server-action is not.
	if strings.Contains(out, "on-click") || strings.Contains(out, "on-blur") {
		t.Errorf("an authored on- attribute reached the output:\n%s", out)
	}
	if !strings.Contains(out, `data-id=\"7\"`) {
		t.Errorf("a sibling attribute was dropped:\n%s", out)
	}
}

func TestClientHandlerNamespaceIsFreeOutsideAScriptBlock(t *testing.T) {
	// rule:event-attribute-context assigns the hyphenated on- space to custom
	// elements, and this feature takes it back only where a handler could resolve.
	out, err := generateHandlers(t,
		`export component Plain(): html { <button on-click="increment">x</button> }`, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `on-click=\"increment\"`) {
		t.Errorf("an ordinary attribute was reinterpreted:\n%s", out)
	}
	if strings.Contains(out, "data-tb-on") {
		t.Errorf("a marker was written outside a script-block component:\n%s", out)
	}
}

func TestUnknownClientHandlerFailsGeneration(t *testing.T) {
	_, err := generateHandlers(t, blockComponent, GenerateOptions{
		ClientHandlers: map[string]ClientHandlerSet{"Counter": {Resolved: []string{"increment"}}},
	})
	if err == nil {
		t.Fatal("expected an error for a name the block does not export")
	}
	if !strings.Contains(err.Error(), "does not export") || !strings.Contains(err.Error(), "validate") {
		t.Errorf("error = %v", err)
	}
}

func TestUnresolvedClientHandlerReportsTheCallersReason(t *testing.T) {
	// The position is the module's and the reason is the caller's, which is what
	// lets the module diagnose a block it never read.
	_, err := generateHandlers(t, blockComponent, GenerateOptions{
		ClientHandlers: map[string]ClientHandlerSet{"Counter": {
			Resolved:   []string{"increment"},
			Unresolved: map[string]string{"validate": "setup returned it conditionally"},
		}},
	})
	if err == nil {
		t.Fatal("expected an error for an explicitly unresolved name")
	}
	if !strings.Contains(err.Error(), "setup returned it conditionally") {
		t.Errorf("error = %v, want it to carry the caller's reason", err)
	}
	if !strings.Contains(err.Error(), "page.tb.html") {
		t.Errorf("error = %v, want it to carry the template position", err)
	}
}

func TestAComponentWithNoResolvedSetIsUnchecked(t *testing.T) {
	// The reporting pass runs before the caller has anything to answer with, so
	// an absent entry must compile rather than reject every name.
	out, err := generateHandlers(t, blockComponent, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `data-tb-on=\"click:increment,blur:validate\"`) {
		t.Errorf("an unchecked component did not lower:\n%s", out)
	}
}

func TestClientHandlerRejectsBadValues(t *testing.T) {
	cases := []struct{ name, attrs, want string }{
		{"computed", `on-click={label}`, "must be a literal handler name"},
		{"bare", `on-click`, "must name a function"},
		{"not an identifier", `on-click="a b"`, "must be a function name"},
		{"repeated event", `on-click="increment" on-click="validate"`, "twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := `export component Counter(label: string): html {
<script component>
export function setup(el) {}
</script>
<div><button ` + tc.attrs + `>x</button></div>
}`
			_, err := generateHandlers(t, source, GenerateOptions{})
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestASecondHyphenIsNotAClientHandler(t *testing.T) {
	// on-my-event keeps the custom-element reading rule:event-attribute-context
	// gives it, so the two rosters divide the on- space along one line.
	out, err := generateHandlers(t, `export component Counter(): html {
<script component>
export function setup(el) {}
</script>
<div><my-el on-my-event="x"></my-el></div>
}`, GenerateOptions{PassthroughElements: []PassthroughElement{{Name: "my-el"}}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `on-my-event=\"x\"`) {
		t.Errorf("a hyphenated custom-element attribute was claimed:\n%s", out)
	}
}
