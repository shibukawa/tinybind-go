package htmlbind

import "strings"

// BlockedURL is what a URL-bearing attribute renders when the value's scheme is
// not one this render permits.
//
// It is a fragment, so it resolves to the current document and reaches nothing.
// Substituting it rather than dropping the attribute is deliberate: a dropped
// href is indistinguishable from an attribute the template never wrote, so a
// URL rejected in error would leave no trace to find it by.
const BlockedURL = "#tb-blocked-url"

// DefaultURLSchemes are the schemes a URL-bearing attribute renders when the
// caller configures none.
//
// The set is deliberately small. It can be, because [WithURLSchemes] exists: an
// app needing ftp, sms, or its own registered scheme says so, which is cheaper
// than defaulting schemes in for every app because one app might want them.
var DefaultURLSchemes = []string{"http", "https", "mailto", "tel"}

// DefaultDataURLMediaTypes are the media types an inline data URL may carry.
//
// An inline image is ordinary authoring, so the scheme is not refused outright.
// The roster is an allowlist of exact media types rather than an image/ prefix,
// which is what keeps image/svg+xml off it: an SVG document carries script, so
// it is a script sink wearing an image's media type.
var DefaultDataURLMediaTypes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
	"image/avif",
	"image/bmp",
	"image/x-icon",
}

// WithURLSchemes replaces the schemes a URL-bearing attribute may carry.
//
// Names are matched case-insensitively against the scheme the browser will read,
// and a value with no scheme at all — a relative path, a scheme-relative URL, or
// a bare fragment — is always permitted, because it cannot leave the origin the
// document already has.
//
// Passing no schemes permits none, which leaves relative URLs as the only form
// that renders.
func WithURLSchemes(schemes ...string) Option {
	normalized := make([]string, len(schemes))
	for i, scheme := range schemes {
		normalized[i] = strings.ToLower(strings.TrimSuffix(scheme, ":"))
	}
	return func(o *renderOptions) {
		o.urlSchemes = normalized
		o.urlSchemesSet = true
	}
}

// WithDataURLMediaTypes replaces the media types an inline data URL may carry.
//
// The data scheme is handled apart from [WithURLSchemes] because permitting it
// wholesale would permit text/html, which is a document rather than an asset.
// Passing no media types refuses every data URL.
func WithDataURLMediaTypes(mediaTypes ...string) Option {
	normalized := make([]string, len(mediaTypes))
	for i, mediaType := range mediaTypes {
		normalized[i] = strings.ToLower(strings.TrimSpace(mediaType))
	}
	return func(o *renderOptions) {
		o.dataURLMediaTypes = normalized
		o.dataURLMediaTypesSet = true
	}
}

func (o *renderOptions) schemes() []string {
	if o.urlSchemesSet {
		return o.urlSchemes
	}
	return DefaultURLSchemes
}

func (o *renderOptions) dataMediaTypes() []string {
	if o.dataURLMediaTypesSet {
		return o.dataURLMediaTypes
	}
	return DefaultDataURLMediaTypes
}

// safeURL returns value unchanged when this render permits its scheme, and
// [BlockedURL] when it does not.
//
// It decides on the text a browser will read rather than on a parsed URL's
// Scheme field, which is not the same thing: url.URL{Opaque: "javascript:x"}
// has an empty Scheme and still renders as javascript:x, so a gate reading the
// field would pass it as a relative URL.
func (o *renderOptions) safeURL(value string) string {
	if o.permitsURL(value) {
		return value
	}
	return BlockedURL
}

func (o *renderOptions) permitsURL(value string) bool {
	normalized := browserNormalizedURL(value)
	scheme, ok := urlScheme(normalized)
	if !ok {
		// Relative, scheme-relative, or a bare fragment. There is no scheme to
		// judge and no way for one of these to reach another protocol.
		return true
	}
	if scheme == "data" {
		return o.permitsDataURL(normalized)
	}
	for _, allowed := range o.schemes() {
		if scheme == allowed {
			return true
		}
	}
	return false
}

func (o *renderOptions) permitsDataURL(normalized string) bool {
	rest := normalized[len("data:"):]
	// The media type runs to the parameter separator or to the comma that ends
	// the header. A data URL carrying neither is malformed, and refusing it
	// costs nothing.
	end := strings.IndexAny(rest, ";,")
	if end < 0 {
		return false
	}
	mediaType := strings.ToLower(strings.TrimSpace(rest[:end]))
	for _, allowed := range o.dataMediaTypes() {
		if mediaType == allowed {
			return true
		}
	}
	return false
}

// browserNormalizedURL removes what a browser removes before it reads a URL, so
// the scheme is read from what will actually be resolved.
//
// Tab, line feed, and carriage return are stripped wherever they appear, which
// is what makes "java\tscript:alert(1)" a javascript URL rather than a relative
// path with an odd name. Leading control characters and spaces are trimmed for
// the same reason.
var urlWhitespaceStripper = strings.NewReplacer("\t", "", "\n", "", "\r", "")

func browserNormalizedURL(value string) string {
	if strings.ContainsAny(value, "\t\n\r") {
		value = urlWhitespaceStripper.Replace(value)
	}
	return strings.TrimLeftFunc(value, func(r rune) bool { return r <= ' ' })
}

// urlScheme reads the scheme off an already-normalized URL, lowercased.
//
// It reports false for a value that carries no scheme, which is the relative
// case and is always permitted. The grammar is RFC 3986's: a letter, then
// letters, digits, plus, minus, and dot, up to a colon.
func urlScheme(normalized string) (string, bool) {
	for i := 0; i < len(normalized); i++ {
		c := normalized[i]
		switch {
		case c == ':':
			if i == 0 {
				return "", false
			}
			return strings.ToLower(normalized[:i]), true
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			// A letter is valid anywhere in a scheme, first position included.
		case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
			if i == 0 {
				// A scheme starts with a letter, so this is a relative path
				// whose first segment happens to contain a colon later on.
				return "", false
			}
		default:
			// A slash, question mark, hash, or anything else ends the search:
			// no scheme can still be opened after one of these.
			return "", false
		}
	}
	return "", false
}

// safeSrcsetURLs applies the scheme policy to every candidate of a srcset,
// dropping the ones it refuses and keeping the rest.
//
// One hostile candidate does not discard the good ones, because the attribute
// is a list of alternatives rather than a single value and dropping all of them
// would turn a scheme rejection into a missing image.
func (o *renderOptions) safeSrcsetURLs(value string) string {
	candidates := strings.Split(value, ",")
	kept := candidates[:0]
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		// A candidate is a URL, then optional whitespace and a descriptor.
		reference := trimmed
		if cut := strings.IndexFunc(trimmed, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }); cut >= 0 {
			reference = trimmed[:cut]
		}
		if o.permitsURL(reference) {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, ", ")
}

// safeSpaceURLs applies the scheme policy to a whitespace-separated URL list,
// which is the shape ping carries.
func (o *renderOptions) safeSpaceURLs(value string) string {
	references := strings.Fields(value)
	kept := references[:0]
	for _, reference := range references {
		if o.permitsURL(reference) {
			kept = append(kept, reference)
		}
	}
	return strings.Join(kept, " ")
}
