package configbindfixture

import "time"

// WebServerConfig is a Bind-style config used by generator and load tests.
type WebServerConfig struct {
	Port        int           `default:"8080" help:"HTTP listen port" opt:"port,p"`
	Host        string        `default:"localhost" help:"listen host"`
	ReadTimeout time.Duration `default:"5s" help:"request read timeout"`
	CorsOrigins []string      `help:"CORS origins"`
	// MaxRequestBody has no default, so it stays out of provenance until a
	// source sets it. It is here to keep a sized integer in the compiled path.
	MaxRequestBody int64 `help:"maximum request body in bytes"`
	// AdminToken is set but never printed: secret hide drops it from provenance.
	AdminToken string `default:"seed-token" secret:"hide" help:"admin API token"`
	Tracing    string `enum:"off,otlp,jaeger" falsy:"off" help:"tracing exporter"`
	TracingURL string `dependon:"webserver.tracing" help:"tracing collector URL"`
	TLS        TLSConfig
	Routes     []RouteConfig `help:"static routes, one [[webserver.routes]] table each"`
}

// RouteConfig is one [[webserver.routes]] element.
type RouteConfig struct {
	Path    string        `help:"URL path prefix"`
	Dir     string        `help:"directory served under the path"`
	Listing bool          `default:"false" help:"allow directory listing"`
	MaxAge  time.Duration `default:"1h" help:"cache max age for this route"`
}

// TLSConfig is nested under webserver.tls.
type TLSConfig struct {
	Enabled bool `default:"false" help:"enable TLS"`
	// The relative parent resolves to webserver.tls.enabled, the same key the
	// absolute form named, and keeps working if this struct is embedded twice.
	CertPath string `env:"TLS_CERT_FILE" dependon:".enabled" help:"TLS certificate path"`
}

// MigrateOptions is a CLI-only subcommand fixture.
type MigrateOptions struct {
	Path   string   `arg:"required" help:"migration path"`
	Label  string   `arg:"optional" help:"migration label"`
	DryRun bool     `default:"false" help:"print changes without applying"`
	Extra  []string `arg:"*" help:"additional migration inputs"`
}
