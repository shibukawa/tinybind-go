---
id: concept:framework-template-extensions
type: concept
title: Framework Template Extensions
---
Let a downstream framework add markup elements and browser script to every template it compiles, without the application declaring or importing them.

```yaml
evidence:
  source: user design discussion
  received: 2026-07-27
review_gate: proposed requirements require user approval
motivation:
  markup: a framework concern such as a CSRF hidden input must appear in application markup without every page importing a component
  script: a framework browser capability such as a telemetry namespace must be reachable from component script without every page linking it
asks:
  builtin_element: an element like csrf-token that the generator recognizes and rewrites, carrying a per-request value at render time
  script_injection: framework JavaScript merged into the document head so a global such as otel.log is callable from component script
vocabulary:
  builtin_element: a whitelisted kebab-case element lowered to framework runtime output; not a decision:template-declaration-kinds component, which stays PascalCase and file-declared
  script_contribution: a registered head script whose source is the framework, not a component style or script block
  script_global: shorthand for an always-included contribution whose global load mode installs a namespace, so component script may call it anywhere; decision:framework-script-delivery keeps the two axes separate
extensions:
  - requirement:builtin-element-registration
  - requirement:builtin-element-lowering
  - requirement:render-value-provider
  - requirement:framework-script-contribution
  - requirement:render-time-script-contribution
  - requirement:render-context-externals
  - requirement:component-asset-requirements
downstream_round:
  when: 2026-07-31, decision:library-component-seams
  what: the framework that owns this concept reached the builtin-element and caller-head asks again from three features, and added two the concept did not hold
  added: a context for a synchronous external, and assets a component brings with it
  widened_actor: a library shipping a component is a third contributor beside the framework and the application, and it owns no route, no scaffold, and no shell
syntax: decision:builtin-element-syntax
script_delivery: decision:framework-script-delivery
script_authoring: decision:script-load-mode-authoring
registration: data:builtin-element-definition
entry_points:
  question: user asked whether builtin element and script implementations arrive as api:render-html-chain options
  answer: mostly no; the three stages carry different things and only the context reaches the render call
  generation_time:
    channel: the requirement:builtin-element-registration whitelist in data:generator-options
    carries: element names, markup templates, parameter types, passthrough entries, and requirement:framework-script-contribution registrations
  link_time:
    channel: the requirement:render-value-provider Go symbol named by data:builtin-element-definition
    carries: the function producing per-request values; generated code calls it directly
    reason: a linked symbol keeps decision:reflection-free and turns a missing implementation into a generation error
  render_time:
    channels:
      - the existing api:render-html-chain context option, or the ctx argument of the async entry, carrying request-scoped state put there by framework middleware
      - a requirement:render-time-script-contribution option selecting head script for this response alone
    correction: an earlier reading of this concept said no new render option was needed; that was wrong, because script audience is a per-path decision a build-time set cannot express
    not_passed: neither the builtin element markup nor a script asset crosses the render call; the markup is baked into the decision:generated-render-plan value and the asset file is already written, so only the inclusion decision is per call
application_wiring:
  note: the render signature is unchanged, but three things still live outside it and must not be described as no wiring at all
  serve_assets:
    what: the application serves requirement:static-asset-extraction PublicURLBase from PublicDir
    why: tinybind ships no static file server, so the generated script and stylesheet files need an application route
    scope: one route for the whole generated asset directory, not one per contribution
  install_middleware:
    what: the framework middleware that puts request-scoped state into the context
    why: requirement:render-value-provider reads only from the context, so nothing reaches a provider without it
  pass_context:
    what: the existing context option on the render call
    why: the provider needs it, and omitting it fails before the first byte
asset_authoring:
  question: whether the framework author writes the public script file by hand
  answer: no; the framework embeds the JavaScript source and registers it, and the generator writes the content-hashed file and its reference tag
  consequence: the only manual step is serving the directory
reuses:
  head: requirement:head-merging collects both kinds of contribution in the existing pre-body pass
  assets: requirement:static-asset-extraction emits and references inline script exactly as it does for component script
  plan: decision:generated-render-plan carries a builtin element as an ordinary instruction
  discovery: api:generator-call-registration and data:generator-options already model framework-supplied generation-time identity
  safety: rule:template-context-safety and requirement:explicit-output-control still classify every inserted value
scope:
  - registration is a whitelist passed to the generate command; no runtime element registry or name lookup
  - the whitelist closes the hyphenated element space, so a misspelled builtin fails generation instead of shipping inert markup
  - a project registering nothing imports no framework package and generates identical Go output
  - per-request values reach markup through requirement:render-value-provider, never through an http.Request parameter on a component
  - a per-request builtin element is excluded from reusable cached output
non_goals:
  - type checking the JavaScript surface a script contribution exposes
  - application-defined builtin elements; an application uses requirement:template-file-scope external components instead
  - runtime registration or plugin loading after generation
owner_framework: petitweb-go, per data:generator-options petitweb entry
milestone: follows concept:html-render-runtime-extensions; depends on requirement:head-merging shipping first
```
