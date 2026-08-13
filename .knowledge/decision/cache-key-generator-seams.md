---
id: decision:cache-key-generator-seams
type: decision
title: Cache Key Generator Requests From The Downstream Framework
---
Accept the tenth downstream round: one blocking generator ask whose stated precedent is half right, one tidying available today, one inventory the reporter asked not to be acted on, and the first round arriving with a built implementation behind it.

```yaml
source:
  - downstream framework change request 2026-08-13, against v0.5.8
  - decision:framework-integration-seams
  - decision:cache-scope-seams
review_gate: proposed
round:
  when: 2026-08-13, the tenth round from this reporter
  previous: generation seams 2026-07-30, live integration 2026-07-31, component and asset seams 2026-07-31, runtime ownership 2026-08-01, composition seams 2026-08-02, caller-owned runtime 2026-08-04, partial transfer 2026-08-08, cache scope 2026-08-09, client handler 2026-08-11
  shape: three asks and one section offered as input rather than as a request
  reporter_position: ask 1 is blocked downstream and nothing else is; ask 2 needs no language change; ask 3 must not be acted on until its trigger fires
  first_named_reporter: this round writes the project name and module path, Popcorn Wave at github.com/shibukawa/popcornwave, which earlier rounds left unqualified
  what_is_new_in_its_shape: the reporter shipped a data result cache before asking, so the asks arrive as residue from a built thing rather than as a design under review; the one hole it could not fill downstream is the only blocking ask
disposition:
  ask_1: accepted as requirement:cache-key-generation and implemented 2026-08-13, with three changes; the default-include half is refused, one struct yields one key, and the version sub-question is answered by removing the version per decision:cache-key-version-declaration
  ask_2: accepted and implemented 2026-08-13, in decision:generic-method-migration available_today
  ask_3: filed unacted, in decision:generic-method-migration deferred; nothing on that list was touched
  helper_relocation: open, at the priority the reporter assigned it
  input_section: folded into requirement:component-output-cache open_questions
verification:
  method: every claim read against v0.5.8, which is the version the round names
  framing_helpers_are_the_named_set: confirmed at htmlbind/cache.go; KeyString over ~string, KeyBytes, KeyBool, KeyInt, KeyFloat, KeyTime, KeyOptional, KeyArray
  scope_prefix_position: confirmed; cacheKey writes KeyString(scope) ahead of KeyString(c.ID), so the prefix reuse the reporter claims is real
  three_entries_carry_no_extra_type_parameter: confirmed at the three cited positions; Require, Bind, and BindWrapper are `[P any]` and introduce nothing beyond the receiver's own
  loadtx_comment_gives_two_reasons: confirmed at firestorebind/tx.go; the language is one and the context-carried handle is the other, and only the first is what a language change removes
  on_entry_inventory: confirmed; all twelve dynamobind names and all eleven firestorebind names exist as listed
  scanrows_takes_an_interface: confirmed; sqlbind/statement.go declares Rows as an interface, so the permanent exclusion holds
  jsonbind_parser_shape: confirmed; ParseSlice and ParseMap are functions over *Parser beside its methods
  method_assertion_precedent: confirmed; both dynamobind and firestorebind emitters write `var _ pkg.Interface = Type{}`, which is the assertion shape ask 1 asks for
  usage_directed_emission: confirmed; a Usage bitset per type selects which methods are emitted, filled from call sites through discoverGenericTypeArgs
two_corrections_to_the_reporter_s_reading:
  the_stated_precedent_is_half_right_and_the_half_that_holds_is_the_useful_one:
    reported: default-include with `-` to exclude is the same shape the dynamo tag already has
    actual_dynamo: correct, and stronger than the reporter could see; an untagged exported field falls through to a plan carrying its Go name, and only `-` skips it
    actual_firestore: the opposite; an untagged exported field is skipped outright, so the sibling package is opt-in
    consequence: the module already carries both polarities, so the ask is not a divergence from house style but a choice of which sibling to follow
    resolved: firestorebind, per the owner's reversal below; the precedent question turned out not to decide it either way
    reading: the reporter argued from a precedent it had only half checked and happened to name the half that supports it, and the half it named lost anyway
  default_include_removes_the_discovery_marker_the_precedent_relies_on:
    finding: dynamobind's generate-all is gated by hasDynamoTag so an unrelated struct does not acquire a codec it never asked for
    why_it_would_not_have_transferred: under default-include a key type with no excluded field carries no `cache` tag at all, so nothing would distinguish it from any other struct
    consequence_under_the_ask: discovery would have been usage-directed only, and generate-all could not serve cachekeybind
    dissolved_by_the_reversal: opt-in puts a marked field on every key-bearing struct, so the gate transfers verbatim
    not_in_the_report: the reporter asked for the tag polarity and the emitted shape and did not reach what the polarity costs discovery
what_the_owner_decided_beyond_the_ask:
  field_inclusion_reversed_to_opt_in:
    asked: every exported field in the key by default, with `cache:"-"` to exclude, argued from the failure asymmetry
    decided: 2026-08-13; only a marked field is in the key
    the_input_shape_the_ask_did_not_carry: the owner wants an entity struct passed to the cache as-is, with the key picked out of the fields marked inside it, rather than a struct written to be a key
    why_that_settles_it: an entity's fields are mostly the result rather than the query, so default-include puts the payload in the key and the key could only be built from the value the lookup exists to avoid fetching; over this input default-include is unusable, not merely slow
    why_the_asymmetry_does_not_survive_the_move: it weighs a wrong answer against a miss, and both outcomes assume every field is a candidate dependency, which is true of a purpose-built key struct and false of an entity
    what_it_costs: the ask's third and most serious problem, a field added and forgotten, is only closed by default-include; under opt-in the author still declares the dependency set by hand
    what_is_kept: the identity prefix and the framing, both wrong-answer failures, stay closed by generation
    new_safeguard: a struct reached as a key with no marked field is a generation error, since an identity-only key would give every instance of one type a single shared entry
    unasked_payoff: it restores the tag gate the second correction above found missing, so generate-all can serve cachekeybind after all
    to_report_back: the reporter argued this ask at length and is owed the entity-struct reason rather than a bare refusal
  one_key_per_struct:
    decided: 2026-08-13; a struct yields one composite key, and an entity wanted in a second cache store gets a second struct
    considered: named keys in the tag, so one entity could carry a marked subset per query
    why_it_was_dropped: identity is derived from the package path and type name and is unique by construction only while the relation is one-to-one; named keys would put a name back in the identity and hand the author something to restate, which is the property the ask named as the reason for deriving it
    keeps_the_ask_s_emitted_shape: one method named CacheKey, so the struct satisfies the interface directly and the per-type assertion the ask asked for stays meaningful
  empty_key_is_an_error:
    confirmed: 2026-08-13; a struct reached as a key with no marked field fails generation
    fires_on_reaching_not_declaring: an untagged struct never passed as a key is not a key type and not an error
    why_it_matters_most_under_opt_in: default-include could not produce an empty key, so this is the hole the reversal opens and the check is what closes it
  the_version_is_removed_rather_than_spelled:
    delegated: the reporter offered three spellings and stated no preference strong enough to request
    first_answer: the marker field, chosen because the two alternatives were a name convention that drifts and a method whose stated weakness was being easy to forget
    reversed_same_day: by the owner, before release; all three spellings share the defect the third was rejected for, which is that a version is remembered by hand
    decided: no version at all, per decision:cache-key-version-declaration
    the_argument_that_settles_it: api:cache-store already states that the runtime never enumerates or invalidates entries, so a version is an invalidation lever declared in the library for a policy the library says belongs to the deployment
    and_the_reporter_already_covers_the_common_case: its own design keys on the build identity beside the version, so a code change invalidates without one
    what_the_owner_owes_the_reporter: the residual case, a meaning change with no code change, now has deployment-level answers written into the guides rather than left implicit
  framing_gap_the_report_did_not_price:
    reported: the field types needed are exactly the ones the existing helpers frame, naming integers and floats in the plural
    actual: KeyInt takes int and KeyFloat takes float64, neither generic over its underlying kind, unlike KeyString which is generic over ~string
    consequence: a named int type or any other integer width has no helper, so the covered set is narrower than the ask assumes and widening it is work the ask does not carry
  blank_field_hazard_that_the_removal_retired: a version marker would have been a blank field, and a blank identifier is unexported, so the collector pattern in use would have skipped it and left every key silently at v1; found while implementing, and it is one reason the removal was easy to accept
accepted_unchanged:
  - the runtime interface is one method returning a string
  - the identity's package path and type name are derived and never restated
  - an unframeable field type is a generation error rather than a skipped field
  - the three deferred-list entries that are available today move now, with the existing functions kept as deprecated wrappers
what_the_reporter_contributed_without_being_asked:
  answered_an_objection_rather_than_overruling_it: the refusal to coalesce rests on one request's cancellation reaching another's work, and the reporter ran the shared fetch on a context with cancellation removed and values kept
  disclosed_its_own_weakest_part: the build identity plus a hand-declared version is named by the reporter as the weakest part of its design, which is what makes ask 1 the blocking one rather than a convenience
catalog_defect_found_while_numbering:
  what: decision:client-handler-seams calls itself the sixth round on 2026-08-11 and lists five previous rounds
  actual: by date it is the ninth, and decision:cache-scope-seams already called 2026-08-09 the eighth
  cause: its previous list follows the update-* chain alone, omitting caller-owned runtime, partial transfer, and cache scope
  effect: the count only; no claim in that entry depends on it
  disposition: recorded, not edited, because it belongs to another round's record
related:
  - requirement:cache-key-generation
  - decision:cache-key-version-declaration
  - decision:generic-method-migration
  - requirement:component-output-cache
```
