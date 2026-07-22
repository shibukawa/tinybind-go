package configmodulea

import "github.com/shibukawa/tinybind-go/configbind"

// Config deliberately shares its unqualified name with configmoduleb.Config.
type Config struct {
	Endpoint string `default:"framework" help:"framework endpoint"`
}

// Bind returns this package's independently generated configuration target.
func Bind() *Config {
	return configbind.Bind[Config]("framework")
}
