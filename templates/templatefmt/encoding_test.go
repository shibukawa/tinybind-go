package templatefmt_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

// The whole-file conventions: UTF-8 without a byte order mark, LF line
// endings, two-space indentation, and one level of it inside a declaration.

func TestFormatNormalizesCRLFEverywhere(t *testing.T) {
	source := "export component Page(): html {\r\n<style>\r\n  body {{ margin: 0; }}\r\n</style>\r\n}\r\n"
	out, err := templatefmt.Source("page.tb.html", []byte(source), templatefmt.Options{})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	// A style body is copied byte for byte, so normalizing on the way out could
	// never have reached it; the source is normalized before parsing instead.
	if strings.Contains(string(out), "\r") {
		t.Fatalf("carriage return survived:\n%q", out)
	}
}

func TestFormatStripsByteOrderMark(t *testing.T) {
	source := "\ufeffexport statement FindUser(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}"
	out, err := templatefmt.Source("q.tb.sql", []byte(source), templatefmt.Options{})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if strings.HasPrefix(string(out), "\ufeff") {
		t.Fatal("byte order mark survived")
	}
	if !strings.HasPrefix(string(out), "export statement FindUser") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestFormatRejectsInvalidUTF8(t *testing.T) {
	source := append([]byte("export component Page(): html {<p>"), 0xff, 0xfe)
	source = append(source, []byte("</p>}")...)
	if _, err := templatefmt.Source("page.tb.html", source, templatefmt.Options{}); err == nil {
		t.Fatal("invalid UTF-8 was accepted")
	} else if !strings.Contains(err.Error(), "not valid UTF-8") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeclarationBodyIsIndentedOneLevelWithTwoSpaces(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"html",
			"export component Page(): html {\n<h1>hi</h1>\n}",
			"export component Page(): html {\n  <h1>hi</h1>\n}\n",
		},
		{
			"sql",
			"export statement FindUser(id: int): sql.one<Row> {SELECT id FROM users WHERE id = {id}}",
			"export statement FindUser(id: int): sql.one<Row> {\n  SELECT id\n  FROM users\n  WHERE id = {id}\n}\n",
		},
		{
			"dynamo",
			"export statement Since(sensor: Sensor): dynamo.many<Reading> {table readings; key sensor = {sensor}}",
			"export statement Since(sensor: Sensor): dynamo.many<Reading> {\n  table readings\n  key sensor = {sensor}\n}\n",
		},
	} {
		out, err := templatefmt.Source("x.tb."+test.name, []byte(test.source), templatefmt.Options{})
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		if string(out) != test.want {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", test.name, out, test.want)
		}
	}
}

func TestFormattedOutputEndsWithExactlyOneNewline(t *testing.T) {
	out, err := templatefmt.Source("x.tb.html", []byte("export component P(): html {\n<p>a</p>\n}\n\n\n"), templatefmt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(out), "}\n") || strings.HasSuffix(string(out), "}\n\n") {
		t.Fatalf("unexpected trailing newlines: %q", out)
	}
}
