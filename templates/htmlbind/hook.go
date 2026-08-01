package htmlbind

import (
	"bytes"
	"encoding/json"
	"errors"
	"path"
	"sort"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// A reference hook rewrites the static value of one attribute on one standard
// HTML element, at generation time, using a transform the caller supplies.
//
// A transform converts and returns the bytes, so it may decide the rewrite from
// what the conversion produced: an encode that came out larger than its source
// is worth declining, and only the converted bytes can say so. That check is
// why the conversion happens here rather than in a later phase.
//
// Converting is seconds of CPU and compiling templates is sequential, so the
// same conversion must not run twice. Two things keep that true. Within a run,
// a transform is called once per distinct value. Across runs, a hook that
// implements CacheKey lets the caller store the whole result - the rewritten
// value, the produced files, and the decision to decline - and reuse it while
// the inputs it named are unchanged. An unchanged asset then costs a digest,
// not an encode, and a source that once lost the size comparison is never
// re-encoded to rediscover it.

// ReferenceHook matches one element and attribute pair and rewrites the static
// values written there.
//
// Registration is per generate command, so a project registering none
// regenerates byte-identical output and pays nothing.
type ReferenceHook struct {
	// Name identifies the hook in diagnostics and in the rewrite report.
	Name string
	// Element is the exact lowercase element name, such as img. A hyphenated
	// name is out of scope: that space belongs to the builtin element
	// whitelist.
	Element string
	// Attribute is the exact attribute name, such as src.
	Attribute string
	// Match reports whether this hook claims a static value. A nil Match claims
	// every static value written at the pair.
	Match func(value string) bool
	// CacheKey names what a conversion of this value depends on, cheaply and
	// without converting anything. A caller holding a store of previous results
	// uses it to answer without calling Transform at all.
	//
	// A nil CacheKey means every build converts, which is correct and slow.
	//
	// The key is only as honest as what it names: an encoder upgrade that no
	// Params string mentions serves stale bytes, and that is the caller's to
	// state because only the caller knows what its converter depends on.
	CacheKey func(ReferenceRequest) (ConversionInputs, error)
	// Transform converts one claimed value and returns both the rewrite and the
	// files it produced, so the rewrite may depend on how the conversion turned
	// out.
	//
	// It is called once per distinct value in the template module being
	// compiled, so a file referenced twenty times on one page is converted once.
	// One module is the widest scope this package has, since it compiles them
	// one at a time; a caller compiling several wraps the transform in its own
	// memo, which is what the generator does.
	//
	// It must be a pure function of what it reads plus its own settings. The
	// file and position on a request are for a diagnostic; deciding an output
	// from them breaks that contract, and any cache built on CacheKey with it.
	Transform func(ReferenceRequest) (ReferenceResult, error)
}

// ConversionInputs is everything one conversion's output depends on, named
// without doing the conversion.
type ConversionInputs struct {
	// Sources are the authored files the conversion reads. Their contents are
	// digested into the cache key, and they are recorded as build inputs, so an
	// edit to one both invalidates the cached result and regenerates.
	Sources []string
	// Params is everything else the output depends on: the target format, the
	// quality setting, the version of the encoder doing the work. Anything left
	// out here is something a cache hit will silently ignore.
	Params string
}

// MarshalJSON gives a hook a stable identity for a caller that hashes its
// options to decide whether a run can be skipped.
//
// A func value cannot be marshalled at all, so without this the whole options
// value becomes unhashable and registering one hook would silently turn the
// incremental skip off. What is emitted is the registration, not the behavior:
// a transform's behavior is covered by the hash of the generator executable that
// contains it, so adding, removing, or repointing a hook regenerates and
// recompiling the command does too.
func (h ReferenceHook) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name      string `json:"name"`
		Element   string `json:"element"`
		Attribute string `json:"attribute"`
		Matched   bool   `json:"matched"`
	}{h.Name, h.Element, h.Attribute, h.Match != nil})
}

// ReferenceRequest is one claimed attribute occurrence handed to a transform.
type ReferenceRequest struct {
	// Hook is the name of the hook that claimed the value.
	Hook string
	// Element and Attribute are the pair that matched.
	Element   string
	Attribute string
	// Value is the attribute value exactly as the template writes it.
	Value string
	// File and Pos locate the first occurrence, for a diagnostic the transform
	// returns.
	File string
	Pos  Position
}

// ReferenceResult is what a transform returns for one value.
//
// Skip and Value are the two outcomes: a skip leaves the markup alone and says
// why, which is how a transform declines a conversion that was not worth it -
// an encode larger than its source, a format already at the target, a vector
// image. Declining is not an error and is not a silent no-op, and it is cached
// like any other outcome, so the losing encode runs once and never again.
type ReferenceResult struct {
	// Value replaces the attribute value. It is ignored when Skip is set.
	Value string
	// Skip leaves the attribute exactly as written.
	Skip bool
	// Reason explains a skip in the rewrite report.
	Reason string
	// Files are the files this conversion produced. They may outnumber the
	// rewrite: a source map is produced and no attribute names it.
	Files []ProducedFile
	// Read lists files the transform read beyond the sources its CacheKey
	// named, such as the modules a TypeScript entry point imports. What is
	// named by neither is not hashed, and an edit to it will not regenerate.
	Read []string
}

// ProducedFile is one file a conversion produced, ready for the caller to
// write.
type ProducedFile struct {
	// Name is the file name relative to the output root. It may carry directory
	// separators and may not escape the root.
	Name string
	// MediaType is what the file should be served as, or empty.
	MediaType string
	Content   []byte
}

// Rewrite records one distinct value a hook handled, for the build report. An
// author cannot see a build-time rewrite by reading the template, so the build
// is the only place it is visible.
type Rewrite struct {
	Hook      string
	Element   string
	Attribute string
	// From is the authored value and To is what replaced it. To equals From for
	// a skip.
	From string
	To   string
	// Occurrences counts the attributes this one conversion rewrote. A transform
	// call count is a count of distinct values, not of elements.
	Occurrences int
	// Skipped and Reason report a declined rewrite.
	Skipped bool
	Reason  string
	// Pos is the first occurrence in the template.
	Pos Position
}

// DynamicReference is an attribute a hook is registered for whose value is a
// template expression, and therefore does not exist at generation time.
//
// It is reported rather than ignored: a page half rewritten with nothing said
// about it is the failure mode this seam exists to avoid. Set
// StrictReferenceHooks to make it a compile error instead.
type DynamicReference struct {
	Hook      string
	Element   string
	Attribute string
	Pos       Position
}

// ValidateReferenceHooks checks a registration set before any template is read,
// so a malformed hook fails at the generate command rather than at a template
// position it has nothing to do with.
//
// Two hooks may share an element and attribute pair; whether they collide
// depends on the values a template writes, so that is checked at use.
func ValidateReferenceHooks(hooks []ReferenceHook) error {
	names := map[string]bool{}
	for _, hook := range hooks {
		switch {
		case hook.Name == "":
			return &HookError{Message: "reference hook has no name"}
		case names[hook.Name]:
			return &HookError{Hook: hook.Name, Message: "duplicate reference hook name"}
		case hook.Element == "":
			return &HookError{Hook: hook.Name, Message: "reference hook declares no element"}
		case hook.Attribute == "":
			return &HookError{Hook: hook.Name, Message: "reference hook declares no attribute"}
		case hook.Transform == nil:
			return &HookError{Hook: hook.Name, Message: "reference hook declares no transform"}
		case !validElementName(hook.Element):
			return &HookError{Hook: hook.Name, Message: "invalid element name " + quoteName(hook.Element) + "; it must be a lowercase HTML element name"}
		case strings.Contains(hook.Element, "-"):
			return &HookError{Hook: hook.Name, Message: "element " + quoteName(hook.Element) +
				" is hyphenated; that space belongs to the builtin element whitelist, not to a reference hook"}
		case !validAttributeName(hook.Attribute):
			return &HookError{Hook: hook.Name, Message: "invalid attribute name " + quoteName(hook.Attribute)}
		}
		names[hook.Name] = true
	}
	return nil
}

// HookError is a registration failure, which has no template position because
// nothing has been read yet.
type HookError struct {
	Hook    string
	Message string
}

func (e *HookError) Error() string {
	if e.Hook == "" {
		return e.Message
	}
	return "reference hook " + quoteName(e.Hook) + ": " + e.Message
}

func validElementName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		case r == '-' && i > 0:
		default:
			return false
		}
	}
	return true
}

func validAttributeName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == ':':
		default:
			return false
		}
	}
	return true
}

// CleanOutputName normalizes a produced file name and refuses one that leaves
// the output root, because a caller writes these files unexamined.
func CleanOutputName(name string) (string, error) {
	switch {
	case name == "":
		return "", errors.New("a produced file has no name")
	case strings.ContainsRune(name, '\\'):
		return "", errors.New("produced file name " + quoteName(name) + " contains a backslash")
	case path.IsAbs(name):
		return "", errors.New("produced file name " + quoteName(name) + " is absolute")
	}
	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("produced file name " + quoteName(name) + " reaches above the output root")
	}
	return cleaned, nil
}

// hookOutcome is everything one rewrite pass produced.
type hookOutcome struct {
	rewrites []Rewrite
	dynamic  []DynamicReference
	produced []ProducedFile
	read     []string
}

// hookRun is the state of one pass over one template module.
type hookRun struct {
	filename string
	hooks    []ReferenceHook
	strict   bool
	// converted memoizes one transform call per hook and distinct value.
	converted map[string]*hookResult
	// order keeps the report in first-occurrence order, which keeps output
	// deterministic and independent of map iteration.
	order    []*hookResult
	dynamic  []DynamicReference
	produced map[string]ProducedFile
	read     map[string]bool
}

type hookResult struct {
	hook   ReferenceHook
	from   string
	to     string
	skip   bool
	reason string
	pos    Position
	count  int
}

// applyReferenceHooks rewrites every claimed static attribute in the module and
// collects the files those conversions produced.
//
// It runs on the parsed module before analysis, so a rewritten value is checked
// and folded exactly as an authored one, and before asset extraction decides an
// external script or link is a passthrough, so extraction sees the rewritten
// URL.
func applyReferenceHooks(filename string, module *Module, hooks []ReferenceHook, strict bool) (hookOutcome, error) {
	if len(hooks) == 0 {
		return hookOutcome{}, nil
	}
	if err := ValidateReferenceHooks(hooks); err != nil {
		return hookOutcome{}, err
	}
	run := &hookRun{
		filename:  filename,
		hooks:     hooks,
		strict:    strict,
		converted: map[string]*hookResult{},
		produced:  map[string]ProducedFile{},
		read:      map[string]bool{},
	}
	for _, declaration := range module.Declarations {
		component, ok := declaration.(*TemplateDecl)
		if !ok {
			continue
		}
		body, ok := component.Body.([]syntax.Node)
		if !ok {
			continue
		}
		if err := run.walk(body, false); err != nil {
			return hookOutcome{}, err
		}
	}
	return run.outcome(), nil
}

func (r *hookRun) outcome() hookOutcome {
	out := hookOutcome{dynamic: r.dynamic}
	for _, result := range r.order {
		out.rewrites = append(out.rewrites, Rewrite{
			Hook: result.hook.Name, Element: result.hook.Element, Attribute: result.hook.Attribute,
			From: result.from, To: result.to, Occurrences: result.count,
			Skipped: result.skip, Reason: result.reason, Pos: result.pos,
		})
	}
	names := make([]string, 0, len(r.produced))
	for name := range r.produced {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out.produced = append(out.produced, r.produced[name])
	}
	files := make([]string, 0, len(r.read))
	for file := range r.read {
		files = append(files, file)
	}
	sort.Strings(files)
	out.read = files
	return out
}

// foreignContentElements open a subtree whose hyphenated and camel-cased names
// are standard foreign-namespace names rather than HTML. A hook never reaches
// inside one.
var foreignContentElements = map[string]bool{"svg": true, "math": true}

func (r *hookRun) walk(nodes []Node, foreign bool) error {
	for _, node := range nodes {
		switch node := node.(type) {
		case *ElementNode:
			inner := foreign || foreignContentElements[node.Name]
			if !inner {
				if err := r.visitElement(node); err != nil {
					return err
				}
			}
			if err := r.walk(node.Children, inner); err != nil {
				return err
			}
		case *HeadNode:
			// A head declaration is where an entry point is ordinarily named,
			// so a seam that skipped it would miss the case it exists for.
			if err := r.walk(node.Children, foreign); err != nil {
				return err
			}
		case *SlotNode:
			if err := r.walk(node.Default, foreign); err != nil {
				return err
			}
		case *ComponentNode:
			if err := r.walk(node.Children, foreign); err != nil {
				return err
			}
		case *syntax.IfNode:
			if err := r.walk(node.Then, foreign); err != nil {
				return err
			}
			if err := r.walk(node.Else, foreign); err != nil {
				return err
			}
		case *syntax.ForNode:
			if err := r.walk(node.Body, foreign); err != nil {
				return err
			}
		case *syntax.AwaitNode:
			if err := r.walk(node.Primary, foreign); err != nil {
				return err
			}
			if err := r.walk(node.Fallback, foreign); err != nil {
				return err
			}
			if err := r.walk(node.Recover, foreign); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *hookRun) visitElement(element *ElementNode) error {
	for i := range element.Attributes {
		attribute := &element.Attributes[i]
		registered := r.registeredFor(element.Name, attribute.Name)
		if len(registered) == 0 {
			continue
		}
		value, static := staticAttributeText(*attribute)
		if !static {
			// The value does not exist yet, so nothing can be rewritten. Saying
			// so is the point: a silently half-optimized page is worse than a
			// noisy one.
			for _, hook := range registered {
				if r.strict {
					return r.error(attribute.Pos, "reference hook "+quoteName(hook.Name)+" is registered for "+
						element.Name+" "+attribute.Name+", and this value is an expression, so it cannot be rewritten at build time")
				}
				r.dynamic = append(r.dynamic, DynamicReference{
					Hook: hook.Name, Element: element.Name, Attribute: attribute.Name, Pos: attribute.Pos,
				})
			}
			continue
		}
		if err := r.rewrite(element, attribute, registered, value); err != nil {
			return err
		}
	}
	return nil
}

// registeredFor collects every hook watching one element and attribute pair.
func (r *hookRun) registeredFor(element, attribute string) []ReferenceHook {
	var hooks []ReferenceHook
	for _, hook := range r.hooks {
		if hook.Element == element && hook.Attribute == attribute {
			hooks = append(hooks, hook)
		}
	}
	return hooks
}

func (r *hookRun) rewrite(element *ElementNode, attribute *Attribute, registered []ReferenceHook, value string) error {
	var claimed []ReferenceHook
	for _, hook := range registered {
		if hook.Match == nil || hook.Match(value) {
			claimed = append(claimed, hook)
		}
	}
	switch len(claimed) {
	case 0:
		return nil
	case 1:
	default:
		// Letting registration order decide would make output depend on the
		// order a command happened to assemble its options, which `--check`
		// compares bytes against.
		names := make([]string, 0, len(claimed))
		for _, hook := range claimed {
			names = append(names, quoteName(hook.Name))
		}
		sort.Strings(names)
		return r.error(attribute.Pos, "reference hooks "+strings.Join(names, " and ")+" both claim "+
			quoteName(value)+" on "+element.Name+" "+attribute.Name+"; at most one hook may rewrite one attribute")
	}
	hook := claimed[0]
	result, err := r.convert(hook, element, attribute, value)
	if err != nil {
		return err
	}
	result.count++
	if result.skip {
		return nil
	}
	// The authored path writes attribute text verbatim, so a transform value
	// carrying a quote or an angle bracket would escape its own attribute. The
	// ampersand passes through as an authored value does, which keeps a
	// rewritten attribute byte-comparable with one written by hand.
	attribute.Value = []AttributePart{{
		Kind: "html:text",
		Pos:  attribute.Pos,
		Text: hookValueEscaper.Replace(result.to),
	}}
	return nil
}

var hookValueEscaper = strings.NewReplacer(
	"\"", "&#34;",
	"<", "&lt;",
	">", "&gt;",
)

// convert calls a transform at most once per hook and distinct value, and
// records what it produced.
func (r *hookRun) convert(hook ReferenceHook, element *ElementNode, attribute *Attribute, value string) (*hookResult, error) {
	key := hook.Name + "\x00" + value
	if cached, ok := r.converted[key]; ok {
		return cached, nil
	}
	result, err := hook.Transform(ReferenceRequest{
		Hook: hook.Name, Element: element.Name, Attribute: attribute.Name,
		Value: value, File: r.filename, Pos: attribute.Pos,
	})
	if err != nil {
		return nil, r.error(attribute.Pos, "reference hook "+quoteName(hook.Name)+": "+err.Error())
	}
	record := &hookResult{hook: hook, from: value, to: value, pos: attribute.Pos, skip: result.Skip, reason: result.Reason}
	if !result.Skip {
		if result.Value == "" {
			return nil, r.error(attribute.Pos, "reference hook "+quoteName(hook.Name)+" returned an empty value for "+
				quoteName(value)+"; return a skip to leave the attribute alone")
		}
		record.to = result.Value
	}
	for _, file := range result.Files {
		if err := r.produce(hook, attribute.Pos, file); err != nil {
			return nil, err
		}
	}
	for _, file := range result.Read {
		r.read[file] = true
	}
	r.converted[key] = record
	r.order = append(r.order, record)
	return record, nil
}

// produce records one produced file, refusing a name that would escape the
// caller's output root and a second file claiming one name with other bytes.
func (r *hookRun) produce(hook ReferenceHook, pos Position, file ProducedFile) error {
	name, err := CleanOutputName(file.Name)
	if err != nil {
		return r.error(pos, "reference hook "+quoteName(hook.Name)+": "+err.Error())
	}
	file.Name = name
	existing, ok := r.produced[name]
	if !ok {
		r.produced[name] = file
		return nil
	}
	if !bytes.Equal(existing.Content, file.Content) {
		return r.error(pos, "reference hook "+quoteName(hook.Name)+" produced two different files named "+quoteName(name))
	}
	return nil
}

func (r *hookRun) error(pos Position, message string) error {
	return &CompileError{Filename: r.filename, Pos: pos, Message: message}
}
