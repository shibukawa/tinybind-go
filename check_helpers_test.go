package httpbinder_test

import (
	"testing"

	httpbinder "github.com/shibukawa/httpbind-go"
)

func TestCheckEmail(t *testing.T) {
	ok := []string{"a@b.co", "user.name+tag@example.com"}
	bad := []string{"", "nope", "@x.com", "a@", "a@b", "a b@c.com", "a@@b.com"}
	for _, s := range ok {
		if !httpbinder.CheckEmail(s) {
			t.Fatalf("expected ok: %q", s)
		}
	}
	for _, s := range bad {
		if httpbinder.CheckEmail(s) {
			t.Fatalf("expected fail: %q", s)
		}
	}
}

func TestCheckUUID(t *testing.T) {
	if !httpbinder.CheckUUID("550e8400-e29b-41d4-a716-446655440000") {
		t.Fatal("valid uuid")
	}
	if httpbinder.CheckUUID("not-a-uuid") || httpbinder.CheckUUID("550e8400e29b41d4a716446655440000") {
		t.Fatal("invalid uuid accepted")
	}
}

func TestCheckDateTimeFormats(t *testing.T) {
	if !httpbinder.CheckDate("2024-01-02") {
		t.Fatal("date")
	}
	if httpbinder.CheckDate("01/02/2024") {
		t.Fatal("non-ISO date")
	}
	if !httpbinder.CheckTime("15:04:05") {
		t.Fatal("time")
	}
	if !httpbinder.CheckDateTime("2024-01-02T15:04:05Z") {
		t.Fatal("datetime")
	}
	if !httpbinder.CheckDateTime("2024-01-02T15:04:05.123456789Z") {
		t.Fatal("datetime nano")
	}
	if httpbinder.CheckDateTime("2024-01-02T15:04:05") {
		t.Fatal("timezone-less datetime should fail")
	}
}
