package configbind

import "strings"

// ProvenanceEntry is one effective config key prepared for logging.
type ProvenanceEntry struct {
	// Key is the stable config key (e.g. "webserver.port").
	Key string
	// Value is the display form after redaction, never the raw secret.
	Value string
	// Place is the source layer that won for this key.
	Place Place
}

// maskedValue replaces a sensitive value in provenance output. The mask has a
// fixed width: a width derived from the real value would leak its length, and a
// randomized one would break the deterministic ordering guarantee.
const maskedValue = "*****"

// sensitiveKeyTokens mark keys whose value is masked without an explicit tag.
var sensitiveKeyTokens = []string{
	"password",
	"secret",
	"apikey",
	"api_key",
	"credential",
	"access_key",
	"accesskey",
	"token",
}

// Provenance returns the effective configuration as an ordered, redacted slice.
//
// Entries follow Bind registration order, and within one binding the field
// declaration order of its struct; keys that belong to no registered binding
// sort lexicographically after all known keys. Values whose key looks sensitive
// are masked. Fields whose dependon parent is empty are omitted, while the
// parent itself is kept: an empty parent is the reason its dependents vanished.
func (r *LoadResult) Provenance() []ProvenanceEntry {
	if r == nil || r.Overlay == nil {
		return nil
	}
	hidden := r.hiddenKeys()
	seen := make(map[string]bool)
	out := make([]ProvenanceEntry, 0, len(r.Overlay.entries))
	appendKey := func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		if hidden[key] {
			return
		}
		entry, ok := r.Overlay.Get(key)
		if !ok {
			return
		}
		out = append(out, ProvenanceEntry{
			Key:   key,
			Value: displayValue(key, entry.Raw),
			Place: entry.Place,
		})
	}
	for _, definition := range r.definitions {
		for _, key := range definition.KnownKeys {
			appendKey(key)
		}
	}
	// Keys owned by no definition (e.g. stray TOML entries) trail the known
	// ones; Keys() is already sorted.
	for _, key := range r.Overlay.Keys() {
		appendKey(key)
	}
	return out
}

// hiddenKeys collects keys suppressed because their dependon parent is empty.
func (r *LoadResult) hiddenKeys() map[string]bool {
	parents := make(map[string]string)
	for _, definition := range r.definitions {
		for key, parent := range definition.DependsOn {
			parents[key] = parent
		}
	}
	if len(parents) == 0 {
		return nil
	}
	falsy := make(map[string]string)
	for _, definition := range r.definitions {
		for key, value := range definition.Falsy {
			falsy[key] = value
		}
	}
	hidden := make(map[string]bool, len(parents))
	for key := range parents {
		if r.dependencyHidden(key, parents, falsy, nil) {
			hidden[key] = true
		}
	}
	return hidden
}

// dependencyHidden walks the parent chain; a hidden parent hides its dependents.
// visiting guards against a cycle that codegen validation failed to reject.
func (r *LoadResult) dependencyHidden(key string, parents map[string]string, falsy map[string]string, visiting map[string]bool) bool {
	parent, ok := parents[key]
	if !ok {
		return false
	}
	if visiting[key] {
		return false
	}
	if visiting == nil {
		visiting = make(map[string]bool)
	}
	visiting[key] = true
	if emptyParent(r.Overlay, parent, falsy[parent]) {
		return true
	}
	return r.dependencyHidden(parent, parents, falsy, visiting)
}

// emptyParent reports whether a parent key reads as unconfigured. The empty
// string, false, and the parent's own falsy choice count; an int 0, an empty
// list, and a zero duration are deliberate settings, not an absent one.
func emptyParent(o *Overlay, key, falsy string) bool {
	entry, ok := o.Get(key)
	if !ok {
		return true
	}
	if entry.IsMulti {
		return false
	}
	if falsy != "" && entry.Raw == falsy {
		return true
	}
	switch entry.Raw {
	case "", "false":
		return true
	}
	return false
}

// displayValue applies the tag-free redaction policy: a key whose path contains
// a sensitive token is masked, everything else shows its raw value.
func displayValue(key, raw string) string {
	if raw == "" {
		return raw
	}
	lower := strings.ToLower(key)
	for _, token := range sensitiveKeyTokens {
		if strings.Contains(lower, token) {
			return maskedValue
		}
	}
	return raw
}
