package configbind

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/hashicorp/go-envparse"
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
	return readEnvMap(defs, environMap(environ))
}

// environMap turns "KEY=value" lines into a lookup map, defaulting to the
// process environment. Load builds it once so the env layer and the file layer's
// ${NAME} expansion can never read different environments.
func environMap(environ []string) map[string]string {
	if environ == nil {
		environ = os.Environ()
	}
	envMap := make(map[string]string, len(environ))
	for _, line := range environ {
		if i := strings.IndexByte(line, '='); i >= 0 {
			envMap[line[:i]] = line[i+1:]
		}
	}
	return envMap
}

func readEnvMap(defs []cliparser.Def, envMap map[string]string) map[string]string {
	out := make(map[string]string)
	for _, d := range defs {
		name, ok := envVarName(d)
		if !ok {
			continue
		}
		if v, ok := envMap[name]; ok {
			out[d.ConfigKey] = v
		}
	}
	return out
}

// envVarName is the environment variable that sets d, or false when d has
// none: no config key, env:"-", or no long option to derive a name from.
func envVarName(d cliparser.Def) (string, bool) {
	if d.ConfigKey == "" || d.Env == "-" {
		return "", false
	}
	if d.Env != "" {
		return d.Env, true
	}
	if len(d.Longs) == 0 {
		return "", false
	}
	return EnvName(d.Longs[0]), true
}

// EnvVariable returns the environment variable that sets key in the registered
// definitions, or "" when the key has none (env:"-", or a repeated-table
// element, which has no variable of its own).
func EnvVariable(key string) string {
	for _, definition := range snapshotDefinitions() {
		defs, err := cliparser.BuildDefs(definition.FlagMetas)
		if err != nil {
			continue
		}
		for _, d := range defs {
			if d.ConfigKey != key {
				continue
			}
			name, _ := envVarName(d)
			return name
		}
	}
	return ""
}

// composedEnviron is the one environment Load reads: the dotenv files in
// order, then the process lines over them. It remembers which file supplied
// each winning name so the env layer can report the file as the place.
type composedEnviron struct {
	values map[string]string
	// files maps a variable name to the file that supplied its winning value.
	// A name the process supplied has no entry.
	files map[string]string
	// secret holds the names whose winning value a secret source supplied: an
	// EnvSecretFiles entry or a file under an EnvSecretDirs entry. A name the
	// process supplied is never here, even when a secret source also set it.
	secret map[string]bool
	// read lists the EnvFiles entries that existed and were parsed, in order;
	// readSecret and readDirs do the same for the two secret inputs.
	read       []string
	readSecret []string
	readDirs   []string
}

// envSources is the ordered set of inputs composeEnviron lays under the
// process environment, lowest first.
type envSources struct {
	files       []string
	secretFiles []string
	secretDirs  []string
}

// composeEnviron reads the plain files, then the secret files, then the secret
// directories, each in slice order, and lays environ (or the process environment
// when environ is nil) over them. A missing file or directory is skipped; one
// that exists but cannot be read or parsed is an error.
func composeEnviron(src envSources, environ []string) (composedEnviron, error) {
	c := composedEnviron{values: make(map[string]string)}
	if len(src.files)+len(src.secretFiles)+len(src.secretDirs) > 0 {
		c.files = make(map[string]string)
		c.secret = make(map[string]bool)
	}
	set := func(name, value, path string, secret bool) {
		c.values[name] = value
		c.files[name] = path
		if secret {
			c.secret[name] = true
		} else {
			delete(c.secret, name)
		}
	}
	for _, path := range src.files {
		parsed, ok, err := readEnvFile(path)
		if err != nil {
			return composedEnviron{}, err
		}
		if !ok {
			continue
		}
		for name, value := range parsed {
			set(name, value, path, false)
		}
		c.read = append(c.read, path)
	}
	for _, path := range src.secretFiles {
		parsed, ok, err := readEnvFile(path)
		if err != nil {
			return composedEnviron{}, err
		}
		if !ok {
			continue
		}
		for name, value := range parsed {
			set(name, value, path, true)
		}
		c.readSecret = append(c.readSecret, path)
	}
	for _, dir := range src.secretDirs {
		entries, ok, err := readEnvDir(dir)
		if err != nil {
			return composedEnviron{}, err
		}
		if !ok {
			continue
		}
		for name, e := range entries {
			set(name, e.value, e.path, true)
		}
		c.readDirs = append(c.readDirs, dir)
	}
	for name, value := range environMap(environ) {
		c.values[name] = value
		delete(c.files, name)
		delete(c.secret, name)
	}
	return c, nil
}

// readEnvFile parses one dotenv file. ok is false when the file does not exist.
func readEnvFile(path string) (map[string]string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("configbind: read env file %q: %w", path, err)
	}
	parsed, err := envparse.Parse(bytes.NewReader(data))
	if err != nil {
		var pe *envparse.ParseError
		if errors.As(err, &pe) {
			return nil, false, fmt.Errorf("configbind: read env file %q line %d: %v", path, pe.Line, pe.Err)
		}
		return nil, false, fmt.Errorf("configbind: read env file %q: %w", path, err)
	}
	return parsed, true, nil
}

// envDirEntry is one variable read from a secret directory.
type envDirEntry struct {
	path  string
	value string
}

// readEnvDir reads a Docker-secret style directory: each regular file is one
// variable, its name the variable name exactly as spelled and its content the
// value with trailing line endings stripped. Names starting with a dot,
// directories, and anything that is not a regular file after following
// symlinks are skipped, which is what a Kubernetes secret mount needs: every
// key is a symlink into a ..data snapshot directory. ok is false when dir does
// not exist.
func readEnvDir(dir string) (map[string]envDirEntry, bool, error) {
	list, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("configbind: read env dir %q: %w", dir, err)
	}
	out := make(map[string]envDirEntry, len(list))
	for _, d := range list {
		name := d.Name()
		if strings.HasPrefix(name, ".") || d.IsDir() {
			continue
		}
		path := dir + string(os.PathSeparator) + name
		info, err := os.Stat(path) // follows a symlink
		if err != nil {
			return nil, false, fmt.Errorf("configbind: read env dir %q entry %q: %w", dir, name, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, fmt.Errorf("configbind: read env dir %q entry %q: %w", dir, name, err)
		}
		out[name] = envDirEntry{path: path, value: strings.TrimRight(string(data), "\r\n")}
	}
	return out, true, nil
}

// mergeEnv sets each def's config key from the composed environment. A name a
// dotenv file supplied reports PlaceEnvFile plus that file; a name the process
// supplied reports PlaceEnv. Defs are walked in order so the overlay fills
// deterministically.
func mergeEnv(o *Overlay, defs []cliparser.Def, env composedEnviron) {
	for _, d := range defs {
		name, ok := envVarName(d)
		if !ok {
			continue
		}
		v, ok := env.values[name]
		if !ok {
			continue
		}
		place := PlaceEnv
		if file, fromFile := env.files[name]; fromFile {
			place = PlaceEnvFile + Place(file)
		}
		o.Set(d.ConfigKey, v, place)
		if env.secret[name] {
			o.MarkSecret(d.ConfigKey)
		}
	}
}
