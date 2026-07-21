package configbind_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
)

// Manual registration mirrors generated RegisterType for unit tests without codegen.
type testServerConfig struct {
	Port int
	Host string
}

func registerTestServerConfig(t *testing.T) {
	t.Helper()
	configbind.RegisterType[testServerConfig]("testServerConfig", configbind.Meta{
		TypeName:  "testServerConfig",
		KnownKeys: []string{"webserver.port", "webserver.host"},
		Defaults: map[string]string{
			"webserver.port": "8080",
			"webserver.host": "localhost",
		},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "webserver", Key: "port", Opt: "port,p"},
			{Prefix: "webserver", Key: "host"},
		},
		Apply: func(dst any, o *configbind.Overlay) error {
			p := dst.(*testServerConfig)
			if v, ok := o.GetString("webserver.port"); ok {
				// parse int
				var n int
				for _, c := range v {
					if c < '0' || c > '9' {
						continue
					}
					n = n*10 + int(c-'0')
				}
				p.Port = n
			} else {
				p.Port = 8080
			}
			if v, ok := o.GetString("webserver.host"); ok {
				p.Host = v
			} else {
				p.Host = "localhost"
			}
			return nil
		},
	})
}

func TestLoadPrecedenceCLIOverEnvOverTOMLOverDefault(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	// TOML sets port=1 host=from-toml
	if err := os.WriteFile(tomlPath, []byte(`
[webserver]
port = 1
host = "from-toml"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Env uses CLI long names: opt port -> PORT; default host -> WEBSERVER_HOST
	environ := []string{
		"PORT=2",
		"WEBSERVER_HOST=from-env",
	}

	cfg := configbind.Bind[testServerConfig]("webserver")
	// CLI overrides port only
	_, err := configbind.Load(configbind.LoadOptions{
		Vendor:             "acme",
		Tool:               "demo",
		FileName:           "config.toml",
		ExplicitConfigPath: tomlPath,
		Environ:            environ,
		Args:               []string{"--port", "3"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 3 {
		t.Fatalf("Port=%d want 3 (CLI wins)", cfg.Port)
	}
	if cfg.Host != "from-env" {
		t.Fatalf("Host=%q want from-env (env over toml)", cfg.Host)
	}
}

func TestLoadDefaultWhenNoSources(t *testing.T) {
	configbind.ResetTargets()
	registerTestServerConfig(t)
	cfg := configbind.Bind[testServerConfig]("webserver")
	// Missing config file via empty search
	_, err := configbind.Load(configbind.LoadOptions{
		Vendor:   "acme-missing-vendor-xyz",
		Tool:     "tool-missing-xyz",
		FileName: "nope.toml",
		Environ:  []string{},
		Args:     []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 || cfg.Host != "localhost" {
		t.Fatalf("cfg=%+v", cfg)
	}
}
