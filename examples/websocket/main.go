// Command websocket serves a typed WebSocket chat room beside ordinary REST
// routes on one port, under both compilers.
//
//	go run ./examples/websocket
//	tinygo build -o wschat ./examples/websocket && ./wschat
//
// Then open http://localhost:8080/ in two tabs.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	httpbind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinygodriver/httpserver"

	// Registers the host Netdever for TinyGo's net package.
	_ "github.com/shibukawa/tinygodriver/netdev"
)

func logf(format string, args ...any) { log.Printf(format, args...) }

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// A socket failure raised after the 101 has no status left to carry it, so
	// it goes wherever the process says. Nothing is logged without this.
	httpbind.SetStreamErrorHandler(func(err error) {
		logf("socket: %v", err)
	})

	// The defaults are already sane; this endpoint only shortens them so an
	// abandoned tab is reclaimed quickly enough to watch.
	httpbind.SetSocketDefaults(httpbind.SocketOptions{
		IdleTimeout:  90 * time.Second,
		PingInterval: 30 * time.Second,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", chatHandler)
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /", indexHandler)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("listen:", err)
		os.Exit(1)
	}
	fmt.Printf("chat listening on http://localhost%s\n", addr)
	fmt.Printf("  health: http://localhost%s/healthz\n", addr)

	// The one line that is not ordinary net/http. Under host Go it is
	// srv.Serve(ln); under TinyGo it routes the upgrade around net/http's
	// background read, which would otherwise deadlock Hijack with no error and
	// no log line. Everything else on this mux still reaches a real
	// http.Server, so the socket costs the other routes nothing.
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := httpserver.Serve(ln, srv); err != nil {
		fmt.Println("serve:", err)
		os.Exit(1)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}

const page = `<!doctype html>
<meta charset="utf-8">
<title>tinybind websocket chat</title>
<style>
  body { font: 15px/1.5 system-ui, sans-serif; margin: 2rem auto; max-width: 40rem; }
  #log { border: 1px solid #ccc; padding: .5rem; height: 18rem; overflow-y: auto; }
  .presence { color: #666; font-style: italic; }
  .error { color: #b00; }
  form { display: flex; gap: .5rem; margin-top: .5rem; }
  input { flex: 1; padding: .4rem; }
</style>
<h1>chat</h1>
<div id="log"></div>
<form id="say"><input id="text" placeholder="message" autocomplete="off"><button>send</button></form>
<script>
const log = document.getElementById("log");
const add = (text, cls) => {
  const line = document.createElement("div");
  if (cls) line.className = cls;
  line.textContent = text;
  log.append(line);
  log.scrollTop = log.scrollHeight;
};

const name = prompt("your name") || "anon";
const ws = new WebSocket("ws://" + location.host + "/ws");

ws.onopen = () => ws.send(JSON.stringify({type: "join", name}));
ws.onclose = () => add("disconnected", "presence");
ws.onmessage = (e) => {
  const msg = JSON.parse(e.data);
  switch (msg.type) {
    case "welcome":  add("joined as " + msg.text + " (" + msg.live + " online)", "presence"); break;
    case "presence": add(msg.text + " (" + msg.live + " online)", "presence"); break;
    case "message":  add(msg.from + ": " + msg.text); break;
    case "error":    add("error: " + msg.code, "error"); break;
  }
};

document.getElementById("say").onsubmit = (e) => {
  e.preventDefault();
  const input = document.getElementById("text");
  if (!input.value) return;
  ws.send(JSON.stringify({type: "say", text: input.value}));
  input.value = "";
};
</script>
`
