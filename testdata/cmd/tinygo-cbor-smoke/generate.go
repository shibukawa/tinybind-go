//go:build ignore

// Command generate writes this package's CBOR codec.
//
// It calls the CBOR generator directly rather than going through
// tinybind-gen generate, which would work but would also write an OpenAPI
// artifact for a package that serves no HTTP. Nothing here asks for a binder.
package main

import (
	"log"

	"github.com/shibukawa/tinybind-go/generator"
)

func main() {
	g := &generator.Generator{Options: generator.DefaultOptions()}
	path, err := g.GenerateCborCodecs(".", "", "")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s", path)
}
