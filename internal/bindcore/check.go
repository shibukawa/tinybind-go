package bindcore

import (
	"strings"
	"time"
)

// CheckEmail reports whether s is a pragmatic (non-RFC5322) email.
// Empty string returns false; callers skip empty optional fields before calling.
func CheckEmail(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	at := strings.IndexByte(s, '@')
	if at <= 0 || at != strings.LastIndexByte(s, '@') {
		return false
	}
	local, domain := s[:at], s[at+1:]
	if local == "" || domain == "" {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}

// CheckUUID reports whether s is a UUID string (8-4-4-4-12 hex with dashes).
// Version/variant bits are not enforced.
func CheckUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHex(byte(c)) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// CheckDate reports whether s is an ISO date (YYYY-MM-DD / time.DateOnly).
func CheckDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

// CheckTime reports whether s is an ISO time (HH:MM:SS / time.TimeOnly).
func CheckTime(s string) bool {
	_, err := time.Parse(time.TimeOnly, s)
	return err == nil
}

// CheckDateTime reports whether s is RFC3339 (fractional seconds accepted).
func CheckDateTime(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
