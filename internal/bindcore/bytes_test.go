package bindcore_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/shibukawa/tinybind-go/internal/bindcore"
)

// A value source is not a document: the same blob has to bind whichever
// alphabet a client reached for, and whether or not it kept the padding.
func TestParseBytes(t *testing.T) {
	blob := []byte{0xfb, 0xef, 0xbe, 0x3f, 0xf0}
	std := base64.StdEncoding.EncodeToString(blob)
	if !bytes.ContainsAny([]byte(std), "+/") {
		t.Fatalf("pick a blob whose standard base64 needs escaping: %s", std)
	}
	for _, encoded := range []string{
		std,
		base64.RawStdEncoding.EncodeToString(blob),
		base64.URLEncoding.EncodeToString(blob),
		base64.RawURLEncoding.EncodeToString(blob),
	} {
		got, err := bindcore.ParseBytes(encoded)
		if err != nil {
			t.Fatalf("%s: %v", encoded, err)
		}
		if !bytes.Equal(got, blob) {
			t.Fatalf("%s decoded to %v", encoded, got)
		}
	}
}

func TestParseBytesRefusesWhatIsNotBase64(t *testing.T) {
	for _, s := range []string{"not base64!!", "++++ ", "A", "AQID===="} {
		if got, err := bindcore.ParseBytes(s); err == nil {
			t.Fatalf("%q was accepted as %v", s, got)
		}
	}
}

// The empty value is the empty blob, matching the empty string a byte field
// encodes to.
func TestParseBytesEmpty(t *testing.T) {
	got, err := bindcore.ParseBytes("")
	if err != nil || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
}
