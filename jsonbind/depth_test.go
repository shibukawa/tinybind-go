package jsonbind_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// nested builds a document open levels deep out of one bracket pair, with inner
// as the innermost value.
func nested(open, inner, close string, levels int) []byte {
	var b bytes.Buffer
	b.Grow(levels*(len(open)+len(close)) + len(inner))
	for range levels {
		b.WriteString(open)
	}
	b.WriteString(inner)
	for range levels {
		b.WriteString(close)
	}
	return b.Bytes()
}

// The walk is recursive, so the document's depth is the Go stack's depth unless
// something bounds it. These pin the bound at both ends: the last accepted
// depth still decodes, and one more is a parse error rather than a stack the
// request chose the size of.
func TestNestingDepthLimit(t *testing.T) {
	for _, shape := range []struct{ name, open, inner, close string }{
		{"array", "[", "", "]"},
		{"object", `{"a":`, "null", "}"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			p := jsonbind.NewParser(nested(shape.open, shape.inner, shape.close, jsonbind.MaxNestingDepth()))
			if err := p.SkipValue(); err != nil {
				t.Fatalf("depth %d should decode: %v", jsonbind.MaxNestingDepth(), err)
			}
			p = jsonbind.NewParser(nested(shape.open, shape.inner, shape.close, jsonbind.MaxNestingDepth()+1))
			if err := p.SkipValue(); err == nil {
				t.Fatalf("depth %d decoded, expected a refusal", jsonbind.MaxNestingDepth()+1)
			} else if !strings.Contains(err.Error(), "nested too deeply") {
				t.Fatalf("depth %d refused with %v, expected a nesting error", jsonbind.MaxNestingDepth()+1, err)
			}
		})
	}
}

// Any walks the same brackets through a different recursion, so it needs the
// same bound and gets it from the same place.
func TestNestingDepthLimitAny(t *testing.T) {
	if _, err := jsonbind.DecodeJSONAny(nested("[", "", "]", jsonbind.MaxNestingDepth()+1)); err == nil {
		t.Fatal("Any accepted a document past the depth limit")
	}
	if _, err := jsonbind.DecodeJSONAny(nested("[", "", "]", jsonbind.MaxNestingDepth())); err != nil {
		t.Fatalf("Any refused a document at the depth limit: %v", err)
	}
}

// A generated decoder reaches SkipValue through an unknown member, which is the
// path an attacker actually has: no field of the target type has to be nested
// at all for the body to be.
func TestNestingDepthLimitUnknownMember(t *testing.T) {
	body := append([]byte(`{"unknown":`), nested("[", "", "]", jsonbind.MaxNestingDepth()+1)...)
	body = append(body, '}')

	p := jsonbind.NewParser(body)
	if _, err := p.ObjectStart(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.ObjectKey(0); err != nil {
		t.Fatal(err)
	}
	if err := p.SkipValue(); err == nil {
		t.Fatal("an unknown member carried a document past the depth limit")
	}
}

// The count is per document, not per parser: a Parser reused across bodies must
// start each one at zero, or a run of deep-but-legal documents would refuse the
// one after them.
func TestNestingDepthResetsBetweenDocuments(t *testing.T) {
	var p jsonbind.Parser
	deep := nested("[", "", "]", jsonbind.MaxNestingDepth())
	for i := range 3 {
		p.Reset(deep)
		if err := p.SkipValue(); err != nil {
			t.Fatalf("document %d refused: %v", i, err)
		}
	}
}

// Depth is what is open here, not what has been seen: a long run of siblings
// closes each one before the next opens, so it never approaches the bound.
func TestSiblingsDoNotAccumulateDepth(t *testing.T) {
	body := []byte("[" + strings.Repeat("[],", jsonbind.MaxNestingDepth()) + "[]]")
	p := jsonbind.NewParser(body)
	if err := p.SkipValue(); err != nil {
		t.Fatalf("a flat array of %d elements was refused: %v", jsonbind.MaxNestingDepth()+1, err)
	}
}

// The bound is a process-wide setting with a default the smallest stack
// survives, so a host with a growable stack can raise it. The test raises it,
// checks that both sides of the new bound moved, and restores the default.
func TestNestingDepthIsConfigurable(t *testing.T) {
	if got := jsonbind.MaxNestingDepth(); got != jsonbind.DefaultMaxNestingDepth {
		t.Fatalf("default bound = %d, want %d", got, jsonbind.DefaultMaxNestingDepth)
	}
	jsonbind.SetMaxNestingDepth(200)
	defer jsonbind.SetMaxNestingDepth(0)
	if err := jsonbind.NewParser(nested("[", "", "]", 200)).SkipValue(); err != nil {
		t.Fatalf("depth 200 should decode after raising the bound: %v", err)
	}
	if err := jsonbind.NewParser(nested("[", "", "]", 201)).SkipValue(); err == nil {
		t.Fatal("depth 201 decoded past the raised bound")
	}
	jsonbind.SetMaxNestingDepth(0)
	if got := jsonbind.MaxNestingDepth(); got != jsonbind.DefaultMaxNestingDepth {
		t.Fatalf("bound after reset = %d, want the default", got)
	}
}
