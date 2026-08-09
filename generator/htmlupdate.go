package generator

// The partial-update surface takes a transport and names no model, so discovery
// reads nothing from it and the transform has to know it anyway. Without these
// patterns a handler that branches on an update request would look like a
// handler making an unrecognized call, and be refused for the one thing the
// feature exists to do.
//
// Every entry has a counterpart over the other transport, including the
// streaming ones: they take a callback rather than handing a stream back, so
// what fasthttp writes from a body stream writer is the same producer body the
// net/http entry runs inline.

const (
	htmlupdateImportPath     = "github.com/shibukawa/tinybind-go/htmlupdate"
	fasthttpupdateImportPath = "github.com/shibukawa/tinybind-go/fasthttpupdate"
)

// htmlupdateTransportOnlyCalls declares the update entries a transformed
// handler may call, and which of their arguments carry the transport.
//
// Every name here exists under the same spelling on both runtimes, which is
// what lets the import rewrite do the rest.
func htmlupdateTransportOnlyCalls() []CallPattern {
	// The read-only entries. Each takes the request first and whatever it needs
	// after, and each returns a value the caller sends, which is why they port
	// at all.
	optionsRequestFirst := []string{
		"WantsUpdate", "Negotiate", "Redraw", "WriteUpdate", "WriteUpdateStatus",
		"Sequence", "CSRFToken", "VerifyCSRF",
		"Headers", "RedrawHeaders", "StreamHeaders", "LiveHeaders",
	}
	patterns := make([]CallPattern, 0, len(optionsRequestFirst)+3)
	for _, name := range optionsRequestFirst {
		patterns = append(patterns, TransportCall(
			Method(htmlupdateImportPath, name, htmlupdateImportPath, "Options"),
			RequestArgument(0),
		))
	}
	// Sending the answer. These are the other half of every entry above: a
	// Response that could be computed and not sent would be a port that stops
	// one call short of useful.
	patterns = append(patterns,
		TransportCall(
			Method(htmlupdateImportPath, "WriteTo", htmlupdateImportPath, "Response"),
			WriterArgument(0),
		),
		TransportCall(
			Method(htmlupdateImportPath, "NotModified", htmlupdateImportPath, "Response"),
			RequestArgument(0),
		),
		// ApplyTo is the function form, so its writer is the second argument:
		// the header set it copies from is ordinary data and stays.
		TransportCall(
			Function(htmlupdateImportPath, "ApplyTo"),
			WriterArgument(1),
		),
		// Redirect is the branch WantsUpdate creates. Without it the entry ports
		// and its documented use does not, because http.Redirect has no
		// transportable spelling and a refused redirect refuses the handler.
		TransportCall(
			Function(htmlupdateImportPath, "Redirect"),
			WriterArgument(0), RequestArgument(1),
		),
	)
	// The writing entries. Each leads with the writer and the request, so both
	// collapse into the one context and whatever follows keeps its place.
	for _, name := range []string{"Render", "RenderStream", "WriteStream", "WriteLiveStream"} {
		patterns = append(patterns, TransportCall(
			Method(htmlupdateImportPath, name, htmlupdateImportPath, "Options"),
			WriterArgument(0), RequestArgument(1),
		))
	}
	// The two that take the caller's cancellation first. It is not a transport
	// value and must survive: on fasthttp the delivery outlives the handler, so
	// this is the only thing that can bound it.
	for _, name := range []string{"RenderStreamAsync", "RenderLiveStream"} {
		patterns = append(patterns, TransportCall(
			Method(htmlupdateImportPath, name, htmlupdateImportPath, "Options"),
			WriterArgument(1), RequestArgument(2),
		))
	}
	return patterns
}
