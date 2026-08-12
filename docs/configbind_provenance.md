# Effective Configuration Output

This is a reference for rendering the configuration a process actually resolved: the struct tags that shape the output, the record type you receive, and the two surfaces one call serves.

For loading configuration in the first place, see the [configbind user guide](configbind.md).

## The one call

```go
result, err := configbind.Load(configbind.LoadOptions{Vendor: "acme", Tool: "myserver"})
if err != nil {
	return err
}

for _, entry := range result.Provenance() {
	fmt.Printf("%s = %s (%s)\n", entry.Key, entry.Value, entry.Place)
}
```

`Provenance()` returns the same slice whatever you are rendering. It is ordered, redacted, and dependency-filtered before you see it. Nothing about the slice depends on which surface you intend to draw — that choice is yours, and the record carries what you need to make it.

## The record

```go
type ProvenanceEntry struct {
	Key       string // "session.redis.dsn", or "rdb.connections[0].dsn" for an array element
	Value     string // display form, never a raw secret
	Place     Place  // the source layer that won
	Masked    bool   // Value is the mask, not the configured value
	Omittable bool   // a short surface may leave this out
	ArrayKey  string // the array this is an element field of; empty otherwise
	Index     int    // element position within ArrayKey; 0 when ArrayKey is empty
}
```

`Place` is one of `PlaceDefault` (`"default"`), `PlaceFile` (`"file_toml"`), `PlaceEnv` (`"env"`), or `PlaceCLI` (`"cli"`).

Two fields exist so you never have to re-derive a policy the library already applied:

- **`Masked`** tells you `Value` is the redaction placeholder. Test the field rather than comparing `Value` against `"*****"` — the mask text is an implementation detail, and a real configured value could equal it.
- **`Omittable`** tells you a short surface may drop the entry. Both halves of the rule are already applied, so two call sites cannot read it differently.

`ArrayKey` and `Index` let you group and order an array of tables without splitting `Key` at its brackets. An ordinary key has an empty `ArrayKey`.

## Coverage and order

Two properties matter if you are building a tree rather than printing lines.

**Not every declared key appears.** The slice covers keys present in the overlay, so a field with no `default` tag that no source set is absent — not present with an empty value. Nothing distinguishes "nobody configured this" from "this field does not exist" in the output, so do not treat the slice as a schema.

**The order is meaningful and deterministic.** Entries follow `Bind` registration order, and within one binding the field declaration order of its struct — not key sort order. An array of tables expands in place into one entry per element field, ordered by index then declaration. Keys belonging to no registered binding (a stray TOML entry, for instance) sort lexicographically after all known keys. Render in the order you receive; sorting throws away the structure.

## Two surfaces, one call

A startup summary wants the short version. A `docker inspect`-style dump wants everything. The difference is one condition:

```go
func render(result *configbind.LoadResult, brief bool) {
	for _, entry := range result.Provenance() {
		if brief && entry.Omittable {
			continue
		}
		fmt.Printf("%-34s %-26s %s\n", entry.Key, entry.Value, entry.Place)
	}
}
```

A dump is still not *literally* everything. Two things never reach you on any surface:

- keys dropped because a `dependon` condition failed — those settings are inert, and printing them would say they are in force
- keys marked `secret:"hide"`

## What the library decides, and what you decide

| Policy | Tag | Effect on the slice |
| --- | --- | --- |
| Dependency visibility | `dependon` | Entry is **absent** |
| Disclosure | `secret:"hide"` | Entry is **absent** |
| Disclosure | `secret:"mask"` / auto | `Value` replaced, `Masked` set |
| Detail rating | `summary:"omit"` | `Omittable` set; **nothing removed** |

The asymmetry is deliberate. A failed `dependon` condition states a *fact about the configuration* — this setting does not apply — which is true wherever it is printed, so the library drops the entry itself. A `summary` rating is a *judgment about one surface*, and the library cannot know which surface you are rendering. So it marks, and you decide.

An entry the library dropped is never marked, because it is not there to mark.

## Notation

### `dependon` — hide what does not apply

Three forms. The parent is a full config key including its prefix, so a field can depend on a key bound by another package; a leading dot resolves against the struct the tag is written in.

| Form | Visible when |
| --- | --- |
| `dependon:"webserver.tls.enabled"` | the parent is not empty |
| `dependon:".enabled"` | the same, naming a sibling |
| `dependon:".mode=oidc_only,oidc_passkey"` | the parent holds one of those values |
| `dependon:".backend!=cookie"` | the parent holds none of those values |

"Empty" means `""` or `false`, plus whatever a `falsy` tag on the parent declares. An `int` of `0`, a zero duration, and an empty list are deliberate settings rather than absent ones.

The **value forms** exist because emptiness cannot express "this subtree belongs to one mode": a mode key holds a non-empty value in every mode, so every mode's block would stay on screen and the inert ones would read as though they were in force.

```go
type AuthConfig struct {
	Enabled bool
	Mode    string        `default:"oidc_only" enum:"oidc_only,oidc_passkey,jwt_only" dependon:".enabled"`
	OIDC    OIDCConfig    `dependon:".mode=oidc_only,oidc_passkey"`
	Passkey PasskeyConfig `dependon:".mode=oidc_passkey"`
	JWT     JWTConfig     `dependon:".mode=jwt_only"`
}
```

At `auth.mode = oidc_only`, the whole `auth.passkey` and `auth.jwt` blocks are absent and `auth.oidc` remains.

Four rules make a condition predictable:

1. **The comma separates values, never parents.** After an operator it lists alternatives, which is why `auth.oidc` survives two of the three modes above. A comma with no operator before it is still a rejected parent list.
2. **An operator states the whole test.** Neither emptiness nor the parent's `falsy` value is also consulted, so `dependon:"obs.tracing=off"` really does keep its field at `off`.
3. **A parent nothing set compares as the empty string.** `=` hides, `!=` shows. Over-showing is the safe direction when nobody has said what the parent is.
4. **Values compare in the parent's own terms.** A duration condition matches `0`, `0s`, and `0ms` alike, and a number or duration named this way needs no `falsy` tag.

You rarely write "enabled *and* mode = x". A mode key normally carries its own `dependon` on the feature switch, as `Mode` does above, and a hidden parent hides its dependents — so turning `auth.enabled` off removes the selected block too.

Add an `enum` tag to the parent. Generation then checks every named value against it, and a mistyped value fails `go generate` instead of hiding a whole subtree silently and permanently:

```text
field Passkey: dependon value "oidc_pass_key" is not one of the enum choices
"oidc_only,oidc_passkey,jwt_only" of parent "auth.mode"
```

The check is best-effort in the same way the parent-kind check is: a parent bound in another package is invisible to this generation run and passes unchecked.

### `summary:"omit"` — rate what is not worth a headline

What survives the filters above is still dominated by keys sitting at their defaults: applicable, but nothing this deployment had an opinion about.

```go
type ObservabilityConfig struct {
	MinimumLevel string      `default:"info"`
	Query        QueryConfig `summary:"omit"`
	Trace        TraceConfig `summary:"omit"`
}
```

**`Omittable` requires both halves: the rating is in force *and* `Place` is `PlaceDefault`.** A rated key that a source set is a decision that deployment made, and a brevity feature which hid a decision would be a bug rather than a shorter output. This is what lets you rate a whole subtree without first auditing which of its leaves someone configured — the configured ones come back on their own:

```text
# session.cookie is rated summary:"omit", and SESSION_COOKIE_SECURE=false is set
session.cookie.name    pw_session  default   <- omittable
session.cookie.secure  false       env       <- notable, stays in the summary
```

Omission is opt-in, so an untagged key is always notable. A forgotten tag only leaves the output longer; the opposite polarity would make a newly added field invisible to whoever operates the service. The cost is that brevity is proportional to tags written, which is why the tag propagates over a subtree — one tag on a nested struct rates every key below it.

## Placement

| Tag | Leaf | Nested struct | Array of tables | Element field |
| --- | --- | --- | --- | --- |
| `dependon` | yes | whole subtree | array and its elements | rejected |
| `secret` | yes | whole subtree | array and its elements | yes |
| `summary` | yes | whole subtree | array and its elements | yes, but see below |
| `falsy` | yes | rejected | rejected | rejected |
| `enum` | yes | rejected | rejected | rejected |

`dependon`, `falsy`, and `enum` need a stable config key, which an array element does not have — its key carries an index that exists only at run time. `secret` and `summary` describe the key being printed rather than naming one to look up, so they resolve by the element's stable path under the array key.

A `summary` rating on an element field is currently inert: `Definition.Defaults` is keyed by stable key and an element has none, so an element overlay is built from the file alone. Every element field that appears was therefore set by a source, and the `Place` half of the rule never holds. The useful placements today are a leaf and a nested struct.

Every placement resolves to a defined outcome — propagate or fail generation. None is accepted and silently dropped.

## Reading the generated definitions

Most callers need only `Provenance()`. If you are building tooling over the registry itself, the resolved policies are public:

```go
type Dependency struct {
	Key    string   // parent's absolute config key, already resolved from any relative form
	Op     string   // "", DependOpEqual ("="), or DependOpNotEqual ("!=")
	Values []string // the tag's value list; empty when Op is empty
}

type Definition struct {
	// ...
	DependsOn map[string][]Dependency // every condition a key answers to
	Falsy     map[string]string       // the value that means "off"
	Secrets   map[string]string       // "hide", "mask", or "show"
	Summary   map[string]string       // SummaryOmit
}
```

`Op` empty implies `Values` empty and means the emptiness test. A key's conditions are conjunctive: its own tag plus every tag declared on a struct above it, all of which must pass.

Parsing happens at generation time, which is why these are resolved structures rather than raw tag strings — a load performs no tag parsing at all.

## Scope

None of this reaches the bound struct. Every field is still populated from its sources, whether or not its key appears in the output. CLI flags, help text, and validation are unaffected. Scaffolds (`ScaffoldTOML`, `ScaffoldEnv`) still list every field: they render before any load, so no `Place` exists yet, and a scaffold advertises what is settable — omitting anything would make it undiscoverable.

## Migrating

`Definition.DependsOn` changed from `map[string][]string` to `map[string][]Dependency`. Regenerate any committed generated configbind files:

```bash
go generate ./...
```

`Definition.Summary` and `ProvenanceEntry.Omittable` are additive; existing code compiles unchanged and behaves as before until you add tags or read the new field.
