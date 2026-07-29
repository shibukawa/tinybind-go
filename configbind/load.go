package configbind

import (
	"fmt"
	"os"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configpath"
	"github.com/shibukawa/tinybind-go/minitoml"
)

// LoadOptions configures multi-source Bind load.
type LoadOptions struct {
	// Vendor is the configdir vendor name (required when resolving via configdir).
	Vendor string
	// Tool is the application/tool name (required when resolving via configdir).
	Tool string
	// FileName is the config basename (default "config.toml").
	FileName string
	// Args are CLI args without the program name (default os.Args[1:]).
	Args []string
	// Environ is KEY=value lines (default os.Environ()).
	Environ []string
	// ExplicitConfigPath forces a config file path (overrides --config-path when set).
	// Prefer leaving empty and passing --config-path via Args in production.
	ExplicitConfigPath string
	// ExtraConfigReadPaths are optional config files searched in slice order
	// after ExplicitConfigPath/--config-path and before user/system config dirs.
	// Missing or unreadable entries are skipped; only the first found file is read.
	ExtraConfigReadPaths []string
}

// LoadResult holds the overlay after load (for tests/provenance).
type LoadResult struct {
	Overlay    *Overlay
	ConfigPath string
	FoundFile  bool
	// definitions keeps the bound definitions in Bind registration order so
	// Provenance can report keys in registration then declaration order even
	// after the process registry is reset.
	definitions []Definition
}

// Load merges default → TOML → env → CLI into Bind targets and applies without reflection.
func Load(opts LoadOptions) (*LoadResult, error) {
	ts := snapshotTargets()
	subcommands := snapshotSubcommandDefinitions()
	if len(ts) == 0 && len(subcommands) == 0 {
		return nil, fmt.Errorf("configbind: no Bind targets registered")
	}
	fileName := opts.FileName
	if fileName == "" {
		fileName = "config.toml"
	}
	args := opts.Args
	if args == nil {
		args = os.Args[1:]
	}
	if len(subcommands) > 0 && len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		return nil, &UsageError{Usage: topLevelUsage(subcommands)}
	}

	// Build flag defs: process --config-path + all Bind field flags.
	// Field defs also drive env var names (EnvName of each long option).
	defs := []cliparser.Def{configpath.ConfigPathDef()}
	var fieldDefs []cliparser.Def
	for _, t := range ts {
		if len(t.meta.FlagMetas) > 0 {
			fd, err := cliparser.BuildDefs(t.meta.FlagMetas)
			if err != nil {
				return nil, err
			}
			fieldDefs = append(fieldDefs, fd...)
			defs = append(defs, fd...)
		}
	}

	cliRes, err := cliparser.Parse(args, defs)
	if err != nil {
		if len(subcommands) > 0 {
			return nil, &UsageError{
				Message: fmt.Sprintf("configbind: cli: %v", err),
				Usage:   topLevelUsage(subcommands),
			}
		}
		return nil, fmt.Errorf("configbind: cli: %w", err)
	}

	var commandName string
	var commandArgs []string
	if len(cliRes.Rest) > 0 && len(subcommands) > 0 {
		commandName = cliRes.Rest[0]
		commandArgs = cliRes.Rest[1:]
		if _, ok := subcommands[commandName]; !ok {
			return nil, &UsageError{
				Message: fmt.Sprintf("configbind: unknown subcommand %q", commandName),
				Usage:   topLevelUsage(subcommands),
			}
		}
	}

	explicit := opts.ExplicitConfigPath
	if explicit == "" {
		explicit = configpath.ExplicitPathFromParse(cliRes)
	}

	var cfgPath string
	var found bool
	if len(ts) > 0 {
		cfgPath, found, err = configpath.ResolveWithExtras(
			opts.Vendor, opts.Tool, fileName, explicit, opts.ExtraConfigReadPaths,
		)
		if err != nil {
			return nil, err
		}
	}

	o := NewOverlay()

	// Defaults (lowest priority). KnownKeys drives the walk so the overlay is
	// filled in declaration order rather than Go map order.
	for _, t := range ts {
		for _, k := range t.meta.KnownKeys {
			if v, ok := t.meta.Defaults[k]; ok {
				o.Set(k, v, PlaceDefault)
			}
		}
	}

	// One environment for both the file layer's ${NAME} expansion and the env
	// layer below it.
	environ := environMap(opts.Environ)

	// TOML file.
	if found {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("configbind: read config %q: %w", cfgPath, err)
		}
		doc, err := minitoml.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("configbind: parse toml %q: %w", cfgPath, err)
		}
		if err := mergeDocument(o, doc, PlaceFile, environ, ""); err != nil {
			return nil, err
		}
	}

	// Env (names from CLI long options, e.g. opt port -> PORT).
	envMap := readEnvMap(fieldDefs, environ)
	o.MergeMap(envMap, PlaceEnv)

	// CLI (highest).
	o.MergeMap(cliRes.Values, PlaceCLI)
	if len(cliRes.Multi) > 0 {
		o.MergeMultiMap(cliRes.Multi, PlaceCLI)
	}
	// Process key must not be applied onto structs.
	o.Delete(configpath.ProcessKey)

	// A falsy choice fills in for an empty value, so an undeclared setting reads
	// as "off" rather than "". A default tag outranks it and is left alone.
	for _, t := range ts {
		for _, k := range t.meta.KnownKeys {
			falsy, ok := t.meta.Falsy[k]
			if !ok {
				continue
			}
			if _, hasDefault := t.meta.Defaults[k]; hasDefault {
				continue
			}
			entry, present := o.Get(k)
			switch {
			case !present:
				o.Set(k, falsy, PlaceDefault)
			case !entry.IsMulti && entry.Raw == "":
				o.Set(k, falsy, entry.Place)
			}
		}
	}

	// Apply to each target.
	for _, t := range ts {
		if err := t.meta.Apply(t.dst, o); err != nil {
			return nil, fmt.Errorf("configbind: apply %s: %w", t.typeName, err)
		}
	}
	if commandName != "" {
		if err := applySubcommand(commandName, commandArgs, subcommands[commandName]); err != nil {
			return nil, err
		}
	} else if len(ts) == 0 && len(subcommands) > 0 {
		return nil, &UsageError{
			Message: "configbind: a subcommand is required",
			Usage:   topLevelUsage(subcommands),
		}
	}

	bound := make([]Definition, 0, len(ts))
	for _, t := range ts {
		bound = append(bound, t.meta)
	}
	return &LoadResult{Overlay: o, ConfigPath: cfgPath, FoundFile: found, definitions: bound}, nil
}

func applySubcommand(name string, args []string, definition SubCommandDefinition) error {
	usage := subcommandUsage(definition)
	if wantsHelp(args) {
		return &UsageError{Usage: usage}
	}
	target, ok := selectedSubcommand(name)
	if !ok {
		return &UsageError{
			Message: fmt.Sprintf("configbind: subcommand %q was selected through LoadOptions.Args but SubCommand returned nil; keep LoadOptions.Args aligned with os.Args[1:]", name),
			Usage:   usage,
		}
	}
	if target.err != nil {
		return target.err
	}
	return applySubcommandValues(name, args, definition, target.dst)
}

func applySubcommandValues(name string, args []string, definition SubCommandDefinition, dst any) error {
	usage := subcommandUsage(definition)
	if wantsHelp(args) {
		return &UsageError{Usage: usage}
	}
	defs, err := cliparser.BuildDefs(definition.FlagMetas)
	if err != nil {
		return fmt.Errorf("configbind: subcommand %q: %w", name, err)
	}
	parsed, err := cliparser.ParseInterspersed(args, defs)
	if err != nil {
		return &UsageError{
			Message: fmt.Sprintf("configbind: subcommand %q: %v", name, err),
			Usage:   usage,
		}
	}

	values := NewOverlay()
	for key, value := range definition.Defaults {
		values.Set(key, value, PlaceDefault)
	}
	values.MergeMap(parsed.Values, PlaceCLI)
	values.MergeMultiMap(parsed.Multi, PlaceCLI)

	position := 0
	for _, positional := range definition.Positionals {
		switch positional.Role {
		case PositionalRequired:
			if position >= len(parsed.Rest) {
				return &UsageError{
					Message: fmt.Sprintf("configbind: subcommand %q: missing required argument <%s>", name, positional.Name),
					Usage:   usage,
				}
			}
			values.Set(positional.ConfigKey, parsed.Rest[position], PlaceCLI)
			position++
		case PositionalOptional:
			if position < len(parsed.Rest) {
				values.Set(positional.ConfigKey, parsed.Rest[position], PlaceCLI)
				position++
			}
		case PositionalRest:
			if position < len(parsed.Rest) {
				values.SetMulti(positional.ConfigKey, parsed.Rest[position:], PlaceCLI)
				position = len(parsed.Rest)
			}
		}
	}
	if position < len(parsed.Rest) {
		return &UsageError{
			Message: fmt.Sprintf("configbind: subcommand %q: unexpected argument %q", name, parsed.Rest[position]),
			Usage:   usage,
		}
	}
	if err := definition.Apply(dst, values); err != nil {
		return fmt.Errorf("configbind: apply subcommand %s: %w", definition.TypeName, err)
	}
	return nil
}

// mergeDocument merges a parsed document into the overlay, expanding ${NAME} in
// string values on the way. diagPrefix carries the path of the enclosing
// table-array element so an error inside [[db]] can name the element it came
// from; it is empty at the top level, where the document keys are already full.
func mergeDocument(o *Overlay, doc minitoml.Document, place Place, environ map[string]string, diagPrefix string) error {
	diagKey := func(key string) string {
		if diagPrefix == "" {
			return key
		}
		return diagPrefix + "." + key
	}
	for _, k := range doc.Keys() {
		v, ok := doc.Get(k)
		if !ok {
			continue
		}
		switch v.Kind {
		case minitoml.KindArray:
			sl, err := v.AsStringSlice()
			if err != nil {
				return err
			}
			// Only elements written as strings can carry a reference; a number
			// or bool has no ${} form to expand.
			for i := range sl {
				if v.Array[i].Kind != minitoml.KindString {
					continue
				}
				expanded, err := expandEnvRefs(sl[i], environ, diagKey(k))
				if err != nil {
					return err
				}
				sl[i] = expanded
			}
			o.SetMulti(k, sl, place)
		case minitoml.KindTableArray:
			// Each [[k]] element becomes its own overlay, keyed relative to k.
			tables := make([]*Overlay, 0, len(v.Tables))
			for i, table := range v.Tables {
				element := NewOverlay()
				elemPrefix := fmt.Sprintf("%s[%d]", diagKey(k), i)
				if err := mergeDocument(element, table, place, environ, elemPrefix); err != nil {
					return err
				}
				tables = append(tables, element)
			}
			o.SetTables(k, tables, place)
		default:
			s, err := v.AsString()
			if err != nil {
				return err
			}
			if v.Kind == minitoml.KindString {
				s, err = expandEnvRefs(s, environ, diagKey(k))
				if err != nil {
					return err
				}
			}
			o.Set(k, s, place)
		}
	}
	return nil
}
