package fasthttpupdate

import (
	"bufio"
	"context"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/internal/updatecore"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The streaming half, over SetBodyStreamWriter.
//
// Every entry here does the same two things: decide everything it can while the
// handler still owns the context, then install a writer that delivers. The split
// is not a style choice — fasthttp documents that access to the RequestCtx is
// forbidden from a body stream writer, so anything read from the request has to
// be captured first. StreamPlan is that capture, and it is the same value the
// net/http entries build.

// DeltaStream is an open record stream a producer writes boundary completions
// to as they settle. It is the same type htmlupdate hands a producer, so a
// handler body is the same text on both transports.
type DeltaStream = updatecore.DeltaStream

// ManifestEntry is what a client stores for one instance beside its markup.
type ManifestEntry = updatecore.ManifestEntry

// StreamPlan is what a stream entry decided before it committed.
type StreamPlan = updatecore.StreamPlan

// WriteStream opens a record stream, runs fn against it, and closes it.
//
// fn runs from the body stream writer, after the handler has returned, so it
// must not read ctx. Everything the stream needs is captured before this call
// returns. An error from fn is reported in band through the terminator and then
// sent to the handler installed with SetStreamErrorHandler, because the
// response committed when the head record went out.
func (o Options) WriteStream(ctx *fasthttp.RequestCtx, head []string, fn func(*DeltaStream) error) {
	o.writeStream(ctx, updatecore.StreamPlan{
		Mode: ModeNavigation, Version: o.Negotiate(ctx).Version, Head: head,
	}, fn)
}

// WriteLiveStream is WriteStream for a delivery stream: the same records on the
// same framing, in the live mode rather than the navigation one, so the
// terminator can mean "come back" rather than "stop".
func (o Options) WriteLiveStream(ctx *fasthttp.RequestCtx, head []string, fn func(*DeltaStream) error) {
	o.writeStream(ctx, updatecore.StreamPlan{
		Mode: ModeLive, Live: true, Version: o.Negotiate(ctx).Version, Head: head,
	}, fn)
}

func (o Options) writeStream(ctx *fasthttp.RequestCtx, plan updatecore.StreamPlan, fn func(*DeltaStream) error) {
	if ctx == nil {
		return
	}
	core := o.core()
	ctx.SetBodyStreamWriter(func(bw *bufio.Writer) {
		stream := core.OpenStream(bw, plan)
		err := fn(stream)
		if err != nil {
			stream.Fail(err.Error())
		}
		if cerr := stream.Close(); err == nil {
			err = cerr
		}
		if ferr := bw.Flush(); err == nil {
			err = ferr
		}
		if err != nil {
			updatecore.ReportStreamError(err)
		}
	})
}

// Render answers one request with either a complete document or a delta.
//
// It buffers, so it holds nothing open and needs no body stream writer: the
// delta path encodes one JSON body and the document path writes the collected
// chain, both inside the handler. Every failure is an ordinary error the caller
// can still turn into a status.
func (o Options) Render(ctx *fasthttp.RequestCtx, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	if ctx == nil {
		return nil
	}
	return o.core().Render(ctx.Response.BodyWriter(), reader(ctx), wrappers, leaf, options)
}

// RenderStream answers a navigation with a record stream instead of one
// buffered body, so each region applies as soon as it is written.
//
// The whole delta is rendered before the first record, so every failure this
// entry can have is still an ordinary error: nothing has been written when it
// returns one.
func (o Options) RenderStream(ctx *fasthttp.RequestCtx, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	if ctx == nil {
		return nil
	}
	core := o.core()
	plan, err := core.PlanBufferedStream(reader(ctx), wrappers, leaf, options)
	if err != nil {
		return err
	}
	if !plan.Streams() {
		return core.RenderDocument(ctx.Response.BodyWriter(), plan, wrappers, leaf)
	}
	ctx.SetBodyStreamWriter(func(bw *bufio.Writer) {
		err := core.RunBufferedStream(core.OpenStream(bw, plan), plan)
		if ferr := bw.Flush(); err == nil {
			err = ferr
		}
		if err != nil {
			updatecore.ReportStreamError(err)
		}
	})
	return nil
}

// RenderStreamAsync answers a navigation with a record stream that also carries
// await boundaries as they settle.
//
// Each region reaches the browser with its fallback in place and is replaced
// when its dependency finishes, so a slow one delays only itself.
//
// cctx is the caller's cancellation and must not be the RequestCtx; see
// [Options.RenderLiveStream] for what happens if it is.
func (o Options) RenderStreamAsync(cctx context.Context, ctx *fasthttp.RequestCtx, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	return o.renderStream(cctx, ctx, false, wrappers, leaf, options)
}

// RenderLiveStream answers a live request by holding the response open for as
// long as the composition's subscriptions live.
//
// cctx is the caller's cancellation. A *fasthttp.RequestCtx passed here is
// replaced, because it is pooled and this delivery outlives the handler that
// owned it; what replaces it carries the one signal a RequestCtx actually has,
// which is server shutdown.
//
// That is the honest difference from net/http and it is worth stating plainly:
// there, the request context is cancelled when the client disconnects, and the
// stream ends promptly. fasthttp has no per-request cancellation at all — its
// Done channel closes on server shutdown alone — so a client that went away is
// noticed when a record fails to write instead. A live stream therefore ends at
// its next delivery rather than at the disconnect, and a subscription that
// never delivers again holds its resources until the server stops. A caller
// that needs a bound should pass a context carrying one.
func (o Options) RenderLiveStream(cctx context.Context, ctx *fasthttp.RequestCtx, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options ...htmlbind.Option) error {
	return o.renderStream(cctx, ctx, true, wrappers, leaf, options)
}

func (o Options) renderStream(cctx context.Context, ctx *fasthttp.RequestCtx, serveLive bool, wrappers []htmlbind.Wrapper, leaf htmlbind.Fragment, options []htmlbind.Option) error {
	if ctx == nil {
		return nil
	}
	core := o.core()
	plan, err := core.PlanStream(reader(ctx), serveLive, wrappers, leaf, options)
	if err != nil {
		return err
	}
	if !plan.Streams() {
		return core.RenderDocument(ctx.Response.BodyWriter(), plan, wrappers, leaf)
	}
	cctx = detachRequestCtx(cctx, ctx)
	ctx.SetBodyStreamWriter(func(bw *bufio.Writer) {
		err := core.RunStream(cctx, core.OpenStream(bw, plan), plan, wrappers, leaf)
		if ferr := bw.Flush(); err == nil {
			err = ferr
		}
		if err != nil {
			updatecore.ReportStreamError(err)
		}
	})
	return nil
}

// detachRequestCtx replaces a RequestCtx used as a cancellation context with one
// that survives the handler.
//
// A handler rewritten from net/http source reaches this with the same
// identifier in both positions, because the transform collapses the writer and
// the request into one context and `r.Context()` rewrites to it. That is
// type-correct and unusable here: the value is pooled and reused for the next
// request, while the delivery this guards runs after the handler returned.
//
// The replacement is not a weaker version of the same thing. It carries the
// server's shutdown channel, which is the only signal a RequestCtx ever had —
// so nothing is lost by the substitution, and what a reader might have assumed
// was there was never there.
func detachRequestCtx(cctx context.Context, ctx *fasthttp.RequestCtx) context.Context {
	if cctx == nil {
		return shutdownContext(ctx)
	}
	if _, isRequest := cctx.(*fasthttp.RequestCtx); isRequest {
		return shutdownContext(ctx)
	}
	return cctx
}

// shutdownContext is cancelled when the server stops, and never per request.
//
// The channel is read out of the RequestCtx while the handler still owns it,
// and it belongs to the server rather than to the pooled request, so holding it
// past the handler is safe where holding the ctx is not.
func shutdownContext(ctx *fasthttp.RequestCtx) context.Context {
	done := ctx.Done()
	if done == nil {
		return context.Background()
	}
	derived, cancel := context.WithCancel(context.Background())
	go func() {
		<-done
		cancel()
	}()
	return derived
}

// SetStreamErrorHandler installs the destination for stream failures raised
// after the response committed. It is the same installation htmlupdate and the
// typed streams use, so one call covers every stream the module opens.
func SetStreamErrorHandler(fn func(error)) { updatecore.SetStreamErrorHandler(fn) }
