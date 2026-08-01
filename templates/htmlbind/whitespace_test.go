package htmlbind_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// generateStatics returns the literal argument of every Static op, in order, so
// a test can assert the bytes a plan writes without depending on the rest of
// the generated file.
func generateStatics(t *testing.T, source string, options htmlbind.GenerateOptions) []string {
	t.Helper()
	generated, err := htmlbind.Generate("whitespace.txt", []byte(source), options)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	rest := string(generated)
	for {
		index := strings.Index(rest, ".Static(\"")
		if index < 0 {
			return out
		}
		// An update boundary writes its instance attribute from an instruction
		// that emits nothing during an ordinary render. It separates two static
		// runs in the plan without separating them in the output, so the two are
		// joined here: this helper reports the bytes a plan writes, not how many
		// instructions it took to write them.
		joins := len(out) > 0 && onlyBoundaryAttr(rest[:index])
		rest = rest[index+len(".Static("):]
		end := strings.Index(rest, "\")")
		if end < 0 {
			t.Fatalf("unterminated Static literal in:\n%s", generated)
		}
		literal := rest[:end+1]
		if joins {
			previous := out[len(out)-1]
			out[len(out)-1] = previous[:len(previous)-1] + literal[1:]
		} else {
			out = append(out, literal)
		}
		rest = rest[end+2:]
	}
}

// onlyBoundaryAttr reports whether the text between two Static instructions is
// nothing but the boundary attribute instruction.
func onlyBoundaryAttr(between string) bool {
	return strings.Contains(between, ".BoundaryAttr()") && strings.Count(between, "(") == 1
}

func TestWhitespaceCollapsesToOneSpace(t *testing.T) {
	statics := generateStatics(t, `package pages
export component Card(): html {
<div class="card">
    <h1>Title</h1>
    <p>Body</p>
</div>
}`, htmlbind.GenerateOptions{})
	want := []string{`" <div class=\"card\"> <h1>Title</h1> <p>Body</p> </div> "`}
	if len(statics) != 1 || statics[0] != want[0] {
		t.Fatalf("statics = %q, want %q", statics, want)
	}
}

// A run between two inline boxes renders as one space, so it survives as one
// space. Deleting it would silently join the two words.
func TestWhitespaceKeepsInlineSeparation(t *testing.T) {
	statics := generateStatics(t, `package pages
export component Line(): html {<p><span>a</span>
<span>b</span></p>}`, htmlbind.GenerateOptions{})
	if len(statics) != 1 || !strings.Contains(statics[0], `</span> <span>`) {
		t.Fatalf("statics = %q, want the inline separator preserved", statics)
	}
}

func TestWhitespacePreservedInSignificantElements(t *testing.T) {
	cases := []struct {
		name    string
		element string
	}{
		{"pre", "pre"},
		{"textarea", "textarea"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			source := `package pages
export component Block(): html {<` + test.element + `>  line one
  line two</` + test.element + `>}`
			statics := generateStatics(t, source, htmlbind.GenerateOptions{})
			if len(statics) != 1 || !strings.Contains(statics[0], `  line one\n  line two`) {
				t.Fatalf("statics = %q, want the %s body verbatim", statics, test.element)
			}
		})
	}
}

// A newline in a script body ends a line comment and drives automatic semicolon
// insertion, so raw text is never rewritten.
func TestWhitespacePreservedInRawText(t *testing.T) {
	statics := generateStatics(t, `package pages
export component Boot(): html {<script>// note
window.ready = true;</script>
<style>body {{ color: red; }}
p {{ color: blue; }}</style>}`, htmlbind.GenerateOptions{})
	joined := strings.Join(statics, "")
	if !strings.Contains(joined, `// note\nwindow.ready = true;`) {
		t.Fatalf("statics = %q, want the script body verbatim", statics)
	}
	if !strings.Contains(joined, `color: red; }\np {`) {
		t.Fatalf("statics = %q, want the style body verbatim", statics)
	}
}

func TestWhitespaceEscapeAttribute(t *testing.T) {
	statics := generateStatics(t, `package pages
export component Art(): html {<div preserve-whitespace>
  /\
 /  \
</div>}`, htmlbind.GenerateOptions{})
	joined := strings.Join(statics, "")
	if strings.Contains(joined, "preserve-whitespace") {
		t.Fatalf("statics = %q, want the reserved attribute stripped", statics)
	}
	if !strings.Contains(joined, `<div>\n  /\\\n /  \\\n</div>`) {
		t.Fatalf("statics = %q, want the marked subtree verbatim", statics)
	}
}

// A valued form is rejected rather than ignored: preserve-whitespace="false"
// reads as a disable yet would enable preservation.
func TestWhitespaceEscapeRejectsValue(t *testing.T) {
	_, err := htmlbind.Generate("whitespace.txt", []byte(`package pages
export component Bad(): html {<div preserve-whitespace="false">x</div>}`), htmlbind.GenerateOptions{})
	if err == nil || !strings.Contains(err.Error(), "preserve-whitespace must be a bare attribute") {
		t.Fatalf("error = %v, want the bare-attribute diagnostic", err)
	}
}

// Character data scoped to a table is foster-parented out of it, so a
// whitespace-only run there is removed rather than collapsed.
func TestWhitespaceDroppedInTableScope(t *testing.T) {
	statics := generateStatics(t, `package pages
export component Grid(): html {<table>
  <tbody>
    <tr>
      <td>one</td>
    </tr>
  </tbody>
</table>}`, htmlbind.GenerateOptions{})
	want := `"<table><tbody><tr><td>one</td></tr></tbody></table>"`
	if len(statics) != 1 || statics[0] != want {
		t.Fatalf("statics = %q, want %q", statics, want)
	}
}

// A document body drops the newlines the declaration braces force on the
// author, because the parser discards a run before the doctype anyway. A
// fragment body keeps them as one space, since its caller may place it between
// two inline boxes.
func TestWhitespaceAtBodyEdges(t *testing.T) {
	document := generateStatics(t, `package pages
export component Page(): html {
<!doctype html>
<html><body><p>x</p></body></html>
}`, htmlbind.GenerateOptions{})
	if len(document) != 1 || document[0] != `"<!doctype html><html><body><p>x</p></body></html>"` {
		t.Fatalf("document statics = %q, want no edge whitespace", document)
	}
	fragment := generateStatics(t, `package pages
export component Chip(): html {
<b>x</b>
}`, htmlbind.GenerateOptions{})
	if len(fragment) != 1 || fragment[0] != `" <b>x</b> "` {
		t.Fatalf("fragment statics = %q, want one space at each edge", fragment)
	}
}

// A head contribution and a named slot fill write nothing at their own
// position, so the line breaks around them are formatting for a construct with
// no output and would otherwise emit two spaces where the source had one break.
func TestWhitespaceAroundSilentSiblings(t *testing.T) {
	statics := generateStatics(t, `package pages
component Panel(header: html, children: html): html {<div>{header}{children}</div>}
export component Page(): html {
<Panel>
  <template name="header"><em>H</em></template>
  <p>body</p>
</Panel>
}`, htmlbind.GenerateOptions{})
	for _, static := range statics {
		if strings.Contains(static, "  ") {
			t.Fatalf("statics = %q, want no doubled space around a silent sibling", statics)
		}
	}
}

func TestWhitespaceAroundHeadContribution(t *testing.T) {
	statics := generateStatics(t, `package pages
export component Card(): html {
<div>
  <head><title>Card</title></head>
  <p>x</p>
</div>
}`, htmlbind.GenerateOptions{})
	for _, static := range statics {
		if strings.Contains(static, "  ") {
			t.Fatalf("statics = %q, want no doubled space around the head contribution", statics)
		}
	}
}

// PreserveWhitespace exists for a project comparing generated markup against
// pre-existing golden files, so it must reproduce the source byte for byte -
// except for the reserved attribute, which is never emitted either way.
func TestPreserveWhitespaceOptionKeepsSourceBytes(t *testing.T) {
	const source = `package pages
export component Card(): html {
<div class="card">
    <h1>Title</h1>
</div>
}`
	statics := generateStatics(t, source, htmlbind.GenerateOptions{PreserveWhitespace: true})
	want := `"\n<div class=\"card\">\n    <h1>Title</h1>\n</div>\n"`
	if len(statics) != 1 || statics[0] != want {
		t.Fatalf("statics = %q, want %q", statics, want)
	}
	generated, err := htmlbind.Generate("whitespace.txt", []byte(`package pages
export component Art(): html {<div preserve-whitespace>  x  </div>}`), htmlbind.GenerateOptions{PreserveWhitespace: true})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(generated, []byte("preserve-whitespace")) {
		t.Fatalf("generated Go leaks the reserved attribute:\n%s", generated)
	}
}
