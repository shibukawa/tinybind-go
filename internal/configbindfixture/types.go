package configbindfixture

// WebServerConfig is a Bind-style config used by generator and load tests.
type WebServerConfig struct {
	Port        int      `default:"8080" help:"HTTP listen port" opt:"port,p"`
	Host        string   `default:"localhost" help:"listen host"`
	CorsOrigins []string `help:"CORS origins"`
	TLS         TLSConfig
}

// TLSConfig is nested under webserver.tls.
type TLSConfig struct {
	Enabled  bool   `default:"false" help:"enable TLS"`
	CertPath string `env:"TLS_CERT_FILE" help:"TLS certificate path"`
}
