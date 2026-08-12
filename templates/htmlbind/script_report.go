package htmlbind

// ComponentScript is one component declaring a script block, reported so a
// caller can read the block without parsing the template itself.
//
// It is the seam GenerateOptions.ClientHandlers and
// GenerateOptions.ComponentParameters are answered from. The module reads no
// JavaScript: it reports the bytes it already holds, the caller interprets them,
// and the answer comes back as a compile option, which is the arrangement
// GenerateOptions.ServerActions already uses for a URL the compiler cannot
// compute.
type ComponentScript struct {
	// Component is the declaration name.
	Component string
	// Script is the block's content as authored, with no reinterpretation. It is
	// reported here rather than read from the extracted asset because that file
	// exists only after the compile that needs this answer, and because finding
	// the block in the template source means reimplementing the parser's own
	// raw-text boundary.
	Script string
	// Pos is the block's position, for a diagnostic the caller wants to anchor.
	Pos Position
	// Handlers are the client handler names this component's markup referenced,
	// deduplicated and in source order. A caller resolves these against what the
	// block exports and answers through GenerateOptions.ClientHandlers.
	Handlers []string
	// Parameters names the component's declared parameters, in declaration
	// order, so a caller choosing which of them to emit picks from the real set
	// rather than from what it believes the signature to be.
	Parameters []string
}

// ComponentScripts parses and analyzes a template module and returns every
// component declaring a script block.
//
// Like [ActionRefs] and [Signatures] it runs the same analysis [Generate] does,
// so a module that fails to compile fails here with the same diagnostic rather
// than yielding a partial answer.
//
// It is called before the caller has resolved anything, so no compile option is
// taken: a component with no entry in GenerateOptions.ClientHandlers is
// unchecked, which is what lets this pass run first.
func ComponentScripts(filename string, source []byte) ([]ComponentScript, error) {
	module, err := Parse(filename, source)
	if err != nil {
		return nil, err
	}
	compiler := newCompiler(filename, string(source), module, true)
	if err := compiler.analyze(); err != nil {
		return nil, err
	}
	return compiler.componentScripts(), nil
}

// componentScripts builds the report from what analysis already collected. The
// order follows the declarations, so the result is stable across runs for
// unchanged input.
func (c *compiler) componentScripts() []ComponentScript {
	handlers := map[string][]string{}
	seen := map[string]bool{}
	for _, ref := range c.clientHandlers {
		key := ref.Component + "\x00" + ref.Handler
		if seen[key] {
			continue
		}
		seen[key] = true
		handlers[ref.Component] = append(handlers[ref.Component], ref.Handler)
	}

	var out []ComponentScript
	for _, declaration := range c.module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		info := c.components[component.Name]
		// A component declaring no block is absent rather than present and empty,
		// so a caller loops over what it must read rather than filtering.
		if info == nil || info.script == "" {
			continue
		}
		entry := ComponentScript{
			Component: component.Name,
			Script:    info.script,
			Pos:       info.scriptPos,
			Handlers:  handlers[component.Name],
		}
		for _, param := range component.Parameters {
			entry.Parameters = append(entry.Parameters, param.Name)
		}
		out = append(out, entry)
	}
	return out
}
