package httpbind_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/httpbind"
	"github.com/shibukawa/tinybind-go/internal/mappingfixture"
)

func TestWriteStatus_Created(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	err := httpbind.WriteStatus(rec, req, http.StatusCreated, mappingfixture.CreateUserResponse{
		ID:    "u1",
		Name:  "Ada",
		Email: "a@example.com",
	})
	if err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type: %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"id":"u1"`) && !strings.Contains(body, `"id": "u1"`) {
		// Encoder may produce compact JSON
		if !strings.Contains(body, "u1") || !strings.Contains(body, "Ada") {
			t.Fatalf("body: %s", body)
		}
	}
}

func TestWriteStatus_NoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	err := httpbind.WriteStatus(rec, req, http.StatusNoContent, mappingfixture.CreateUserResponse{})
	if err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 body should be empty, got %q", rec.Body.String())
	}
}

func TestWrite_Still200(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	err := httpbind.Write(rec, req, mappingfixture.CreateUserResponse{ID: "x", Name: "n", Email: "e"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
}
