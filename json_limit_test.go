package httpbinder

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadJSONMapRejectsOversize(t *testing.T) {
	old := MaxJSONBodyBytes()
	SetMaxJSONBodyBytes(16)
	defer SetMaxJSONBodyBytes(old)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"value":"this is too large"}`))
	_, err := ReadJSONMap(r)
	he, ok := AsHTTPError(err)
	if !ok || he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %#v", err)
	}
}

type limitedDocument struct{ Value string }

func TestDecodeJSONLimitRejectsUnknownLengthReader(t *testing.T) {
	RegisterDecode[limitedDocument](func([]byte) (limitedDocument, error) { return limitedDocument{}, nil })
	_, err := DecodeJSONLimit[limitedDocument](strings.NewReader(`{"value":"too large"}`), 8)
	he, ok := AsHTTPError(err)
	if !ok || he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %#v", err)
	}
}

type globallyLimitedDocument struct{ Value string }

func TestDecodeJSONUsesGlobalLimit(t *testing.T) {
	old := MaxJSONBodyBytes()
	SetMaxJSONBodyBytes(8)
	defer SetMaxJSONBodyBytes(old)
	RegisterDecode[globallyLimitedDocument](func([]byte) (globallyLimitedDocument, error) {
		return globallyLimitedDocument{}, nil
	})
	_, err := DecodeJSON[globallyLimitedDocument](strings.NewReader(`{"value":"too large"}`))
	he, ok := AsHTTPError(err)
	if !ok || he.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %#v", err)
	}
}
