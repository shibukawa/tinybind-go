package aggregatefixture_test

import (
	"encoding/json"
	"strings"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/configbind"
	_ "github.com/shibukawa/tinybind-go/internal/checkfixture"
	_ "github.com/shibukawa/tinybind-go/internal/openapifixture"
	configmodulea "github.com/shibukawa/tinybind-go/testdata/configmodulea"
	configmoduleb "github.com/shibukawa/tinybind-go/testdata/configmoduleb"
)

func TestGeneratedPackageFragmentsAggregate(t *testing.T) {
	jsonDoc, yamlDoc, err := httpbind.AssembleOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	if len(yamlDoc) == 0 {
		t.Fatal("empty assembled YAML")
	}
	var document map[string]any
	if err := json.Unmarshal(jsonDoc, &document); err != nil {
		t.Fatal(err)
	}
	paths := document["paths"].(map[string]any)
	for _, path := range []string{"/check", "/orgs/{org_id}/users", "/search"} {
		if paths[path] == nil {
			t.Fatalf("assembled document missing %s: %v", path, paths)
		}
	}
}

func TestGeneratedConfigFragmentsAndSameNamedTypesAggregate(t *testing.T) {
	configbind.ResetTargets()
	framework := configmodulea.Bind()
	application := configmoduleb.Bind()
	if _, err := configbind.Load(configbind.LoadOptions{
		ExplicitConfigPath: "",
		Vendor:             "tinybind-test",
		Tool:               "aggregatefixture",
		Args:               []string{},
		Environ:            []string{},
	}); err != nil {
		t.Fatal(err)
	}
	if framework.Endpoint != "framework" || application.Endpoint != "application" {
		t.Fatalf("same-named configs collided: framework=%q application=%q", framework.Endpoint, application.Endpoint)
	}
	tomlText, err := configbind.ScaffoldTOML()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"[application]", "[framework]"} {
		if !strings.Contains(tomlText, table) {
			t.Fatalf("combined scaffold missing %s:\n%s", table, tomlText)
		}
	}
}
