package updatecore

import (
	"context"
	"net/url"
	"testing"
)

// headerReader is a Reader answering from a fixed header map, enough to drive
// negotiation.
type headerReader struct {
	method  string
	headers map[string]string
}

func (r headerReader) Header(name string) string { return r.headers[name] }
func (r headerReader) Method() string {
	if r.method == "" {
		return "GET"
	}
	return r.method
}
func (r headerReader) RawQuery() string         { return "" }
func (r headerReader) Query() url.Values        { return nil }
func (r headerReader) FormValue(string) string  { return "" }
func (r headerReader) Context() context.Context { return context.Background() }

// Negotiate and NegotiateMode must agree on mode and version for every request;
// the only difference is that Negotiate decodes the manifest into Known and
// NegotiateMode leaves it empty. That equivalence is what lets the header, mode,
// and version callers take the cheap path while the delta render alone decodes.
func TestNegotiateModeMatchesNegotiateWithoutTheManifest(t *testing.T) {
	var o Options
	build := o.Build()

	cases := []map[string]string{
		{}, // no render header -> document
		{o.RenderHeader(): "navigation;v=3", o.BuildHeader(): build, o.ManifestHeader(): "a:f1,b:f2,c:f3"},
		{o.RenderHeader(): "live", o.BuildHeader(): build, o.ManifestHeader(): "x:f"},
		{o.RenderHeader(): "redraw;v=2", o.BuildHeader(): build, o.ManifestHeader(): "r:f:children"},
		{o.RenderHeader(): "sequence"}, // needs no build, carries no manifest
		{o.RenderHeader(): "navigation", o.BuildHeader(): "other-build", o.ManifestHeader(): "a:f"}, // build mismatch -> document
	}
	for i, headers := range cases {
		r := headerReader{headers: headers}
		full := o.Negotiate(r)
		cheap := o.NegotiateMode(r)

		if full.Mode != cheap.Mode || full.Version != cheap.Version {
			t.Errorf("case %d: Negotiate=(mode %v, v%d) NegotiateMode=(mode %v, v%d)",
				i, full.Mode, full.Version, cheap.Mode, cheap.Version)
		}
		if len(cheap.Known.Instances) != 0 {
			t.Errorf("case %d: NegotiateMode decoded %d instances, want none", i, len(cheap.Known.Instances))
		}
		// A delta mode with a manifest header decodes into Known; every other
		// mode carries none, so nothing decoded the header needlessly.
		wantKnown := 0
		switch full.Mode {
		case ModeNavigation, ModeLive, ModeRedraw:
			wantKnown = len(DecodeManifest(headers[o.ManifestHeader()]).Instances)
		}
		if len(full.Known.Instances) != wantKnown {
			t.Errorf("case %d: Negotiate decoded %d instances, want %d", i, len(full.Known.Instances), wantKnown)
		}
	}
}

// A non-GET in a delta mode is a client error, not a delta — and NegotiateMode
// has to reach that verdict without touching the manifest.
func TestNegotiateModeRejectsNonGET(t *testing.T) {
	var o Options
	r := headerReader{
		method: "POST",
		headers: map[string]string{
			o.RenderHeader():   "navigation",
			o.BuildHeader():    o.Build(),
			o.ManifestHeader(): "a:f",
		},
	}
	if got := o.NegotiateMode(r).Mode; got != ModeDocument {
		t.Errorf("POST navigation resolved to %v, want ModeDocument", got)
	}
}
