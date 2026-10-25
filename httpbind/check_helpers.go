package httpbind

import "github.com/shibukawa/tinybind-go/internal/bindcore"

// The check helpers read bound values and never touch a request, so they live
// in bindcore and both transport runtimes call the same implementation.

// CheckEmail reports whether s is a pragmatic (non-RFC5322) email.
// Empty string returns false; callers skip empty optional fields before calling.
func CheckEmail(s string) bool { return bindcore.CheckEmail(s) }

// CheckUUID reports whether s is a UUID string (8-4-4-4-12 hex with dashes).
func CheckUUID(s string) bool { return bindcore.CheckUUID(s) }

// CheckDate reports whether s is an ISO date (YYYY-MM-DD / time.DateOnly).
func CheckDate(s string) bool { return bindcore.CheckDate(s) }

// CheckTime reports whether s is an ISO time (HH:MM:SS / time.TimeOnly).
func CheckTime(s string) bool { return bindcore.CheckTime(s) }

// CheckDateTime reports whether s is RFC3339 (fractional seconds accepted).
func CheckDateTime(s string) bool { return bindcore.CheckDateTime(s) }
