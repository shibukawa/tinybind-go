package htmlupdate_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestBrowserRuntimeProtocol drives the shipped runtime under node against a
// stubbed DOM. It covers the half a Go test otherwise cannot reach: header
// construction, version checking, validator bookkeeping, supersession, and the
// fallback that keeps a user action from being lost.
//
// The harness deliberately stubs DOM insertion rather than emulating it, so a
// pass here is not a claim that the markup lands correctly in a real browser.
func TestBrowserRuntimeProtocol(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	// Only the node subprocess reads these, so the Go test cache cannot see
	// them. Reading them here makes them cache inputs; without this an edited
	// runtime would keep reporting the previous run's result.
	for _, path := range []string{"runtime.js", "testdata/runtime_harness.js"} {
		if _, err := os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(node, "testdata/runtime_harness.js", "runtime.js")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime harness failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "ok") {
		t.Fatalf("unexpected harness output:\n%s", output)
	}
}
