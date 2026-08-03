package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

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
	// A run may convert ahead of its compile, on several goroutines, so the memo
	// is single-flight rather than check-then-set: twenty templates naming one
	// image convert it once whether they ask one after another or together.
	memo := &conversionMemo{entries: map[string]*memoEntry{}}
	scoped := make([]htmlbind.ReferenceHook, len(hooks))
	for i, hook := range hooks {
		hook := hook
		scoped[i] = hook
		scoped[i].Transform = func(request htmlbind.ReferenceRequest) (htmlbind.ReferenceResult, error) {
			return memo.do(hook.Name+"\x00"+request.Value, func() (htmlbind.ReferenceResult, error) {
				return cache.convert(hook, request)
			})
		}
	}
	return scoped
}

// conversionMemo holds one outcome per hook and distinct value for the length of
// a run.
//
// A failure is memoized with the successes. That is what defers a conversion
// error to the compile that needed it: a value converted ahead of time and
// failing reports nothing then, and the sequential compile raises it at the
// template position it belongs to. Which error a build reports therefore stays
// the first one in template order rather than whichever goroutine lost a race.
type conversionMemo struct {
	mu      sync.Mutex
	entries map[string]*memoEntry
}

type memoEntry struct {
	once   sync.Once
	result htmlbind.ReferenceResult
	err    error
}

func (m *conversionMemo) do(key string, convert func() (htmlbind.ReferenceResult, error)) (htmlbind.ReferenceResult, error) {
	m.mu.Lock()
	entry, ok := m.entries[key]
	if !ok {
		entry = &memoEntry{}
		m.entries[key] = entry
	}
	m.mu.Unlock()
	// The lock is not held across the conversion: an encode is seconds, and
	// holding it would serialize exactly what this exists to parallelize.
	entry.once.Do(func() { entry.result, entry.err = convert() })
	return entry.result, entry.err
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
	// mu guards sources, which every converting goroutine writes to while
	// building its key.
	mu sync.Mutex
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
	Head    []htmlbind.HeadEntry    `json:"head,omitempty"`
}

// cacheVersion is bumped when the entry shape changes, so an older file is
// ignored rather than misread. An ignored entry converts again.
//
// Version 2 carries the head contributions. A version 1 entry is silent about
// them, which is indistinguishable from a conversion that contributes none, so
// reading one would serve a page missing its stylesheet from cache alone.
const cacheVersion = 2

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
			Files: entry.Files, Read: entry.Read, Head: entry.Head,
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
	c.mu.Lock()
	for _, source := range sources {
		c.sources[source] = true
	}
	c.mu.Unlock()
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
		Reason: result.Reason, Files: result.Files, Read: result.Read, Head: result.Head,
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := c.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	// Written beside the entry and renamed over it, because a store is not the
	// only thing touching this directory: a concurrent run, or the next one
	// after an interrupted build, must find either a whole entry or none. A
	// half-written one would be read as a valid outcome of the wrong shape.
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return
	}
	if _, err := temp.Write(encoded); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return
	}
	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		os.Remove(temp.Name())
	}
}

// namedSources reports every file a cache key named, so they are recorded as
// build inputs whether their conversion ran or was answered from the cache.
func (c *conversionCache) namedSources() []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
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

// prewarmConversions converts everything the compile is about to ask for, on
// several goroutines, before the compile starts asking.
//
// The compile is sequential because each rewritten value has to fold into the
// module being compiled, and the transform stays inline because the rewrite
// depends on the bytes it produced. Neither of those has to mean the encoding
// happens one file at a time: what the compile will claim is discoverable
// without converting any of it, so the work moves ahead of the compile rather
// than out of it, and the compile finds every answer already in the memo.
//
// Nothing here reports anything. A discovery parse failure is left for the
// compile to raise against the file, and a conversion failure is memoized and
// raised by the compile that needed it. A build's diagnostics are therefore
// identical whether or not this ran.
func (g *Generator) prewarmConversions(files []templateFile, hooks []htmlbind.ReferenceHook) {
	workers := g.Options.ConversionWorkers
	if workers <= 1 || len(hooks) == 0 {
		return
	}
	var claims []htmlbind.ReferenceRequest
	seen := map[string]bool{}
	for _, file := range files {
		if file.kind != htmlTemplate {
			continue
		}
		source, err := os.ReadFile(file.path)
		if err != nil {
			continue
		}
		found, err := htmlbind.CollectReferences(file.path, source, hooks)
		if err != nil {
			continue
		}
		for _, claim := range found {
			// The memo is keyed by hook and value, so a value two templates
			// both name is one conversion and belongs in the queue once.
			key := claim.Hook + "\x00" + claim.Value
			if seen[key] {
				continue
			}
			seen[key] = true
			claims = append(claims, claim)
		}
	}
	if len(claims) < 2 {
		return
	}
	byName := map[string]htmlbind.ReferenceHook{}
	for _, hook := range hooks {
		byName[hook.Name] = hook
	}
	if workers > len(claims) {
		workers = len(claims)
	}
	queue := make(chan htmlbind.ReferenceRequest)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for claim := range queue {
				hook, ok := byName[claim.Hook]
				if !ok {
					continue
				}
				// The error is deliberately dropped: it is already in the memo,
				// and the compile that needs this value will return it with the
				// template position that belongs to it.
				_, _ = hook.Transform(claim)
			}
		}()
	}
	for _, claim := range claims {
		queue <- claim
	}
	close(queue)
	group.Wait()
}
