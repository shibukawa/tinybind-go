---
id: rule:transform-rewrite-table
type: rule
title: Transform Rewrite Table
---
The substitutions that turn an admitted net/http function into its fasthttp form: one context replaces both transport values, transport slots drop from every recognized call, and only enumerated selectors are rewritten.

```yaml
status: implemented 2026-08-08
applies_to: functions admitted by rule:transform-eligibility
as_built:
  entry: RewriteTransform over the analysis plan, returning the generated source and the layout warnings
  mechanism: position-keyed textual edits over the original bytes, not a mutated syntax tree, so comments and formatting survive and the loaded AST stays usable by the other phases that share one type check
  uniform_collapse_rule: in a signature and in a call alike, the first transport position becomes the context and the remaining ones are deleted with their separator; Bind, Write, WriteStatus and a transitive local call all follow it
  context_identifier: chosen per function, avoiding names the function already uses, so a handler with its own ctx is not shadowed
  imports: derived from the references that survive the edits, which is why net/http drops out when the transport types were its only use and stays when a status constant still reads it
  verified: the emitted source is compiled with -tags fasthttp against the real runtime, and the authored half is compiled untagged in the same test
signature:
  rule: the writer and request parameters collapse into one context parameter in the first transport position; other parameters keep their order and types
  examples:
    - "func h(w http.ResponseWriter, r *http.Request) -> func h(ctx *fasthttp.RequestCtx)"
    - "func h(w http.ResponseWriter, r *http.Request, id string) -> func h(ctx *fasthttp.RequestCtx, id string)"
    - "func renderError(w http.ResponseWriter, r *http.Request, err error) -> func renderError(ctx *fasthttp.RequestCtx, err error)"
  one_sided: a function taking only a writer or only a request still yields exactly one context parameter
  method_receiver: a ServeHTTP method keeps its name and receiver and takes the context; it no longer satisfies http.Handler, which nothing in a tagged build asks it to
  identifier_capture: the introduced name must not shadow or collide with an existing identifier in scope, so it is allocated fresh rather than fixed to ctx
body:
  identifier: every admitted occurrence of the writer and of the request becomes the context identifier
  significance: both map to one value, which is why arity collapses as a consequence of substitution rather than as a modelled change
runtime_calls:
  rule: the writer and request arguments are removed from the call, and remaining arguments keep their order
  examples:
    - "httpbind.Bind[T](r) -> httpbind.Bind[T](ctx)"
    - "httpbind.Write[T](w, r, v) -> httpbind.Write[T](ctx, v)"
    - "httpbind.WriteStatus[T](w, r, status, v) -> httpbind.WriteStatus[T](ctx, status, v)"
    - "httpbind.WriteError(w, r, err) -> httpbind.WriteError(ctx, err)"
    - "httpbind.WriteStream[T](w, r, fn) -> httpbind.WriteStream[T](ctx, fn)"
  names_unchanged: decision:backend-build-tag-mode keeps the fasthttp declarations under the net/http names, so only argument lists move
  framework_calls: the same removal, driven by the transport slots the registered call pattern declares
transport_slots_2026_08_08:
  status: implemented
  shape: CallPattern carries a TransportSlots value naming the writer and request argument indices, set through the WriterArgument and RequestArgument options
  defaults: Bind declares request 0; Write, WriteStatus, NewStream and WriteStream declare writer 0 and request 1
  scoped: slots attach only for the HTTP runtime path, because the canonical call names are spelled once per runtime package and a same-named function elsewhere takes no transport
  validated: a slot index cannot be negative, the two cannot name one argument, and an argument cannot both be dropped and supply a value or type role
  queried_by: TransportSlots.Drops, which is what the rewriter asks per argument
transitive_calls:
  rule: a call to another admitted function drops the same arguments, matching that function's rewritten signature
selectors:
  policy: an enumerated table; a selector absent from it refuses the function rather than receiving a guessed equivalent
  seed:
    - "r.Context() -> ctx, valid because RequestCtx satisfies context.Context"
  growth: each addition is a named entry with its own justification, so the table's size is the visible measure of how much net/http semantics the transform claims to reproduce
  deliberately_absent: r.Method, r.URL, r.Header, r.RemoteAddr, r.TLS, and http.SetCookie; each has a plausible fasthttp spelling and a semantic difference worth deciding one at a time rather than in a batch
imports:
  rule: the generated file's imports are derived from the rewritten body, not copied from the source file
  runtime_path: the fasthttp runtime package is imported under the httpbind alias, per decision:backend-build-tag-mode, so no call selector in the body changes
  configurable_2026_08_08:
    field: TransformOptions.ImportRewrites, a map from an authored import path to the path the generated file imports instead
    local_name_preserved: the selector text in a rewritten body never moves; only the import line does
    not_built_in: the default carries the module's own runtime pair and nothing else, so a framework shipping the same helper names over the other transport registers its own pair and its calls are rewritten like the built-in ones
    validated: both paths required, and a path may not map to itself
  consequence: net/http drops out unless a rewritten expression still names it, which is what keeps rule:transport-dead-code-elimination true by construction here
lifetime_check: no rewrite may produce a value that outlives the handler, per rule:fasthttpbind-requestctx-lifetime; a rewritten context stored or captured is a refusal, not a rewrite
related:
  - rule:transform-eligibility
  - decision:transport-source-transform
  - api:fasthttpbind-bind
  - api:fasthttpbind-write
  - api:write-stream
```
