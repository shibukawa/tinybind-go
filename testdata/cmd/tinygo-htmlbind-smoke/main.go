// Command tinygo-htmlbind-smoke renders a live boundary end to end, so the
// TinyGo check covers the one place htmlbind starts goroutines and selects on a
// context. A scheduler difference shows up here before anywhere else, and the
// wasm target's cooperative scheduler is the case that differs most.
package main

import (
	"bytes"
	"context"
	"fmt"
	"iter"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// ticks stands in for a live external: an ordinary function returning a
// sequence the runtime pulls.
func ticks(values ...string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for _, value := range values {
			if !yield(value, nil) {
				return
			}
		}
	}
}

// plan is the shape generation emits for {await point = Watch(id)}.
func plan() *htmlbind.Plan[struct{}] {
	return &htmlbind.Plan[struct{}]{
		HasAwaitBlock: true,
		HasLiveBlock:  true,
		Ops: []htmlbind.Op[struct{}]{
			htmlbind.Live(
				func(ctx context.Context, _ struct{}) []htmlbind.LiveBinding[string] {
					return []htmlbind.LiveBinding[string]{
						func(deliver func(func(*string), error) bool) error {
							for value, err := range ticks("10", "20", "30") {
								if !deliver(func(scope *string) { *scope = value }, err) {
									return nil
								}
							}
							return nil
						},
					}
				},
				func(struct{}) string { return "" },
				func(_ struct{}, err htmlbind.AsyncError) htmlbind.AsyncError { return err },
				[]htmlbind.Op[string]{htmlbind.Builder[string]{}.Text(func(v string) string { return v })},
				[]htmlbind.Op[struct{}]{htmlbind.Builder[struct{}]{}.Static("pending")},
				nil,
			),
		},
	}
}

func main() {
	fragment := htmlbind.Bind(plan(), struct{}{})

	// The synchronous entry: what a client with no JavaScript receives.
	var page bytes.Buffer
	if err := htmlbind.Render(&page, fragment); err != nil {
		panic(err)
	}
	fmt.Println("sync:", page.String())

	// The live entry: goroutines, a context, and a pull sequence.
	var document bytes.Buffer
	deliveries := 0
	for content, err := range htmlbind.RenderLive(context.Background(), &document, fragment) {
		if err != nil {
			panic(err)
		}
		deliveries++
		fmt.Println("delivery:", content.BoundaryID, string(content.HTML), string(content.AppendJSON(nil)))
	}
	if deliveries != 3 {
		panic(fmt.Sprintf("deliveries = %d, want 3", deliveries))
	}
	fmt.Println("fallback committed:", document.String())
}
