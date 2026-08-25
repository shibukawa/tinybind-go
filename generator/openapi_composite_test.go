package generator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// documentFor builds the OpenAPI document of a one-file package.
func documentFor(t *testing.T, src string) map[string]any {
	t.Helper()
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	doc, err := generator.BuildOpenAPI(dir)
	if err != nil {
		t.Fatalf("BuildOpenAPI: %v", err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// schemaOf digs one component schema's property out of a built document.
func propertyOf(t *testing.T, doc map[string]any, typeName, property string) map[string]any {
	t.Helper()
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	schema, ok := schemas[typeName].(map[string]any)
	if !ok {
		t.Fatalf("no component schema for %s; have %v", typeName, keysOf(schemas))
	}
	props, _ := schema["properties"].(map[string]any)
	value, ok := props[property].(map[string]any)
	if !ok {
		t.Fatalf("%s has no property %q; have %v", typeName, property, keysOf(props))
	}
	return value
}

const compositeBodySource = `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

// Item is one element.
type Item struct {
	Name string ` + "`json:\"name\"`" + `
}

// Doc holds every shape a body property can be.
type Doc struct {
	Tags    []string          ` + "`json:\"tags\"`" + `
	Sizes   []int32           ` + "`json:\"sizes\"`" + `
	Items   []Item            ` + "`json:\"items\"`" + `
	Labels  map[string]string ` + "`json:\"labels\"`" + `
	ByName  map[string]Item   ` + "`json:\"byName\"`" + `
	One     Item              ` + "`json:\"one\"`" + `
	Blob    []byte            ` + "`json:\"blob\"`" + `
	Nick    string            ` + "`json:\"nick\" check:\"maxlen=8\"`" + `
}

func handler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[Doc](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Doc](w, r, in)
}

func main() { http.HandleFunc("POST /docs", handler) }
`

// A slice property is an array of its element, not the string the default arm
// of schemaForKind produced before.
func TestOpenAPIDocumentsSlicePropertiesAsArrays(t *testing.T) {
	doc := documentFor(t, compositeBodySource)

	tags := propertyOf(t, doc, "Doc", "tags")
	if tags["type"] != "array" {
		t.Errorf("tags = %v, want an array", tags)
	}
	if items, _ := tags["items"].(map[string]any); items["type"] != "string" {
		t.Errorf("tags items = %v, want a string", tags["items"])
	}

	// The element keeps the width the binder enforces, exactly as a scalar of
	// that width does.
	sizes := propertyOf(t, doc, "Doc", "sizes")
	items, _ := sizes["items"].(map[string]any)
	if items["type"] != "integer" || items["format"] != "int32" {
		t.Errorf("sizes items = %v, want an int32 integer", sizes["items"])
	}
}

// A struct element is a reference, and the type it names is registered once as
// a component rather than inlined at each use.
func TestOpenAPIReferencesAStructElement(t *testing.T) {
	doc := documentFor(t, compositeBodySource)

	list := propertyOf(t, doc, "Doc", "items")
	elem, _ := list["items"].(map[string]any)
	if elem["$ref"] != "#/components/schemas/Item" {
		t.Errorf("items element = %v, want a reference to Item", list["items"])
	}

	// The referenced component exists and describes the element's own fields.
	name := propertyOf(t, doc, "Item", "name")
	if name["type"] != "string" {
		t.Errorf("Item.name = %v, want a string", name)
	}
}

// A map is an object whose additionalProperties is the value type, so the key
// type — always a string on the wire — says nothing the document repeats.
func TestOpenAPIDocumentsMapPropertiesAsObjects(t *testing.T) {
	doc := documentFor(t, compositeBodySource)

	labels := propertyOf(t, doc, "Doc", "labels")
	if labels["type"] != "object" {
		t.Errorf("labels = %v, want an object", labels)
	}
	if value, _ := labels["additionalProperties"].(map[string]any); value["type"] != "string" {
		t.Errorf("labels values = %v, want a string", labels["additionalProperties"])
	}

	byName := propertyOf(t, doc, "Doc", "byName")
	if value, _ := byName["additionalProperties"].(map[string]any); value["$ref"] != "#/components/schemas/Item" {
		t.Errorf("byName values = %v, want a reference to Item", byName["additionalProperties"])
	}
}

// A nested struct is a reference too. It fell to the same default arm.
func TestOpenAPIReferencesANestedStruct(t *testing.T) {
	one := propertyOf(t, documentFor(t, compositeBodySource), "Doc", "one")
	if one["$ref"] != "#/components/schemas/Item" {
		t.Errorf("one = %v, want a reference to Item", one)
	}
}

// The two composites that already had a spelling keep it: a byte sequence is a
// base64 string, and a scalar still carries its check-tag constraints.
func TestOpenAPILeavesTheShapesThatWereAlreadyRightAlone(t *testing.T) {
	doc := documentFor(t, compositeBodySource)

	blob := propertyOf(t, doc, "Doc", "blob")
	if blob["type"] != "string" || blob["format"] != "byte" {
		t.Errorf("blob = %v, want a base64 string", blob)
	}

	nick := propertyOf(t, doc, "Doc", "nick")
	if nick["type"] != "string" || nick["maxLength"] == nil {
		t.Errorf("nick = %v, want a string carrying its maxLength", nick)
	}
}

// A type holding a slice of itself resolves through the component name, so the
// walk terminates instead of recursing until the stack runs out.
func TestOpenAPIDocumentsASelfReferentialType(t *testing.T) {
	src := `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

// Node is a tree.
type Node struct {
	Name     string ` + "`json:\"name\"`" + `
	Children []Node ` + "`json:\"children\"`" + `
}

func handler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[Node](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Node](w, r, in)
}

func main() { http.HandleFunc("POST /nodes", handler) }
`
	children := propertyOf(t, documentFor(t, src), "Node", "children")
	if children["type"] != "array" {
		t.Fatalf("children = %v, want an array", children)
	}
	elem, _ := children["items"].(map[string]any)
	if elem["$ref"] != "#/components/schemas/Node" {
		t.Errorf("children element = %v, want a reference back to Node", children["items"])
	}
}

// A fixed-length array holds exactly that many elements and the codec enforces
// it, so the document states the bound.
func TestOpenAPIDocumentsAFixedLengthArrayBound(t *testing.T) {
	src := `package main

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

// Board is a fixed grid.
type Board struct {
	Cells [3]int ` + "`json:\"cells\"`" + `
}

func handler(w http.ResponseWriter, r *http.Request) {
	in, err := httpbind.Bind[Board](r)
	if err != nil {
		httpbind.WriteError(w, r, err)
		return
	}
	_ = httpbind.Write[Board](w, r, in)
}

func main() { http.HandleFunc("POST /boards", handler) }
`
	cells := propertyOf(t, documentFor(t, src), "Board", "cells")
	if cells["type"] != "array" {
		t.Fatalf("cells = %v, want an array", cells)
	}
	if cells["minItems"] != float64(3) || cells["maxItems"] != float64(3) {
		t.Errorf("cells bounds = %v/%v, want 3/3", cells["minItems"], cells["maxItems"])
	}
}
