package htmlbind

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPendingSettlesOnceAndIsReadMany(t *testing.T) {
	var calls atomic.Int32
	value := Go(context.Background(), func(context.Context) (int, error) {
		calls.Add(1)
		return 7, nil
	})
	// Every reader observes the same settled result. A channel would have
	// handed the value to the first receiver and left the rest waiting, which
	// is the case a layout and the page inside it hit.
	var wg sync.WaitGroup
	results := make([]int, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := value.Wait(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = got
		}()
	}
	wg.Wait()
	for i, got := range results {
		if got != 7 {
			t.Fatalf("reader %d got %d, want 7", i, got)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("work ran %d times, want 1", got)
	}
}

func TestPendingZeroValueIsAbsentRatherThanBlocking(t *testing.T) {
	var unset Pending[string]
	if unset.IsSet() {
		t.Fatal("the zero handle reports itself as set")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Neither a panic nor a wait: the caller supplied nothing, and the
		// zero value of the field is exactly what absence looks like.
		got, err := unset.Wait(context.Background())
		if got != "" || err != nil {
			t.Errorf("unset handle returned %q, %v", got, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("waiting on an unset handle blocked")
	}
}

func TestPendingContextBoundsTheWaitNotTheWork(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})
	value := Go(context.Background(), func(context.Context) (int, error) {
		<-release
		close(finished)
		return 1, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := value.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait returned %v, want context.Canceled", err)
	}
	// The work is the caller's, so abandoning the wait does not stop it.
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("abandoned work was cancelled by the render")
	}
}

func TestPendingPrefersASettledResultOverACancelledContext(t *testing.T) {
	value := Resolved(3)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Both cases of the select are ready. Reporting the value it already has is
	// the only answer that does not vary from run to run.
	for i := 0; i < 50; i++ {
		got, err := value.Wait(ctx)
		if err != nil || got != 3 {
			t.Fatalf("settled handle returned %d, %v on attempt %d", got, err, i)
		}
	}
}

func TestPendingTurnsAPanicIntoTheError(t *testing.T) {
	if !panicRecovery {
		t.Log("recover does not run on this target, so a panicking worker ends the program instead")
		return
	}
	value := Go(context.Background(), func(context.Context) (int, error) {
		panic("nil map write")
	})
	_, err := value.Wait(context.Background())
	if err == nil {
		t.Fatal("a panicking worker settled without an error")
	}
	if normalizeAsyncError(err).Code != ErrorCodeInternal {
		t.Fatalf("panic was not normalized to the internal code: %v", err)
	}
}

func TestFailedHandleCarriesItsError(t *testing.T) {
	want := errors.New("upstream")
	if _, err := Failed[int](want).Wait(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Failed returned %v, want %v", err, want)
	}
}
