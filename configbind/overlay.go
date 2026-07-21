// Package configbind loads Bind-style config from defaults, TOML, env, and CLI into structs.
package configbind

import (
	"sort"
	"strings"
)

// Place is the winning source layer for an overlay entry.
type Place string

const (
	PlaceDefault Place = "default"
	PlaceFile    Place = "file_toml"
	PlaceEnv     Place = "env"
	PlaceCLI     Place = "cli"
)

// Entry is one winning raw value in the overlay.
type Entry struct {
	Raw     string
	Multi   []string
	IsMulti bool
	Place   Place
}

// Overlay is a key-wise multi-source merge buffer (later Set wins).
type Overlay struct {
	entries map[string]Entry
}

// NewOverlay returns an empty overlay.
func NewOverlay() *Overlay {
	return &Overlay{entries: make(map[string]Entry)}
}

// Set stores a scalar raw value for key from place (overwrites prior).
func (o *Overlay) Set(key, raw string, place Place) {
	if o.entries == nil {
		o.entries = make(map[string]Entry)
	}
	o.entries[key] = Entry{Raw: raw, Place: place}
}

// SetMulti stores a multi-value for key from place.
func (o *Overlay) SetMulti(key string, values []string, place Place) {
	if o.entries == nil {
		o.entries = make(map[string]Entry)
	}
	cp := append([]string(nil), values...)
	o.entries[key] = Entry{
		Raw:     strings.Join(cp, ","),
		Multi:   cp,
		IsMulti: true,
		Place:   place,
	}
}

// Get returns the entry for key.
func (o *Overlay) Get(key string) (Entry, bool) {
	if o == nil || o.entries == nil {
		return Entry{}, false
	}
	e, ok := o.entries[key]
	return e, ok
}

// GetString returns a scalar raw string for key.
func (o *Overlay) GetString(key string) (string, bool) {
	e, ok := o.Get(key)
	if !ok {
		return "", false
	}
	return e.Raw, true
}

// GetMulti returns multi values when present; otherwise splits Raw by comma if needed.
func (o *Overlay) GetMulti(key string) ([]string, bool) {
	e, ok := o.Get(key)
	if !ok {
		return nil, false
	}
	if e.IsMulti {
		return append([]string(nil), e.Multi...), true
	}
	if e.Raw == "" {
		return []string{}, true
	}
	return []string{e.Raw}, true
}

// Keys returns sorted config keys.
func (o *Overlay) Keys() []string {
	if o == nil || o.entries == nil {
		return nil
	}
	keys := make([]string, 0, len(o.entries))
	for k := range o.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// MergeMap merges scalar string values from m with the given place.
func (o *Overlay) MergeMap(m map[string]string, place Place) {
	for k, v := range m {
		o.Set(k, v, place)
	}
}

// MergeMultiMap merges multi-value maps with the given place.
func (o *Overlay) MergeMultiMap(m map[string][]string, place Place) {
	for k, v := range m {
		o.SetMulti(k, v, place)
	}
}

// Delete removes a key from the overlay if present.
func (o *Overlay) Delete(key string) {
	if o == nil || o.entries == nil {
		return
	}
	delete(o.entries, key)
}
