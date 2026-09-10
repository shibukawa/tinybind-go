package htmlbind_test

import "testing"

// A head declared outside the document shell is a contribution, and the parser
// reads its script and style bodies verbatim: no brace there is an insertion
// and none is decoded. A component script block is read the same way. The
// printer still ran the raw-text escape over those bodies, so a brace in the
// shape of an insertion, such as the {z}/{x}/{y} of a tile URL, gained a pair
// on every pass and the file never settled.
func TestFormatHeadContributionRawTextIsVerbatim(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			"tile url placeholders stay bare",
			"export component Map(): html {\n  <head>\n    <script>\n      const tiles = \"https://tile.openstreetmap.org/{z}/{x}/{y}.png\";\n    </script>\n  </head>\n  <div id=\"map\"></div>\n}\n",
		},
		{
			"an authored double brace stays double",
			"export component Map(): html {\n  <head>\n    <script>\n      const t = \"{{z}}\";\n    </script>\n  </head>\n  <div id=\"map\"></div>\n}\n",
		},
		{
			"a component script block stays bare too",
			"export component Map(): html {\n  <div id=\"map\"></div>\n  <script component>\n    const tiles = \"https://tile.openstreetmap.org/{z}/{x}/{y}.png\";\n  </script>\n}\n",
		},
		{
			"a style body keeps its braces",
			"export component Map(): html {\n  <head>\n    <style>\n      .map {height: 100%}\n    </style>\n  </head>\n  <div id=\"map\"></div>\n}\n",
		},
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
