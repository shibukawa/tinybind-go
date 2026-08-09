package app

import (
	"context"
	"net/url"

	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// This file carries no build tag, so both halves of the split compile it.
//
// That works because a Registry and the Reloadable values in it are the same
// types whichever backend is built: Reloadable.Render takes a context rather
// than a request, so nothing here names a transport, and both runtimes alias one
// declaration rather than redeclaring it. Generated component registrations have
// the same property, which is what keeps them out of the tagged half.
func redrawRegistry() *htmlupdate.Registry {
	registry := &htmlupdate.Registry{}
	registry.MustRegister(htmlupdate.Reloadable{
		KindID: "Cart@v1",
		Render: func(_ context.Context, instanceID string, values url.Values) (htmlbind.Fragment, error) {
			return htmlbind.Fragment{}, nil
		},
	})
	return registry
}
