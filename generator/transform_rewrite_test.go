package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func rewriteFixture(t *testing.T, dir string) *TransformOutput {
	t.Helper()
	pkg, err := loadPackage(filepath.Join("..", "testdata", dir))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	plan, err := AnalyzeTransform(pkg, DefaultTransformOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(plan.Refusals) != 0 {
		t.Fatalf("fixture should be fully transformable, got:\n%s", plan.Refusals.Error())
	}
	out, err := RewriteTransform(pkg, plan, DefaultTransformOptions())
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	return out
}

func TestRewriteProducesTheFasthttpShape(t *testing.T) {
	got := string(rewriteFixture(t, "transform_rewrite").Source)

	for _, want := range []string{
		"//go:build fasthttp",
		"func createUser(ctx *fasthttp.RequestCtx)",
		"func renderUser(ctx *fasthttp.RequestCtx, out CreateUserResponse)",
		"func cancelAware(ctx *fasthttp.RequestCtx)",
		// the import swaps, and the local name is preserved so the selectors do not move
		`httpbind "github.com/shibukawa/tinybind-go/fasthttpbind"`,
		`"github.com/shibukawa/tinygodriver/fasthttp"`,
		// transport arguments collapse into the one context
		"httpbind.Bind[CreateUserRequest](ctx)",
		"httpbind.WriteError(ctx, err)",
		"httpbind.Write[CreateUserResponse](ctx, out)",
		"renderUser(ctx, CreateUserResponse{ID: \"u_1\", Name: input.Name})",
		// the enumerated selector rewrite
		"ctx.Err()",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q:\n%s", want, got)
		}
	}

	for _, unwanted := range []string{
		"http.ResponseWriter",
		"*http.Request",
		"r.Context()",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("generated source still contains %q:\n%s", unwanted, got)
		}
	}
}

// The update surface reaches a handler as methods on Options and on Response,
// which a pattern index keyed by package and name alone could not tell apart.
// This is the test that the receiver is part of the key: Options.Headers and
// Response.WriteTo differ only in it, and drop different arguments.
func TestRewriteCollapsesTheUpdateEntries(t *testing.T) {
	got := string(rewriteFixture(t, "transform_rewrite").Source)

	for _, want := range []string{
		"func updateAction(ctx *fasthttp.RequestCtx)",
		"func updatePage(ctx *fasthttp.RequestCtx)",
		`htmlupdate "github.com/shibukawa/tinybind-go/fasthttpupdate"`,
		// the request drops out of every read-only entry
		"options.WantsUpdate(ctx)",
		`options.VerifyCSRF(ctx, "session-token")`,
		"options.WriteUpdate(ctx, []htmlupdate.Update{",
		"options.Redraw(ctx, registry)",
		"options.Sequence(ctx)",
		"options.Negotiate(ctx).Mode",
		"options.RedrawHeaders(ctx)",
		"options.Headers(ctx, nil, htmlbind.Fragment{})",
		// the writer drops out of the sending half
		"answer.WriteTo(ctx)",
		"answer.NotModified(ctx)",
		// ApplyTo keeps its first argument, which is data rather than transport
		"htmlupdate.ApplyTo(answer.Header, ctx)",
		"htmlupdate.ApplyTo(options.RedrawHeaders(ctx), ctx)",
		// and the branch WantsUpdate exists to create
		`htmlupdate.Redirect(ctx, "/cart", http.StatusSeeOther)`,
		// the streaming entries take a callback, so both transport arguments
		// drop and the producer body is untouched
		`options.WriteStream(ctx, []string{"<title>Feed</title>"}, func(stream *htmlupdate.DeltaStream) error {`,
		"stream.Replace(\"feed\", `<main id=\"feed\">one</main>`, htmlupdate.ManifestEntry{Frame: \"f1\"})",
		// the caller's cancellation is not a transport value and keeps its place
		"options.RenderLiveStream(ctx, ctx, nil, htmlbind.Fragment{})",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q:\n%s", want, got)
		}
	}
}

// The producer body has to survive the rewrite unchanged. It is the reason the
// stream type is one type rather than two that match: a wrapper renaming a
// single method would put a difference here, and this transform moves signature
// lines and argument lists, not method names.
func TestRewriteLeavesTheProducerBodyAlone(t *testing.T) {
	authored, err := os.ReadFile(filepath.Join("..", "testdata", "transform_rewrite", "update.go"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(rewriteFixture(t, "transform_rewrite").Source)

	const producer = "\t\tstream.Replace(\"feed\", `<main id=\"feed\">one</main>`, htmlupdate.ManifestEntry{Frame: \"f1\"})\n\t\treturn nil\n"
	if !strings.Contains(string(authored), producer) {
		t.Fatalf("the fixture no longer contains the producer body this asserts on")
	}
	if !strings.Contains(got, producer) {
		t.Errorf("the producer body changed in the rewrite:\n%s", got)
	}
}

// net/http is dropped because the transport types were its only use, not
// because the transform deletes it on sight. This fixture also reads
// http.StatusAccepted, so the import has to survive; a rewrite that removed it
// would emit source that does not compile.
func TestRewriteKeepsNetHTTPWhenSomethingElseNeedsIt(t *testing.T) {
	got := string(rewriteFixture(t, "transform_rewrite").Source)
	if !strings.Contains(got, `"net/http"`) {
		t.Errorf("net/http was dropped although http.StatusAccepted is still read:\n%s", got)
	}
	if !strings.Contains(got, "http.StatusAccepted") {
		t.Errorf("the reference that keeps net/http is gone:\n%s", got)
	}
}

// The status argument is not a transport slot and has to keep its place after
// the two that are collapse into one.
func TestRewriteKeepsNonTransportArguments(t *testing.T) {
	got := string(rewriteFixture(t, "transform_rewrite").Source)
	want := "httpbind.WriteStatus[CreateUserResponse](ctx, http.StatusAccepted, CreateUserResponse{})"
	if !strings.Contains(got, want) {
		t.Errorf("missing %q:\n%s", want, got)
	}
}

// Comments are part of the authored source, and a rewrite that dropped them
// would make the generated file harder to read than the original.
func TestRewriteKeepsDocComments(t *testing.T) {
	got := string(rewriteFixture(t, "transform_rewrite").Source)
	for _, want := range []string{
		"// createUser is the ordinary shape",
		"// renderUser is the shared helper",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// The real check: the emitted source is Go that compiles against the actual
// fasthttp runtime. Everything above is a claim about text; this is the one
// that would catch a rewrite that merely looks right.
func TestRewrittenSourceCompiles(t *testing.T) {
	out := rewriteFixture(t, "transform_rewrite")

	dir := filepath.Join("..", "testdata", "transform_rewrite")
	path := filepath.Join(dir, "tinybind_fasthttp_gen.go")
	if err := os.WriteFile(path, out.Source, 0o644); err != nil {
		t.Fatalf("write generated file: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })

	// cmd.Dir is the module root, so the package path is relative to that
	// rather than to this test's directory.
	const pkgPath = "./testdata/transform_rewrite"

	cmd := exec.Command("go", "build", "-tags", "fasthttp", "-o", os.DevNull, pkgPath)
	cmd.Dir = ".."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated source does not compile: %v\n%s\n--- generated ---\n%s", err, output, out.Source)
	}

	// And the authored half still compiles on its own, untagged.
	cmd = exec.Command("go", "build", "-o", os.DevNull, pkgPath)
	cmd.Dir = ".."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("authored source stopped compiling: %v\n%s", err, output)
	}
}

// A build tag excludes a whole file, so an authored file mixing handlers with
// declarations both tags need cannot be excluded cleanly.
func TestLayoutWarningNamesMixedFiles(t *testing.T) {
	pkg, err := loadPackage(filepath.Join("..", "testdata", "transform_eligibility"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	plan, err := AnalyzeTransform(pkg, DefaultTransformOptions())
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	out, err := RewriteTransform(pkg, plan, DefaultTransformOptions())
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if len(out.LayoutWarnings) == 0 {
		t.Fatal("a file holding handlers beside type declarations produced no warning")
	}
	warning := strings.Join(out.LayoutWarnings, "\n")
	for _, want := range []string{"main.go", "type declaration", "file of their own"} {
		if !strings.Contains(warning, want) {
			t.Errorf("warning missing %q:\n%s", want, warning)
		}
	}
}

// The fixture laid out correctly must not warn, or the warning is noise.
func TestNoLayoutWarningWhenHandlersAreSeparated(t *testing.T) {
	if warnings := rewriteFixture(t, "transform_rewrite").LayoutWarnings; len(warnings) != 0 {
		t.Errorf("clean layout warned anyway: %v", warnings)
	}
}
