---
id: requirement:effective-config-brevity
type: requirement
title: Effective Config Brevity
---
The effective-config output must let a reader find what this deployment actually decided, which needs two separate levers rather than one.

```yaml
priority: should
intent: a reader answers "what is in force here" from the output without filtering it by eye
measured_case:
  source: one Popcorn Wave deployment, session.backend=redis and auth.mode=oidc_only
  lines: 189 rendered, 150 carrying a value
  from_file_or_env: 44
  from_default: 106
two_levers:
  inapplicable_keys:
    lever: decision:dependon-value-condition
    removes: a subtree the selected variant did not select
    measured: 21 of 150 value lines in the case above
    subtrees: auth.jwt 12, auth.passkey 3, session.rdb 2, session.dynamo 2, session.firestore 1, session.cookie_store 1
    kind_of_problem: correctness
    why: >
      an inert setting printed beside a live one reads as in force; a reader who
      trusts auth.jwt.leeway under mode=oidc_only has been told something false
    state: implemented
  unremarkable_keys:
    lever: decision:summary-tag-form
    removes: a key rated as detail whose winning Place is default
    ceiling: 106 of 150 value lines in the case above
    measured_first_pass: 20 struct-level tags reach 73 of those 106
    kind_of_problem: volume
    why: >
      the dominant mass is defaults, not inapplicable keys, so the value-condition
      lever alone cannot make this output short
    kind_of_change: >
      an author's tag rating the key, and a caller's choice of surface; not a
      library-side filter, per rule:summary-key-omission
    state: implemented
    correction: >
      an earlier sketch here made this "suppress every key whose Place is default",
      chosen per invocation with no tag. That hides settings a reader wants, because
      an applicable key at its default is often exactly what they came to read. The
      author's rating is what separates the boring default from the load-bearing one
why_both:
  - the levers cut along different axes and neither subsumes the other
  - an inapplicable key set from a file survives the default filter and still misleads
  - an unremarkable key is not an inert one; it applies, and a dump still prints it
shared_safety_property:
  statement: neither lever ever removes a value a source set
  dependon: hides a subtree by the parent's value, and the subtree is inert whoever set it
  summary: requires Place default, so any key a file or env set survives
ownership:
  library_side: >
    the generated definitions, the dependon filter, and the omittable classification
    on each record
  render_side: >
    which surface is being drawn, and therefore whether omittable records are
    skipped; the tree itself is drawn downstream
non_goals:
  - a shorter output at the cost of omitting a key that is in force
  - hiding a key from scaffolds, which must stay discoverable before any load
  - a fixed key allowlist maintained by hand
surfaces:
  boot_summary: the short one; skips omittable records
  config_dump: the complete one, in the shape docker inspect answers for a container
  neither_shows: a key hidden by rule:dependent-key-visibility or rule:secret-redaction
related:
  - decision:dependon-value-condition
  - decision:summary-tag-form
  - rule:summary-key-omission
  - requirement:dependent-field-visibility
  - requirement:source-provenance-logging
  - rule:dependent-key-visibility
  - rule:secret-redaction
  - api:configbind-provenance
  - concept:provenance-log-helper
  - data:provenance-event
  - term:config-source
  - system:configbind
acceptance:
  - 'session.backend=redis leaves no session.rdb, session.dynamo, or session.firestore line in the output'
  - 'auth.mode=oidc_only leaves no auth.passkey or auth.jwt line in the output'
  - session.cookie and session.keyring stay, because they serve every server backend
  - a key set by file or env is never removed by either lever
  - 'a summary:"omit" subtree with one file-set leaf still prints that leaf and nothing else of the subtree'
  - the dump surface prints every omittable key, from the same one Provenance call
  - an untagged key at its default appears on both surfaces
```
