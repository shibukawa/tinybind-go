---
id: decision:cache-key-version-declaration
type: decision
title: A Cache Key Declares No Version
---
Drop the key version entirely, because it closes no failure, and invalidating on a meaning change is a concern this module has already refused to hold.

```yaml
source:
  - downstream framework change request 2026-08-13, sub-question delegated to the owner
  - owner reversal 2026-08-13, during implementation
  - requirement:cache-key-generation
review_gate: proposed
decided: no version; the identity is wholly derived as `<package path>.<TypeName>`
history:
  first_decision: a marker field, `_ struct{} `cache:"version=2"``, chosen over a companion const and an optional method
  reversed_same_day: by the owner, before release, on the ground that a bump only makes the cache miss and the machinery is not worth it
  what_the_reversal_added_to_the_owner_s_reason: the module had already refused this concern elsewhere, which makes it a consistency question rather than a taste one
why_no_version:
  it_relocates_a_failure_rather_than_closing_one: a version is a number an author must remember to raise, and forgetting it is silent; requirement:cache-key-generation exists to reduce exactly that class, so adding one spends complexity to move a failure rather than remove it
  the_module_already_refused_this: api:cache-store constraints state that the runtime never enumerates or invalidates entries and that expiry is the only removal path it relies on; a version is an invalidation lever, declared in the library, for a policy the library says belongs to the deployment
  the_common_case_is_already_covered: the reporter's own design keys on the build identity beside the version, so a code change already invalidates; the version's residual job is a meaning change with no code change, which is rare
  the_residual_case_has_deployment_answers: namespace the store per build or release, delete the scope prefix as a range, or wait out the TTL
  it_removed_machinery_that_could_fail_quietly: the marker was a blank field, and a blank identifier is unexported, so the collector pattern in use would have skipped it and left every key silently at v1
what_the_removal_costs:
  renaming_is_not_a_universal_substitute: renaming the Go type changes every key and the compiler finds each call site, which serves a purpose-built key struct; it does not serve a storage entity, where the type name feeds the Datastore kind or the table mapping and a rename would repoint storage as a side effect
  so_the_answer_is_deployment_level: recorded in the guides rather than left implicit, because an author who wants a version will look for one
enforcement:
  blank_field_is_an_error: a `cache` tag on a blank `_` field is refused with the reason, rather than ignored; an ignored one would read as a version that silently did nothing
  one_tag_value: `key` is the only option, so every other spelling is a mistake rather than a variant
consequence:
  identity_is_wholly_derived: nothing an author writes reaches the identity beyond the type's own name, which is the property the ask named as its reason for deriving the package path and type name in the first place
  contrast_with_the_component_cache: decision:cache-key-derivation versions on a fingerprint of the emitted instruction list, which is derived too; neither cache asks an author for a version, and the earlier framing that a hand-written body needs a declared equivalent is withdrawn
related:
  - requirement:cache-key-generation
  - decision:cache-key-generator-seams
  - decision:cache-key-derivation
  - api:cache-store
```
