package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
)

// Generation is dominated by type checking the package, so every run is repeated
// work whenever nothing it reads has changed. Each generated file therefore
// carries a stamp comment holding the SHA-256 of the inputs that produced it,
// and a run whose inputs still hash to the recorded value writes nothing.

// stampVersion is bumped whenever the stamp encoding or the set of hashed
// inputs changes, so stamps written by an older generator are ignored instead
// of misread.
const stampVersion = "1"

// stampMarker introduces the cache comment written into every generated file.
const stampMarker = "// tinybind:generated"

// stampScanLines bounds how far into a generated file the marker is searched.
const stampScanLines = 8

// stamp records what one generation run consumed and produced. It is written
// into every file the run wrote, so the next run can verify the inputs, confirm
// that the whole output set is still on disk and detect a generated file that
// was edited or left half-written.
type stamp struct {
	// inputs is the hex SHA-256 of every input the run depended on.
	inputs string
	// self is the hex SHA-256 of the containing file without this stamp line.
	self string
	// outputs holds the base names of the files the run wrote, in Paths order.
	outputs []string
}

func (s stamp) line() string {
	return fmt.Sprintf("%s v%s inputs=sha256:%s self=sha256:%s outputs=%s",
		stampMarker, stampVersion, s.inputs, s.self, strings.Join(s.outputs, ","))
}

// parseStamp reads one stamp comment. Anything it does not fully understand,
// including a stamp from another format version, reports no stamp so the caller
// regenerates.
func parseStamp(line string) (stamp, bool) {
	fields := strings.Fields(line)
	if len(fields) != 6 || fields[0]+" "+fields[1] != stampMarker || fields[2] != "v"+stampVersion {
		return stamp{}, false
	}
	inputs, ok := strings.CutPrefix(fields[3], "inputs=sha256:")
	if !ok || inputs == "" {
		return stamp{}, false
	}
	self, ok := strings.CutPrefix(fields[4], "self=sha256:")
	if !ok || self == "" {
		return stamp{}, false
	}
	outputs, ok := strings.CutPrefix(fields[5], "outputs=")
	if !ok || outputs == "" {
		return stamp{}, false
	}
	return stamp{inputs: inputs, self: self, outputs: strings.Split(outputs, ",")}, true
}

// readStamp returns the stamp of an already generated file, and reports no
// stamp unless the rest of the file is still exactly what the stamping run
// wrote. A missing, unstamped, edited or truncated file is therefore never
// treated as up to date.
func readStamp(path string) (stamp, bool) {
	source, err := os.ReadFile(path)
	if err != nil {
		return stamp{}, false
	}
	recorded, unstamped, ok := splitStamp(source)
	if !ok || recorded.self != contentHash(unstamped) {
		return stamp{}, false
	}
	return recorded, true
}

// splitStamp finds the stamp comment near the top of a generated file and
// returns the file content without it, which is what the stamp hashes.
func splitStamp(source []byte) (stamp, []byte, bool) {
	for line, offset := 0, 0; line < stampScanLines && offset < len(source); line++ {
		end := len(source)
		if index := bytes.IndexByte(source[offset:], '\n'); index >= 0 {
			end = offset + index + 1
		}
		if recorded, ok := parseStamp(string(source[offset:end])); ok {
			unstamped := make([]byte, 0, len(source)-(end-offset))
			unstamped = append(unstamped, source[:offset]...)
			return recorded, append(unstamped, source[end:]...), true
		}
		offset = end
	}
	return stamp{}, nil, false
}

// writeStamp inserts the cache comment directly below the generated-code header
// of a file the run just wrote, leaving that header on the first line where Go
// tooling expects it.
func writeStamp(path, fingerprint string, outputs []string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	insert := 0
	if end := bytes.IndexByte(source, '\n'); end >= 0 && bytes.HasPrefix(source, []byte("// Code generated")) {
		insert = end + 1
	}
	var stamped bytes.Buffer
	stamped.Write(source[:insert])
	stamped.WriteString(stamp{inputs: fingerprint, self: contentHash(source), outputs: outputs}.line())
	stamped.WriteByte('\n')
	stamped.Write(source[insert:])
	return os.WriteFile(path, stamped.Bytes(), 0o644)
}

// stampGeneration records one fingerprint in every Go file the run wrote.
//
// An extracted public asset is left alone: its name already carries the hash of
// its bytes, so inserting a comment would both contradict that name and put a
// Go comment in a stylesheet. A cache hit therefore rewrites no asset; -force
// restores one that was deleted by hand.
func stampGeneration(fingerprint string, result GenerateResult) error {
	paths := result.goPaths()
	names := make([]string, len(paths))
	for i, path := range paths {
		names[i] = filepath.Base(path)
	}
	for _, path := range paths {
		if err := writeStamp(path, fingerprint, names); err != nil {
			return err
		}
	}
	return nil
}

func contentHash(source []byte) string {
	digest := sha256.Sum256(source)
	return hex.EncodeToString(digest[:])
}

// cachedGeneration returns the previous run's result when every file it wrote is
// still present and still records the current input fingerprint.
func cachedGeneration(outDir, fingerprint string, request GenerateRequest) (GenerateResult, bool) {
	for _, name := range outputNames(request) {
		recorded, ok := readStamp(filepath.Join(outDir, name))
		if !ok || recorded.inputs != fingerprint {
			continue
		}
		return stampedResult(outDir, fingerprint, recorded, request)
	}
	return GenerateResult{}, false
}

// stampedResult rebuilds a GenerateResult from a recorded output set. Every
// recorded file must still exist, still carry the same fingerprint and still be
// one of the files this request would write; otherwise the cache is stale.
func stampedResult(outDir, fingerprint string, recorded stamp, request GenerateRequest) (GenerateResult, bool) {
	result := GenerateResult{Cached: true}
	for _, name := range recorded.outputs {
		path := filepath.Join(outDir, name)
		other, ok := readStamp(path)
		if !ok || other.inputs != fingerprint {
			return GenerateResult{}, false
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return GenerateResult{}, false
		}
		switch name {
		case request.TemplatesName:
			result.TemplatesPath = abs
		case request.Name:
			result.BinderPath = abs
		case request.ConfigBindName:
			result.ConfigBindPath = abs
		case request.DynamoName:
			result.DynamoPath = abs
		case request.OpenAPIName:
			result.OpenAPIPath = abs
		default:
			return GenerateResult{}, false
		}
	}
	if len(result.Paths()) != len(recorded.outputs) {
		return GenerateResult{}, false
	}
	return result, true
}

// outputNames lists every file name this request can write. GeneratePackage
// fills the defaults in before either side of the cache uses them.
func outputNames(request GenerateRequest) []string {
	return []string{request.TemplatesName, request.Name, request.ConfigBindName, request.DynamoName, request.OpenAPIName}
}

// generationInputs is the non-file part of the fingerprint: the generator
// itself, the discovery configuration and the switches of this run.
type generationInputs struct {
	Version     string
	Generator   string
	Options     Options
	OpenAPI     bool
	Outputs     []string
	PackagePath string
}

// generationFingerprint hashes everything a generation run reads: the generator
// binary, the effective options, the run switches, the module files and every
// input file in the package directory. Files this run writes are excluded, so
// the fingerprint a run records still describes its own inputs.
//
// The hash covers the package directory rather than the whole module, so
// editing one package leaves the generated files of its siblings cached. A
// change in another package that reaches generated output - a moved runtime
// wrapper, for instance - is therefore not detected; -force regenerates.
func generationFingerprint(dir, outDir string, request GenerateRequest, options Options) (string, error) {
	identity, err := generatorIdentity()
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	moduleDir, relative := modulePosition(dir)
	header, err := json.Marshal(generationInputs{
		Version:     stampVersion,
		Generator:   identity,
		Options:     options,
		OpenAPI:     request.OpenAPI,
		Outputs:     outputNames(request),
		PackagePath: relative,
	})
	if err != nil {
		return "", err
	}
	digest.Write(header)
	digest.Write([]byte{'\n'})
	if moduleDir != "" {
		for _, name := range []string{"go.mod", "go.sum"} {
			if err := hashInto(digest, name, filepath.Join(moduleDir, name)); err != nil {
				return "", err
			}
		}
	}
	if err := hashPackageInputs(digest, dir, options, generatedInDir(dir, outDir, request)); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// hashPackageInputs hashes the name and content of every file in dir that
// generation reads.
func hashPackageInputs(digest hash.Hash, dir string, options Options, skip map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	htmlPattern := templatePattern(options.HTMLTemplatePattern, DefaultHTMLTemplatePattern)
	sqlPattern := templatePattern(options.SQLTemplatePattern, DefaultSQLTemplatePattern)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if skip[name] || !isGenerationInput(name, htmlPattern, sqlPattern) {
			continue
		}
		if err := hashInto(digest, name, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// isGenerationInput reports whether a file in the package directory can change
// generated output. Test files cannot: no phase loads them.
func isGenerationInput(name, htmlPattern, sqlPattern string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	if strings.HasSuffix(name, ".go") {
		return true
	}
	if matched, err := filepath.Match(htmlPattern, name); err == nil && matched {
		return true
	}
	matched, err := filepath.Match(sqlPattern, name)
	return err == nil && matched
}

// generatedInDir names the files this run writes into the hashed directory.
func generatedInDir(dir, outDir string, request GenerateRequest) map[string]bool {
	if outDir != "" && !sameDir(dir, outDir) {
		return nil
	}
	skip := make(map[string]bool, 4)
	for _, name := range outputNames(request) {
		skip[name] = true
	}
	return skip
}

func sameDir(left, right string) bool {
	leftAbs, err := filepath.Abs(left)
	if err != nil {
		return false
	}
	rightAbs, err := filepath.Abs(right)
	if err != nil {
		return false
	}
	return leftAbs == rightAbs
}

// hashInto adds one named file to the fingerprint. A missing file contributes
// its absence, so creating or deleting it invalidates the stamp.
func hashInto(digest hash.Hash, name, path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(digest, "%s absent\n", name)
			return nil
		}
		return err
	}
	defer file.Close()
	content := sha256.New()
	if _, err := io.Copy(content, file); err != nil {
		return err
	}
	fmt.Fprintf(digest, "%s %s\n", name, hex.EncodeToString(content.Sum(nil)))
	return nil
}

// modulePosition returns the module root containing dir and the slash-separated
// path of dir inside it. The relative path is part of the fingerprint because it
// determines the import path baked into generated OpenAPI registration.
func modulePosition(dir string) (moduleDir, relative string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", ""
	}
	for current := abs; ; {
		if info, err := os.Stat(filepath.Join(current, "go.mod")); err == nil && !info.IsDir() {
			rel, err := filepath.Rel(current, abs)
			if err != nil {
				return current, ""
			}
			return current, filepath.ToSlash(rel)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", ""
		}
		current = parent
	}
}

var generatorIdentityOnce struct {
	sync.Once
	value string
	err   error
}

// generatorIdentity identifies the running generator, so generated output is
// refreshed whenever the code that produces it changes. The executable is
// hashed rather than versioned because `go run` reports no useful version, yet
// links a binary whose content follows the generator sources exactly.
func generatorIdentity() (string, error) {
	generatorIdentityOnce.Do(func() {
		generatorIdentityOnce.value, generatorIdentityOnce.err = readGeneratorIdentity()
	})
	return generatorIdentityOnce.value, generatorIdentityOnce.err
}

func readGeneratorIdentity() (string, error) {
	executable, err := os.Executable()
	if err == nil {
		file, err := os.Open(executable)
		if err == nil {
			defer file.Close()
			digest := sha256.New()
			if _, err := io.Copy(digest, file); err == nil {
				return "exe:" + hex.EncodeToString(digest.Sum(nil)), nil
			}
		}
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "", fmt.Errorf("generator: no build identity for the generated-file stamp")
	}
	return "build:" + info.Main.Path + "@" + info.Main.Version, nil
}
