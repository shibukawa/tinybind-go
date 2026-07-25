package fixture

import "tempmod/pw"

// AppConfig is the application configuration section.
type AppConfig struct {
	Addr  string `default:":8080"`
	Debug bool
}

// GenerateConfigCommand is the generate-config subcommand.
type GenerateConfigCommand struct {
	Output string `arg:"required"`
	Force  bool
}

// LoadAppConfig returns the process configuration target.
func LoadAppConfig() *AppConfig {
	return pw.RegisterConfig[AppConfig]("app")
}

// GenerateConfigOptions returns the parsed generate-config options, or nil when
// another subcommand was selected.
func GenerateConfigOptions() *GenerateConfigCommand {
	return pw.SubCommand[GenerateConfigCommand](
		"generate-config",
		"write merged configuration scaffolds",
	)
}
