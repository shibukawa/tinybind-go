package tinycheck_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/httpbind-go"
	"github.com/shibukawa/httpbind-go/internal/tinycheck"
)

func TestBindWrite(t *testing.T) {
	body := `{"name":"Alice","email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	req.SetPathValue("org_id", "acme")
	got, err := httpbinder.Bind[tinycheck.CreateUserRequest](req)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if got.Name != "Alice" || got.Email != "a@example.com" || got.OrgID != "acme" || got.Token != "Bearer secret" {
		t.Fatalf("got %+v", got)
	}
	rec := httptest.NewRecorder()
	if err := httpbinder.Write[tinycheck.CreateUserResponse](rec, req, tinycheck.CreateUserResponse{
		ID: "1", Name: "Alice", Email: "a@example.com",
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if rec.Code != 200 {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"1"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}
