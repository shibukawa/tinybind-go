package bindcore

import (
	"encoding/base64"
	"strings"
)

// ParseBytes decodes a base64 value carried by a query, path, header or cookie
// into a byte field.
//
// Both base64 alphabets are accepted, padded or not, because a value source is
// not a document. Two things force that: a query value carrying + arrives as a
// space unless the client percent-encoded it, and a client putting a blob in a
// URL will have reached for the URL-safe alphabet anyway. Refusing either would
// mean the same blob binds from a body and 400s from a query, which is a
// difference the field never asked for.
//
// The writing side has no such choice to make. A byte field is written as
// padded standard base64 wherever it is written, so being permissive here
// widens what is accepted without widening what is produced.
//
// An empty value is an empty blob rather than an error, matching the empty
// string a byte field encodes to.
func ParseBytes(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	if strings.ContainsAny(s, "-_") {
		s = strings.ReplaceAll(s, "-", "+")
		s = strings.ReplaceAll(s, "_", "/")
	}
	// A length of one past a multiple of four is not base64 at all; the pad
	// added for it is invalid on purpose, so the decoder reports it.
	if n := len(s) % 4; n != 0 {
		s += "===="[n:]
	}
	return base64.StdEncoding.DecodeString(s)
}
