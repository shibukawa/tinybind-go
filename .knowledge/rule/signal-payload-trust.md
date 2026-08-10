---
id: rule:signal-payload-trust
type: rule
title: What A Signal Payload Is Trusted With
---
State what the module guarantees about a signal payload and what it does not, because a signal is the one value on this wire that reaches the client without a projection, an escape, or a render.

```yaml
source:
  - data:signal
  - requirement:live-signal-emission
  - concept:signal-channel
review_gate: proposed
applies_to: server-authored signals; a requirement:runtime-lifecycle-signals name carries only what the client itself observed, so none of this reaches it
the_contrast_that_makes_this_necessary:
  recover_clause: data:async-render-error projects an error down to four public fields, so raw text cannot leak into a page by accident
  boundary_html: rule:template-context-safety checks every rendered fragment in its own buffer before it is yielded
  signal_payload: neither applies; the author names a struct and the module encodes it verbatim
  why_not_project: a projection needs to know which fields are safe, and the module does not know what an author's instruction means; the recover case works because the module defined the type
  therefore: the author owns what leaves the server, and this rule says so rather than leaving it to be inferred from the absence of a check
module_guarantees:
  encoding: valid JSON produced by a generated codec, escaped for a script context as well as a JSON one, so framing it inline is as safe as sending it as a body
  integrity: the payload the source constructed is the payload the record carries; nothing rewrites, merges, or reorders it
  no_inspection: no field is read, so no field can be used for routing, caching, or authorization
author_owns:
  content: every field is transferred as written, so a server-only identifier, an internal error string, or a full record placed in a payload reaches the browser
  size: a payload is transferred per signal and per subscriber, bounded per record by data:signal and in aggregate by the response
  naming: a collision between two application names is the author's, since the module reserves only its own prefix
fan_out_is_the_leak_to_watch:
  premise: decision:live-external-signature fan_out puts sharing one upstream across clients inside the source, because the module renders per client and owns no broadcast topology
  consequence: a source that fans one upstream out to many subscriptions emits each signal to every one of them, so a payload meant for one user reaches all of them
  contrast_with_deliveries: a shared delivery is usually shared data by construction, where an instruction is usually addressed to someone
  rule: a signal carrying anything user-specific must come from a per-subscription source, and a shared source must emit only what every subscriber may see
  not_enforceable: the module cannot tell the two apart, which is why this is a rule rather than a check
cache:
  render_cache: requirement:component-output-cache stores rendered output, and a signal is emitted by the source rather than produced by a render, so a cache hit neither replays nor suppresses one
  no_signal_caching: a signal is never stored, keyed, or reused; it belongs to the subscription that produced it
reserved_names:
  rule: the module's prefix is reserved and enforced at emit, per data:signal reserved_prefix
  why: navigate and reload from data:component-delta-response directives instruct the client to leave or rebuild the page, and the requirement:runtime-lifecycle-signals set is trusted precisely because only the client's own runtime produces it; a name that reached either from application data would be an instruction-injection
logging:
  rule: an unclassified signal reaching a log or an error page must not print the payload, which is why decision:signal-in-the-error-slot error_text keeps it out of Error()
  reason: a signal is an error value, and error values are logged by code that has no idea what this one is
constraints:
  - a payload is data the client parses, never markup the client inserts and never code the client runs, per requirement:client-signal-dispatch
  - a signal crosses no authorization boundary of its own; the subscription's own authorization is the only one, and it ran when the page executed
  - a source must not mutate a payload after yielding it, which holds by construction because data:signal encodes at construction
```
