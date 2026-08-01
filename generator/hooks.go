package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// A reference hook converts inline, because the rewrite may depend on how the
// conversion turned out: an encode larger than its source is worth declining,
// and only the converted bytes can say so.
//
// Converting inline would be unaffordable if it happened every build, so two
// layers keep it from happening twice. runScopedHooks memoizes within a run, so
// two templates naming one image convert once. conversionCache memoizes across
// runs on disk, so an unchanged asset costs a digest instead of an encode - and
// a source that once lost the size comparison is never re-encoded to rediscover
// that it loses, because the decision to decline is cached like any other
// outcome.

// runScopedHooks widens transform memoization from one template to the whole
// run.
//
// htmlbind compiles one module at a time and can only remember the values it
// saw in that module, so two templates referencing one image would convert it
// twice. Converting is the expensive part of a hook, so the duplicate is worth
// removing where the run, and not the module, is the unit that knows about it.
//
// Memoizing across files matches the declared contract: a transform is a pure
// function of what it reads plus its own settings. The file and position on a
// request are there for a diagnostic, and a transform deciding its output from
// them was already outside the contract.
func runScopedHooks(hooks []htmlbind.ReferenceHook, cache *conversionCache) []htmlbind.ReferenceHook {
	if len(hooks) == 0 {
		return nil
	}
	type memo struct {
		result htmlbind.ReferenceResult
		err    error
	}
	// One run compiles its templates in sequence, and every run builds its own
	// map, so nothing here is shared across goroutines.
	seen := map[string]memo{}
	scoped := make([]htmlbind.ReferenceHook, len(hooks))
	for i, hook := range hooks {
		hook := hook
		scoped[i] = hook
		scoped[i].Transform = func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			key := hook.Name + "\x00" + request.Value
			if cached, ok := seen[key]; ok {
				return cached.result, cached.err
			}
			result, err := cache.convert(hook, request)
			seen[key] = memo{result: result, err: err}
			return result, err
		}
	}
	return scoped
}

// conversionCache stores the whole outcome of one conversion - the rewritten
// value, the files, the decision to decline, and what was read - under a key
// the hook itself declares.
//
// A nil cache, or a hook with no CacheKey, converts every time. That is correct
// and slow, which is the right default for something whose invalidation the
// caller has not described.
type conversionCache struct {
	dir string
	// sources collects the files every cache key named, so they join the
	// recorded build inputs whether the conversion ran or was reused.
	sources map[string]bool
}

func newConversionCache(dir string) *conversionCache {
	return &conversionCache{dir: dir, sources: map[string]bool{}}
}

// cacheEntry is one stored outcome. It holds the decision, not only the bytes:
// a skip is as expensive to rediscover as a conversion is to redo.
type cacheEntry struct {
	Version int                     `json:"version"`
	Value   string                  `json:"value,omitempty"`
	Skip    bool                    `json:"skip,omitempty"`
	Reason  string                  `json:"reason,omitempty"`
	Files   []htmlbind.ProducedFile `json:"files,omitempty"`
	Read    []string                `json:"read,omitempty"`
}

// cacheVersion is bumped when the entry shape changes, so an older file is
// ignored rather than misread. An ignored entry converts again.
const cacheVersion = 1

func (c *conversionCache) convert(hook htmlbind.ReferenceHook, request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
	key, ok, err := c.key(hook, request)
	if err != nil {
		return htmlbind.ReferenceResult{}, err
	}
	if !ok {
		return hook.Transform(request)
	}
	if entry, hit := c.load(key); hit {
		return htmlbind.ReferenceResult{
			Value: entry.Value, Skip: entry.Skip, Reason: entry.Reason,
			Files: entry.Files, Read: entry.Read,
		}, nil
	}
	result, err := hook.Transform(request)
	if err != nil {
		return result, err
	}
	// A failed store is a lost optimization, not a failed build: the result in
	// hand is already correct.
	c.store(key, result)
	return result, nil
}

// key digests everything the hook said its output depends on, and records the
// sources it named whether or not the result is cacheable.
//
// Recording is not conditional on caching: a source a conversion reads is a
// build input, and leaving it out of the recorded set would let an edit to it
// go unnoticed on the next run. Only the reuse depends on a configured cache.
//
// It reports false when this conversion cannot be cached, which is an absent
// cache directory, a hook that declared no key, or a source that cannot be read
// now and so could not be compared later.
func (c *conversionCache) key(hook htmlbind.ReferenceHook, request htmlbind.ReferenceRequest) (string, bool, error) {
	if c == nil || hook.CacheKey == nil {
		return "", false, nil
	}
	inputs, err := hook.CacheKey(request)
	if err != nil {
		return "", false, err
	}
	sources := sortedStrings(inputs.Sources)
	for _, source := range sources {
		c.sources[source] = true
	}
	if c.dir == "" {
		return "", false, nil
	}
	digest := sha256.New()
	// The hook name and the claimed value are part of the identity: two hooks
	// converting one file to different things must not share an entry.
	fmt.Fprintf(digest, "v%d\x00%s\x00%s\x00%s\x00", cacheVersion, hook.Name, request.Value, inputs.Params)
	for _, source := range sources {
		content, err := os.ReadFile(source)
		if err != nil {
			// The transform reports the real problem; this only declines to
			// remember an outcome it could never verify.
			return "", false, nil
		}
		fmt.Fprintf(digest, "%s\x00%s\x00", source, contentHash(content))
	}
	return hex.EncodeToString(digest.Sum(nil)), true, nil
}

func (c *conversionCache) path(key string) string {
	// Two levels of fan-out keep a directory listing usable on a project with
	// thousands of assets.
	return filepath.Join(c.dir, key[:2], key[2:]+".json")
}

func (c *conversionCache) load(key string) (cacheEntry, bool) {
	source, err := os.ReadFile(c.path(key))
	if err != nil {
		return cacheEntry{}, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(source, &entry); err != nil || entry.Version != cacheVersion {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *conversionCache) store(key string, result htmlbind.ReferenceResult) {
	entry := cacheEntry{
		Version: cacheVersion, Value: result.Value, Skip: result.Skip,
		Reason: result.Reason, Files: result.Files, Read: result.Read,
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := c.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, encoded, 0o644)
}

// namedSources reports every file a cache key named, so they are recorded as
// build inputs whether their conversion ran or was answered from the cache.
func (c *conversionCache) namedSources() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.sources))
	for source := range c.sources {
		out = append(out, source)
	}
	sort.Strings(out)
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
