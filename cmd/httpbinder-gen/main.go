package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shibukawa/httpbind-go/generator"
)

func main() {
	dir := flag.String("dir", ".", "package directory to analyze")
	out := flag.String("out", "", "output directory (default: same as -dir)")
	name := flag.String("name", "httpbinder_gen.go", "binder/writer output file name")
	openapi := flag.Bool("openapi", true, "also generate OpenAPI embed (httpbinder_openapi_gen.go)")
	openapiName := flag.String("openapi-name", "httpbinder_openapi_gen.go", "OpenAPI output file name")
	flag.Parse()

	path, err := generator.Generate(*dir, *out, *name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "httpbinder-gen: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(path)

	if *openapi {
		op, err := generator.GenerateOpenAPI(*dir, *out, *openapiName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "httpbinder-gen openapi: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(op)
	}
}
