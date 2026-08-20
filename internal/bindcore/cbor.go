package bindcore

import (
	"strings"
	"sync/atomic"
)

// CBOR-side logic both transport runtimes share, mirroring the multipart
// arrangement above: one process-wide limit and one reading of the request
// headers, so the two surfaces cannot drift apart. Nothing here names a CBOR
// type — encoding stays with the driver and the generated code, and this file
// only answers "is this request CBOR" and "how much may it be".

// CBORContentType is the media type CBOR bodies are read and written under.
const CBORContentType = "application/cbor"

// DefaultMaxCBORBodyBytes is the default cap for CBOR body reads (1 MiB),
// matching the JSON default so switching a client's encoding does not change
// how much it may send.
const DefaultMaxCBORBodyBytes int64 = 1 << 20

// maxCBORBodyBytes holds the process-wide CBOR body limit.
// Zero means "use DefaultMaxCBORBodyBytes".
var maxCBORBodyBytes atomic.Int64

// SetMaxCBORBodyBytes sets the global CBOR body size limit.
//
//	n > 0  → use n bytes
//	n <= 0 → restore DefaultMaxCBORBodyBytes (1 MiB)
func SetMaxCBORBodyBytes(n int64) {
	if n <= 0 {
		maxCBORBodyBytes.Store(0)
		return
	}
	maxCBORBodyBytes.Store(n)
}

// MaxCBORBodyBytes returns the effective global CBOR body limit.
func MaxCBORBodyBytes() int64 {
	n := maxCBORBodyBytes.Load()
	if n <= 0 {
		return DefaultMaxCBORBodyBytes
	}
	return n
}

// IsCBORMediaType reports whether media is CBOR or a +cbor structured syntax
// suffix type (RFC 6839), e.g. application/cbor, application/senml+cbor.
func IsCBORMediaType(media string) bool {
	if media == "" {
		return false
	}
	if media == CBORContentType {
		return true
	}
	return strings.HasSuffix(media, "+cbor")
}

// AcceptsCBOR reports whether an Accept header value asks for a CBOR response.
//
// Only an explicit application/cbor entry counts. Wildcards do not: a browser
// sends */* on every navigation, and answering it with CBOR would switch the
// default response format on clients that never asked. A q=0 entry is a
// refusal and does not count either. Relative preference between CBOR and
// JSON beyond that is not weighed — an explicit ask wins.
func AcceptsCBOR(accept string) bool {
	for accept != "" {
		var entry string
		entry, accept, _ = strings.Cut(accept, ",")
		media, params, _ := strings.Cut(entry, ";")
		if strings.TrimSpace(strings.ToLower(media)) != CBORContentType {
			continue
		}
		if acceptParamsRefuse(params) {
			continue
		}
		return true
	}
	return false
}

// acceptParamsRefuse reports whether the parameter list carries q=0 (in any of
// its spellings), which RFC 9110 defines as "not acceptable".
func acceptParamsRefuse(params string) bool {
	for params != "" {
		var p string
		p, params, _ = strings.Cut(params, ";")
		name, value, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
			continue
		}
		switch strings.TrimSpace(value) {
		case "0", "0.", "0.0", "0.00", "0.000":
			return true
		}
	}
	return false
}
