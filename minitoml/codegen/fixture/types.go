package fixture

// WebServiceConfig is a representative Bind-style config struct for codegen tests.
type WebServiceConfig struct {
	ListenAddr  string
	MaxConns    int
	CorsOrigins []string
	TLS         TLSConfig
}

// TLSConfig is a nested config struct.
type TLSConfig struct {
	Enabled  bool
	CertPath string
}
