package htmlbind

import (
	"sort"
	"strings"
)

// ContentHook compiles the body of a component script block whose lang
// attribute names a language this module does not know.
//
// It is the content-side counterpart of [ReferenceHook]: that one claims an
// attribute naming an authored file, and this one claims a block whose bytes
// are already in hand. The division is the same one
// decision:transform-seam-ownership draws for the other seam. This module
// decides which block is claimed, when the transform runs, what its identity is,
// and what regenerates it; the caller owns the compiler, the settings, and the
// name the output is written under.
//
// Registration is per generate command, so a project registering none
// regenerates byte-identical output and pays nothing. No compiler enters this
// module's dependencies, which is the rule concept:build-time-asset-transforms
// already states and requirement:browser-runtime-asset-ownership repeats.
type ContentHook struct {
	// Name identifies the hook in diagnostics.
	Name string
	// Lang is the exact lang attribute value this hook claims, such as ts.
	//
	// The set is open: this module validates that a written marker was
	// registered and never that it names a language it recognizes.
	Lang string
	// Extension is what the produced file is written as, without a dot. Empty
	// keeps js, which is what a block compiles to unless the caller says
	// otherwise.
	Extension string
	// Transform compiles one block's content.
	//
	// It is called once per distinct content in the template module being
	// compiled, and must be a pure function of what it reads plus its own
	// settings, exactly as [ReferenceHook.Transform] must be. The component,
	// file, and position on a request are for a diagnostic; deciding an output
	// from them breaks that contract.
	//
	// It compiles and does not bundle. decision:share-by-module-url records
	// why: an import specifier left alone is one URL in the browser's module
	// map and is evaluated once, so bundling is what would duplicate a shared
	// module rather than what avoids it.
	Transform func(ContentRequest) (ContentResult, error)
}

// ContentRequest is one claimed block handed to a transform.
type ContentRequest struct {
	// Hook is the name of the hook that claimed the block.
	Hook string
	// Lang is the marker the block wrote.
	Lang string
	// Content is the block's authored body, exactly as written.
	Content string
	// Component names the component that declared the block.
	Component string
	// Dir is the directory of the template that declared the block. It is what
	// a transform resolving an import specifier resolves against, because the
	// template's own location is the only one an author means.
	Dir string
	// File and Pos locate the block, for a diagnostic the transform returns.
	File string
	Pos  Position
}

// ContentResult is what a transform returns for one block.
type ContentResult struct {
	// Content replaces the block's body.
	Content string
	// Extension overrides the hook's, without a dot. Empty keeps it.
	Extension string
	// Read lists the files the transform read, so an edit to one regenerates.
	// What is named here is what stays honest across builds; what is left out
	// is not hashed and will serve a stale result.
	Read []string
}

// ValidateContentHooks reports a registration this module cannot act on. It is
// checked once per generate command so a mistake names whoever wrote the
// command rather than the first template that happens to write the marker.
func ValidateContentHooks(hooks []ContentHook) error {
	claimed := map[string]string{}
	for _, hook := range hooks {
		switch {
		case strings.TrimSpace(hook.Name) == "":
			return &CompileError{Message: "content hook has no name"}
		case strings.TrimSpace(hook.Lang) == "":
			return &CompileError{Message: "content hook " + hook.Name + " claims no lang"}
		case hook.Transform == nil:
			return &CompileError{Message: "content hook " + hook.Name + " has no transform"}
		case claimed[hook.Lang] != "":
			return &CompileError{Message: "content hooks " + claimed[hook.Lang] + " and " + hook.Name + " both claim lang " + hook.Lang}
		}
		claimed[hook.Lang] = hook.Name
	}
	return nil
}

// findContentHook returns the hook claiming a marker, or a diagnostic naming
// what is registered. An unregistered marker fails rather than passing through,
// because a block written as TypeScript and shipped uncompiled is a page that
// breaks in the browser with nothing in the build to point at.
func findContentHook(hooks []ContentHook, lang string) (ContentHook, error) {
	for _, hook := range hooks {
		if hook.Lang == lang {
			return hook, nil
		}
	}
	registered := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		registered = append(registered, hook.Lang)
	}
	sort.Strings(registered)
	message := "no content hook is registered for lang " + lang
	if len(registered) == 0 {
		message += "; this generate command registers none"
	} else {
		message += "; registered: " + strings.Join(registered, ", ")
	}
	return ContentHook{}, &CompileError{Message: message}
}
