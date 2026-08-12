---
id: decision:summary-tag-form
type: decision
title: summary Tag Form
---
A summary tag marks a key as detail rather than headline, so a boot summary may drop it while nobody has set it and a full dump still prints it.

```yaml
status: accepted
state: implemented
form: 'summary:"omit"'
value: one mode; only omit exists today
motivation: requirement:effective-config-brevity unremarkable_keys
problem: >
  the dominant mass of an effective-config output is keys sitting at their
  defaults, which decision:dependon-value-condition cannot touch: they are
  applicable, just unremarkable
conjunction:
  rule: the key is droppable only when it is tagged and its winning Place is default
  why: >
    a tagged key someone actually set is a decision that deployment made, and a
    brevity feature that hides a decision is a bug; the tag rates the key, the
    Place decides whether this run has anything to say about it
consequence: an author may tag a whole subtree without auditing which of its leaves are set
polarity:
  chosen: opt-in omission; an untagged key stays visible
  rejected: opt-out, where an untagged key is droppable
  why: >
    a forgotten tag under the chosen polarity only leaves the output longer, while
    under the other it makes a new field invisible to the operator; this is the
    same over-showing direction rule:secret-redaction and
    decision:dependon-value-condition already take
cost_of_the_polarity:
  measured_on: requirement:effective-config-brevity measured_case
  subtree_tags: 20 struct-level tags cover 73 of the 106 default-sourced lines
  demonstrated: >
    on a module mirroring the real session and auth structs, 5 tags took the boot
    surface from 13 lines to 8, and a struct-level rating still printed the one
    leaf under it that env had set
  leaf_tags: 33 further lines are flat scalars under a prefix and need one tag each
  reading: >
    a first pass of about 20 tags roughly halves the output; the rest is
    diminishing returns the author can take or leave
library_never_decides:
  rule: the tag classifies, it never drops; api:configbind-provenance returns the entry either way
  carried_on: data:provenance-event Omittable
  why: >
    dependon states a fact about the configuration, that a setting is inert, which
    is true on every surface, so the library may drop it. This states a judgment
    about one surface, and the library cannot know which surface a caller is
    rendering
surfaces:
  boot_summary: caller skips entries marked omittable
  config_dump: caller renders every entry it is given
  dump_still_filtered: >
    a dump is not "everything": the inert keys of rule:dependent-key-visibility and
    the hidden keys of rule:secret-redaction never reach the caller on any surface
placement:
  - a leaf field
  - a nested struct field, covering every key of the subtree
  - an array-of-tables field, covering the array and its element fields
  - an array-of-tables element field, resolved by its stable path under the array key
element_fields_allowed: >
  unlike dependon, falsy, and enum, this needs no key of its own to look a parent
  up; it is a property of the key being printed, which is what lets it follow the
  same element-path mechanism rule:secret-redaction already uses
element_fields_inert_today:
  finding: an element field can never be omittable, so the rating resolves and never fires
  why: >
    Definition.Defaults is keyed by stable key and an element has none, so
    mergeDocument builds an element overlay from the file alone; every element
    field that appears was therefore set by a source, and the Place half of
    rule:summary-key-omission never holds
  kept_anyway: >
    the rating costs nothing, since it shares the secret walk, and stays correct
    rather than special-cased if element defaults are ever seeded
  useful_placements_today: a leaf, and a nested struct covering its leaves
  array_field_too: >
    a rating on the array field itself reaches only its elements, because the bare
    array key is never reported, so it is inert for the same reason
validated_at_codegen:
  - a value other than omit is rejected, as an unknown secret mode is
  - a subcommand field is rejected, its own tag or one inherited from a struct above it, because it never reaches provenance
  - every placement has a defined outcome, per requirement:struct-tag-placement-totality
wire_form: 'Definition.Summary map[string]string, keyed and inherited exactly like Secrets'
exported_constant: 'configbind.SummaryOmit, for a caller re-deriving the rating from Definition.Summary'
implementation_note: >
  collectSecrets and this share one walk, collectInheritedModes, rather than the
  near-copy first planned; duplicating a subtree-plus-element inheritance walk is
  where a divergence between the two would have hidden
naming_alternatives_considered:
  - 'verbose:"true"', a boolean, which leaves no room for a second mode
  - 'brief:"omit"', which names the surface less well than the summary it is omitted from
  - 'detail:"true"', which states a category rather than an action
future_not_now:
  - 'summary:"always", for a key that must print even at its default'
  - a diagnostic surface that prints an inert key together with the condition that hid it
scope_unchanged:
  - the bound struct, apply, CLI flags, and validation are untouched
  - scaffolds still list every key; a scaffold advertises what is settable
example:
  go: |
    type ObservabilityConfig struct {
      MinimumLevel string       `default:"info"`
      Query        QueryConfig  `summary:"omit"`
      Trace        TraceConfig  `summary:"omit"`
    }
  effect: >
    the 14 keys under query and trace leave the boot summary while they sit at
    their defaults, and any one of them set in a file comes back
related:
  - requirement:effective-config-brevity
  - rule:summary-key-omission
  - rule:secret-redaction
  - rule:dependent-key-visibility
  - decision:dependon-value-condition
  - decision:struct-field-tags
  - requirement:struct-tag-placement-totality
  - requirement:source-provenance-logging
  - data:provenance-event
  - api:configbind-provenance
  - concept:provenance-log-helper
  - term:config-source
  - system:configbind
```
