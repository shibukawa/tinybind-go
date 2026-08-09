---
id: decision:transport-neutral-handler
type: decision
title: Transport-Neutral Handler Form
---
Give REST handlers a declared form with no transport in the signature, so portable application code is portable by declaration rather than by analysis.

```yaml
status: not the chosen mechanism, 2026-08-08; decision:transport-source-transform is
promoted_2026_08_08:
  fact: decision:backend-build-tag-mode leaves no adapter, so a refused handler has no fallback
  consequence: rewriting into this form is one of the few remedies rule:transform-eligibility can offer an author, alongside hiding the work behind a transport-free function or registering a call pattern
  effect: this form moves from a nice-to-have to the documented escape from a fatal refusal
still_useful_because:
  - a handler in this form is portable with no transform and no adapter, so it is the cheapest case for any backend
  - the discovered router already ships the equivalent at its lower rungs, so the form exists and is proven
  - it remains the answer for a handler the transform refuses and the author would rather rewrite than wrap
correction_to_the_argument_below:
  claim_made: portability cannot be inferred, only declared
  what_holds: a blacklist over behaviour is undecidable, and the reasons listed under why_declaration_beats_analysis are the reasons
  what_does_not: decision:transport-source-transform admits handlers by a whitelist over identifier occurrences, which is decidable and refuses whatever it cannot rewrite, so the objection does not reach it
shape: "func CreateUser(ctx context.Context, in CreateUserRequest) (CreateUserResponse, error)"
generated: the per-transport shim that binds, calls, writes, and maps the error
precedent_already_shipped:
  where: the discovered router
  rung_1: page.tb.html alone, handler fully generated
  rung_2: "func Load(ctx context.Context, id string) (User, error)"
  fact: both rungs are already transport-free, so a backend change costs their source nothing
  gap: no equivalent exists on the REST side, where every handler names ResponseWriter and Request
why_declaration_beats_analysis:
  the_alternative: infer from the handler body whether it touches the request beyond our own calls, and run the safe ones natively
  why_it_fails:
    - parser analyzeBody collects recognized calls and walks past everything else, so absence of an unrecognized call is never established
    - custom middleware unwrapping is best-effort and guesses which argument is the handler
    - a handler reached through a package selector is resolved with no body at all
    - the common escapes are invisible to it: context.WithValue in auth middleware, http.SetCookie, a Flusher assertion, and above all handing r to a third-party library
  cost_asymmetry: today a missed handler yields no binder, which fails at compile time or at init with a named error; the same miss used to authorize a transport swap yields a behavioural difference in production
consequence:
  transport_free_handler: portable across backends, shim generated per transport
  handler_naming_w_and_r: the author has declared ownership of the transport, and that handler stays on its transport or goes through decision:fasthttpbind-adapter-boundary
  no_inference_anywhere: generation never decides portability by looking at a body
scope_note: this form is additive; concept:net-http-handler remains the default and is not deprecated
open_questions:
  - whether the shim registers on the router or is named by an explicit registration call
  - how WriteStatus, streaming responses, and per-handler status codes are expressed without a writer in scope
  - whether the same form should replace rung 3 of the discovered router
related:
  - concept:net-http-handler
  - concept:fasthttp-handler
  - concept:handler-forms
  - decision:fasthttpbind-no-transport-interface
  - requirement:colocated-route-logic
  - requirement:typed-page-context-parameter
```
