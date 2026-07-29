package configbind

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ProvenanceEntry is one effective config key prepared for logging.
type ProvenanceEntry struct {
	// Key is the stable config key (e.g. "webserver.port"), or the indexed form
	// for a field of an array-of-tables element (e.g. "rdb.connections[0].dsn").
	Key string
	// Value is the display form after redaction, never the raw secret.
	Value string
	// Place is the source layer that won for this key.
	Place Place
	// Masked reports that Value is the redaction placeholder rather than the
	// configured value, so a caller re-rendering these entries can tell the two
	// apart without comparing against the mask text.
	Masked bool
	// ArrayKey is the array of tables this entry is an element field of, with
	// the indices of any enclosing arrays already in place. It is empty for an
	// ordinary key, so a caller groups a tree by it and orders by Index without
	// parsing Key apart at its brackets.
	ArrayKey string
	// Index is the element's position within ArrayKey, and 0 when ArrayKey is
	// empty.
	Index int
}

// maskedValue replaces a sensitive value in provenance output. The mask has a
// fixed width: a width derived from the real value would leak its length, and a
// randomized one would break the deterministic ordering guarantee.
const maskedValue = "*****"

// sensitiveKeyTokens mark keys whose value is masked without an explicit tag.
// A connection string carries its password inline (postgres://user:pass@host),
// and a private key is a credential under a name that matches no other token,
// so both belong here. The match is a substring, so an innocent compound such
// as token_bucket_size is masked too; over-masking is the safe direction for a
// key whose author wrote no secret tag to say otherwise.
var sensitiveKeyTokens = []string{
	"password",
	"secret",
	"apikey",
	"api_key",
	"credential",
	"access_key",
	"accesskey",
	"token",
	"dsn",
	"private_key",
}

// Provenance returns the effective configuration as an ordered, redacted slice.
//
// Entries follow Bind registration order, and within one binding the field
// declaration order of its struct; keys that belong to no registered binding
// sort lexicographically after all known keys. An array of tables expands in
// place into one entry per element field, keyed key[index].field and ordered by
// index then declaration. A secret tag decides whether a value is shown, masked,
// or dropped, and a key with no tag is masked when its name looks sensitive.
// Fields whose dependon parent is empty are omitted, while the parent itself is
// kept: an empty parent is the reason its dependents vanished.
func (r *LoadResult) Provenance() []ProvenanceEntry {
	if r == nil || r.Overlay == nil {
		return nil
	}
	hidden := r.hiddenKeys()
	secrets := r.secretModes()
	elements := r.tableArrayFields()
	seen := make(map[string]bool)
	out := make([]ProvenanceEntry, 0, len(r.Overlay.entries))
	appendKey := func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		if hidden[key] || secrets[key] == secretHide {
			return
		}
		entry, ok := r.Overlay.Get(key)
		if !ok {
			return
		}
		if entry.IsTables {
			// An array of tables holds no value of its own, so reporting the key
			// alone would say only that some elements exist.
			out = append(out, expandTables(key, key, entry.Tables, elements[key], secrets)...)
			return
		}
		value, masked := displayValue(key, entry.Raw, secrets[key])
		out = append(out, ProvenanceEntry{
			Key:    key,
			Value:  value,
			Place:  entry.Place,
			Masked: masked,
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

// hiddenKeys collects keys suppressed because a dependon parent is empty.
func (r *LoadResult) hiddenKeys() map[string]bool {
	parents := make(map[string][]string)
	for _, definition := range r.definitions {
		for key, keyParents := range definition.DependsOn {
			parents[key] = append(parents[key], keyParents...)
		}
	}
	if len(parents) == 0 {
		return nil
	}
	falsy := make(map[string]string)
	kinds := make(map[string]ScaffoldKind)
	for _, definition := range r.definitions {
		for key, value := range definition.Falsy {
			falsy[key] = value
		}
		collectScaffoldKinds(kinds, definition.Prefix, definition.Scaffold)
	}
	hidden := make(map[string]bool, len(parents))
	for key := range parents {
		if r.dependencyHidden(key, parents, falsy, kinds, nil) {
			hidden[key] = true
		}
	}
	return hidden
}

// tableArrayFields indexes the element fields of every array of tables by the
// array's absolute key. The generated scaffold already lists them in struct
// declaration order, which is the order provenance reports them in; the overlay
// cannot supply it, because its own key list is sorted.
func (r *LoadResult) tableArrayFields() map[string][]ScaffoldField {
	out := make(map[string][]ScaffoldField)
	for _, definition := range r.definitions {
		collectTableArrayFields(out, definition.Prefix, definition.Scaffold)
	}
	return out
}

func collectTableArrayFields(out map[string][]ScaffoldField, prefix string, fields []ScaffoldField) {
	for _, field := range fields {
		key := field.Key
		if prefix != "" {
			key = prefix + "." + field.Key
		}
		if field.Kind != ScaffoldTableArray {
			continue
		}
		out[key] = field.Nested
		// A nested array's fields are indexed under the path with no indices in
		// it, which is the same path the generated secret map uses.
		collectTableArrayFields(out, key, field.Nested)
	}
}

// expandTables turns one array of tables into per-element entries.
//
// displayKey carries the indices of every enclosing array, because that is what
// a reader needs to find the element in the file. secretKey carries none, since
// an index exists only at run time and the generated secret map is keyed by the
// stable path under the array.
func expandTables(displayKey, secretKey string, tables []*Overlay, fields []ScaffoldField, secrets map[string]string) []ProvenanceEntry {
	var out []ProvenanceEntry
	for index, element := range tables {
		if element == nil {
			continue
		}
		arrayKey := displayKey
		for _, field := range fields {
			fullKey := fmt.Sprintf("%s[%d].%s", displayKey, index, field.Key)
			mode := secrets[secretKey+"."+field.Key]
			if mode == secretHide {
				continue
			}
			if field.Kind == ScaffoldTableArray {
				nested, ok := element.GetTables(field.Key)
				if !ok {
					continue
				}
				out = append(out, expandTables(
					fmt.Sprintf("%s[%d].%s", displayKey, index, field.Key),
					secretKey+"."+field.Key,
					nested,
					field.Nested,
					secrets,
				)...)
				continue
			}
			entry, ok := element.Get(field.Key)
			if !ok {
				continue
			}
			value, masked := displayValue(fullKey, entry.Raw, mode)
			out = append(out, ProvenanceEntry{
				Key:      fullKey,
				Value:    value,
				Place:    entry.Place,
				Masked:   masked,
				ArrayKey: arrayKey,
				Index:    index,
			})
		}
	}
	return out
}

// secretModes indexes every generated secret tag by absolute key.
func (r *LoadResult) secretModes() map[string]string {
	modes := make(map[string]string)
	for _, definition := range r.definitions {
		for key, mode := range definition.Secrets {
			modes[key] = mode
		}
	}
	return modes
}

// collectScaffoldKinds indexes the value kind of every leaf key. Scaffold
// already carries one entry per stable key, so the kinds the resolution rules
// need are on hand without a second generated table. Array-of-tables elements
// are addressed per element and have no stable key, so they are skipped.
func collectScaffoldKinds(out map[string]ScaffoldKind, prefix string, fields []ScaffoldField) {
	for _, field := range fields {
		if field.Kind == ScaffoldTableArray {
			continue
		}
		key := field.Key
		if prefix != "" {
			key = prefix + "." + key
		}
		out[key] = field.Kind
	}
}

// dependencyHidden walks the parent chains; one empty or hidden parent hides
// the key. visiting guards against a cycle codegen validation failed to reject.
func (r *LoadResult) dependencyHidden(key string, parents map[string][]string, falsy map[string]string, kinds map[string]ScaffoldKind, visiting map[string]bool) bool {
	keyParents, ok := parents[key]
	if !ok || visiting[key] {
		return false
	}
	if visiting == nil {
		visiting = make(map[string]bool)
	}
	visiting[key] = true
	defer delete(visiting, key)
	for _, parent := range keyParents {
		if emptyParent(r.Overlay, parent, falsy[parent], kinds[parent]) {
			return true
		}
		if r.dependencyHidden(parent, parents, falsy, kinds, visiting) {
			return true
		}
	}
	return false
}

// emptyParent reports whether a parent key reads as unconfigured. The empty
// string, false, and the parent's own falsy choice count. A number, a duration,
// and an empty list are deliberate settings on their own, so zero disables its
// dependents only where a falsy tag says that is what zero means.
func emptyParent(o *Overlay, key, falsy string, kind ScaffoldKind) bool {
	entry, ok := o.Get(key)
	if !ok {
		return true
	}
	if entry.IsMulti {
		return false
	}
	if falsy != "" && sameValue(kind, entry.Raw, falsy) {
		return true
	}
	switch kind {
	case ScaffoldInt, ScaffoldDuration:
		return false
	}
	switch entry.Raw {
	case "", "false":
		return true
	}
	return false
}

// sameValue compares an effective value with a falsy choice in the terms of its
// own kind, so 0, 0s, and 0ms all mean off for a duration.
func sameValue(kind ScaffoldKind, raw, choice string) bool {
	switch kind {
	case ScaffoldDuration:
		value, valueErr := time.ParseDuration(raw)
		want, wantErr := time.ParseDuration(choice)
		return valueErr == nil && wantErr == nil && value == want
	case ScaffoldInt:
		if value, valueErr := strconv.ParseInt(raw, 10, 64); valueErr == nil {
			want, wantErr := strconv.ParseInt(choice, 10, 64)
			return wantErr == nil && value == want
		}
		// A uint64 above math.MaxInt64 only parses unsigned.
		value, valueErr := strconv.ParseUint(raw, 10, 64)
		want, wantErr := strconv.ParseUint(choice, 10, 64)
		return valueErr == nil && wantErr == nil && value == want
	default:
		return raw == choice
	}
}

const (
	secretHide = "hide"
	secretMask = "mask"
	secretShow = "show"
)

// displayValue applies the disclosure policy for one key and reports whether
// the returned text is the mask. An explicit secret tag decides on its own; a
// key with no tag is masked when its path contains a sensitive token.
func displayValue(key, raw, mode string) (string, bool) {
	switch mode {
	case secretMask:
		if raw == "" {
			return raw, false
		}
		return maskedValue, true
	case secretShow:
		return raw, false
	}
	if raw == "" {
		return raw, false
	}
	lower := strings.ToLower(key)
	for _, token := range sensitiveKeyTokens {
		if strings.Contains(lower, token) {
			return maskedValue, true
		}
	}
	return raw, false
}
