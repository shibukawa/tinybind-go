package generator_test

import (
	"path/filepath"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

const socketHandlerSource = `package sample

import (
	"net/http"

	"github.com/shibukawa/tinybind-go"
)

type ClientMsg struct{ Text string }

type ServerMsg struct{ Text string }

func chat(w http.ResponseWriter, r *http.Request) {
	_ = httpbind.WebSocket(w, r, func(s *httpbind.Socket[ClientMsg, ServerMsg]) error {
		in, err := s.Read()
		if err != nil {
			return err
		}
		return s.Write(ServerMsg{Text: in.Text})
	})
}

func register(mux *http.ServeMux) { mux.HandleFunc("GET /ws", chat) }
`

// A socket names two types running in opposite directions, so discovery has to
// give each one the codec it needs and not the other: a decoder that is never
// called is dead weight in a TinyGo binary, and a missing one is a connection
// that opens and then dies.
func TestSocketTypesGetOneCodecEachInTheRightDirection(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "main.go"), socketHandlerSource)
	tidyTempModule(t, dir)

	plan, err := generator.New(generator.DefaultOptions()).Analyze(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	assertTypeUsage(t, plan, "ClientMsg", generator.UsageDecodeJSON)
	assertTypeUsage(t, plan, "ServerMsg", generator.UsageEncodeJSON)
}

// The two directions are configured as two patterns against one target, and
// the feature flag has to reach both halves of that pair.
func TestDisablingTheWebSocketFeatureDropsBothDirections(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	writeTestFile(t, filepath.Join(dir, "main.go"), socketHandlerSource)
	tidyTempModule(t, dir)

	opts := generator.DefaultOptions()
	opts.DisableFeatures = append(opts.DisableFeatures, generator.FeatureWebSocket)
	plan, err := generator.New(opts).Analyze(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	assertTypeUsage(t, plan, "ClientMsg", 0)
	assertTypeUsage(t, plan, "ServerMsg", 0)
}
