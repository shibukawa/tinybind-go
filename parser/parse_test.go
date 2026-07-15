package parser_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/httpbind-go/parser"
)

var update = flag.Bool("update", false, "update expected.json golden files")

func TestParsePackage_testdata(t *testing.T) {
	root := filepath.Join("..", "testdata")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	var cases []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "expected.json")); err != nil {
			continue
		}
		cases = append(cases, e.Name())
	}
	if len(cases) < 10 {
		t.Fatalf("expected many testdata cases, found %d: %v", len(cases), cases)
	}

	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			got, err := parser.ParsePackage(dir)
			if err != nil {
				t.Fatalf("ParsePackage(%s): %v", dir, err)
			}
			gotJSON, err := got.JSON()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			expectedPath := filepath.Join(dir, "expected.json")
			if *update {
				if err := os.WriteFile(expectedPath, append(gotJSON, '\n'), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}
			// Canonicalize both sides via JSON round-trip for stable compare.
			if !jsonEqual(t, want, gotJSON) {
				t.Fatalf("parse result mismatch for %s\n--- want ---\n%s\n--- got ---\n%s",
					name, string(want), string(gotJSON))
			}
		})
	}
}

func TestParsePackage_unsupportedDoNotDiscover(t *testing.T) {
	unsupported := []string{
		"unsupported_string_concat",
		"unsupported_loop",
		"unsupported_di_crosspkg",
	}
	for _, name := range unsupported {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("..", "testdata", name)
			got, err := parser.ParsePackage(dir)
			if err != nil {
				t.Fatalf("ParsePackage: %v", err)
			}
			if len(got.Routes) != 0 {
				t.Fatalf("expected zero routes for unsupported case %s, got %+v", name, got.Routes)
			}
		})
	}
}

func TestParsePackage_representativeSamples(t *testing.T) {
	// Drive the real entry point on a few key cases and assert critical fields
	// without re-implementing discovery logic in the test.
	samples := map[string]func(t *testing.T, r *parser.Result){
		"basic_handlefunc": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 {
				t.Fatalf("routes: %d", len(r.Routes))
			}
			rt := r.Routes[0]
			if rt.Method != "POST" || rt.Path != "/users/{id}" {
				t.Fatalf("method/path: %s %s", rt.Method, rt.Path)
			}
			if rt.Handler.Form != "named" || rt.Handler.Name != "createUserHandler" {
				t.Fatalf("handler: %+v", rt.Handler)
			}
			if rt.Request != "CreateUserRequest" || rt.Response != "CreateUserResponse" {
				t.Fatalf("models: %s / %s", rt.Request, rt.Response)
			}
		},
		"stream_write": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 {
				t.Fatalf("routes: %d", len(r.Routes))
			}
			rt := r.Routes[0]
			if !strings.Contains(rt.Response, "Stream") || rt.Stream != "ChatEvent" {
				t.Fatalf("stream response: resp=%q stream=%q", rt.Response, rt.Stream)
			}
		},
		"stream_newstream": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 {
				t.Fatalf("routes: %d", len(r.Routes))
			}
			rt := r.Routes[0]
			if rt.Method != "POST" || rt.Path != "/chat" {
				t.Fatalf("method/path: %s %s", rt.Method, rt.Path)
			}
			if rt.Stream != "ChatEvent" || !strings.Contains(rt.Response, "Stream[ChatEvent]") {
				t.Fatalf("NewStream discovery: resp=%q stream=%q", rt.Response, rt.Stream)
			}
			if rt.Request != "ChatRequest" {
				t.Fatalf("request: %s", rt.Request)
			}
		},
		"nested_wrappers": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 {
				t.Fatalf("routes: %d", len(r.Routes))
			}
			w := r.Routes[0].Wrappers
			if w.MaxRequestBodyBytes == nil || *w.MaxRequestBodyBytes != 10485760 {
				t.Fatalf("max bytes: %+v", w.MaxRequestBodyBytes)
			}
			if w.Timeout != "30s" || w.TimeoutMessage != "timeout" {
				t.Fatalf("timeout meta: %+v", w)
			}
		},
		"struct_handler": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 || r.Routes[0].Handler.Form != "struct" {
				t.Fatalf("struct handler: %+v", r.Routes)
			}
		},
		"inline_handler": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 || r.Routes[0].Handler.Form != "inline" {
				t.Fatalf("inline handler: %+v", r.Routes)
			}
		},
		"error_constructors": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 {
				t.Fatalf("routes: %d", len(r.Routes))
			}
			if len(r.Routes[0].Errors) < 7 {
				t.Fatalf("errors: %v", r.Routes[0].Errors)
			}
		},
		"ignore_http_notfound": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 {
				t.Fatalf("routes: %d", len(r.Routes))
			}
			if len(r.Routes[0].Errors) != 0 {
				t.Fatalf("http.NotFound must not populate errors, got %v", r.Routes[0].Errors)
			}
		},
		"wrapper_timeout_bare": func(t *testing.T, r *parser.Result) {
			if len(r.Routes) != 1 {
				t.Fatalf("routes: %d", len(r.Routes))
			}
			if r.Routes[0].Wrappers.Timeout != "1s" {
				t.Fatalf("bare time.Second timeout want 1s, got %q", r.Routes[0].Wrappers.Timeout)
			}
		},
	}

	for name, check := range samples {
		t.Run(name, func(t *testing.T) {
			got, err := parser.ParsePackage(filepath.Join("..", "testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			check(t, got)
		})
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var aj, bj any
	if err := json.Unmarshal(a, &aj); err != nil {
		t.Fatalf("want json: %v", err)
	}
	if err := json.Unmarshal(b, &bj); err != nil {
		t.Fatalf("got json: %v", err)
	}
	ab, _ := json.Marshal(aj)
	bb, _ := json.Marshal(bj)
	return bytes.Equal(ab, bb)
}
