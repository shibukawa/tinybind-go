---
id: decision:url-context-escaper
type: decision
title: The URL Context Is A Second Escaper, Not A Wider Escape
---
Add a URL-context escaper beside htmlbind.Escape and have the emitter select it by attribute position; do not teach Escape about contexts and do not move the check into the url type.

```yaml
source: security review 2026-08-06
review_gate: proposed
decides_for: requirement:url-attribute-scheme-safety
shape:
  compile_time: templates/htmlbind/compiler.go keeps the type gate and widens isURLAttribute to rule:url-bearing-attributes, so a plain string still cannot reach a URL attribute
  emit_time: templates/htmlbind/emit.go attributeValueCode selects the URL escaper for an attribute on the roster, passing the url.URL rather than its String() form
  run_time: htmlbind exports the escaper, so generated code reuses the runtime's rules exactly as Escape's own comment already promises
  list_forms: srcset, imagesrcset, and ping get their own entry points, since their grammar is per entry
why_the_typed_value_and_not_the_string:
  reason: generate.go valueString lowers a url to code.String() before Escape sees it, so a string-taking escaper would have to re-parse what the caller already parsed
  benefit: taking url.URL keeps one parse and lets the escaper decide on the rendered form, which the measured Opaque case in the requirement makes mandatory
configuration:
  requirement: requirement:url-attribute-scheme-safety settled_2026_08_06 makes the allowlist and the data-scheme policy caller-configurable
  seam: the existing htmlbind.Option pattern, alongside WithCSRFToken, WithBoundaryPrefix and WithValidatorTag; renderOptions gains the allowlist and Renderer.opts is documented never nil at htmlbind/plan.go
  proposed_options: one naming the permitted schemes and one stating the data-scheme media-type policy, so an app that inlines images says so without reopening the whole allowlist
  structural_constraint:
    problem: Attr takes value func(P) (string, bool), so the closure holding the current htmlbind.Escape call has no Renderer and cannot see a render option
    consequence: the check cannot stay where Escape sits today; it moves into a new op whose Exec(r *Renderer, params P) reads r.opts
    shape: a URLAttr op taking func(P) (url.URL, bool), which also delivers the typed value the section above requires; the two constraints agree
    precedent: AttrCtx already does exactly this, reaching r.boundaryContext() inside Exec for a value the closure cannot supply
  generation_time_vs_render_time:
    chosen: render time, because it is the seam that already exists and because one binary serving two apps can differ
    not_generation_time: baking an allowlist into generated source would make the policy a property of the checked-in artifact, and changing it would require regeneration
  default_is_allowed_to_be_conservative: because an app needing another scheme can configure it, so nothing has to be defaulted in defensively
rejected_alternatives:
  context_parameter_on_escape:
    what: Escape(value, context) with an HTML-text and a URL context
    why_not: it changes every call site including the four inside htmlbind itself, and the compatible default stays the unsafe one, so the signature grows without the hole closing
  escape_learns_about_schemes:
    what: keep one function and reject javascript: inside it
    why_not: it would corrupt ordinary text, since a paragraph may legitimately contain the word javascript: and Escape is the text escaper too
  validating_url_newtype:
    what: an htmlbind.URL that checks its scheme on construction, replacing url.URL in generated parameter structs
    why_not_now: it changes the generated struct field type, which every caller constructs directly, so it is a breaking change to the generated API for a hole a render-time gate closes
    why_it_is_still_interesting: it checks once per value instead of once per render, and it moves the diagnostic to where the bad URL enters
    disposition: revisit after the render-time gate ships and the allowlist has settled; the two compose rather than conflict
  denylist:
    what: reject javascript, vbscript, and data
    why_not: it enumerates the attackers, and the roster of dangerous schemes is not closed
  error_on_rejection:
    what: fail the render when a scheme is not allowed
    why_not: the value is untrusted input on the reflected path in requirement:url-attribute-scheme-safety, so a caller could turn any page into a 500 by supplying one
  content_security_policy_instead:
    what: rely on a CSP that forbids inline script
    why_not: it is deployment configuration this module does not own, it does not stop navigation to a text/html data URL from a link, and a template language that emits executable attributes from typed values is wrong whether or not a header saves it
  sanitize_at_the_app:
    what: document that an app must check its own URLs
    why_not: the type gate exists precisely because this language does not delegate context safety to its callers; leaving the last step to the app inverts rule:template-context-safety
neutralization:
  decided: 2026-08-06, substitute a fixed marker rather than dropping the attribute
  precedent: html/template does the same for this exact case, which keeps the failure diagnosable in the rendered DOM
  reason: a dropped href is indistinguishable from an authoring mistake, so a URL rejected in error would be invisible
  open: the marker's spelling, and whether an option can turn rejection into a render error for an app that prefers to fail loudly
placement_of_the_roster:
  where: the roster is compiler-side, because the compile-time gate and the emit-time selection must agree on membership by construction
  risk_if_split: a name present in one and absent in the other is a silent hole, which is the shape of the reported bug
coverage_beyond_the_render_path:
  redraw: htmlupdate.QueryURL accepts every scheme; whether it rejects at decode as well as at render is open, but the two must not disagree
  head_contributions: htmlbind/head.go writes attribute values through Escape, so a head link href takes the same path and the same fix
  htmlupdate_runtime: htmlupdate/runtime.go has its own htmlAttrEscape for the bootstrap script tag; its href-like value is o.RuntimePath(), which is app configuration rather than user input, so it is in scope for consistency and not for severity
verified:
  status: implemented 2026-08-06
  scheme_table: TestSafeURLDecidesOnWhatTheBrowserReads, twenty cases including the tab-split and newline-split forms, the leading-space and control-character forms, and the relative value whose first segment carries a colon
  the_two_traps:
    - TestSafeURLRejectsTheEmptySchemeOpaqueForm, the url.URL whose Scheme field is empty and whose String() is executable; it asserts the precondition first, so the test fails loudly rather than silently passing if net/url changes
    - TestSafeURLHandBuiltUppercaseScheme, the field assigned directly and never folded by Parse
  option_reach: TestURLAttrOpAppliesThePolicy and TestURLAttrOpHonoursAConfiguredRoster exercise the op rather than the helper, which is what proves a render option reaches an attribute whose value closure never sees the renderer
  lists: TestSafeSrcsetKeepsTheGoodCandidates and TestSafeSpaceURLsKeepsTheGoodEntries, one hostile entry dropped and the rest kept
  emitter_routing: TestURLAttributesRouteThroughThePolicyOp asserts per attribute that the op is URLAttr and that its line carries no Escape call, since escaping inside the closure would put the check on the wrong side of the encoding
  scoping: TestOrdinaryAttributesAreUntouched, plus the measured blast radius of one changed golden line across the whole testdata tree
  not_covered_yet: a redraw test supplying a hostile scheme through the query string; the render path is what was closed, and htmlupdate.QueryURL still accepts every scheme at decode
related:
  - requirement:url-attribute-scheme-safety
  - rule:url-bearing-attributes
  - rule:template-context-safety
  - rule:redraw-input-trust
  - requirement:explicit-output-control
  - decision:generated-runtime-in-module
```
