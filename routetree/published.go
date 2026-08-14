package routetree

import "unicode"

// PublishedName derives the identifier client script calls an action through,
// from its Go name.
//
// Go's export rule leaves no choice about the identifier, and a script writes
// actions.getUser, so the published name is a second name and something has to
// say what it is. This is the default; a declaration may override it.
//
// The rule is the leading run of capitals lowercased, leaving the last of the
// run intact when a lowercase letter follows it:
//
//	GetUser -> getUser
//	GetURL  -> getURL
//	URLFor  -> urlFor
//	ID      -> id
//
// That last clause is what separates the run that is an initialism from the one
// capital that starts the next word. It is deliberately not the lowerFirst rule
// the JSON field default uses, which lowercases one rune and so reads URLFor as
// uRLFor. The field rule is a shipped default whose change would move the wire
// under every existing project; a published action name has no installed base,
// so the better rule is affordable here and only here.
func PublishedName(name string) string {
	runes := []rune(name)
	if len(runes) == 0 {
		return name
	}
	run := 0
	for run < len(runes) && unicode.IsUpper(runes[run]) {
		run++
	}
	if run == 0 {
		return name
	}
	// A lowercase letter after the run means its last capital begins the next
	// word rather than belonging to the initialism, so it stays as written.
	if run < len(runes) && unicode.IsLower(runes[run]) && run > 1 {
		run--
	}
	for i := 0; i < run; i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
