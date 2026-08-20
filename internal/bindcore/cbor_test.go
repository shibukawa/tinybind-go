package bindcore

import "testing"

func TestIsCBORMediaType(t *testing.T) {
	for media, want := range map[string]bool{
		"application/cbor":       true,
		"application/senml+cbor": true,
		"application/json":       false,
		"text/plain":             false,
		"":                       false,
	} {
		if got := IsCBORMediaType(media); got != want {
			t.Errorf("IsCBORMediaType(%q) = %v, want %v", media, got, want)
		}
	}
}

func TestAcceptsCBOR(t *testing.T) {
	for accept, want := range map[string]bool{
		"application/cbor":                       true,
		"application/json, application/cbor":     true,
		"application/cbor;q=0.5":                 true,
		"Application/CBOR":                       true,
		" application/cbor ":                     true,
		"application/cbor;q=0":                   false,
		"application/cbor; q=0.000":              false,
		"*/*":                                    false,
		"application/*":                          false,
		"application/json":                       false,
		"":                                       false,
		"application/cbor;q=0, application/cbor": true,
	} {
		if got := AcceptsCBOR(accept); got != want {
			t.Errorf("AcceptsCBOR(%q) = %v, want %v", accept, got, want)
		}
	}
}

func TestMaxCBORBodyBytes(t *testing.T) {
	if got := MaxCBORBodyBytes(); got != DefaultMaxCBORBodyBytes {
		t.Fatalf("default limit %d", got)
	}
	SetMaxCBORBodyBytes(64)
	defer SetMaxCBORBodyBytes(0)
	if got := MaxCBORBodyBytes(); got != 64 {
		t.Fatalf("limit %d, want 64", got)
	}
	SetMaxCBORBodyBytes(-1)
	if got := MaxCBORBodyBytes(); got != DefaultMaxCBORBodyBytes {
		t.Fatalf("restored limit %d", got)
	}
}
