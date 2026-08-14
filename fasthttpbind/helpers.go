package fasthttpbind

import (
	"encoding/json"
	"strconv"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
	"github.com/shibukawa/tinybind-go/jsonbind"
)

// Scalar parsers and JSON helpers used by generated binders. These are the same
// one-line delegations the net/http runtime declares: routing them through a
// shared package would add an indirection and save nothing, so each surface
// spells its own.

// DefaultMaxJSONBodyBytes is the default cap for JSON document reads (1 MiB).
const DefaultMaxJSONBodyBytes = jsonbind.DefaultMaxJSONBodyBytes

// SetMaxJSONBodyBytes changes the process-wide JSON body limit. A non-positive
// value restores DefaultMaxJSONBodyBytes.
func SetMaxJSONBodyBytes(n int64) { jsonbind.SetMaxJSONBodyBytes(n) }

// MaxJSONBodyBytes returns the effective JSON body limit.
func MaxJSONBodyBytes() int64 { return jsonbind.MaxJSONBodyBytes() }

// DefaultMaxMultipartBodyBytes is the default cap on multipart request bodies.
const DefaultMaxMultipartBodyBytes = bindcore.DefaultMaxMultipartBodyBytes

// SetMaxMultipartBodyBytes sets the global multipart body size limit. The value
// is shared with the net/http runtime, so configuring it once configures both.
func SetMaxMultipartBodyBytes(n int64) { bindcore.SetMaxMultipartBodyBytes(n) }

// MaxMultipartBodyBytes returns the effective global multipart body limit.
func MaxMultipartBodyBytes() int64 { return bindcore.MaxMultipartBodyBytes() }

// RestJSONAny builds map[string]any from leftover JSON object keys not in exclude.
func RestJSONAny(jsonBody *jsonbind.Object, exclude []string) (map[string]any, error) {
	return jsonbind.RestJSONAny(jsonBody, exclude)
}

// RestJSONNames lists leftover JSON object keys not in exclude.
func RestJSONNames(jsonBody *jsonbind.Object, exclude []string) []string {
	return jsonbind.RestJSONNames(jsonBody, exclude)
}

// RestFormAny builds map[string]any from leftover form keys not in exclude.
func RestFormAny(formBody map[string]string, exclude []string) map[string]any {
	return bindcore.RestFormAny(formBody, exclude)
}

// RestFormRaw builds map[string]json.RawMessage from leftover form keys.
func RestFormRaw(formBody map[string]string, exclude []string) map[string]json.RawMessage {
	return bindcore.RestFormRaw(formBody, exclude)
}

// ParseInt converts a string to int.
func ParseInt(s string) (int, error) { return strconv.Atoi(s) }

// ParseInt64 converts a string to int64.
func ParseInt64(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }

// ParseBool converts a string to bool.
func ParseBool(s string) (bool, error) { return strconv.ParseBool(s) }

// ParseFloat64 converts a string to float64.
func ParseFloat64(s string) (float64, error) { return strconv.ParseFloat(s, 64) }
