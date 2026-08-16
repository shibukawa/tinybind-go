package htmlbind_test

import (
	"strings"
	"testing"

	htmlbind "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const cachedAwaitHead = "package pages\n\n" +
	"type Record {\n  title: string\n}\n\n" +
	"external async LoadRecord(id: string): Record\n" +
	"external live Watch(id: string): Record\n\n"

// The motivating case of the whole round, compiled: a component that takes a
// primary key, loads through a boundary, and carries one annotation over the
// load and the render.
//
// Generation and the runtime were each covered on their own — the refusal by a
// diagnostics case, the delivery by hand-built plans — and this is the seam
// between them, where a narrowed refusal that emitted the wrong plan would have
// passed both.
func TestACachedComponentMayAwait(t *testing.T) {
	source := cachedAwaitHead + "@cache(ttl: \"5m\")\nexport component Card(id: string): html {\n" +
		"{await record = LoadRecord(id)}<h1>{record.title}</h1>{fallback}<p>loading</p>{/await}\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	for _, want := range []string{
		// The cache survives the boundary,
		"CachePolicy[CardParams]",
		// the boundary survives the cache,
		"htmlbind.Await(",
		// and the plan still declares that it opens one, which is what a
		// framework reads to choose the streaming path.
		"HasAwaitBlock",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated code is missing %q:\n%s", want, generated)
		}
	}
}

// A live source never settles, so no stored range could stand for it. That is
// the whole eligibility test, and it has to still hold once the await half is
// allowed.
func TestACachedComponentMayNotWatch(t *testing.T) {
	source := cachedAwaitHead + "@cache(ttl: \"5m\")\nexport component Card(id: string): html {\n" +
		"{await record = Watch(id)}<h1>{record.title}</h1>{fallback}<p>loading</p>{/await}\n}\n"
	message := generateError(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(message, "cannot reach a live boundary") {
		t.Fatalf("want the live refusal, got %q", message)
	}
	if !strings.Contains(message, "never settles") {
		t.Fatalf("the diagnostic does not say why: %q", message)
	}
}
