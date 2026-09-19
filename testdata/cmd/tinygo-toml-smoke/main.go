// A TOML-only program: it imports tomlbind and nothing of configbind, so the
// dependency graph the TinyGo check reads carries no CLI, env or config-dir
// parser, and the generated decoder links for wasm and wasip1.
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/tomlbind"
)

type Listener struct {
	Addr string
	Port int `default:"80"`
}

type Settings struct {
	Name      string `default:"svc"`
	Ratio     float64
	Timeout   time.Duration `default:"5s"`
	Origins   []string
	Listeners []Listener
}

const doc = `
name = "edge"
ratio = 0.25
origins = ["https://a.example"]

[[listeners]]
addr = ":8443"
port = 8443
`

func main() {
	s, err := tomlbind.DecodeTOML[Settings](strings.NewReader(doc))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s %v %s %d %s:%d\n", s.Name, s.Ratio, s.Timeout, len(s.Origins), s.Listeners[0].Addr, s.Listeners[0].Port)
}
