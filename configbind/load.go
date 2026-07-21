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
}

// LoadResult holds the overlay after load (for tests/provenance).
type LoadResult struct {
	Overlay    *Overlay
	ConfigPath string
	FoundFile  bool
}

// Load merges default → TOML → env → CLI into Bind targets and applies without reflection.
func Load(opts LoadOptions) (*LoadResult, error) {
	ts := snapshotTargets()
	if len(ts) == 0 {
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
		return nil, fmt.Errorf("configbind: cli: %w", err)
	}

	explicit := opts.ExplicitConfigPath
	if explicit == "" {
		explicit = configpath.ExplicitPathFromParse(cliRes)
	}

	cfgPath, found, err := configpath.Resolve(opts.Vendor, opts.Tool, fileName, explicit)
	if err != nil {
		return nil, err
	}

	o := NewOverlay()

	// Defaults (lowest priority).
	for _, t := range ts {
		for k, v := range t.meta.Defaults {
			o.Set(k, v, PlaceDefault)
		}
	}

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
		if err := mergeDocument(o, doc, PlaceFile); err != nil {
			return nil, err
		}
	}

	// Env (names from CLI long options, e.g. opt port -> PORT).
	envMap := ReadEnv(fieldDefs, opts.Environ)
	o.MergeMap(envMap, PlaceEnv)

	// CLI (highest).
	o.MergeMap(cliRes.Values, PlaceCLI)
	if len(cliRes.Multi) > 0 {
		o.MergeMultiMap(cliRes.Multi, PlaceCLI)
	}
	// Process key must not be applied onto structs.
	o.Delete(configpath.ProcessKey)

	// Apply to each target.
	for _, t := range ts {
		if err := t.meta.Apply(t.dst, o); err != nil {
			return nil, fmt.Errorf("configbind: apply %s: %w", t.typeName, err)
		}
	}

	return &LoadResult{Overlay: o, ConfigPath: cfgPath, FoundFile: found}, nil
}

func mergeDocument(o *Overlay, doc minitoml.Document, place Place) error {
	for _, k := range doc.Keys() {
		v, ok := doc.Get(k)
		if !ok {
			continue
		}
		if v.Kind == minitoml.KindArray {
			sl, err := v.AsStringSlice()
			if err != nil {
				return err
			}
			o.SetMulti(k, sl, place)
			continue
		}
		s, err := v.AsString()
		if err != nil {
			return err
		}
		o.Set(k, s, place)
	}
	return nil
}
