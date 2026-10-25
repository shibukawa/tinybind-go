package httpbind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsCBORRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Content-Type", "application/cbor; charset=utf-8")
	if !IsCBORRequest(r) {
		t.Fatal("application/cbor not recognized")
	}
	r.Header.Set("Content-Type", "application/json")
	if IsCBORRequest(r) {
		t.Fatal("application/json recognized as CBOR")
	}
}

func TestAcceptsCBOR(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if AcceptsCBOR(r) {
		t.Fatal("no Accept header accepted CBOR")
	}
	r.Header.Set("Accept", "application/json, application/cbor")
	if !AcceptsCBOR(r) {
		t.Fatal("explicit application/cbor not accepted")
	}
	r.Header.Set("Accept", "*/*")
	if AcceptsCBOR(r) {
		t.Fatal("a wildcard flipped the response format")
	}
}

func TestReadCBORBodyHonoursTheLimit(t *testing.T) {
	SetMaxCBORBodyBytes(8)
	defer SetMaxCBORBodyBytes(0)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789"))
	_, err := ReadCBORBody(r)
	he, ok := AsHTTPError(err)
	if !ok || he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %#v", err)
	}

	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345678"))
	data, err := ReadCBORBody(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "12345678" {
		t.Fatalf("read %q", data)
	}
}

func TestVaryAccept(t *testing.T) {
	w := httptest.NewRecorder()
	VaryAccept(w)
	if got := w.Header().Get("Vary"); got != "Accept" {
		t.Fatalf("Vary %q, want Accept", got)
	}
}

func TestWriteCBORBytes(t *testing.T) {
	w := httptest.NewRecorder()
	if err := WriteCBORBytes(w, http.StatusCreated, []byte{0xa0}); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("content type %q", ct)
	}
	if body := w.Body.Bytes(); len(body) != 1 || body[0] != 0xa0 {
		t.Fatalf("body %x", body)
	}
}
