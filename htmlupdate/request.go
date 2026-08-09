package htmlupdate

import (
	"context"
	"net/http"
	"net/url"

	"github.com/shibukawa/tinybind-go/internal/updatecore"
)

// request is *http.Request seen through the reader the update entries take.
//
// It is the whole of what this package's read-only half needed from net/http:
// six accessors, none of which writes. The other backend's wrapper is the same
// six over its own request, which is why one implementation answers both.
type request struct{ r *http.Request }

func reader(r *http.Request) updatecore.Reader { return request{r} }

func (q request) Header(name string) string {
	if q.r == nil {
		return ""
	}
	return q.r.Header.Get(name)
}

func (q request) Method() string {
	if q.r == nil {
		return ""
	}
	return q.r.Method
}

func (q request) RawQuery() string {
	if q.r == nil || q.r.URL == nil {
		return ""
	}
	return q.r.URL.RawQuery
}

func (q request) Query() url.Values {
	if q.r == nil || q.r.URL == nil {
		return nil
	}
	return q.r.URL.Query()
}

// FormValue reads the request body, as any handler reading a form does. Only
// the CSRF entries reach it, and only after the header channel came up empty.
func (q request) FormValue(name string) string {
	if q.r == nil {
		return ""
	}
	return q.r.PostFormValue(name)
}

func (q request) Context() context.Context {
	if q.r == nil {
		return context.Background()
	}
	return q.r.Context()
}
