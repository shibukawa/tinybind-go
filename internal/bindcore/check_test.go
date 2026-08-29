package bindcore

import "testing"

// CheckUUID must judge bytes, not runes: range over a string steps by rune, so
// a multi-byte rune used to skip the byte positions it covered — a dash
// position included — and the byte() truncation let a Cyrillic а pass as '0'.
// Every input here is exactly 36 bytes, which is what got them past the
// length gate.
func TestCheckUUIDRejectsMultibyteRunes(t *testing.T) {
	for _, s := range []string{
		"12345678-1234-1234-1234-а0123456789", // Cyrillic а: truncates to '0'
		"1234567а1234-1234-1234-123456789012", // rune spans the dash position
		"а2345678-1234-1234-1234-01234567890", // rune opens the string
	} {
		if len(s) != 36 {
			t.Fatalf("corpus entry %q is %d bytes; the gate under test needs 36", s, len(s))
		}
		if CheckUUID(s) {
			t.Errorf("CheckUUID(%q) = true, want false", s)
		}
	}
	if !CheckUUID("12345678-1234-5678-9abc-DEF012345678") {
		t.Error("a canonical UUID stopped validating")
	}
}
