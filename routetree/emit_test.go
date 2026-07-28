package routetree

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func emit(t *testing.T, route Route, inputs []Value) string {
	t.Helper()
	source, err := EmitDecoder(route, inputs)
	if err != nil {
		t.Fatalf("EmitDecoder: %v", err)
	}
	// Everything emitted must be parsable Go; format.Source inside EmitDecoder
	// already enforces it, but parsing here keeps the failure local.
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", source, parser.SkipObjectResolution); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, source)
	}
	return string(source)
}

// contains matches ignoring run length of whitespace, because gofmt aligns
// struct fields against their widest sibling and that alignment is not part of
// what any of these tests is asserting.
func contains(source, want string) bool {
	return strings.Contains(collapse(source), collapse(want))
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func mustContain(t *testing.T, source string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !contains(source, want) {
			t.Errorf("generated source missing %q:\n%s", want, source)
		}
	}
}

func decoderRoute(path, pkg string, params ...Segment) Route {
	return Route{Path: path, Package: pkg, Params: params, PageFile: "app/page.tb.html"}
}

func dyn(name string) Segment {
	return Segment{Dir: name + "_", Name: name, Kind: DynamicSegment}
}

func catchAll(name string) Segment {
	return Segment{Dir: name + "__", Name: name, Kind: CatchAllSegment}
}

func TestEmitDecoderPathAndQuery(t *testing.T) {
	route := decoderRoute("/users/{id}", "id_", dyn("id"))
	source := emit(t, route, []Value{
		{Name: "id", Type: "string"},
		{Name: "page", Type: "int"},
	})

	mustContain(t, source,
		"package id_",
		"type RouteParams struct",
		"ID string",
		"Page int",
		"func DecodeRoute(r *http.Request) (RouteParams, error)",
		`r.PathValue("id")`,
		`query.Get("page")`,
		"strconv.Atoi(raw)",
	)
}

func TestEmitDecoderStringOnlyRouteSkipsStrconv(t *testing.T) {
	route := decoderRoute("/users/{id}", "id_", dyn("id"))
	source := emit(t, route, []Value{{Name: "id", Type: "string"}})

	if strings.Contains(source, "strconv") {
		t.Errorf("string-only decoder imports strconv:\n%s", source)
	}
	if !strings.Contains(source, `"net/http"`) {
		t.Errorf("decoder does not import net/http:\n%s", source)
	}
}

func TestEmitDecoderNoInputsIsStillValid(t *testing.T) {
	route := decoderRoute("/about", "about")
	source := emit(t, route, nil)

	if !contains(source, "type RouteParams struct { }") && !contains(source, "type RouteParams struct{}") {
		t.Errorf("expected an empty params struct:\n%s", source)
	}
	if strings.Contains(source, "query :=") {
		t.Errorf("no query inputs but query was read:\n%s", source)
	}
}

func TestEmitDecoderCatchAllIsNotRequired(t *testing.T) {
	route := decoderRoute("/files/{rest...}", "rest__", catchAll("rest"))
	source := emit(t, route, []Value{{Name: "rest", Type: "string"}})

	if !strings.Contains(source, `r.PathValue("rest")`) {
		t.Errorf("catch-all not read:\n%s", source)
	}
	// An empty remainder is a legal match for {rest...}, so it must not 400.
	if strings.Contains(source, "missing_path_parameter") {
		t.Errorf("catch-all treated as required:\n%s", source)
	}
}

func TestEmitDecoderRequiresPathValues(t *testing.T) {
	route := decoderRoute("/users/{id}", "id_", dyn("id"))
	source := emit(t, route, []Value{{Name: "id", Type: "string"}})

	if !strings.Contains(source, "missing_path_parameter") {
		t.Errorf("dynamic segment not checked for presence:\n%s", source)
	}
}

func TestEmitDecoderInvalidValueBecomesBadRequest(t *testing.T) {
	route := decoderRoute("/posts/{n}", "n_", dyn("n"))
	source := emit(t, route, []Value{{Name: "n", Type: "int"}})

	if !strings.Contains(source, "httpbind.BadRequest") {
		t.Errorf("unparsable value does not produce a 400:\n%s", source)
	}
	if !strings.Contains(source, "invalid_path_parameter") {
		t.Errorf("error code missing:\n%s", source)
	}
}

func TestEmitDecoderNarrowsSizedIntegers(t *testing.T) {
	route := decoderRoute("/x", "x")
	source := emit(t, route, []Value{
		{Name: "small", Type: "int32"},
		{Name: "big", Type: "int64"},
		{Name: "ratio", Type: "float32"},
	})

	if !contains(source, "strconv.ParseInt(raw, 10, 32)") {
		t.Errorf("int32 bit size wrong:\n%s", source)
	}
	if !contains(source, "out.Small = int32(v)") {
		t.Errorf("int32 result not narrowed:\n%s", source)
	}
	// ParseInt already returns int64, so no conversion should be emitted.
	if !contains(source, "out.Big = v") {
		t.Errorf("int64 result converted unnecessarily:\n%s", source)
	}
	if !contains(source, "out.Ratio = float32(v)") {
		t.Errorf("float32 result not narrowed:\n%s", source)
	}
}

func TestEmitDecoderSupportsEveryScalar(t *testing.T) {
	types := []string{
		"string", "bool", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64",
	}
	inputs := make([]Value, 0, len(types))
	for i, goType := range types {
		inputs = append(inputs, Value{Name: "v" + string(rune('a'+i)), Type: goType})
	}
	route := decoderRoute("/x", "x")
	source := emit(t, route, inputs)
	if !strings.Contains(source, "strconv.ParseBool") {
		t.Errorf("bool not handled:\n%s", source)
	}
}

func TestEmitDecoderRejectsCollidingFieldNames(t *testing.T) {
	route := decoderRoute("/users/{id}", "id_", dyn("id"))
	_, err := EmitDecoder(route, []Value{
		{Name: "id", Type: "string"},
		{Name: "ID", Type: "string"},
	})
	if err == nil {
		t.Fatal("colliding names accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "colliding") {
		t.Errorf("error = %v, want it to name the collision", err)
	}
}

func TestEmitDecoderRejectsTooFewInputs(t *testing.T) {
	route := decoderRoute("/orgs/{org}/users/{id}", "id_", dyn("org"), dyn("id"))
	if _, err := EmitDecoder(route, []Value{{Name: "org", Type: "string"}}); err == nil {
		t.Fatal("missing input accepted, want rejection")
	}
}

func TestEmitDecoderIsDeterministic(t *testing.T) {
	route := decoderRoute("/users/{id}", "id_", dyn("id"))
	inputs := []Value{{Name: "id", Type: "string"}, {Name: "page", Type: "int"}}
	first := emit(t, route, inputs)
	for range 5 {
		if got := emit(t, route, inputs); got != first {
			t.Fatal("EmitDecoder is not deterministic")
		}
	}
}

func TestEmitDecoderCarriesTheGeneratedHeader(t *testing.T) {
	route := decoderRoute("/x", "x")
	source := emit(t, route, nil)
	if !strings.HasPrefix(source, "// Code generated by tinybind; DO NOT EDIT.") {
		t.Errorf("missing generated header:\n%s", source)
	}
}

func TestExportedName(t *testing.T) {
	cases := map[string]string{
		"id":       "ID",
		"user_id":  "UserID",
		"userID":   "UserID",
		"page":     "Page",
		"api_key":  "APIKey",
		"url":      "URL",
		"htmlBody": "HTMLBody",
		"rest":     "Rest",
		"a":        "A",
	}
	for in, want := range cases {
		if got := ExportedName(in); got != want {
			t.Errorf("ExportedName(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- customization surface ---

func TestEmitterRepointsSymbolsWithoutTouchingTemplates(t *testing.T) {
	e := NewEmitter()
	e.Symbols.ErrorImport = "example.com/fw/web"
	e.Symbols.ErrorAlias = "web"
	e.Symbols.BadRequest = "Invalid"
	e.Symbols.Problem = "Fault"

	route := decoderRoute("/posts/{n}", "n_", dyn("n"))
	source, err := e.Decoder(route, []Value{{Name: "n", Type: "int"}})
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	got := string(source)

	mustContain(t, got,
		`"example.com/fw/web"`,
		"web.Invalid(web.Fault{",
	)
	// The alias matches the last path element, so idiomatic Go omits it.
	if strings.Contains(got, `web "example.com/fw/web"`) {
		t.Errorf("redundant import alias emitted:\n%s", got)
	}
	if strings.Contains(got, "httpbind") {
		t.Errorf("default runtime still referenced:\n%s", got)
	}
}

func TestEmitterRenamesGeneratedDeclarations(t *testing.T) {
	e := NewEmitter()
	e.ParamsType = "PageInput"
	e.DecodeFunc = "BindRoute"

	source, err := e.Decoder(decoderRoute("/x", "x"), []Value{{Name: "q", Type: "string"}})
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	mustContain(t, string(source),
		"type PageInput struct",
		"func BindRoute(r *http.Request) (PageInput, error)",
	)
}

func TestEmitterOverridesTheWholeDecoderTemplate(t *testing.T) {
	e := NewEmitter()
	err := e.Parse(TemplateDecoder, `{{ .Header }}

package {{ .Package }}

// {{ .Pattern }} has {{ len .Fields }} input(s).
const Inputs = {{ len .Fields }}
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	source, err := e.Decoder(decoderRoute("/users/{id}", "id_", dyn("id")), []Value{
		{Name: "id", Type: "string"},
		{Name: "page", Type: "int"},
	})
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	mustContain(t, string(source), "const Inputs = 2")
	if strings.Contains(string(source), "DecodeRoute") {
		t.Errorf("default body still emitted:\n%s", source)
	}
}

func TestEmitterOverridesOneNestedTemplate(t *testing.T) {
	// Replacing only the error block keeps the whole file shape and changes
	// every error the decoder can produce.
	e := NewEmitter()
	if err := e.Parse("error", `fmt.Errorf("route input %s: %s", {{ .Code | quote }}, {{ .Message | quote }})`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	e.Symbols.ErrorImport = "fmt"
	e.Symbols.ErrorAlias = "fmt"

	source, err := e.Decoder(decoderRoute("/posts/{n}", "n_", dyn("n")), []Value{{Name: "n", Type: "int"}})
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	got := string(source)
	mustContain(t, got, "fmt.Errorf(", "func DecodeRoute(")
	if strings.Contains(got, "BadRequest") {
		t.Errorf("default error call still emitted:\n%s", got)
	}
}

func TestEmitterParseErrorLeavesTheEmitterUsable(t *testing.T) {
	e := NewEmitter()
	if err := e.Parse(TemplateDecoder, "{{ this is not a template"); err == nil {
		t.Fatal("bad template accepted, want error")
	}
	// The failed parse must not have replaced the working set.
	source, err := e.Decoder(decoderRoute("/x", "x"), nil)
	if err != nil {
		t.Fatalf("Decoder after failed Parse: %v", err)
	}
	mustContain(t, string(source), "func DecodeRoute(")
}

func TestEmitterCloneIsIndependent(t *testing.T) {
	base := NewEmitter()
	derived, err := base.Clone()
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if err := derived.Parse(TemplateDecoder, "{{ .Header }}\n\npackage {{ .Package }}\n"); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	derived.ParamsType = "Other"

	source, err := base.Decoder(decoderRoute("/x", "x"), nil)
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	mustContain(t, string(source), "type RouteParams struct", "func DecodeRoute(")
}

func TestEmitterReportsUnparsableTemplateOutput(t *testing.T) {
	e := NewEmitter()
	if err := e.Parse(TemplateDecoder, "this is not Go at all"); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err := e.Decoder(decoderRoute("/x", "x"), nil)
	if err == nil {
		t.Fatal("unparsable output accepted, want error")
	}
	// The message must show what was produced, or the author cannot debug it.
	if !strings.Contains(err.Error(), "this is not Go at all") {
		t.Errorf("error = %v, want it to include the rendered output", err)
	}
}

func TestEmitterRejectsUnbindableTypeBeforeRendering(t *testing.T) {
	_, err := NewEmitter().Decoder(decoderRoute("/x", "x"), []Value{{Name: "f", Type: "Filter"}})
	if err == nil {
		t.Fatal("non-scalar accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "Filter") {
		t.Errorf("error = %v, want it to name the type", err)
	}
}

func TestGeneratedHeaderPrecedesABlankLine(t *testing.T) {
	// Without the blank line the marker becomes the package doc comment.
	source := emit(t, decoderRoute("/x", "x"), nil)
	if !strings.HasPrefix(source, GeneratedHeader+"\n\npackage ") {
		t.Errorf("header is attached to the package clause:\n%s", source)
	}
}
