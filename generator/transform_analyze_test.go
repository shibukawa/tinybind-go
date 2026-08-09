package generator

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func analyzeFixture(t *testing.T, dir string) *TransformPlan {
	t.Helper()
	pkg, err := loadPackage(filepath.Join("..", "testdata", dir))
	if err != nil {
		t.Fatalf("load %s: %v", dir, err)
	}
	plan, err := AnalyzeTransform(pkg, DefaultTransformOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	return plan
}

func names(plan *TransformPlan) (admitted, refused []string) {
	for _, c := range plan.Admitted {
		admitted = append(admitted, c.Name)
	}
	for _, r := range plan.Refusals {
		refused = append(refused, r.Function)
	}
	sort.Strings(admitted)
	sort.Strings(refused)
	return
}

func TestAnalyzeTransformSplitsThePackage(t *testing.T) {
	plan := analyzeFixture(t, "transform_eligibility")
	admitted, refused := names(plan)

	wantAdmitted := []string{"callsAdmittedHelper", "contextHandler", "discardHandler", "plainHandler", "renderOK"}
	wantRefused := []string{
		"closureHandler", "escapeHandler", "inheritsRefusal",
		"refusedHelper", "typeAssertionHandler", "unknownCallHandler", "unknownSelectorHandler",
	}
	if strings.Join(admitted, ",") != strings.Join(wantAdmitted, ",") {
		t.Errorf("admitted = %v\n    want %v", admitted, wantAdmitted)
	}
	if strings.Join(refused, ",") != strings.Join(wantRefused, ",") {
		t.Errorf("refused = %v\n    want %v", refused, wantRefused)
	}
}

func refusalFor(t *testing.T, plan *TransformPlan, name string) TransformRefusal {
	t.Helper()
	for _, r := range plan.Refusals {
		if r.Function == name {
			return r
		}
	}
	t.Fatalf("%s was not refused", name)
	return TransformRefusal{}
}

func TestRefusalKinds(t *testing.T) {
	plan := analyzeFixture(t, "transform_eligibility")
	for name, want := range map[string]TransformRefusalKind{
		"unknownCallHandler":     RefusalUnknownCall,
		"unknownSelectorHandler": RefusalUnknownSelector,
		"typeAssertionHandler":   RefusalTypeAssertion,
		"closureHandler":         RefusalEscapes,
		"escapeHandler":          RefusalEscapes,
		"inheritsRefusal":        RefusalInheritedFromCallee,
	} {
		if got := refusalFor(t, plan, name).Kind; got != want {
			t.Errorf("%s kind = %q, want %q", name, got, want)
		}
	}
}

// A refusal a developer cannot act on is indistinguishable from the feature not
// working, so the message has to carry the occurrence and the way out.
func TestRefusalMessageNamesTheOccurrenceAndARemedy(t *testing.T) {
	plan := analyzeFixture(t, "transform_eligibility")
	msg := refusalFor(t, plan, "unknownCallHandler").Error()

	for _, want := range []string{
		"unknownCallHandler is not transformable",
		"main.go:", // the position of the occurrence
		"passes w to fmt.Fprint",
		"remedy:",
		"register it as a call pattern",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

// An inherited refusal is useless without the hop that caused it.
func TestInheritedRefusalPrintsItsChain(t *testing.T) {
	plan := analyzeFixture(t, "transform_eligibility")
	refusal := refusalFor(t, plan, "inheritsRefusal")

	if len(refusal.Chain) == 0 {
		t.Fatal("inherited refusal carries no chain")
	}
	if refusal.Chain[0].Function != "refusedHelper" {
		t.Errorf("first hop = %q, want refusedHelper", refusal.Chain[0].Function)
	}
	msg := refusal.Error()
	for _, want := range []string{"calls refusedHelper", "r.URL"} {
		if !strings.Contains(msg, want) {
			t.Errorf("chain message missing %q:\n%s", want, msg)
		}
	}
}

// Refusals cluster on shared helpers, so one run has to report all of them.
func TestAllRefusalsAreReportedTogether(t *testing.T) {
	plan := analyzeFixture(t, "transform_eligibility")
	if len(plan.Refusals) < 7 {
		t.Errorf("reported %d refusals, expected every one in the package", len(plan.Refusals))
	}
	for i := 1; i < len(plan.Refusals); i++ {
		if plan.Refusals[i-1].Position.Line > plan.Refusals[i].Position.Line {
			t.Errorf("refusals are not in source order at %d", i)
		}
	}
}

func TestTransportParamsAreRecorded(t *testing.T) {
	plan := analyzeFixture(t, "transform_eligibility")
	for _, c := range plan.Admitted {
		if c.Name != "renderOK" {
			continue
		}
		if strings.Join(c.TransportParams, ",") != "w,r" {
			t.Errorf("renderOK transport params = %v, want [w r]", c.TransportParams)
		}
		return
	}
	t.Fatal("renderOK was not admitted")
}

func TestImportRewriteIsConfiguration(t *testing.T) {
	options := DefaultTransformOptions()
	to, ok := options.RewriteImport("github.com/shibukawa/tinybind-go")
	if !ok || to != fasthttpbindImportPath {
		t.Errorf("default runtime rewrite = %q (%v), want %q", to, ok, fasthttpbindImportPath)
	}
	if _, ok := options.RewriteImport("example.com/unmapped"); ok {
		t.Error("an unmapped path reported a rewrite")
	}

	// A framework supplies its own pair; nothing about it is built in.
	options.ImportRewrites["example.com/fw/render"] = "example.com/fw/render/fast"
	if to, ok := options.RewriteImport("example.com/fw/render"); !ok || to != "example.com/fw/render/fast" {
		t.Errorf("framework rewrite = %q (%v)", to, ok)
	}
}

func TestTransformOptionsValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*TransformOptions)
		wantErr string
	}{
		{"no context type", func(o *TransformOptions) { o.ContextType = "" }, "requires a ContextType"},
		{"empty rewrite target", func(o *TransformOptions) { o.ImportRewrites["a"] = "" }, "needs both paths"},
		{"self rewrite", func(o *TransformOptions) { o.ImportRewrites["a"] = "a" }, "maps to itself"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := DefaultTransformOptions()
			tc.mutate(&options)
			if _, err := options.normalized(); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}
