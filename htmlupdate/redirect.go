package htmlupdate

import "net/http"

// Redirect sends the browser somewhere else, for the branch WantsUpdate exists
// to create.
//
// A form submission that cannot apply an update response has to be answered
// with a redirect, which is what keeps a page working without JavaScript, so
// the redirect is not an aside — it is the other half of every action handler
// this package serves.
//
// It is here rather than left to net/http because http.Redirect has no
// transportable spelling: the other backend redirects through a method on its
// context, and the transform rewrites argument lists rather than turning a
// function call into a method call. Declaring the same name over both is what
// lets one handler body compile on either.
//
// It is http.Redirect and nothing more. The relative-path resolution, the
// hyperlink body on a GET, and the status handling are the standard library's,
// because a second implementation of those would be a second set of bugs.
func Redirect(w http.ResponseWriter, r *http.Request, url string, status int) {
	http.Redirect(w, r, url, status)
}
