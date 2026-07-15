package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/httpbind-go/parser"
)

func main() {
	root := "testdata"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	cases := []string{
		"basic_handlefunc",
		"stream_write",
		"nested_wrappers",
		"struct_handler",
		"inline_handler",
		"error_constructors",
		"ignore_http_notfound",
		"wrapper_timeout_bare",
		"unsupported_loop",
		"wrapper_strip_prefix",
		"custom_middleware",
	}
	fmt.Println("{")
	for i, c := range cases {
		res, err := parser.ParsePackage(filepath.Join(root, c))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c, err)
			os.Exit(1)
		}
		b, err := res.JSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "json: %v\n", err)
			os.Exit(1)
		}
		comma := ","
		if i == len(cases)-1 {
			comma = ""
		}
		fmt.Printf("  %q: %s%s\n", c, string(b), comma)
	}
	fmt.Println("}")
}
