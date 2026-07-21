package fixture_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/cliparser/codegen/fixture"
)

func TestWebServerFlagDefsShippedParse(t *testing.T) {
	defs := fixture.WebServerFlagDefs()
	res, err := cliparser.Parse([]string{"-p", "8080"}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["webserver.port"] != "8080" {
		t.Fatalf("Values=%v", res.Values)
	}
}
