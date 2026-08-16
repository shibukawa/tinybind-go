// Package id_ serves one record with no typed entry point.
//
// It is the rung 1 shape doing what rung 2 was for: the component takes the
// path parameter, loads its own data through an external, and the loader's
// error chooses the response. There is no func Load here, and nothing is
// threaded from a Go function into the component's parameters.
package id_

import (
	"context"

	httpbind "github.com/shibukawa/tinybind-go"
)

// LoadRecord answers for one id. Record is declared in the template and emitted
// beside this file, so the external's result type is the generated one.
//
// It declares both things an implementation may ask for — a leading context and
// a trailing error — so the generated call is the variant that carries each. The
// error is what lets a template's own loader choose the response: httpbind
// .NotFound here becomes a 404 with nothing written, because a leaf's leading
// bindings run during assembly.
func LoadRecord(ctx context.Context, id string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	if id == "missing" {
		return Record{}, httpbind.NotFound(httpbind.Problem{
			Code:    "no_such_record",
			Message: "no record " + id,
		})
	}
	if id == "moved" {
		return Record{}, httpbind.Redirect("/records/here")
	}
	return Record{Title: id, Summary: "summary of " + id}, nil
}
