package sqlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// Every emitter-owned local, used as a template parameter name.
func TestGeneratedIdentifierNamespace(t *testing.T) {
	for _, name := range []string{"b", "err", "statement", "rows", "result", "value", "executor", "yield"} {
		t.Run(name, func(t *testing.T) {
			source := "package queries\ntype U { id: int }\n" +
				"export statement Q(" + name + ": int, flagA: bool): sql.many<U> {\n" +
				"SELECT id FROM t WHERE n = {" + name + "} {if flagA}AND flag{/if}\n}"
			generated, err := sqlbind.Generate("q.tb.sql", []byte(source), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL, ContextAPI: true})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			runGenerated(t, generated, []byte("package queries\nimport \"testing\"\nfunc TestNS(t *testing.T) {\n"+
				"s, err := BuildQ(1, true)\nif err != nil { t.Fatal(err) }\nif len(s.Args) != 1 { t.Fatalf(\"%#v\", s.Args) }\n}"))
		})
	}
}

func TestGeneratedIdentifierNamespaceRefusals(t *testing.T) {
	cases := map[string]string{
		"ctx param": `export statement Q(ctx: int): sql.exec {DELETE FROM t WHERE id = {ctx}}`,
		"db param":  `export statement Q(db: int): sql.exec {DELETE FROM t WHERE id = {db}}`,
		// An underscore prefix is already impossible: rule:template-name-casing
		// refuses it as not lowerCamelCase, so the namespace is reserved already.
		"underscore param": `export statement Q(_x: int): sql.exec {DELETE FROM t WHERE id = {_x}}`,
		"underscore val":   `export statement Q(n: int): sql.exec {DELETE FROM t WHERE id = {n} {val _y = n}AND x = {_y}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := sqlbind.Generate("q.tb.sql", []byte("package queries\n"+body), sqlbind.GenerateOptions{Dialect: sqlbind.DialectPostgreSQL})
			if err == nil {
				t.Fatalf("should have been refused: %s", body)
			}
			if !strings.Contains(err.Error(), "reserved") && !strings.Contains(err.Error(), "underscore") &&
				!strings.Contains(err.Error(), "lowerCamelCase") {
				t.Fatalf("diagnostic should explain: %v", err)
			}
		})
	}
}
