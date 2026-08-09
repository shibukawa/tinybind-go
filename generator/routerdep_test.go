package generator

// The route-registration test compiles a testdata package that imports the
// forked router, and go mod tidy does not look inside testdata. Naming the
// dependency here is what keeps it in go.mod and go.sum.
//
// It is a test dependency only. Generated code names the router, and the
// application generating it records the requirement in its own module.
import _ "github.com/shibukawa/tinygodriver/fasthttprouter"
