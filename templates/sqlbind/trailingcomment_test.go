package sqlbind_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// requirement:template-comment-retention asks that a comment after the last
// declaration comes back in place. Two things had to agree for that to hold:
// the printer's tail path had to respect the blank the source wrote, and the
// parser had to be able to see that blank in the first place. The line-comment
// branch used to swallow its own terminating newline, so BlankBefore was
// unreachable for any comment following a line comment, and the two spellings
// below parsed identically.
func TestFormatKeepsTrailingCommentSpacing(t *testing.T) {
	head := "package queries\n\nexport statement F(id: int): sql.one<E> {\n  SELECT id\n  FROM e\n  WHERE id = {id}\n}\n\n"
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"adjacent lines stay adjacent", head + "// tail one\n// tail two\n"},
		{"a deliberate blank survives", head + "// group one\n\n// group two\n"},
		{"block comments keep their blank", head + "/* one */\n\n/* two */\n"},
		{"a leading blank run is unchanged", "package queries\n\n// group one\n\n// group two\nexport statement F(id: int): sql.one<E> {\n  SELECT id\n  FROM e\n  WHERE id = {id}\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSource(t, tc.source)
			if got != tc.source {
				t.Errorf("format is not a fixed point\n got: %q\nwant: %q", got, tc.source)
			}
			if again := formatSource(t, got); again != got {
				t.Errorf("not idempotent\nfirst: %q\nsecond: %q", got, again)
			}
		})
	}
}

// The blank between two line comments has to reach the AST, or a printer has
// nothing to preserve.
func TestParseSeesBlankBetweenLineComments(t *testing.T) {
	source := "package queries\n\n// group one\n\n// group two\n// group three\n"
	module, err := sqlbind.Parse("x.tb.sql", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(module.Comments) != 3 {
		t.Fatalf("got %d comments, want 3", len(module.Comments))
	}
	for i, want := range []bool{true, true, false} {
		if got := module.Comments[i].BlankBefore; got != want {
			t.Errorf("comment %d (%s) BlankBefore = %v, want %v",
				i, module.Comments[i].Text, got, want)
		}
	}
}
