package configmoduleb

import "github.com/shibukawa/tinybind-go/configbind"

// Config deliberately shares its unqualified name with configmodulea.Config.
type Config struct {
	Endpoint string `default:"application" help:"application endpoint"`
}

// Bind returns this package's independently generated configuration target.
func Bind() *Config {
	return configbind.Bind[Config]("application")
}
