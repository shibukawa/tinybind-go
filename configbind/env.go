package configbind

import (
	"os"
	"strings"

	"github.com/shibukawa/tinybind-go/cliparser"
)

// EnvName converts a CLI long option name (without leading dashes) to an env var name.
// Hyphens become underscores; the result is uppercased.
//
//	"port" -> "PORT"
//	"webserver-host" -> "WEBSERVER_HOST"
//	"webserver-tls-cert_path" -> "WEBSERVER_TLS_CERT_PATH"
func EnvName(longOpt string) string {
	s := strings.TrimSpace(longOpt)
	s = strings.TrimPrefix(s, "--")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return strings.ToUpper(s)
}

// ReadEnv maps present environment variables onto stable config keys using CLI long names.
// For each def, the first Longs entry determines the env var via EnvName; the value is
// stored under def.ConfigKey. Unset vars are absent from the result.
// environ is "KEY=value" lines as from os.Environ(); if nil, os.Environ() is used.
func ReadEnv(defs []cliparser.Def, environ []string) map[string]string {
	if environ == nil {
		environ = os.Environ()
	}
	envMap := make(map[string]string, len(environ))
	for _, line := range environ {
		if i := strings.IndexByte(line, '='); i >= 0 {
			envMap[line[:i]] = line[i+1:]
		}
	}
	out := make(map[string]string)
	for _, d := range defs {
		if d.ConfigKey == "" || len(d.Longs) == 0 {
			continue
		}
		name := EnvName(d.Longs[0])
		if v, ok := envMap[name]; ok {
			out[d.ConfigKey] = v
		}
	}
	return out
}
