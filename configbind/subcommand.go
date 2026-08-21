package configbind

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configpath"
)

// PositionalRole describes how a generated subcommand field consumes
// positional command-line arguments.
type PositionalRole uint8

const (
	PositionalRequired PositionalRole = iota + 1
	PositionalOptional
	PositionalRest
)

// Positional defines one generated positional argument.
type Positional struct {
	ConfigKey string
	Name      string
	Help      string
	Role      PositionalRole
	// Enum is the allowlist the argument accepts, already split and trimmed at
	// generation time, and listed in the usage text beside Help.
	Enum []string
}

// SubCommandDefinition describes one generated CLI-only subcommand.
type SubCommandDefinition struct {
	TypeName    string
	Name        string
	Help        string
	FlagMetas   []cliparser.FieldMeta
	Defaults    map[string]string
	Positionals []Positional
	Apply       ApplyFunc
}

// UsageError reports a CLI parse failure together with generated usage text.
type UsageError struct {
	Message string
	Usage   string
}

func (e *UsageError) Error() string {
	if e.Message == "" {
		return e.Usage
	}
	return e.Message + "\n\n" + e.Usage
}

type subcommandKey struct {
	typeID any
	name   string
}

type subcommandTarget struct {
	name string
	dst  any
	meta SubCommandDefinition
	err  error
}

var (
	subcommandsMu         sync.RWMutex
	subcommandDefinitions = map[subcommandKey]SubCommandDefinition{}
	subcommandNames       = map[string]subcommandKey{}
	selectedSubcommands   = map[string]subcommandTarget{}
)

// RegisterSubCommand installs one generated subcommand definition.
func RegisterSubCommand[T any](definition SubCommandDefinition) {
	if definition.TypeName == "" || definition.Name == "" || definition.Help == "" || definition.Apply == nil {
		panic("configbind: RegisterSubCommand requires TypeName, Name, Help, and Apply")
	}
	if strings.ContainsAny(definition.Name, " \t\r\n") || strings.HasPrefix(definition.Name, "-") {
		panic(fmt.Sprintf("configbind: invalid subcommand name %q", definition.Name))
	}
	definition.FlagMetas = append([]cliparser.FieldMeta(nil), definition.FlagMetas...)
	definition.Positionals = append([]Positional(nil), definition.Positionals...)
	definition.Defaults = cloneStrings(definition.Defaults)
	if _, err := cliparser.BuildDefs(definition.FlagMetas); err != nil {
		panic(fmt.Sprintf("configbind: subcommand %q: %v", definition.Name, err))
	}
	if err := validatePositionals(definition.Positionals); err != nil {
		panic(fmt.Sprintf("configbind: subcommand %q: %v", definition.Name, err))
	}

	subcommandsMu.Lock()
	defer subcommandsMu.Unlock()
	key := subcommandKey{typeID: typeKey[T](), name: definition.Name}
	if _, exists := subcommandDefinitions[key]; exists {
		panic(fmt.Sprintf("configbind: duplicate definition for subcommand %q", definition.Name))
	}
	if previous, exists := subcommandNames[definition.Name]; exists && previous != key {
		panic(fmt.Sprintf("configbind: duplicate subcommand name %q", definition.Name))
	}
	subcommandDefinitions[key] = definition
	subcommandNames[definition.Name] = key
}

// SubCommand returns a generated *T only when name is the process-selected
// subcommand. Load fills the selected value from CLI flags and positionals.
//
// Selection uses os.Args because the nil/non-nil result must be known when this
// function returns. LoadOptions.Args should therefore mirror os.Args[1:] when
// an application uses SubCommand.
func SubCommand[T any](name, help string) *T {
	subcommandsMu.RLock()
	meta, ok := subcommandDefinitions[subcommandKey{typeID: typeKey[T](), name: name}]
	subcommandsMu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("configbind: subcommand type/name not registered; run go generate (SubCommand[%T](%q, ...))", *new(T), name))
	}
	if help != meta.Help {
		panic(fmt.Sprintf("configbind: subcommand %q help changed; run go generate", name))
	}
	selectedName, args := processSubcommand()
	if selectedName != name {
		return nil
	}

	dst := new(T)
	parseErr := applySubcommandValues(name, args, meta, dst)
	subcommandsMu.Lock()
	if _, exists := selectedSubcommands[name]; exists {
		subcommandsMu.Unlock()
		panic(fmt.Sprintf("configbind: subcommand %q registered more than once", name))
	}
	selectedSubcommands[name] = subcommandTarget{name: name, dst: dst, meta: meta, err: parseErr}
	subcommandsMu.Unlock()
	if parseErr != nil {
		return nil
	}
	return dst
}

func processSubcommand() (string, []string) {
	if len(os.Args) == 0 {
		// No argv at all, which is what a wasm component is started with. An
		// empty command line selects no subcommand, and that is the answer
		// rather than a panic.
		return "", nil
	}
	args := os.Args[1:]
	defs := []cliparser.Def{configpath.ConfigPathDef()}
	var fields []cliparser.FieldMeta
	for _, definition := range snapshotDefinitions() {
		fields = append(fields, definition.FlagMetas...)
	}
	fieldDefs, err := cliparser.BuildDefs(fields)
	if err != nil {
		return "", nil
	}
	defs = append(defs, fieldDefs...)
	result, err := cliparser.Parse(args, defs)
	if err != nil || len(result.Rest) == 0 {
		return "", nil
	}
	return result.Rest[0], result.Rest[1:]
}

func snapshotSubcommandDefinitions() map[string]SubCommandDefinition {
	subcommandsMu.RLock()
	defer subcommandsMu.RUnlock()
	out := make(map[string]SubCommandDefinition, len(subcommandNames))
	for name, key := range subcommandNames {
		out[name] = subcommandDefinitions[key]
	}
	return out
}

func selectedSubcommand(name string) (subcommandTarget, bool) {
	subcommandsMu.RLock()
	defer subcommandsMu.RUnlock()
	target, ok := selectedSubcommands[name]
	return target, ok
}

func resetSubcommandDefinitions() {
	subcommandsMu.Lock()
	subcommandDefinitions = map[subcommandKey]SubCommandDefinition{}
	subcommandNames = map[string]subcommandKey{}
	selectedSubcommands = map[string]subcommandTarget{}
	subcommandsMu.Unlock()
}

func resetSubcommandTargets() {
	subcommandsMu.Lock()
	selectedSubcommands = map[string]subcommandTarget{}
	subcommandsMu.Unlock()
}

func validatePositionals(positionals []Positional) error {
	optionalSeen := false
	restSeen := false
	keys := make(map[string]bool, len(positionals))
	for i, positional := range positionals {
		if positional.ConfigKey == "" || positional.Name == "" {
			return fmt.Errorf("positional argument requires ConfigKey and Name")
		}
		if keys[positional.ConfigKey] {
			return fmt.Errorf("duplicate positional key %q", positional.ConfigKey)
		}
		keys[positional.ConfigKey] = true
		switch positional.Role {
		case PositionalRequired:
			if optionalSeen || restSeen {
				return fmt.Errorf("required positional %q must precede optional and rest arguments", positional.Name)
			}
		case PositionalOptional:
			if restSeen {
				return fmt.Errorf("optional positional %q must precede the rest argument", positional.Name)
			}
			optionalSeen = true
		case PositionalRest:
			if restSeen || i != len(positionals)-1 {
				return fmt.Errorf("rest positional %q must be unique and last", positional.Name)
			}
			restSeen = true
		default:
			return fmt.Errorf("positional %q has invalid role", positional.Name)
		}
	}
	return nil
}

func cloneStrings(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func commandNames(definitions map[string]SubCommandDefinition) []string {
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

func topLevelUsage(definitions map[string]SubCommandDefinition) string {
	var b strings.Builder
	program := "app"
	if len(os.Args) > 0 && os.Args[0] != "" {
		program = os.Args[0]
	}
	fmt.Fprintf(&b, "Usage: %s [global options] <command> [options] [arguments]\n\nCommands:\n", program)
	for _, name := range commandNames(definitions) {
		fmt.Fprintf(&b, "  %-16s %s\n", name, definitions[name].Help)
	}
	return strings.TrimRight(b.String(), "\n")
}

func subcommandUsage(definition SubCommandDefinition) string {
	var b strings.Builder
	program := "app"
	if len(os.Args) > 0 && os.Args[0] != "" {
		program = os.Args[0]
	}
	fmt.Fprintf(&b, "Usage: %s %s", program, definition.Name)
	if len(definition.FlagMetas) > 0 {
		b.WriteString(" [options]")
	}
	for _, positional := range definition.Positionals {
		switch positional.Role {
		case PositionalRequired:
			fmt.Fprintf(&b, " <%s>", positional.Name)
		case PositionalOptional:
			fmt.Fprintf(&b, " [%s]", positional.Name)
		case PositionalRest:
			fmt.Fprintf(&b, " [%s...]", positional.Name)
		}
	}
	b.WriteString("\n")
	if definition.Help != "" {
		fmt.Fprintf(&b, "\n%s\n", definition.Help)
	}
	defs, _ := cliparser.BuildDefs(definition.FlagMetas)
	if len(defs) > 0 {
		b.WriteString("\nOptions:\n")
		for _, def := range defs {
			names := make([]string, 0, len(def.Longs)+len(def.Shorts))
			for _, long := range def.Longs {
				names = append(names, "--"+long)
			}
			for _, short := range def.Shorts {
				names = append(names, "-"+short)
			}
			label := strings.Join(names, ", ")
			if def.Kind != cliparser.KindBool {
				label += " <value>"
			}
			fmt.Fprintf(&b, "  %-24s %s\n", label, withEnumNote(def.Help, def.Enum))
		}
	}
	if len(definition.Positionals) > 0 {
		b.WriteString("\nArguments:\n")
		for _, positional := range definition.Positionals {
			fmt.Fprintf(&b, "  %-24s %s\n", positional.Name, withEnumNote(positional.Help, positional.Enum))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// withEnumNote appends a field's allowlist to its usage text. A field with no
// help of its own still lists its choices: they are the more useful half.
func withEnumNote(help string, enum []string) string {
	if len(enum) == 0 {
		return help
	}
	if help == "" {
		return enumNote(enum)
	}
	return help + " (" + enumNote(enum) + ")"
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
		if arg == "--" {
			return false
		}
	}
	return false
}
