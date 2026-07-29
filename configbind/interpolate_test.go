package configbind

import (
	"strings"
	"testing"
)

func TestExpandEnvRefs(t *testing.T) {
	env := map[string]string{
		"DB_PASSWORD": "s3cret",
		"HOST":        "db.internal",
		"EMPTY":       "",
	}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"no dollar", "plain value", "plain value"},
		{"whole value", "${DB_PASSWORD}", "s3cret"},
		{"partial", "postgres://app:${DB_PASSWORD}@${HOST}:5432/app", "postgres://app:s3cret@db.internal:5432/app"},
		{"repeated name", "${HOST},${HOST}", "db.internal,db.internal"},
		{"set but empty", "prefix-${EMPTY}-suffix", "prefix--suffix"},
		{"escaped dollar", "pa$$word", "pa$word"},
		{"escape beats reference", "$${HOST}", "${HOST}"},
		{"lone dollar stays literal", "cost is 5$ total", "cost is 5$ total"},
		{"unbraced name stays literal", "$HOST", "$HOST"},
		{"trailing dollar", "value$", "value$"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandEnvRefs(tt.raw, env, "webserver.dsn")
			if err != nil {
				t.Fatalf("expandEnvRefs(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("expandEnvRefs(%q)=%q want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestExpandEnvRefsErrors(t *testing.T) {
	env := map[string]string{"HOST": "db.internal"}
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"undefined", "${DB_PASSWORD}", []string{"webserver.dsn", "DB_PASSWORD", "undefined"}},
		{"unterminated", "${HOST", []string{"webserver.dsn", "unterminated"}},
		{"empty name", "${}", []string{"webserver.dsn", "empty environment variable name"}},
		{"invalid character", "${FOO-BAR}", []string{"webserver.dsn", "invalid environment variable name"}},
		{"leading digit", "${1HOST}", []string{"webserver.dsn", "invalid environment variable name"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := expandEnvRefs(tt.raw, env, "webserver.dsn")
			if err == nil {
				t.Fatalf("expandEnvRefs(%q) succeeded, want error", tt.raw)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

// A name that looks undefined but is misspelled reports the syntax problem, so
// the reader is not sent hunting for a variable they never meant to name.
func TestExpandEnvRefsInvalidNameBeatsUndefined(t *testing.T) {
	_, err := expandEnvRefs("${FOO BAR}", map[string]string{}, "webserver.dsn")
	if err == nil || !strings.Contains(err.Error(), "invalid environment variable name") {
		t.Fatalf("err=%v want invalid name", err)
	}
}
