package httpbind

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type openAPIFragment struct {
	ID   string
	JSON []byte
}

// OpenAPIInfo is application-owned metadata for the assembled document.
type OpenAPIInfo struct {
	Title   string
	Version string
}

type parsedOpenAPIFragment struct {
	id  string
	doc map[string]any
}

type openAPIComponentOccurrence struct {
	fragmentID string
	value      any
}

var (
	openAPIMu        sync.RWMutex
	openAPIFragments []openAPIFragment
	openAPIInfo      *OpenAPIInfo
)

// SetOpenAPIInfo sets application-level metadata for the assembled document.
// Repeating the same value is harmless; a different second value is an error.
func SetOpenAPIInfo(info OpenAPIInfo) error {
	if strings.TrimSpace(info.Title) == "" || strings.TrimSpace(info.Version) == "" {
		return fmt.Errorf("httpbind: OpenAPI info requires title and version")
	}
	openAPIMu.Lock()
	defer openAPIMu.Unlock()
	if openAPIInfo != nil && *openAPIInfo != info {
		return fmt.Errorf("httpbind: conflicting application OpenAPI info")
	}
	copy := info
	openAPIInfo = &copy
	return nil
}

// RegisterOpenAPIFragment registers a generated package fragment. ID should be
// the package import path. Assembly reports conflicting repeated IDs.
func RegisterOpenAPIFragment(id string, jsonDoc []byte) {
	fragment := openAPIFragment{ID: id, JSON: append([]byte(nil), jsonDoc...)}
	openAPIMu.Lock()
	openAPIFragments = append(openAPIFragments, fragment)
	openAPIMu.Unlock()
}

// ResetOpenAPIFragments clears registered fragments. It is intended for tests.
func ResetOpenAPIFragments() {
	openAPIMu.Lock()
	openAPIFragments = nil
	openAPIInfo = nil
	openAPIMu.Unlock()
}

// AssembleOpenAPI merges every registered package fragment and returns a
// deterministic OpenAPI 3.1 JSON document.
func AssembleOpenAPI() (jsonDoc []byte, err error) {
	fragments := snapshotOpenAPIFragments()
	if len(fragments) == 0 {
		return nil, errNoOpenAPI
	}
	sort.Slice(fragments, func(i, j int) bool { return fragments[i].ID < fragments[j].ID })

	parsed := make([]parsedOpenAPIFragment, 0, len(fragments))
	seenIDs := map[string]struct{}{}
	for _, fragment := range fragments {
		if fragment.ID == "" || len(fragment.JSON) == 0 {
			return nil, fmt.Errorf("httpbind: OpenAPI fragment requires ID and JSON")
		}
		var doc map[string]any
		if err := json.Unmarshal(fragment.JSON, &doc); err != nil {
			return nil, fmt.Errorf("httpbind: parse OpenAPI fragment %q: %w", fragment.ID, err)
		}
		if _, ok := seenIDs[fragment.ID]; ok {
			return nil, fmt.Errorf("httpbind: conflicting OpenAPI fragment ID %q", fragment.ID)
		}
		seenIDs[fragment.ID] = struct{}{}
		parsed = append(parsed, parsedOpenAPIFragment{id: fragment.ID, doc: doc})
	}

	renames, err := openAPIComponentRenames(parsed)
	if err != nil {
		return nil, err
	}
	for i := range parsed {
		if len(renames[parsed[i].id]) > 0 {
			rewriteOpenAPIRefs(parsed[i].doc, renames[parsed[i].id])
		}
	}

	info := snapshotOpenAPIInfo()
	result := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   info.Title,
			"version": info.Version,
		},
		"paths":      map[string]any{},
		"components": map[string]any{"schemas": map[string]any{}},
	}
	paths := result["paths"].(map[string]any)
	components := result["components"].(map[string]any)
	for _, fragment := range parsed {
		if sourcePaths, ok := fragment.doc["paths"].(map[string]any); ok {
			if err := mergeOpenAPIMap(paths, sourcePaths, "path", fragment.id); err != nil {
				return nil, err
			}
		}
		if sourceComponents, ok := fragment.doc["components"].(map[string]any); ok {
			for section, raw := range sourceComponents {
				sourceSection, ok := raw.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("httpbind: OpenAPI fragment %q components.%s must be an object", fragment.id, section)
				}
				targetSection, _ := components[section].(map[string]any)
				if targetSection == nil {
					targetSection = map[string]any{}
					components[section] = targetSection
				}
				if err := mergeOpenAPIMap(targetSection, sourceSection, "component "+section, fragment.id); err != nil {
					return nil, err
				}
			}
		}
	}

	jsonDoc, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("httpbind: marshal assembled OpenAPI JSON: %w", err)
	}
	return jsonDoc, nil
}

// OpenAPIJSON serves the assembled OpenAPI document as application/json.
func OpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	doc, err := AssembleOpenAPI()
	if err != nil {
		WriteError(w, r, Internal(err))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc)
}

func snapshotOpenAPIFragments() []openAPIFragment {
	openAPIMu.RLock()
	defer openAPIMu.RUnlock()
	result := make([]openAPIFragment, len(openAPIFragments))
	for i, fragment := range openAPIFragments {
		result[i] = openAPIFragment{ID: fragment.ID, JSON: append([]byte(nil), fragment.JSON...)}
	}
	return result
}

func snapshotOpenAPIInfo() OpenAPIInfo {
	openAPIMu.RLock()
	defer openAPIMu.RUnlock()
	if openAPIInfo == nil {
		return OpenAPIInfo{Title: "Application API", Version: "0.0.0"}
	}
	return *openAPIInfo
}

func openAPIComponentRenames(fragments []parsedOpenAPIFragment) (map[string]map[string]string, error) {
	byName := map[string][]openAPIComponentOccurrence{}
	for _, fragment := range fragments {
		components, _ := fragment.doc["components"].(map[string]any)
		schemas, _ := components["schemas"].(map[string]any)
		for name, schema := range schemas {
			byName[name] = append(byName[name], openAPIComponentOccurrence{fragmentID: fragment.id, value: schema})
		}
	}
	renames := map[string]map[string]string{}
	usedNames := map[string]string{}
	names := make([]string, 0, len(byName))
	needsRename := map[string]bool{}
	for name, occurrences := range byName {
		names = append(names, name)
		needsRename[name] = len(occurrences) > 1 && !allOpenAPIValuesEqual(occurrences[0].value, occurrences[1:])
	}
	sort.Strings(names)
	for _, name := range names {
		if !needsRename[name] {
			occurrences := byName[name]
			usedNames[name] = occurrences[0].fragmentID
		}
	}
	for _, name := range names {
		if !needsRename[name] {
			continue
		}
		occurrences := byName[name]
		for _, occurrence := range occurrences {
			newName := qualifiedOpenAPIComponentName(occurrence.fragmentID, name)
			if previous, ok := usedNames[newName]; ok && previous != occurrence.fragmentID {
				return nil, fmt.Errorf("httpbind: OpenAPI component name collision %q for fragments %q and %q", newName, previous, occurrence.fragmentID)
			}
			usedNames[newName] = occurrence.fragmentID
			if renames[occurrence.fragmentID] == nil {
				renames[occurrence.fragmentID] = map[string]string{}
			}
			renames[occurrence.fragmentID][name] = newName
		}
	}
	for i := range fragments {
		mapping := renames[fragments[i].id]
		if len(mapping) == 0 {
			continue
		}
		components, _ := fragments[i].doc["components"].(map[string]any)
		schemas, _ := components["schemas"].(map[string]any)
		for oldName, newName := range mapping {
			schemas[newName] = schemas[oldName]
			delete(schemas, oldName)
		}
	}
	return renames, nil
}

func allOpenAPIValuesEqual(first any, rest []openAPIComponentOccurrence) bool {
	firstJSON, _ := json.Marshal(first)
	for _, occurrence := range rest {
		otherJSON, _ := json.Marshal(occurrence.value)
		if !bytes.Equal(firstJSON, otherJSON) {
			return false
		}
	}
	return true
}

func qualifiedOpenAPIComponentName(fragmentID, name string) string {
	base := fragmentID
	if index := strings.LastIndex(base, "/"); index >= 0 {
		base = base[index+1:]
	}
	var clean strings.Builder
	for _, r := range base {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			clean.WriteRune(r)
		} else {
			clean.WriteByte('_')
		}
	}
	sum := sha256.Sum256([]byte(fragmentID))
	return fmt.Sprintf("%s_%s__%s", clean.String(), hex.EncodeToString(sum[:4]), name)
}

func rewriteOpenAPIRefs(value any, renames map[string]string) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if key == "$ref" {
				if ref, ok := child.(string); ok {
					const prefix = "#/components/schemas/"
					if name := strings.TrimPrefix(ref, prefix); name != ref {
						if renamed, ok := renames[name]; ok {
							current[key] = prefix + renamed
						}
					}
				}
				continue
			}
			rewriteOpenAPIRefs(child, renames)
		}
	case []any:
		for _, child := range current {
			rewriteOpenAPIRefs(child, renames)
		}
	}
}

func mergeOpenAPIMap(target, source map[string]any, kind, fragmentID string) error {
	for key, value := range source {
		if previous, exists := target[key]; exists {
			previousMap, previousIsMap := previous.(map[string]any)
			valueMap, valueIsMap := value.(map[string]any)
			if kind == "path" && previousIsMap && valueIsMap {
				if err := mergeOpenAPIMap(previousMap, valueMap, "operation at "+key, fragmentID); err != nil {
					return err
				}
				continue
			}
			if openAPIValuesEqual(previous, value) {
				continue
			}
			return fmt.Errorf("httpbind: conflicting OpenAPI %s %q from fragment %q", kind, key, fragmentID)
		}
		target[key] = value
	}
	return nil
}

func openAPIValuesEqual(a, b any) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return bytes.Equal(aJSON, bJSON)
}

type openAPIMissingError struct{}

func (openAPIMissingError) Error() string {
	return "httpbind: no OpenAPI fragments registered"
}

var errNoOpenAPI error = openAPIMissingError{}
