package generator

import (
	"strings"
	"testing"
)

func TestParseJSONTag(t *testing.T) {
	type parsed struct {
		skip      bool
		name      string
		omitEmpty bool
		omitZero  bool
	}
	for raw, want := range map[string]parsed{
		"":                        {},
		"name":                    {name: "name"},
		"name,omitempty":          {name: "name", omitEmpty: true},
		"name,omitzero":           {name: "name", omitZero: true},
		"name,omitempty,omitzero": {name: "name", omitEmpty: true, omitZero: true},
		",omitempty":              {omitEmpty: true},
		"name,":                   {name: "name"},
		// encoding/json's punctuation trivia: a bare dash drops the field,
		// while "-," names it "-".
		"-":           {skip: true},
		"-,":          {name: "-"},
		"-,omitempty": {name: "-", omitEmpty: true},
	} {
		t.Run(raw, func(t *testing.T) {
			skip, name, omitEmpty, omitZero, err := parseJSONTag(raw)
			if err != nil {
				t.Fatalf("parseJSONTag(%q): %v", raw, err)
			}
			got := parsed{skip: skip, name: name, omitEmpty: omitEmpty, omitZero: omitZero}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

// An option nobody implements has to fail rather than sit there looking like it
// works: a misspelled omitempty is invisible until someone diffs the output.
func TestParseJSONTag_RejectsUnknownOption(t *testing.T) {
	for _, raw := range []string{"name,omitempy", "name,string", "name,omitempty,case:ignore"} {
		_, _, _, _, err := parseJSONTag(raw)
		if err == nil {
			t.Errorf("parseJSONTag(%q) accepted an unknown option", raw)
			continue
		}
		if !strings.Contains(err.Error(), "json tag option") {
			t.Errorf("parseJSONTag(%q) error = %v", raw, err)
		}
	}
}
