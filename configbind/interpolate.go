package configbind

import (
	"fmt"
	"strings"
)

// expandEnvRefs replaces every ${NAME} in raw with the environment value of NAME.
// Literal text and references may be mixed freely, so a value can carry a secret
// in the middle of a URL. "$$" writes one literal "$" and starts no reference; a
// "$" followed by anything else stays literal, which keeps values that merely
// contain a dollar sign working.
//
// An undefined name is an error rather than an empty string: the file layer
// outranks defaults, so a silently empty expansion would erase a default tag
// value instead of failing the load. A name that is set but empty is defined and
// expands to "".
//
// key names the config key under expansion and only appears in errors.
func expandEnvRefs(raw string, env map[string]string, key string) (string, error) {
	if !strings.ContainsRune(raw, '$') {
		return raw, nil
	}
	var b strings.Builder
	b.Grow(len(raw))
	for i := 0; i < len(raw); {
		if raw[i] != '$' {
			b.WriteByte(raw[i])
			i++
			continue
		}
		if i+1 < len(raw) && raw[i+1] == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}
		if i+1 >= len(raw) || raw[i+1] != '{' {
			b.WriteByte('$')
			i++
			continue
		}
		end := strings.IndexByte(raw[i+2:], '}')
		if end < 0 {
			return "", fmt.Errorf("configbind: %s: unterminated %q in value", key, "${")
		}
		name := raw[i+2 : i+2+end]
		if err := checkEnvRefName(name, key); err != nil {
			return "", err
		}
		value, ok := env[name]
		if !ok {
			return "", fmt.Errorf("configbind: %s: undefined environment variable ${%s}", key, name)
		}
		b.WriteString(value)
		i += 2 + end + 1
	}
	return b.String(), nil
}

// checkEnvRefName enforces the [A-Za-z_][A-Za-z0-9_]* name shape. A typo such as
// ${FOO-BAR} is rejected instead of being reported as merely undefined, which
// would send the reader looking for a variable they never meant to name.
func checkEnvRefName(name, key string) error {
	if name == "" {
		return fmt.Errorf("configbind: %s: empty environment variable name in ${}", key)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_':
		case i > 0 && c >= '0' && c <= '9':
		default:
			return fmt.Errorf("configbind: %s: invalid environment variable name ${%s}", key, name)
		}
	}
	return nil
}
