package htmlbind

import "testing"

func TestJSONMemberOmitsRatherThanNulls(t *testing.T) {
	// An absent optional is never appended at all, so the object carries one
	// absence for JavaScript to test rather than a key holding null.
	body := ""
	body = JSONMember(body, "label", JSONString("hi"))
	body = JSONMember(body, "row", `{"id":"7"}`)
	if want := `"label":"hi","row":{"id":"7"}`; body != want {
		t.Errorf("body = %s, want %s", body, want)
	}
	if first := JSONMember("", "only", "1"); first != `"only":1` {
		t.Errorf("first member = %s, want no leading separator", first)
	}
}

func TestJSONMemberEscapesTheName(t *testing.T) {
	if got := JSONMember("", `a"b`, "1"); got != `"a\"b":1` {
		t.Errorf("member = %s, want the name escaped", got)
	}
}
