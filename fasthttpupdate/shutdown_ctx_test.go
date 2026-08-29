package fasthttpupdate

import (
	"context"
	"runtime"
	"testing"
)

// The channel-backed context has to behave like a context: not done while the
// channel is open, done with context.Canceled once it closes, and goroutine-free
// no matter how many are created.
func TestShutdownCtxSemantics(t *testing.T) {
	done := make(chan struct{})
	before := runtime.NumGoroutine()
	ctxs := make([]context.Context, 1000)
	for i := range ctxs {
		ctxs[i] = shutdownCtx{done: done}
	}
	if after := runtime.NumGoroutine(); after > before+5 {
		t.Fatalf("building 1000 shutdown contexts grew goroutines from %d to %d", before, after)
	}
	c := ctxs[0]
	if err := c.Err(); err != nil {
		t.Fatalf("open channel: Err = %v", err)
	}
	select {
	case <-c.Done():
		t.Fatal("Done fired while the channel was open")
	default:
	}
	if _, ok := c.Deadline(); ok {
		t.Fatal("a shutdown context has no deadline")
	}
	if v := c.Value("k"); v != nil {
		t.Fatalf("a shutdown context carries no values, got %v", v)
	}
	close(done)
	<-c.Done()
	if err := c.Err(); err != context.Canceled {
		t.Fatalf("closed channel: Err = %v, want context.Canceled", err)
	}
	// Deriving from it must still cancel through: this is how updatecore's own
	// WithCancel children learn about shutdown.
	derived, cancel := context.WithCancel(shutdownCtx{done: done})
	defer cancel()
	<-derived.Done()
}
