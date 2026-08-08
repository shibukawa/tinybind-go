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
func ReadBody(r *http.Request, wantForm, wantFiles bool) (*jsonbind.Object, map[string]string, map[string]File, error) {
	if IsJSONRequest(r) {
		obj, err := ReadJSONObject(r)
		if err != nil {
			return nil, nil, nil, err
		}
		return obj, nil, nil, nil
	}
	if wantForm || wantFiles {
		if IsFormRequest(r) {
			m, err := ParseFormMap(r)
			if err != nil {
				return nil, nil, nil, err
			}
			return nil, m, nil, nil
		}
		if IsMultipartRequest(r) {
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
