package routetree

import "testing"

func TestPublishedName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"GetUser", "getUser"},
		{"GetURL", "getURL"},
		{"URLFor", "urlFor"},
		{"ID", "id"},
		{"Rename", "rename"},
		{"A", "a"},
		{"AB", "ab"},
		{"ABc", "aBc"},
		{"HTTPServer", "httpServer"},
		{"", ""},
		// Already lower, or opening with a digit: nothing to move.
		{"getUser", "getUser"},
	} {
		if got := PublishedName(tc.in); got != tc.want {
			t.Errorf("PublishedName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
