---
id: decision:websocket-origin-check-seam
type: decision
title: The Origin Check Takes Two Strings
---
Express the origin check as a function of the Origin and Host values rather than of a request, so the default and any application policy live in the transport-free core and each shell only reads its own headers.

```yaml
status: implemented 2026-08-10
problem:
  driver_hooks_name_their_request: "CheckOrigin is func(*http.Request) bool on one transport and func(*fasthttp.RequestCtx) bool on the other"
  effect_if_carried_through: the options value cannot be one type, so a caller writing a policy writes it twice, and the two copies are what drift
chosen:
  shape: "func(origin, host string) bool, held in the transport-free options"
  each_shell: reads the two headers its own way and calls the shared function, exactly as its upgrader hook demands
  precedent: "bindcore NegotiateStream(streamQuery, accept, userAgent string) already does this — the decision lives in one place and each transport supplies strings"
default:
  behaviour: refuse when Origin is present and its host differs from Host; admit when Origin is absent
  same_as: the driver's own default on both transports, so adopting this changes no security posture
  why_not_permissive: a socket that accepts any origin is cross-site request forgery with a persistent connection, and a default that has to be tightened is one nobody tightens
what_the_two_strings_cannot_express:
  - a policy reading a cookie, a token, or the path
  - a policy consulting per-tenant configuration keyed by something else in the request
  answer: authenticate before the entry and capture the result, which the callback needs anyway because rule:fasthttpbind-requestctx-lifetime forbids reading the request after the handler returns
  effect: the origin check stays an origin check, and authorization stays where the application already does it
refusal_response:
  shape: policy:problem-details, written through the driver's Error hook
  why_the_hook: both upgraders write their own error response before any 101, so the only way to shape it is to install the hook
  consequence: a refused handshake looks like every other refusal in the application rather than like gorilla's plain text
related:
  - decision:websocket-lifecycle-ownership
  - api:websocket-entry
  - policy:problem-details
  - rule:fasthttpbind-requestctx-lifetime
  - system:tinygodriver-websocket
```
