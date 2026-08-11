package htmlbind_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const typescriptBlockSource = `package pages

export component Clock(label: string): html {
<script component lang="ts">
export function setup(el: HTMLElement): () => void {
  const tick = (): void => { el.textContent = String(Date.now()) };
  const id = setInterval(tick, 1000);
  return () => clearInterval(id);
}
</script>
<div class="clock">{label}</div>
}
`

// A transform standing in for esbuild: it reports what it was handed and
// returns something recognizably compiled, which is all this seam promises.
func stubTypeScript(t *testing.T, seen *htmlbind.ContentRequest) htmlbind.ContentHook {
	t.Helper()
	return htmlbind.ContentHook{
		Name: "typescript",
		Lang: "ts",
		Transform: func(request htmlbind.ContentRequest) (htmlbind.ContentResult, error) {
			*seen = request
			return htmlbind.ContentResult{
				Content: strings.ReplaceAll(
					strings.ReplaceAll(request.Content, ": HTMLElement", ""),
					"): () => void {", ") {"),
				Read: []string{"tsconfig.json"},
			}, nil
		},
	}
}

func TestContentHookCompilesATypeScriptBlock(t *testing.T) {
	var seen htmlbind.ContentRequest
	result, err := htmlbind.GenerateModule("clock.tb.html", []byte(typescriptBlockSource), htmlbind.GenerateOptions{
		ContentHooks: []htmlbind.ContentHook{stubTypeScript(t, &seen)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen.Lang != "ts" || seen.Component != "Clock" {
		t.Fatalf("request = %+v, want lang ts for Clock", seen)
	}
	// A bundling transform resolves an import against the template's own
	// directory, so the seam has to carry it.
	if seen.Dir != "." || seen.File != "clock.tb.html" {
		t.Fatalf("request located at dir %q file %q, want the template's own", seen.Dir, seen.File)
	}
	if !strings.Contains(seen.Content, ": HTMLElement") {
		t.Fatalf("transform received compiled content, want it authored:\n%s", seen.Content)
	}

	if len(result.Assets) != 1 {
		t.Fatalf("want one asset, got %+v", result.Assets)
	}
	asset := result.Assets[0]
	if strings.Contains(string(asset.Content), ": HTMLElement") {
		t.Fatalf("written file kept the TypeScript annotation:\n%s", asset.Content)
	}
	if asset.Owner != "pages.clock.Clock" {
		t.Fatalf("asset owner = %q, want pages.clock.Clock", asset.Owner)
	}
	// The marker describes the authored block, not the served file.
	if strings.Contains(string(result.GoSource), "lang=") {
		t.Fatalf("lang marker reached the emitted tag:\n%s", result.GoSource)
	}
	// An edit to what the transform read has to regenerate the block.
	if len(result.ReadSet) != 1 || result.ReadSet[0] != "tsconfig.json" {
		t.Fatalf("read set = %v, want the file the transform reported", result.ReadSet)
	}
}

// The extension is the caller's: only it knows what its converter produces.
func TestContentHookNamesTheProducedExtension(t *testing.T) {
	result, err := htmlbind.GenerateModule("clock.tb.html", []byte(typescriptBlockSource), htmlbind.GenerateOptions{
		ContentHooks: []htmlbind.ContentHook{{
			Name: "typescript", Lang: "ts", Extension: "mjs",
			Transform: func(request htmlbind.ContentRequest) (htmlbind.ContentResult, error) {
				return htmlbind.ContentResult{Content: "export function setup() {}"}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Assets[0].Extension; got != "mjs" {
		t.Fatalf("extension = %q, want mjs", got)
	}
	if !strings.HasSuffix(result.Assets[0].URL, ".mjs") {
		t.Fatalf("reference URL %q does not follow the extension", result.Assets[0].URL)
	}
}

// A block marked for a language nobody compiles must not ship uncompiled: the
// page would break in the browser with nothing in the build to point at.
func TestUnregisteredLangFailsGeneration(t *testing.T) {
	_, err := htmlbind.GenerateModule("clock.tb.html", []byte(typescriptBlockSource), htmlbind.GenerateOptions{})
	if err == nil {
		t.Fatal("want an error for an unregistered lang, got none")
	}
	for _, want := range []string{"no content hook is registered for lang ts", "registers none"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want it to say %q", err, want)
		}
	}
}

func TestTransformFailureNamesTheBlock(t *testing.T) {
	_, err := htmlbind.GenerateModule("clock.tb.html", []byte(typescriptBlockSource), htmlbind.GenerateOptions{
		ContentHooks: []htmlbind.ContentHook{{
			Name: "typescript", Lang: "ts",
			Transform: func(htmlbind.ContentRequest) (htmlbind.ContentResult, error) {
				return htmlbind.ContentResult{}, errors.New("TS1005: ';' expected")
			},
		}},
	})
	if err == nil {
		t.Fatal("want the transform's error, got none")
	}
	for _, want := range []string{"Clock", "typescript", "TS1005"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want it to name %q", err, want)
		}
	}
}

func TestContentHookRegistrationIsValidated(t *testing.T) {
	for _, testcase := range []struct {
		name  string
		hooks []htmlbind.ContentHook
		want  string
	}{
		{
			name:  "no lang",
			hooks: []htmlbind.ContentHook{{Name: "x", Transform: func(htmlbind.ContentRequest) (htmlbind.ContentResult, error) { return htmlbind.ContentResult{}, nil }}},
			want:  "claims no lang",
		},
		{
			name:  "no transform",
			hooks: []htmlbind.ContentHook{{Name: "x", Lang: "ts"}},
			want:  "has no transform",
		},
		{
			name: "two hooks one lang",
			hooks: []htmlbind.ContentHook{
				{Name: "a", Lang: "ts", Transform: func(htmlbind.ContentRequest) (htmlbind.ContentResult, error) { return htmlbind.ContentResult{}, nil }},
				{Name: "b", Lang: "ts", Transform: func(htmlbind.ContentRequest) (htmlbind.ContentResult, error) { return htmlbind.ContentResult{}, nil }},
			},
			want: "both claim lang ts",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			err := htmlbind.ValidateContentHooks(testcase.hooks)
			if err == nil || !strings.Contains(err.Error(), testcase.want) {
				t.Fatalf("error = %v, want it to say %q", err, testcase.want)
			}
		})
	}
}

// A project registering hooks it never uses still regenerates what it did
// before, which is what makes registration free.
func TestUnmarkedBlockIgnoresRegisteredHooks(t *testing.T) {
	const source = `package pages

export component Counter(): html {
<script component>export function setup() {}</script>
<div></div>
}
`
	called := false
	result, err := htmlbind.GenerateModule("counter.tb.html", []byte(source), htmlbind.GenerateOptions{
		ContentHooks: []htmlbind.ContentHook{{
			Name: "typescript", Lang: "ts",
			Transform: func(htmlbind.ContentRequest) (htmlbind.ContentResult, error) {
				called = true
				return htmlbind.ContentResult{}, nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("an unmarked block called a transform")
	}
	if !strings.Contains(string(result.Assets[0].Content), "export function setup() {}") {
		t.Fatalf("unmarked block was rewritten:\n%s", result.Assets[0].Content)
	}
}
