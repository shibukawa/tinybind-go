package configpath

import "github.com/shibukawa/tinybind-go/cliparser"

// ConfigPathDef returns the process-level cliparser definition for --config-path.
// ConfigKey is ProcessKey ("config-path"), not a Bind-prefix field key.
func ConfigPathDef() cliparser.Def {
	return cliparser.Def{
		ConfigKey: ProcessKey,
		Longs:     []string{"config-path"},
		Help:      "path to configuration file (overrides OS config directories)",
		Kind:      cliparser.KindString,
	}
}

// ExplicitPathFromParse extracts the --config-path value from a cliparser.Result, if set.
func ExplicitPathFromParse(res cliparser.Result) string {
	if res.Values == nil {
		return ""
	}
	return res.Values[ProcessKey]
}
