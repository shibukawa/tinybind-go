package httpbind_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpbind "github.com/shibukawa/tinybind-go"
)

func postForm(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/users/123?tab=x", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestSilentHandlerRedirectsBackToThePage(t *testing.T) {
	w := httptest.NewRecorder()
	httpbind.DispatchAction(w, postForm("title=new"), func(http.ResponseWriter, *http.Request) {})

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	// Post-redirect-get lands on a fresh GET of the same page, so a reload does
	// not resubmit and the address bar keeps showing the page, query included.
	if got := w.Header().Get("Location"); got != "/users/123?tab=x" {
		t.Errorf("Location = %q, want the page it was submitted from", got)
	}
}

func TestHandlerWritingABodyKeepsItsOwnResponse(t *testing.T) {
	w := httptest.NewRecorder()
	httpbind.DispatchAction(w, postForm(""), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("name is required"))
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want the handler's own", w.Code)
	}
	if body := w.Body.String(); body != "name is required" {
		t.Errorf("body = %q, want the handler's own", body)
	}
	if w.Header().Get("Location") != "" {
		t.Error("a redirect was added over a handler that answered")
	}
}

func TestHandlerRedirectingElsewhereIsNotOverridden(t *testing.T) {
	w := httptest.NewRecorder()
	httpbind.DispatchAction(w, postForm(""), func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/users", http.StatusFound)
	})

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want the handler's own redirect", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/users" {
		t.Errorf("Location = %q, want where the handler chose", got)
	}
}

func TestHandlerSettingOnlyAHeaderCountsAsAnswering(t *testing.T) {
	w := httptest.NewRecorder()
	httpbind.DispatchAction(w, postForm(""), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Result", "queued")
	})

	// A handler setting a header and returning has chosen its response as surely
	// as one writing a body, so the default must not overwrite it.
	if w.Code == http.StatusSeeOther {
		t.Error("the default redirect displaced a handler that set a header")
	}
	if got := w.Header().Get("X-Result"); got != "queued" {
		t.Errorf("X-Result = %q, want the handler's own header", got)
	}
}

func TestPreexistingHeadersAreNotMistakenForAResponse(t *testing.T) {
	w := httptest.NewRecorder()
	// Middleware commonly sets a header before any handler runs, and counting one
	// would suppress the redirect for every silent handler behind it.
	w.Header().Set("X-Request-Id", "abc")
	httpbind.DispatchAction(w, postForm(""), func(http.ResponseWriter, *http.Request) {})

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want the default redirect", w.Code)
	}
}

func TestActionSelectorReadsTheHiddenField(t *testing.T) {
	got := httpbind.ActionSelector(postForm("_action=00369cf962b6%2FRename&title=x"), "_action")
	if want := "00369cf962b6/Rename"; got != want {
		t.Errorf("selector = %q, want %q", got, want)
	}
}

func TestActionSelectorPrefersTheQuery(t *testing.T) {
	// A submit button's formaction carries the selector in the query so one form
	// dispatches to several handlers, and that channel has to win over the form's
	// own hidden field rather than merely coexist with it.
	r := httptest.NewRequest(http.MethodPost, "/users/123?_action=aaa%2FDelete",
		strings.NewReader("_action=bbb/Rename"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if got := httpbind.ActionSelector(r, "_action"); got != "aaa/Delete" {
		t.Errorf("selector = %q, want the query to win", got)
	}
}

func TestActionSelectorDefaultsItsFieldName(t *testing.T) {
	if got := httpbind.ActionSelector(postForm("_action=aaa%2FRename"), ""); got != "aaa/Rename" {
		t.Errorf("selector = %q, want the default field to be read", got)
	}
}

func TestActionSelectorIsEmptyWhenNoneWasSubmitted(t *testing.T) {
	if got := httpbind.ActionSelector(postForm("title=x"), "_action"); got != "" {
		t.Errorf("selector = %q, want empty", got)
	}
}
