package tomlbind_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/tomlbind"
)

// standIn carries the method the generator would write, so the entry points
// are exercised without a generated package.
type standIn struct {
	Seen string
}

var errRefused = errors.New("refused")

func (v *standIn) DecodeTOMLFrom(data []byte) error {
	if strings.HasPrefix(string(data), "!") {
		return errRefused
	}
	*v = standIn{Seen: string(data)}
	return nil
}

func TestDecodeTOMLReadsTheWholeReader(t *testing.T) {
	got, err := tomlbind.DecodeTOML[standIn](strings.NewReader("a = 1\nb = 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Seen != "a = 1\nb = 2\n" {
		t.Fatalf("got %q", got.Seen)
	}
}

func TestDecodeTOMLBytesReturnsTheMethodsError(t *testing.T) {
	if _, err := tomlbind.DecodeTOMLBytes[standIn]([]byte("!")); !errors.Is(err, errRefused) {
		t.Fatalf("got %v", err)
	}
}

// The declaration compiles for a type with no method at all, since it is
// written before the method exists.
type undeclared struct{}

var _ = tomlbind.GenerateDecoder[undeclared]()
