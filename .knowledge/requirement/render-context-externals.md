---
id: requirement:render-context-externals
type: requirement
title: Render Context For Synchronous Externals
---
Let a synchronous external's Go implementation declare a leading context.Context and receive the render context, so the one external shape that renders inline is not the one shape that cannot read the request.

```yaml
priority: should
source:
  - downstream framework component seam report 2026-07-31
  - requirement:async-external-functions
  - decision:live-external-signature
review_gate: proposed
status: shipped 2026-07-31; the context instructions, the emitter's use of them, the import condition, and the one rejected position are implemented
downstream_id: the reporting framework tracks this as a requirement named render-time-request-context, in that project's catalog rather than this one
problem:
  asymmetry: an async external's implementation may declare a leading context and a live one must, while a sync external is always called as Name(args...)
  consequence: a token or a cookie test travels handler to params to template, every form-bearing page repeats that plumbing, and the value becomes an ordinary template variable that any interpolation can place in a URL, an attribute, or a log line
  async_is_not_the_substitute: awaiting it opens a boundary, so a value known before the first byte costs a placeholder, a fallback, the client runtime, and a region that can settle late
already_half_built:
  detection: the generator's contextExternals pass reads the package's Go sources and reports which functions declare a leading context.Context; it ships and is tested
  applied_where: await bindings only, where the emitter prepends ctx; the ordinary expression path emits a bare call
  meaning: the discovery half arrived with requirement:async-external-functions optional_context, so only the sync call path is missing
  syntactic: the check stays on the parsed parameter list, because it runs before the package compiles
  qualifier: the context import name is resolved from the file rather than matched literally, corrected 2026-08-05; an aliased import now opts in, and a package aliased to the name context no longer opts in by accident
shape:
  template: unchanged, exactly as for the async form; `external CSRFField(): html` says nothing about the context
  go: func Name(ctx context.Context, args...) Result
  choice_owner: whoever writes the implementation, function by function
  unchanged_default: an implementation taking no context keeps its current generated call byte for byte
fragment_result_already_works:
  finding: `external CSRFField(): html` compiles in v0.2.8 and lowers to a Slot op binding the returned Fragment, verified against the released compiler
  consequence: the report's secondary ask, that such a function return a fragment rather than a string, needs no change; the result is rendered as a subtree under the ordinary rule:template-context-safety checks rather than escaped as text or trusted as raw
  head_caveat: a Fragment produced while rendering contributes no head, because requirement:head-merging fixes the merged head before the first body byte; requirement:component-asset-requirements is therefore not reachable by returning a fragment
cost_correction:
  reported: only the generated call withholds the context
  actual: a plan value closure is func(P) ..., so there is no ctx in scope at the call; Op.Exec already receives the Renderer that holds it, so the context exists one frame out
  needed: context-carrying variants of the op forms whose closures can contain such a call, not one more argument at the call site
  still_small: additive; existing op forms and every plan that uses them stay as they are
  positions: an external call is an ordinary expression, so it can appear in text, attribute, boolean attribute, condition, loop iterable, component argument, and slot positions
  scoping_option: emit a ctx-taking variant only where a context-taking external actually appears, per rule:usage-directed-generation, which keeps a project using none byte-identical
relation_to_render_value_provider:
  overlap: requirement:render-value-provider already carries per-request values into markup, through a generation-time symbol behind a registered builtin element
  difference: that seam is the framework's, is placement-checked, and never puts the value in template scope; this one is the application's own external, and its result is an ordinary typed value the template may interpolate anywhere
  both_read_only: neither writes the response, and neither introduces an http.Request parameter, so requirement:html-component-api is unchanged
  not_a_replacement: the reporter asks for both and states the difference the same way; this is the cheap unblock, and the builtin element is the checkable one
execution:
  when: at the plan step position during the initial pass of requirement:chain-render-pipeline
  head_pass: never; requirement:head-merging contributions stay static markup
  missing_context: the render context always exists, defaulting to background, so a sync external taking one cannot fail for want of it the way requirement:render-value-provider validation must
  cancellation: the implementation sees the render context and may return early; a sync external still has no error result, so it cannot report why
constraints:
  - htmlbind gains no net/http dependency, so decision:runtime-package-boundaries is unchanged
  - no reflection and no runtime lookup; the leading parameter is read from parsed Go source at generation time
  - a project whose externals take no context regenerates byte-identical Go
acceptance:
  - a sync external whose Go function declares a leading context receives the render context, with no template change
  - the same external declared without one generates exactly the bytes it generates today
  - a context-taking sync external in an attribute position is escaped for that position, unchanged
  - an html-returning sync external renders as a subtree, which already holds
  - a cancelled request reaches an implementation that took the context
related:
  - requirement:template-server-functions
  - requirement:html-component-api
  - decision:library-component-seams
as_built:
  instructions: TextCtx, RawCtx, AttrCtx, BoolAttrCtx, IfCtx, SlotCtx, ComponentCtx, and the package-level ForCtx, each the existing form with a leading context parameter on its value closure
  context_read: the boundary context at the instruction's position, so a subtree inside an await or live boundary sees that boundary's context rather than the root render's
  selection: per expression; an instruction takes the context only when the expression it holds reaches such an external, so a mixed template emits both forms
  rejected_position: an await binding over a caller-supplied value, whose unset check lowers to Require and runs before anything is written; naming a context-taking external there is a generation error
  import: the context import is decided from the typed expressions, because imports are written before any plan is emitted
  measured: every existing generator fixture and golden file regenerated unchanged, so the unused-is-free rule holds as stated
caller_scope:
  discovered: downstream framework request-context report 2026-08-05
  rule: the detection reaches the compiler through GenerateOptions.ContextExternals, so this requirement holds only where the caller fills it
  holds: a templates package, on both paths the generator compiles a template through
  missing: a route package, because routetree compileTemplate never filled the field; requirement:route-package-context-externals closed it 2026-08-05
  render_option: filling the field is only half of it, because the render context also has to be the request's; that half is recorded with the fix
  async_gate:
    asked: whether takesRenderContext excluding an async external is deliberate
    answer: yes; it gates the synchronous expression path only, and the await binding path reads the map directly and does prepend the context
    reason: an async external may be called only in an await binding, so the sync path has no legal async call site to serve
    not_the_reported_cause: the report's async reproduction failed for the missing caller above rather than for this condition
open_questions:
  - whether every expression position gains a ctx-carrying op variant, or generation restricts a context-taking sync external to the positions that have one
  - whether a sync external should be allowed an error result once it can observe cancellation, or stay total as requirement:async-external-functions splits it
  - whether one call is shared when the same context-taking external appears several times in one render
```
