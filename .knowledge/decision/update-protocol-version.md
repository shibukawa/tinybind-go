---
id: decision:update-protocol-version
type: decision
title: Update Protocol Version
---
Version the wire contract as one framework-owned integer, and treat any mismatch as a complete document rather than an error.

```yaml
superseded_2026_08_04:
  by: decision:caller-owned-wire-versioning
  what_is_reversed: ownership, comparison, and validator_binding; the module stops deciding a version, stops comparing one, and stops mixing one into a digest
  what_survives: the mismatch behavior, because serving a complete document rather than an error was never a versioning rule but the requirement:client-update-rollout fallback invariant
  replacement_axis: the build identity, already compared on every update request and already caller-overridable through Options.BuildID
  read_the_rest_as: the reasoning that made a single framework-owned integer the right answer while the module also shipped the client
source:
  - requirement:render-mode-negotiation
  - user protocol question 2026-08-01
identifies:
  - decision:update-manifest-transport attribute names
  - data:component-update-manifest field set and its header encoding
  - data:component-delta-response operation kinds and body shape
  - rule:update-validator-computation construction, including keying and length
  - requirement:render-mode-negotiation header names and mode spellings
does_not_identify:
  application_templates: a template edit changes the component version inside a validator, never this number
  validator_key: a key rotation invalidates comparisons without changing the contract
format:
  shape: single monotonic integer
  rejected_semver: there is no independently compatible minor axis; a client either speaks the contract or it does not
  wire: mode header value 'mode;v=N', echoed on the response
ownership:
  owner: framework constant, not a project option, because it names a contract rather than a namespace
  contrast: the attribute prefix and header prefix are namespaces and stay configurable
comparison:
  rule: exact equality between client and server
  mismatch: serve a complete document; never an error status
  rolling_deploy: a page rendered by the old version whose next request reaches a new server falls back cleanly, which is why the mismatch path must be ordinary rather than exceptional
  future_range: a server may later accept a set of versions; the mismatch behavior does not change
validator_binding:
  rule: the version is mixed into every digest
  reason: two versions can never accidentally compare equal, even if a client ignored the header
bump_when:
  - an attribute name, its meaning, or the single-root rule changes
  - an operation kind is added, removed, or given new semantics
  - the manifest field set or its encoding changes
  - the validator algorithm, keying, canonical input encoding, or digest length changes
  - the record framing of requirement:streaming-delta-response changes
no_bump_when:
  - a template, component, or application route changes
  - the validator key rotates
  - a configurable prefix changes, which is a deployment mismatch rather than a protocol one
runtime_agreement:
  rule: the browser runtime hardcodes the same integer, per requirement:html-runtime-bootstrap
  guard: a test asserts the shipped runtime and the server agree, because a silent disagreement disables every update without failing anything
```
