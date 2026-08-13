---
id: requirement:cache-key-generation
type: requirement
title: Cache Key Generation From Struct Tags
---
Emit a cache key method from a Go struct so the whole dependency set of a cached result is carried by the key without an author restating it.

```yaml
source:
  - downstream framework change request 2026-08-13, against v0.5.8
  - decision:cache-key-generator-seams
package: cachekeybind
runtime_interface: |
  type CacheKey interface{ CacheKey() string }
declared: |
  type UserSummary struct {
      UserID string `cache:"key"`
      Page   int    `cache:"key"`
      Name   string
      Total  int
  }
emitted: |
  func (k UserSummary) CacheKey() string {
      return cachekeybind.KeyString("example.com/app.UserSummary") +
          cachekeybind.KeyString(k.UserID) +
          cachekeybind.KeyInt(k.Page)
  }
  var _ cachekeybind.CacheKey = UserSummary{}
reading_of_the_example: Name and Total are the result, so they are absent from the key by not being marked; the entity is passed to the cache as-is and the key is picked out of it
why_generation_rather_than_hand_writing:
  noise: proportional to the field count
  omitted_identity: two key types holding equal field values reach one entry, which is a wrong answer rather than a stale one
  framing: a hand-written concatenation can alias across field boundaries; the emitted one cannot
  what_opt_in_gives_up: the ask's third reason, that a field added and forgotten is silently wrong, is only closed by default-include; under the owner's opt-in the author still declares the dependency set by hand, and generation enforces its spelling rather than its completeness
  why_that_is_still_worth_generating: the two failures generation does close are both wrong-answer failures, and the one it leaves is the one an author is looking straight at while adding the dependency
  why_here: the requirement is over application Go types, and generation over application Go types is this module's
identity:
  derived: package path plus type name, unique by construction, never restated by an author
  no_version: decision:cache-key-version-declaration; the identity is wholly derived, so nothing in it can be forgotten
  invalidating_a_meaning_change: a deployment concern, per api:cache-store, which already states the runtime never invalidates entries
  contrast_with_the_component_cache: decision:cache-key-derivation versions on a fingerprint of the emitted instruction list, which is derived too; neither cache asks an author for a version
key_source:
  purpose_built: a struct written to be a key, with every participating field marked
  entity: a storage entity passed as-is, with the key picked out of the fields marked inside it
  consequence: one struct may carry storage tags and cache tags at once, so the two vocabularies share a type and must not collide
cardinality:
  rule: one struct yields one key; the key is composite when several fields are marked
  decided: 2026-08-13 by the owner
  second_key_for_one_entity: define a second struct; an entity going into more than one cache store is not one type with two keys
  why_the_rule_earns_its_restriction: identity is derived from the package path and type name and is unique by construction only while the relation is one-to-one; a type carrying several keys would need a key name in the identity, and the author would be restating something again, which is the property the ask asked for by name
  what_it_keeps_in_the_emitted_shape: one method named CacheKey, so the struct satisfies the runtime interface directly and the per-type assertion the ask requested stays meaningful
  rejected_alternative:
    what: named keys in the tag, emitting CacheKey<Name> returning a wrapper type that carries the interface
    cost: the identity stops being derivable, every call site gains a method name to pick, and a typo in a key name is a different cache rather than a compile error
field_inclusion:
  rule: only a marked field is in the key; everything else is excluded
  decided: 2026-08-13 by the owner, opt-in, reversing the ask and the dynamobind polarity the ask cited
  why_the_ask_s_asymmetry_does_not_decide_it:
    what_the_ask_assumed: a purpose-built key struct, where every exported field is plausibly a dependency and an extra one costs only a miss
    what_the_owner_wants_to_pass: an entity struct as-is, with the key picked out of it
    what_breaks: an entity's fields are mostly the result rather than the query, so default-include puts the payload in the key, and the key could then only be built from the value the lookup exists to avoid fetching
    so: over an entity struct default-include is not slow instead of wrong, it is unusable; the asymmetry never reaches this input shape
  precedent_now_followed: firestorebind, which skips an untagged exported field; dynamobind's opposite polarity serves a case where every field is storable and none of them is a query
  what_the_reversal_costs: a new dependency added to the fetch and left unmarked is still silently wrong, so generation no longer closes that hole; it keeps the framing correct and the identity prefix present, which are the other two
  safeguard:
    rule: a struct reached as a key with no marked field is a generation error, never an identity-only key
    confirmed: 2026-08-13 by the owner
    why: an identity-only key gives every instance of one type a single shared entry, so the first caller's result answers every later one; that is the wrong-answer failure rather than a cold cache
    what_it_is_not: an untagged struct that is never passed as a key is not an error, it is simply not a key type; the error fires on reaching, which is why discovery has to see the call site and not only the declaration
    covers_the_migration_case: an entity already tagged for storage and passed to the cache before anyone marks a cache field fails loudly instead of silently collapsing to one entry
  order: declaration order, matching decision:cache-key-derivation parameters
  reorder_hazard: moving a marked field changes the key, so a reordered struct goes cold; cold rather than wrong, and recorded so it is not diagnosed as a bug
framing:
  helpers: the decision:cache-key-derivation framing rule, length-prefixed so concatenated fields cannot alias
  covered: ~string, ~bool, every signed and unsigned integer width, ~float32 and ~float64, []byte, time.Time, pointer through KeyOptional, slice through KeyArray
  gap_closed_on_implementation: htmlbind's KeyInt takes int and its KeyFloat takes float64, neither generic over its underlying kind; cachekeybind's are generic over every width, and KeyUint is new, so a named int type or an int64 needs no conversion
  unframeable_type: a generation error, never a skipped field, because a skipped field is the silent failure this requirement exists to remove
  rejected_shapes: struct and map fields, which have no framing; the diagnostic points at keying on the fields they are derived from instead
discovery:
  usage_directed: through api:generator-call-registration, since the call consuming a key is a downstream symbol rather than one of this module's; the key is an argument type at a fixed position, which data:generator-call-pattern already selects
  tag_gate: a marked field always exists under opt-in, so the hasDynamoTag-style gate transfers verbatim and generate-all can serve this
  resolved_by_the_reversal: under the default-include the round asked for, a key type with no excluded field would carry no tag at all and nothing would distinguish it from any other struct; the opt-in the owner chose for the entity case restores the marker as a side effect
collision:
  rule: a type already declaring CacheKey is a generation error at the declaration
  precedent: the dynamobind method collision check, which excuses declarations in files the run skipped so a regenerated codec does not refuse its own output
constraints:
  - decision:reflection-free holds; every encoder is statically typed generated code
  - equal keys imply equal marked field values and the same declaring type
acceptance:
  - a marked field added to a key struct changes the emitted key with no method edit
  - an unmarked field added to a key struct does not change the emitted key, so an entity gains payload without going cold
  - two key types with equal marked values produce different keys
  - a struct reached as a key with no marked field fails generation rather than emitting an identity-only key
  - a field type with no framing fails generation rather than being dropped
  - a type already declaring CacheKey fails generation rather than being overwritten
  - one entity cached under two queries is two structs, and their keys differ by type name without either author declaring a name
  - a field carrying both a storage identity tag and a cache mark generates without a diagnostic
storage_tag_overlap:
  rule: a marked field that is also a storage identity field is the author's call; nothing checks the two key sets against each other
  decided: 2026-08-13 by the owner
  why_a_check_would_be_wrong: the two sets answer different questions, so they legitimately differ in both directions; a cache key is narrower when a list is cached by partition key alone, and wider when it carries a query parameter that is never stored, such as a page number
  what_a_check_would_do: fire on correct code, which is worse than not checking, because a diagnostic an author learns to dismiss stops being read
  architectural_payoff: cachekeybind never parses `dynamo` or `firestore` tags, so it carries no knowledge of any storage dialect and a new storage package needs no change here
helper_home:
  raised_as: tidiness, explicitly not a problem the reporter has, since it re-exports the helpers from its own runtime
  observation: an application caching an upstream JSON call imports an HTML render runtime to frame an integer
  option_offered: the helpers move to cachekeybind and htmlbind aliases them
  floor_stated_by_the_reporter: if it costs anything at all, leave them
  decided_on_implementation: cachekeybind frames its own; htmlbind is untouched
  why: forwarding htmlbind's helpers would add an inter-package dependency to a shipped render runtime, which costs more than nothing, and the floor said to stop there
  drift_is_not_a_risk_here: the two serve different caches with different identity spaces, so no key ever has to be equal across them; only the framing rule is shared, and it is stated in both
  what_the_split_bought: cachekeybind's helper set is wider than htmlbind's on integer and float widths, which it could not have been as an alias
implemented:
  when: 2026-08-13
  runtime: cachekeybind, with CacheKey and the framing helpers, no dependency beyond stdlib
  tag: `cache:"key"` marks a field, and is the only value the tag takes
  emitted: one CacheKey method per type, plus the per-type interface assertion
  discovery_seam: OperationCacheKey and CacheKeyCall, whose key role reads api:generator-call-registration ArgumentType, since a memo call is generic over its result and the key is the value beside it
  feature_flag: cache-key, so generation is switchable like every sibling
  output_file: cachekeybind_gen.go, wired into GeneratePackage beside the dynamo and firestore passes, so the CLI emits it rather than only the direct entry point
  generated_code_is_compiled_in_test: the emitter picks a generic helper per go/types kind, and only building the output proves the two agree
  emitter_has_no_fallback: EmitCacheKeys is exported, so a hand-built plan can bypass the collector; an unplannable kind and an identity-only plan are errors there too, because every fallback available is a key missing a field
  cross_package_key: unsupported, as in dynamobind; discovery reports a bare type name and the method has to be declared in the type's own package, so the generator runs where the type is
  interface_typed_key_parameter: a framework declares the parameter as the runtime interface, and discovery still reads the argument expression's static type rather than the declared one, so each key type arrives as itself; tested, because the framework-owner guide documents that shape
documented:
  package_guide: docs/cachekeybind.md and its ja pair
  framework_owner: the httpbind framework-owner guide gains the ArgumentType case, which had no worked example; GenericType was the only role source shown and it cannot express a key beside a generic result
  stale_example_fixed: the update-surface guide called htmlbind.Bind and htmlbind.BindWrapper, which decision:generic-method-migration deprecated in the same change; it now spells the methods and states that the functions still work
```
