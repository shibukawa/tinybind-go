package httpbind

import (
	"net/http"

	"github.com/shibukawa/tinybind-go/jsonbind"
)

// ReadBody dispatches on the request content type and reads the body at most
// once on behalf of a generated binder. wantForm/wantFiles mirror which body
// kinds the binder's fields can consume; a request whose content type matches
// none of them yields all-nil results without error, and the binder then
// falls back to its per-field defaults.
//
// Deprecated: current binders read the JSON body through ReadJSONBody and the
// form kinds through ReadFormBody. This exists for generated code that
// predates the inline body walk and is removed once that code is regenerated.
func ReadBody(r *http.Request, wantForm, wantFiles bool) (*jsonbind.Object, map[string]string, map[string]File, error) {
	// The media type is derived once and compared three times, rather than
	// re-reading and re-normalizing the header per content kind.
	media := mediaType(r.Header.Get("Content-Type"))
	if isJSONMediaType(media) {
		obj, err := ReadJSONObject(r)
		if err != nil {
			return nil, nil, nil, err
		}
		return obj, nil, nil, nil
	}
	if wantForm || wantFiles {
		if media == "application/x-www-form-urlencoded" {
			m, err := ParseFormMap(r)
			if err != nil {
				return nil, nil, nil, err
			}
			return nil, m, nil, nil
		}
		if media == "multipart/form-data" {
			m, files, err := ParseMultipartMap(r)
			if err != nil {
				return nil, nil, nil, err
			}
			if !wantFiles {
				files = nil
			}
			return nil, m, files, nil
		}
	}
	return nil, nil, nil, nil
}
