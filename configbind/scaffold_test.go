package configbind_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/minitoml"
)

func TestScaffoldsCombineRegisteredFragmentsDeterministically(t *testing.T) {
	configbind.ResetScaffolds()
	t.Cleanup(configbind.ResetScaffolds)

	// Register in reverse output order to prove init order is not observable.
	server := configbind.ScaffoldFragment{
		ID:     "example/app.ServerConfig@webserver",
		Prefix: "webserver",
		Fields: []configbind.ScaffoldField{
			{Key: "port", Kind: configbind.ScaffoldInt, Default: "8080", Opt: "port,p", Help: "HTTP listen port"},
			{Key: "host", Kind: configbind.ScaffoldString, Default: "localhost", Help: "listen host"},
			{Key: "origins", Kind: configbind.ScaffoldStringSlice},
			{Key: "secret", Kind: configbind.ScaffoldString, Env: "-"},
			{Key: "tls.enabled", Kind: configbind.ScaffoldBool, Default: "true", Help: "enable TLS"},
		},
	}
	configbind.RegisterScaffold(server)
	configbind.RegisterScaffold(configbind.ScaffoldFragment{
		ID:     "example/framework.CacheConfig@middleware.cache",
		Prefix: "middleware.cache",
		Fields: []configbind.ScaffoldField{
			{Key: "service_name", Kind: configbind.ScaffoldString, Env: "OTEL_SERVICE_NAME"},
		},
	})
	// Identical package initialization is harmless.
	configbind.RegisterScaffold(server)

	tomlText, err := configbind.ScaffoldTOML()
	if err != nil {
		t.Fatal(err)
	}
	wantTOML := `[middleware.cache]
service_name = ""

[webserver]
# listen host
host = "localhost"
origins = []
# HTTP listen port
port = 8080
secret = ""
# enable TLS
tls.enabled = true
`
	if tomlText != wantTOML {
		t.Fatalf("TOML scaffold:\n--- got ---\n%s--- want ---\n%s", tomlText, wantTOML)
	}
	if _, err := minitoml.ParseString(tomlText); err != nil {
		t.Fatalf("generated TOML does not parse: %v\n%s", err, tomlText)
	}

	envText, err := configbind.ScaffoldEnv()
	if err != nil {
		t.Fatal(err)
	}
	wantEnv := `OTEL_SERVICE_NAME=""
# HTTP listen port
PORT=8080
# listen host
WEBSERVER_HOST="localhost"
WEBSERVER_ORIGINS=""
# enable TLS
WEBSERVER_TLS_ENABLED=true
`
	if envText != wantEnv {
		t.Fatalf("env scaffold:\n--- got ---\n%s--- want ---\n%s", envText, wantEnv)
	}

	var output bytes.Buffer
	if err := configbind.WriteScaffoldTOML(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != tomlText {
		t.Fatalf("written scaffold differs: %q", output.String())
	}
}

func TestScaffoldReportsCrossPackageConflicts(t *testing.T) {
	configbind.ResetScaffolds()
	t.Cleanup(configbind.ResetScaffolds)
	configbind.RegisterScaffold(configbind.ScaffoldFragment{
		ID:     "example/framework.Config@server",
		Prefix: "server",
		Fields: []configbind.ScaffoldField{{Key: "port", Kind: configbind.ScaffoldInt, Env: "PORT"}},
	})
	configbind.RegisterScaffold(configbind.ScaffoldFragment{
		ID:     "example/app.Config@server",
		Prefix: "server",
		Fields: []configbind.ScaffoldField{{Key: "port", Kind: configbind.ScaffoldInt, Env: "APP_PORT"}},
	})
	if _, err := configbind.ScaffoldTOML(); err == nil || !strings.Contains(err.Error(), "duplicate scaffold key") {
		t.Fatalf("TOML conflict error=%v", err)
	}
}

func TestScaffoldReportsDuplicateEnvironmentName(t *testing.T) {
	configbind.ResetScaffolds()
	t.Cleanup(configbind.ResetScaffolds)
	configbind.RegisterScaffold(configbind.ScaffoldFragment{
		ID:     "example/framework.Config@framework",
		Prefix: "framework",
		Fields: []configbind.ScaffoldField{{Key: "port", Kind: configbind.ScaffoldInt, Env: "PORT"}},
	})
	configbind.RegisterScaffold(configbind.ScaffoldFragment{
		ID:     "example/app.Config@app",
		Prefix: "app",
		Fields: []configbind.ScaffoldField{{Key: "port", Kind: configbind.ScaffoldInt, Env: "PORT"}},
	})
	if _, err := configbind.ScaffoldEnv(); err == nil || !strings.Contains(err.Error(), "duplicate scaffold environment") {
		t.Fatalf("env conflict error=%v", err)
	}
}
