package htmlbind

import (
	"errors"
)

// A CSRF token is not a framework extension. It is a property of every unsafe
// form this package renders, which is why the field is emitted rather than
// offered as a seam for someone else to fill: a security control an author has
// to remember to write is one an author will forget, and the omission renders a
// working page that fails only on submission.
//
// The token itself belongs to the caller. This package has no session, no
// cookie, and no net/http, so it takes the value and puts it where a request can
// carry it — and does nothing else about it.

// ErrNoCSRFToken reports a render that reached an unsafe form with no token to
// put in it.
var ErrNoCSRFToken = errors.New("htmlbind: form needs a CSRF token")

// errCSRFTokenUnset is the full failure CSRFField returns, built once because
// nothing in it varies per render.
var errCSRFTokenUnset = wrapError(ErrNoCSRFToken, ": this render supplied none; pass WithCSRFToken, or WithoutCSRFToken for a render with no session behind it")

// WithCSRFToken supplies the session's CSRF token for this render.
//
// It is an option rather than something read from the context because this
// package cannot read it from a context: the key belongs to whoever owns the
// session, and there is nothing here to look up. A framework passes it once
// inside its own render entry, from its own accessor, so no handler changes.
//
// The same value reaches the browser twice — in the hidden field of every unsafe
// form, and in the request header the runtime sends — because a form must submit
// with scripting disabled and cannot set a header, while a fetch may not be
// carrying a form at all. One value per session is what keeps the two agreeing.
func WithCSRFToken(token string) Option {
	return func(o *renderOptions) { o.csrf, o.csrfSupplied = token, true }
}

// WithoutCSRFToken says this render has no session behind it, so an unsafe form
// may render with an empty token instead of failing.
//
// It exists for the renders that are not responses: a mail body, a static
// export, a golden test. It is explicit because the alternative — treating an
// absent token as "none wanted" — turns a forgotten option into a form that
// submits and is rejected, with nothing pointing at the cause.
func WithoutCSRFToken() Option {
	return func(o *renderOptions) { o.csrfOmitted = true }
}

// CSRFField writes the hidden input carrying the session's CSRF token.
//
// Generation emits it as the first child of every unsafe form, so a later field
// cannot displace it and an author writes nothing. A GET form never gets one:
// its fields become the query string, and a token in a URL reaches history,
// logs, and referrers.
func (Builder[P]) CSRFField(name string) Op[P] { return csrfFieldOp[P](name) }

type csrfFieldOp[P any] string

func (o csrfFieldOp[P]) Exec(r *Renderer, _ P) error {
	token, ok := r.csrfToken()
	if !ok {
		return errCSRFTokenUnset
	}
	return r.Write(`<input type="hidden" name="` + string(o) + `" value="` + Escape(token) + `">`)
}

// csrfToken returns the token this render was given, and whether it may be
// written at all.
func (r *Renderer) csrfToken() (string, bool) {
	if r.opts == nil {
		return "", false
	}
	if r.opts.csrfSupplied {
		return r.opts.csrf, true
	}
	if r.opts.csrfOmitted {
		return "", true
	}
	return "", false
}
