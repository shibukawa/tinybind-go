---
id: decision:cache-scope-declaration
type: decision
title: Cache Scope Declaration
---
Give `@cache` a scope option, report an undeclared chain as private, and let the same annotation on a slot owner declare scope while storing nothing.

```yaml
source:
  - downstream framework cache scope report 2026-08-09, against v0.4.8
  - decision:cache-scope-seams
  - user default and mode decision 2026-08-09
review_gate: proposed
status: delivered 2026-08-09; see as_built
as_built:
  runtime:
    policy: CachePolicy.Scoped, and cacheKey takes the render's scope and prepends it framed; an unscoped policy builds the key it always did
    option: WithCacheScope, beside WithCache on renderOptions
    absent_scope: execCached renders the ops directly when Scoped and the scope is empty, so nothing is keyed and nothing is stored
    plan: DeclaresPrivate, DeclaresPublic, and PrivateSource
    fold: foldSlots unions private and its source; public is left where it was written
    accessors: Fragment.IsPrivate, Wrapper.IsPrivate, and PrivateSource on both
    chain: IsPrivate(wrappers, leaf) and PrivateSource(wrappers, leaf)
  generation:
    policy: cachePolicy carries ttl, public, and the scope's position; stores() is ttl > 0
    parse: scope accepts private or public; a ttl on a slot owner and an annotation with neither argument are generation errors
    walk: reachesDeclaredPrivate, beside reachesAwait and reachesPerRequest, reading the declaration bit
    refusal: reported at the position the scope was written, naming the component that declared private
    emission: the two bits and the source are written only when something declared, so a project declaring none regenerates byte for byte
  found_while_building:
    key_encoders_are_gated_on_storage: the per-record encoder walk ran for every annotated component, which would emit dead encoders for a declaration and reach a declaring layout's html parameter, which has no encoding at all; gated on stores()
    the_chain_rule_needed_its_exact_form: private wins anywhere, and public has to be on the outermost member, because a wrapper contains everything below it while an inner member says nothing about the markup around it; a leaf asserting public under an undeclared layout therefore stays private
  fixture_evidence: the async fixture's cached Badge became private-scoped and stopped storing without a scope value, which is the migration cliff firing in a test rather than in production
supersedes:
  what: the decision:cache-key-derivation constraint that request, session, and locale variation must be declared parameters or the component must not be cached
  approved: 2026-07-26, so this is an amendment to a passed gate rather than a gap being filled
  why_it_moves: the rule made a component reading per-user state uncacheable rather than cacheable per user, and the framing that fixes it costs one string in front of a key
three_states:
  reason: two states cannot hold both halves; a default has to be safe for the header, and an assertion has to be refusable
  undeclared:
    header: private; requirement:chain-render-pipeline members with no annotation anywhere report private
    assertion: does not contradict a public declaration, so an ordinary component inherits whatever the chain asserts
    why_both: a default that also vetoed would make nothing publishable, and a default that also inherited would publish a login-gated page
  declared_private:
    written: `@cache(scope: "private")`, and the default of a storing `@cache` with no scope written
    header: private
    assertion: vetoes a public declaration in the same call graph, which undeclared does not
    key: the storing form prefixes the framed scope value
  declared_public:
    written: `@cache(scope: "public")`
    header: private only when something else in the chain declares private
    key: parameters alone, which is v0.4.9 behavior
default_is_the_framework_default:
  decided: 2026-08-09 by the owner; undeclared reports private, not merely undeclared
  larger_than_the_ask: the reporter asked for private as the annotation's default, which leaves an unannotated component silent; this makes it the value a caller reads
  consequence: a login-gated application writes nothing and gets `private`; the shared page is the one that writes an annotation
  failure_asymmetry: a public component that is per-user serves one user's output to another; a private one that is shared costs a miss, so the undeclared side sits where forgetting is slow rather than wrong
  what_static_analysis_cannot_see: a component's call graph is fully visible, but an external Go function is opaque, so a component calling one that reads request identity from ctx looks shared to every check either side can write
modes:
  selector: the ttl argument alone; writing one asks for storage, omitting one declares scope and nothing else
  settled: 2026-08-09 by the owner, as ttl required for storage rather than required on a component
  storing:
    written: ttl present
    target: a component with no html parameter that does not own the document head; ttl on a slot owner or shell is a generation error, because a duration there describes an expiry that cannot happen
    effect: stores bytes and declares scope
    eligibility: decision:cache-component-declaration unchanged
  declaring:
    written: ttl absent
    target: anywhere, including a component, a slot owner, and a document shell
    effect: declares scope only; nothing is stored and no key is computed
    eligibility: none of it applies; see eligibility_is_about_storage
  neither: `@cache` with no ttl and no scope says nothing and is a generation error
  why_a_component_declares_without_storing:
    first_reason: a component that cannot store still has a scope; a page that awaits is ineligible for storage and would otherwise be unable to assert public, leaving it permanently private under the default
    second_reason_is_the_larger_one: it is the only way to declare what static analysis cannot see. A component calling an external Go function that reads request identity from ctx looks shared to every check either side can write, and a bare `@cache(scope: "private")` on it turns the author's knowledge into a call-graph fact that vetoes any public assertion above it
    reading: the declaring mode was reached to close a hole in the public direction and is worth more in the private one
eligibility_is_about_storage:
  claim: every condition in decision:cache-component-declaration eligibility exists because bytes are stored
  single_root: a decomposition hole needs an element to hold the place
  no_html_parameters: a bound continuation cannot enter decision:cache-key-derivation
  no_await: a boundary emits in two pieces and is not one byte range
  no_nested_boundary: a hole is structure a stored range cannot express
  no_shell: requirement:head-merging output depends on the chain rather than on parameters
  so: a declaration storing nothing removes the premise of each, which makes the declaring mode an application of the rule rather than an exception to it
  reading: the shell case gains from it, because the outermost layout owns the head and is the natural place to say a whole document is private
key_effect:
  where: decision:cache-key-derivation, prefixing the framed scope value ahead of the plan fingerprint
  framing: the existing length-prefixed form, so a scope value cannot spell out another key
  reflection_free: decision:reflection-free untouched; this is one string in front of a string generated code already builds
  store: scoping happens above the api:cache-store interface, so an adapter needs no change
runtime_option:
  spelling: `WithCacheScope(scope string)`, beside `WithCache`
  value: opaque to this module; the caller supplies a unique-per-user identifier and this module never learns what it means
  absent:
    rule: a storing component declaring private with no scope value stores nothing
    not_empty_scope: an entry under an empty scope is a shared entry wearing a private label
    status: a fallback rather than a design; a miss is preferred to a decision
readable_before_the_first_byte:
  why: `Cache-Control` is on the wire before the first byte, and a private component four levels down renders long after, so a signal computed during a render exists only on the buffered branch and would make a security-relevant header depend on whether streaming is on
  plan: two bits, declared private and declared public, computed over the call graph as `HasAwaitBlock` already is
  fold: unioned in foldSlots beside hasAwait, hasLive, head, assets, and vary
  accessors: Fragment and Wrapper report both, as they do for Vary
  chain: `IsPrivate(wrappers, leaf)` beside `HasAwaitBlock` and `MergeVary`
  chain_rule: private unless some member declares public and none declares private, so private wins a combination assembled at run time that generation never saw
  diagnostics: name the component that declared it, as Plan.HeadSources does for a head contribution a caller cannot deliver
generation_refusal:
  rule: `@cache(scope: "public")` on a component whose call graph reaches a declared private one is a generation error at the declaration position
  declared_not_undeclared: an undeclared component must not block a public assertion, or nothing could ever be public
  not_a_warning: the source says public and the behavior is private, and a reviewer reads the source
  precedent: decision:cache-component-declaration await_rationale settled the same shape for the same reason
  cost: the call-graph walk reachesAwait and reachesPerRequest already perform, reading a different bit
consumer_owns_the_header:
  what: turning IsPrivate into `Cache-Control: private, no-store` belongs to the caller
  reason: decision:caller-writes-the-response
not_in_scope:
  identity: the scope value is opaque; nothing about sessions or authentication enters this module
  store: api:cache-store is unchanged, and no shared or network-backed store is implied
  config_switch: none, on either side; a knob flipping a security default across a deployment gets flipped once during an investigation and never flipped back
  proof: private is a declaration, not a proof; a component whose Go body reads per-user state and declares nothing stays wrong, and neither side can catch it
operational_cost:
  what: private keying multiplies entry count by the number of active scopes
  where: the bundled MemoryCache takes a maximum entry count and evicts in approximate insertion order, so a default entry cap sized for public keys thrashes once keys are per user
  not_a_reason_to_change_the_default: it is a number to publish with the default, because the reporter's framing that a wrong private guess costs one miss understates it
migration:
  decided: 2026-08-09 by the owner; v0.5.0 defaults to private, with no intervening version requiring scope explicitly
  vehicle: a minor bump is the breaking-change vehicle before 1.0, so this clears the reporter's floor of no silent flip in a patch release
  owner_reason: the module is not yet a public framework, so the consumer set is small and known and a release note reaches all of it
  declined: the reporter's preferred option of requiring scope for one minor version first, which buys a per-call-site decision this consumer set does not need
  honest_cost: nothing about the flip is a compile error. Every existing `@cache(ttl: ...)` becomes private-scoped, and a caller passing no scope value stores nothing, so the observable result is a cache that quietly stops being reused on exactly the components an author declared most reusable
  release_note_must_name: the key change, the header change reaching components carrying no annotation at all, and WithCacheScope as what a storing private component now needs
acceptance:
  - a chain with no annotation anywhere reports private
  - a layout carrying `@cache(scope: "public")` makes an undeclared page beneath it report public
  - a page declaring private under a layout declaring public reports private
  - `@cache(scope: "public")` reaching a declared private component fails generation at the declaration position
  - a ttl on a layout, and an annotation carrying neither ttl nor scope, fail generation
  - a storing private component rendered with no scope value stores nothing and computes no entry
```
