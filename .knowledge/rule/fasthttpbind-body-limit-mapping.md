---
id: rule:fasthttpbind-body-limit-mapping
type: rule
title: Per-Route Body Limit And Timeout Mapping
---
Per-route body limits are a security control, so generation must enforce them before the body is read, through the fasthttp HeaderReceived hook rather than through a check inside the binder.

```yaml
status: proposed 2026-08-08
decided_2026_08_08: implement the limit ourselves; a server-wide setting is not a substitute for a per-route control
discovered_in_ir: WrapperMeta carries MaxRequestBodyBytes and Timeout per route, from rule:wrapper-max-bytes-handler and rule:wrapper-timeout-handler
why_a_binder_check_is_not_the_control:
  fact: fasthttp reads the whole body into memory before the handler runs, unless Server.StreamRequestBody is set
  consequence: a limit checked inside the binder rejects a request whose bytes are already buffered, so it enforces policy and not the memory bound the limit exists for
  reading: recording a binder check as the answer would record a false guarantee
mechanism:
  hook: Server.HeaderReceived func(*RequestHeader) RequestConfig
  when: after the request header is read and before the body, which is the only point where the read can still be bounded
  field: RequestConfig.MaxRequestBodySize, which overrides the server value for that request alone
  generated: a route-to-limit table plus the hook that resolves one header to one limit
  verified_2026_08_08: present in the tinygodriver fork at server.go, and applied at the read site ahead of Server.MaxRequestBodySize
constraint_this_creates:
  fact: the hook receives only the header, so the lookup runs on method and raw request URI, before the router has matched anything
  consequence: the limit table needs its own matcher over route patterns, duplicating path matching that the router also does
  bounded_by: only routes declaring a limit need an entry, so the table is usually far smaller than the route set
  open: whether the matcher is generated from the same pattern set as the router, or is a coarser prefix match
default_is_not_unlimited:
  value: DefaultMaxRequestBodySize is 4 MiB
  effect: a route that declares nothing is already capped, unlike net/http where an unwrapped handler has no body limit
  reading: this is a tightening, not a loss, but it changes behaviour for large uploads that worked on httpbind
error_body_parity:
  problem: exceeding the limit is refused by fasthttp itself, so the 413 does not come from api:fasthttpbind-write and is not policy:problem-details
  required: install Server.ErrorHandler and render Problem Details for ErrBodyTooLarge, or requirement:fasthttpbind-parity-scope byte parity fails on this path
  note: ErrorHandler is documented to receive ErrBodyTooLarge among others
defense_in_depth:
  keep: a binder-side check as a second gate, for a request that reached the binder with an oversized body because no hook was installed
  status: secondary; never described as the control
alternative_considered:
  shape: Server.StreamRequestBody, which restores a bounded reader and keeps memory flat
  why_not_default: it changes how PostBody and multipart parsing behave, so it reshapes api:fasthttpbind-bind rather than only its limits
  revisit_if: an application needs uploads larger than it is willing to buffer
timeout_still_unresolved:
  net_http: http.TimeoutHandler substitutes a response while the handler keeps running
  fasthttp: RequestConfig.ReadTimeout and WriteTimeout are per request through the same hook, but they bound the transport rather than the handler, so the observable behaviour after expiry differs
  required: emit a diagnostic naming the route when Timeout cannot be reproduced
related:
  - rule:wrapper-max-bytes-handler
  - rule:wrapper-timeout-handler
  - policy:json-read-limit
  - api:fasthttpbind-bind
  - system:tinygodriver-fasthttp
  - requirement:fasthttpbind-parity-scope
```
