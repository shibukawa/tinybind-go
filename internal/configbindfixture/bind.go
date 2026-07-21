package configbindfixture

import "github.com/shibukawa/tinybind-go/configbind"

// Register returns Bind handles for Load tests (discovered by tinybind-gen).
func Register() *WebServerConfig {
	return configbind.Bind[WebServerConfig]("webserver")
}
