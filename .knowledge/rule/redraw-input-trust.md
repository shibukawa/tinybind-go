---
id: rule:redraw-input-trust
type: rule
title: Redraw Input Trust
---
Treat every parameter of a reloadable component as untrusted public input, because registering it publishes an endpoint anyone can call with any values.

```yaml
source:
  - requirement:component-redraw-endpoint
  - user redraw proposal 2026-08-01
change_of_model:
  before: a component's arguments were derived by a page that had already authenticated, authorized, and validated
  after: a registered component is reachable directly, so its declared parameters arrive from the caller
  example: a component taking an identifier and loading that record now answers for any identifier a caller supplies
rules:
  - a reloadable component authorizes its own inputs against the current request identity, exactly as an ordinary handler does
  - the typed decoder validates shape and range; it does not establish permission
  - a parameter that names a resource requires an ownership or visibility check before that resource is read
  - server-only values, request identity, and authorization state are never parameters
  - a component whose safety depends on its caller having already checked something must not be registered
registration_as_review:
  rule: registration is the review point, because it is where a component stops being an internal call and becomes an endpoint
  guidance: a component that only formats values it is handed is safe to register; one that fetches by an identifier needs an explicit check
enumeration:
  risk: sequential or guessable parameters let a caller enumerate content one redraw at a time
  mitigation: the same defenses an ordinary endpoint uses, including authorization, opaque identifiers, and rate limits
side_effects:
  rule: a redraw is a GET and must be repeatable with no observable effect
  reason: it is retried on supersession, may be prefetched, and may be answered from a cache
caching:
  rule: per-user output must not be publicly cacheable
  reason: the URL alone identifies the response, so a shared cache would serve one user's render to another
acceptance:
  - requesting a redraw with another user's identifier is refused rather than rendered
  - an unregistered component is unreachable regardless of its parameters
  - a redraw repeated with identical arguments changes nothing on the server
```
