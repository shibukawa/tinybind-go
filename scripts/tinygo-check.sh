#!/usr/bin/env bash
# Verify tinybind runtimes and generated code under TinyGo.
# Project baseline: TinyGo 0.41.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v tinygo >/dev/null; then
  echo "tinygo not installed" >&2
  exit 1
fi

echo "==> tinygo version"
tinygo version

TINYGO_VERSION="$(tinygo version | awk '{print $3}')"
GO_VERSION="$(go env GOVERSION)"
if [[ "$TINYGO_VERSION" != "0.41.1" ]]; then
  echo "expected TinyGo 0.41.1, got $TINYGO_VERSION" >&2
  exit 1
fi
if [[ "$GO_VERSION" != go1.26.* ]]; then
  echo "expected Go 1.26.x, got $GO_VERSION" >&2
  exit 1
fi
echo "validated toolchain: TinyGo $TINYGO_VERSION + $GO_VERSION"

echo "==> tinygo test (runtime + generated mapping)"
# mappingfixture also contains host-generator tests that invoke os/exec; those
# remain covered by go test and are intentionally excluded from TinyGo runtime.
tinygo test -run 'Test(Bind|Decode|Write|RoundTrip|GeneratedFile)' ./internal/mappingfixture
tinygo test ./internal/tinycheck .
# htmlbind is a rendering runtime with no net/http dependency, so it is a TinyGo
# target like the rest. It is listed separately because its await and live
# boundaries are the only place the package starts goroutines and selects on a
# context, which is where a scheduler difference would show up first.
tinygo test ./htmlbind
# The same suite again on wasm, whose cooperative scheduler is the case that
# differs most from the native one. Tests needing recover return early there:
# TinyGo's wasm targets do not run deferred recover at all, so a panicking
# external ends the program instead of becoming an AsyncError. That is a platform
# limitation rather than a project one, and htmlbind.panicRecovery names it.
# They log rather than call t.Skip, because TinyGo implements neither SkipNow nor
# the runtime.Goexit it needs and reports a skipped test as a failure.
tinygo test -target=wasip1 ./htmlbind

echo "==> tinygo run smoke (Bind/Write via generated code)"
tinygo run ./testdata/cmd/tinygo-bind-smoke

echo "==> tinygo run smoke (live boundary rendering, native and wasm)"
# The unit tests cover the runtime in isolation; this renders a live boundary the
# way a handler does, on both schedulers, so the goroutine and pull-sequence path
# is exercised end to end rather than only under the test harness.
tinygo run ./testdata/cmd/tinygo-htmlbind-smoke
tinygo run -target=wasip1 ./testdata/cmd/tinygo-htmlbind-smoke

echo "==> tinygo build -target wasm (JSON-only generated code)"
JSON_DEPS="$(go list -deps ./testdata/cmd/tinygo-json-smoke)"
if grep -Eq '^(net/http|database/sql|github.com/shibukawa/tinybind-go)$' <<<"$JSON_DEPS"; then
  echo "JSON-only dependency graph contains an HTTP, SQL, or root runtime" >&2
  exit 1
fi
tinygo build -target wasm -o /tmp/tinybind-json-smoke.wasm ./testdata/cmd/tinygo-json-smoke

echo "OK: TinyGo checks passed"
