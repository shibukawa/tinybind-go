package sqlbind_test

import "testing"

// TestInsertItemAgreement checks that a column count which can disagree with its
// value count is a generation error, and that a matched pair still generates.
func TestInsertItemAgreement(t *testing.T) {
	accepted := map[string]string{
		"static": `export statement A(id: int, n: string): sql.exec {
INSERT INTO users (id, name) VALUES ({id}, {n})}`,
		"same condition guards both": `export statement A(id: int, n: string, c: string, withCity: bool): sql.exec {
INSERT INTO users (id, name{if withCity}, city{/if}) VALUES ({id}, {n}{if withCity}, {c}{/if})}`,
		"two independent pairs": `export statement A(id: int, n: string, c: string, a: bool, b: bool): sql.exec {
INSERT INTO users (id{if a}, name{/if}{if b}, city{/if}) VALUES ({id}{if a}, {n}{/if}{if b}, {c}{/if})}`,
		"else branches on both": `export statement A(id: int, n: string, c: string, flag: bool): sql.exec {
INSERT INTO users (id, {if flag}name{else}city{/if}) VALUES ({id}, {if flag}{n}{else}{c}{/if})}`,
		"function call in an item": `export statement A(id: int, n: string): sql.exec {
INSERT INTO users (id, name) VALUES ({id}, coalesce({n}, 'x'))}`,
		"literal is a whole item": `export statement A(id: int, n: string): sql.exec {
INSERT INTO users (id, kind, name) VALUES ({id}, 'bid', {n})}`,
		"literal is the last item": `export statement A(id: int): sql.exec {
INSERT INTO users (id, kind) VALUES ({id}, 'bid')}`,
		"quoted identifier column": `export statement A(id: int, n: string): sql.exec {
INSERT INTO users ("id", "name") VALUES ({id}, {n})}`,
		"dollar quoted literal item": `export statement A(id: int): sql.exec {
INSERT INTO users (id, body) VALUES ({id}, $tag$hello$tag$)}`,
		"literal guarded by the same condition": `export statement A(id: int, withKind: bool): sql.exec {
INSERT INTO users (id{if withKind}, kind{/if}) VALUES ({id}{if withKind}, 'bid'{/if})}`,
		"no column list": `export statement A(id: int, n: string): sql.exec {
INSERT INTO users VALUES ({id}, {n})}`,
		"insert from select": `export statement A(id: int): sql.exec {
INSERT INTO users (id, name) SELECT id, name FROM staging WHERE id = {id}}`,
	}
	for name, body := range accepted {
		t.Run("ok/"+name, func(t *testing.T) {
			if _, err := generateSQL(t, "package queries\n"+body); err != nil {
				t.Errorf("should generate: %v", err)
			}
		})
	}

	refused := map[string]string{
		"column conditional, value not": `export statement A(id: int, n: string, c: string, withCity: bool): sql.exec {
INSERT INTO users (id, name{if withCity}, city{/if}) VALUES ({id}, {n}, {c})}`,
		"value conditional, column not": `export statement A(id: int, n: string, c: string, withCity: bool): sql.exec {
INSERT INTO users (id, name, city) VALUES ({id}, {n}{if withCity}, {c}{/if})}`,
		"different conditions": `export statement A(id: int, n: string, c: string, a: bool, b: bool): sql.exec {
INSERT INTO users (id, name{if a}, city{/if}) VALUES ({id}, {n}{if b}, {c}{/if})}`,
		"else on one side only": `export statement A(id: int, n: string, c: string, flag: bool): sql.exec {
INSERT INTO users (id, {if flag}name{else}city{/if}) VALUES ({id}{if flag}, {n}{/if})}`,
		// Counting a literal as content must not stop the check from counting.
		"literal value short one column": `export statement A(id: int): sql.exec {
INSERT INTO users (id, kind, name) VALUES ({id}, 'bid')}`,
		"literal value guarded alone": `export statement A(id: int, withKind: bool): sql.exec {
INSERT INTO users (id, kind) VALUES ({id}{if withKind}, 'bid'{/if})}`,
	}
	for name, body := range refused {
		t.Run("refused/"+name, func(t *testing.T) {
			refuses(t, "package queries\n"+body, "disagree")
		})
	}
}
