---
id: requirement:redirect-error
type: requirement
title: Redirect Carrying Error
---
Let an error value carry a redirect target, so a handler that returns rather than writes can still send the browser elsewhere.

```yaml
priority: should
source:
  - requirement:colocated-route-logic
  - user redirect decision 2026-07-27
review_gate: approved and implemented 2026-08-14
problem:
  shape: a rung 2 func Page from decision:route-handler-shape returns values and an error; it holds no http.ResponseWriter
  need: a page that must send the browser somewhere, such as an unauthenticated visitor going to a sign-in page, has no way to express that
  today: only rung 3 can redirect, which forces a whole page down a rung for one branch
model:
  channel: the existing error return, because a redirect and an error both end the normal render before it starts
  not_an_exception: the value is an ordinary error; nothing panics and no control-flow exception is thrown
  contrast: React server functions redirect by throwing a framework-handled control-flow exception, which Go has no idiom for
surface:
  constructor: a status helper beside the ones in concept:error-helpers, taking a target and a redirect status
  status: 303 by default for a page, with 301, 302, 307, and 308 available
  payload: the target location, plus the optional data:problem fields the other helpers already carry
  identity: discovered by the generator through rule:go-types-symbol-identity, like every other error constructor
behavior:
  write_path: api:write-error recognizes the redirect value and emits the status with a Location header instead of a problem document
  ordering: it runs before any byte is written, so the status is still free
  body: none required; a short body is allowed for non-browser clients
  logging: the same diagnostics as any other returned error, so a redirect is observable
not_an_error_semantically:
  tension: a successful redirect travels through the error return, which reads oddly
  why_accepted: the alternative is a second return value on every page function to express something almost no page uses
  precedent: the same tradeoff the framework ecosystem makes when redirect is a thrown value rather than a returned one
  mitigation: the constructor name says redirect, so a reader at the call site sees intent rather than failure
scope:
  applies_to: any code path that returns an httpbind error, so a flat-mode handler gains it too
  implemented_2026_08_14:
    value: Redirect(target, status...) beside the other status helpers, carrying a Location on the shared error value; a status outside 301, 302, 303, 307, and 308 is refused where it is written rather than emitted for a client that will not follow it
    write_path: both WriteError surfaces recognize it before building a problem document and emit the status with a Location and no body, kept pair-wise so the net/http and fasthttp surfaces stay symmetric
    reached_from_three_places: a page function's return, a handler's return, and a template's failing external, which all express a redirect as the same value
  widened_2026_08_14: decision:value-binding-hoisting adds a second position — a chain member's top-level value binding, whose failing external returns the same value; the constructor, the ordering rule, and the api:write-error behaviour are unchanged, only where the value may come from
  why_it_transfers: requirement:template-value-binding propagates a failing external's error unwrapped, so the value arrives intact, and hoisting is what keeps the ordering rule above true for it
  openapi: a redirect status participates in rule:openapi-error-statuses like the other helpers, for documented API routes
  server_functions: requirement:template-server-functions handlers already hold a writer and use http.Redirect directly; this changes nothing for them
constraints:
  - the target is a value the application supplies, never taken from unvalidated request input by the framework
  - an open-redirect check is the application's responsibility, and the documentation must say so
  - no reflection; the constructor and its type are resolved at generation
acceptance:
  - a typed page function returning a redirect error sends the browser to that target with no page rendered
  - the response carries the chosen status and a Location header, and no problem document
  - the same value returned from an ordinary flat-mode handler behaves identically through api:write-error
  - an existing error path is unchanged
open_questions:
  - the constructor name, and whether one helper covers every redirect status or each status gets its own
  - whether a relative target is resolved against the current request or required to be absolute
  - whether the same value may be returned from a layout function, and what that means for a partially assembled chain
```
