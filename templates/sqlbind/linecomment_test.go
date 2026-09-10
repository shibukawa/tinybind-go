package sqlbind_test

import "testing"

// rule:sql-template-layout says a line comment ends its line and whatever
// followed it in the source moves to the next one, because joining them would
// comment that content out. The layout tree knew this only as "not flat": a run
// that had to be written anyway still went out on one line, so a comment at
// the head of a body, at the head of a branch, or after a comma took the rest
// of the statement with it.
func TestFormatLineCommentOwnsItsLine(t *testing.T) {
	t.Parallel()
	head := "export statement Find(id: int, name: string): sql.many<Row> {\n"
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{
			"a leading comment is a line of its own",
			head + "-- by id\nSELECT id, name FROM rows WHERE id = {id}\n}\n",
			head + "  -- by id\n  SELECT id, name\n  FROM rows\n  WHERE id = {id}\n}\n",
		},
		{
			"two leading comments stay two lines",
			head + "-- one\n-- two\nSELECT id FROM rows\n}\n",
			head + "  -- one\n  -- two\n  SELECT id\n  FROM rows\n}\n",
		},
		{
			"a leading block comment does not absorb the clauses",
			head + "/* lead */ SELECT id, name FROM rows WHERE id = {id}\n}\n",
			head + "  /* lead */\n  SELECT id, name\n  FROM rows\n  WHERE id = {id}\n}\n",
		},
		{
			"a comment after a comma keeps the next item off its line",
			head + "SELECT id, -- the id\n  name\nFROM rows\n}\n",
			head + "  SELECT\n    id,\n    -- the id\n    name\n  FROM rows\n}\n",
		},
		{
			"a comment before a comma does not swallow the comma",
			head + "SELECT id -- the id\n, name\nFROM rows\n}\n",
			head + "  SELECT\n    id,\n    -- the id\n    name\n  FROM rows\n}\n",
		},
		{
			"a comment after a keyword keeps the items off its line",
			head + "SELECT -- columns\n  id, name\nFROM rows\n}\n",
			head + "  SELECT\n    -- columns\n    id,\n    name\n  FROM rows\n}\n",
		},
		{
			"a comment opening a branch keeps the condition off its line",
			head + "SELECT id FROM rows WHERE 1 = 1\n{if name != \"\"}\n  -- by name\n  AND name = {name}\n{/if}\n}\n",
			head + "  SELECT id\n  FROM rows\n  WHERE\n    1 = 1 {if name != \"\"}\n      -- by name\n      AND name = {name}\n    {/if}\n}\n",
		},
		{
			"a trailing comment keeps its clause on one line",
			head + "SELECT id, name\nFROM rows\nWHERE id = {id} -- done\n}\n",
			head + "  SELECT id, name\n  FROM rows\n  WHERE id = {id} -- done\n}\n",
		},
		{
			"a trailing comment inside a subquery keeps the closer off its line",
			head + "SELECT id FROM rows WHERE id IN (SELECT id FROM other -- inner\n)\n}\n",
			head + "  SELECT id\n  FROM rows\n  WHERE\n    id IN (\n      SELECT id\n      FROM other -- inner\n    )\n}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSource(t, tc.source)
			if got != tc.want {
				t.Errorf("format\n got: %q\nwant: %q\n\n%s", got, tc.want, got)
			}
			if again := formatSource(t, got); again != got {
				t.Errorf("not idempotent\nfirst: %q\nsecond: %q", got, again)
			}
		})
	}
}
