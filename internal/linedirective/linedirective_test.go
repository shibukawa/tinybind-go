package linedirective

import "testing"

func TestDirective(t *testing.T) {
	if got := Directive("home.tb.html", 12); got != "//line home.tb.html:12" {
		t.Fatalf("Directive = %q", got)
	}
	// A directive with nothing to say is nothing, so a caller does not have to
	// check before writing one.
	if got := Directive("home.tb.html", 0); got != "" {
		t.Fatalf("Directive with no line = %q", got)
	}
	if got := Directive("", 12); got != "" {
		t.Fatalf("Directive with no path = %q", got)
	}
}

// A directive maps the line after it exactly and the line after that as
// line+1, so a span of several lines needs the directive repeated or only its
// first line is right.
func TestPinRepeatsTheDirectiveThroughASpan(t *testing.T) {
	source := []byte("//line q.tb.sql:40\n" +
		"\tfirst()\n" +
		"\tsecond()\n" +
		"\tthird()\n" +
		Restore() + "\n" +
		"\tscaffolding()\n")
	got := string(Pin(source))
	want := "//line q.tb.sql:40\n" +
		"\tfirst()\n" +
		"//line q.tb.sql:40\n" +
		"\tsecond()\n" +
		"//line q.tb.sql:40\n" +
		"\tthird()\n" +
		Restore() + "\n" +
		"\tscaffolding()\n"
	if got != want {
		t.Fatalf("Pin =\n%s\nwant\n%s", got, want)
	}
}

// Scaffolding is not a span: the generated file's own lines advance naturally,
// so repeating a restore there would be wrong as well as noisy.
func TestPinLeavesScaffoldingAlone(t *testing.T) {
	source := []byte(Restore() + "\n\tone()\n\ttwo()\n")
	if got := string(Pin(source)); got != string(source) {
		t.Fatalf("Pin touched scaffolding:\n%s", got)
	}
}

// Pinning runs on output a caller may already have pinned, so a second pass
// must add nothing.
func TestPinIsIdempotent(t *testing.T) {
	source := []byte("//line q.tb.sql:40\n\tfirst()\n\tsecond()\n")
	once := Pin(source)
	if twice := Pin(once); string(twice) != string(once) {
		t.Fatalf("Pin is not idempotent:\n%s", twice)
	}
}

func TestPinIsANoOpWithoutDirectives(t *testing.T) {
	source := []byte("package p\n\nvar x = 1\n")
	if got := string(Pin(source)); got != string(source) {
		t.Fatalf("Pin changed undirected source:\n%s", got)
	}
}

func TestResolveLinesNamesTheFollowingLine(t *testing.T) {
	source := []byte("package p\n" + // 1
		"\n" + // 2
		"func f() {\n" + // 3
		"//line q.tb.sql:40\n" + // 4
		"\tmapped()\n" + // 5
		Restore() + "\n" + // 6
		"\tscaffolding()\n" + // 7
		"}\n") // 8
	got := string(ResolveLines(source))
	if want := "//line tinybind_restore.go:7\n"; !contains(got, want) {
		t.Fatalf("restore on line 6 should map line 7:\n%s", got)
	}
	// The template directive is left alone: its line comes from the template
	// and has nothing to do with where it landed.
	if !contains(got, "//line q.tb.sql:40\n") {
		t.Fatalf("template directive was rewritten:\n%s", got)
	}
}

func TestResolveLinesIsANoOpWithoutRestores(t *testing.T) {
	source := []byte("package p\n\n//line q.tb.sql:1\nvar x = 1\n")
	if got := string(ResolveLines(source)); got != string(source) {
		t.Fatalf("ResolveLines changed source with no restore:\n%s", got)
	}
}

func TestRenameOnlyTouchesTheRestoreName(t *testing.T) {
	source := []byte("//line q.tb.sql:40\n//line tinybind_restore.go:7\n")
	got := string(Rename(source, "home_pw_gen.go"))
	want := "//line q.tb.sql:40\n//line home_pw_gen.go:7\n"
	if got != want {
		t.Fatalf("Rename = %q, want %q", got, want)
	}
}

// Renaming is what an artifact caller does after this package has already
// fixed the numbers, so it must not need them again.
func TestRenamePreservesResolvedLines(t *testing.T) {
	source := ResolveLines([]byte("package p\n" + Restore() + "\nvar x = 1\n"))
	if got := string(Rename(source, "out.go")); !contains(got, "//line out.go:3\n") {
		t.Fatalf("rename lost the resolved line:\n%s", got)
	}
}

func TestRenameIsANoOpWithoutAName(t *testing.T) {
	source := []byte("//line tinybind_restore.go:7\n")
	if got := string(Rename(source, "")); got != string(source) {
		t.Fatalf("Rename with no name changed source: %q", got)
	}
}

func TestIsDirective(t *testing.T) {
	if !IsDirective("//line q.tb.sql:40") {
		t.Fatal("a directive was not recognized")
	}
	// An indented one is an ordinary comment to the compiler, and recognizing
	// it here is what lets an indenting helper pull it back to the margin.
	if IsDirective("\t//line q.tb.sql:40") {
		t.Fatal("an indented comment was taken for a directive")
	}
	if IsDirective("// line q.tb.sql:40") {
		t.Fatal("a prose comment was taken for a directive")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
