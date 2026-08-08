---
id: requirement:url-attribute-scheme-safety
type: requirement
title: URL Attribute Scheme Safety
---
A value rendered into a URL-bearing attribute carries an allowed scheme or is neutralized; the attribute position decides the escaping, which today it does not.

```yaml
source: security review 2026-08-06
review_gate: implemented 2026-08-06
shipped:
  runtime: htmlbind/url.go, holding the scheme policy, the two list grammars, WithURLSchemes, WithDataURLMediaTypes and BlockedURL
  ops: htmlbind/ops.go URLAttr, URLAttrCtx, URLListAttr and URLListAttrCtx
  compiler: templates/htmlbind/compiler.go isURLAttribute widened, isURLListAttribute and isEventAttribute added, html:event added to validateInsertion
  emitter: templates/htmlbind/emit.go emitAttributeOp routes by roster, and attributeValueCode gained a raw mode so the value reaches the op unescaped
  tests: htmlbind/url_test.go, templates/htmlbind/urlattr_test.go, and eight cases in TestGenerateDiagnostics
  blast_radius: one line of one golden fixture changed across the whole testdata tree, which is the href in testdata/templates/htmlbind/dynamic/output.go; go build, go vet and go test over the module are clean
  defaults_chosen: http, https, mailto and tel; the data media types are the raster image types, so image/svg+xml is absent because an SVG document carries script
  marker_chosen: '#tb-blocked-url', a fragment, so it resolves to the current document and reaches nothing
  not_shipped: meta http-equiv=refresh content, which is still open on its matching shape below
contradicts_in_force:
  concept: rule:template-context-safety
  its_claim: HTML strings use context-specific escaping
  actual: htmlbind.Escape is the only HTML escaper and has exactly one context, HTML text; url is a valueKind but never an escaping context
  status: the rule is aspirational for attributes, not descriptive; closing this requirement is what makes it true
finding_escape_has_no_url_context:
  where: htmlbind/ops.go Escape
  what: escapes the five characters '&<>"' and returns the input unchanged when none appear
  consequence: javascript:alert(1) contains none of the five, so it takes the early return and reaches the attribute verbatim
  lowering: templates/htmlbind/emit.go attributeValueCode wraps every attribute value in htmlbind.Escape, and templates/htmlbind/generate.go valueString lowers a url to code.String(); href={u} therefore emits htmlbind.Escape(u.String())
  witness: testdata/templates/htmlbind/dynamic/output.go line 72
  quoting_is_not_the_hole: the five-character set does close attribute-delimiter escape, which builtin_test.go and csrf_test.go cover; the hole is that a value needing no delimiter escape is still executable in this position
finding_roster_is_five_names:
  where: templates/htmlbind/compiler.go isURLAttribute
  what: href, src, action, formaction, poster
  consequence: an attribute outside the five is analyzed as ordinary text, accepts a plain string, and never reaches even the type gate below
  roster: rule:url-bearing-attributes
what_the_type_gate_already_buys:
  rule: compiler.go analyzeAttribute rejects any non-url expression in one of the five, so a raw string cannot reach them; the app must hold a url.URL
  measured_2026_08_06: url.Parse rejects "java\tscript:alert(1)" as a control character and " javascript:alert(1)" as a colon in the first path segment, and folds "JaVaScRiPt:" to a lowercase scheme
  therefore: the classic string obfuscations never survive parsing, so the missing check is a scheme allowlist over an already-normalized value rather than a fuzzy sanitizer
  keep_it: the gate is the reason the fix is small; the requirement extends it rather than replacing it
scheme_field_is_not_the_check:
  measured_2026_08_06: url.URL{Opaque: "javascript:alert(1)"} has an empty Scheme field and String() returns "javascript:alert(1)"
  also: url.URL{Scheme: "JAVASCRIPT"} keeps its case, because folding happens in Parse and not in the struct
  consequence: a gate reading u.Scheme against an allowlist passes the first as a relative URL, which is the obvious implementation and is wrong
  requirement: the gate decides on the rendered string, or on a re-parse of it, never on the struct field alone
reachability:
  stored: any url the app holds from user data
  reflected: htmlupdate.QueryURL decodes a redraw parameter with url.Parse and accepts every scheme, so a reloadable component with a url parameter rendered into href is reachable as GET redraw?param=javascript:alert(1)
  standing_rule: rule:redraw-input-trust already calls these parameters untrusted public input and says the typed decoder validates shape and range without establishing permission; it does not yet say that surviving the decoder is not a licence to render
  severity_note: the reflected path needs no stored data and no prior authentication, which is what lifts this above a latent hardening item
requirements:
  - a URL-bearing attribute renders its value only when the value's scheme is on an allowlist
  - the allowlist is positive; no denylist of javascript, vbscript, and data is acceptable, because it enumerates the attackers
  - a rejected value neutralizes the attribute rather than failing the render, because hostile input must not become a 500
  - neutralization is inert and visible, so a wrongly rejected URL is diagnosable from the output instead of silently absent
  - the check runs at one chokepoint reused by generated code, matching the contract Escape already states in its own comment
  - the roster covers every attribute in rule:url-bearing-attributes, including the list-valued and the embedded forms, not only the single-URL ones
  - a list-valued attribute rejects per entry; one bad entry does not discard the good ones
  - the redraw decode path and the render path apply the same allowlist, so neither is a way around the other
  - the allowlist is configurable, because mailto, tel, and an app's own scheme are legitimate and the default set cannot know them
settled_2026_08_06:
  by: owner, on the requirements review
  data_scheme:
    decision: allow an image-media-type data URL rather than excluding the scheme outright
    reason: blocking every data URL was judged more than the risk warrants, since an inline image is ordinary authoring
    consequence: the check is by media type and not by scheme alone, so the data scheme needs its own predicate reading the media type prefix
    still_never: a text/html or image/svg+xml data URL, the first because it is a document and the second because SVG carries script
  configurability:
    decision: the allowlist and the data-scheme policy are caller-configurable, and that is what makes the default set acceptable
    mechanism: decision:url-context-escaper configuration section
    consequence: the default may be conservative because an app that needs another scheme can say so, so no scheme has to be defaulted in defensively
  scope:
    decision: cover the whole roster rather than only the reported names, and close the event-handler gap in the same pass
    reversal: the event handlers were deferred earlier in the same review and then pulled back in
    event_mechanism: rule:event-attribute-context, a separate rule because an event handler has no scheme and the allowlist does not apply to it
  neutralization:
    decision: substitute a fixed marker, following html/template's handling of the same case
    reason: the attribute stays present, so a URL rejected in error is visible in the rendered output instead of looking like an authoring omission
open_questions:
  default_allowlist: http, https, mailto, tel and the scheme-relative and relative forms is the proposed default; ftp and sms are plausible and unrequested
  marker_spelling: the substituted value's exact text, which should be inert as a URL and recognizable in a DOM inspector
  meta_refresh_shape: content on meta http-equiv=refresh carries a URL that no attribute-name match finds, and rule:url-bearing-attributes ranks it second by impact; matching it needs the element and a sibling attribute rather than a name
what_the_allowlist_does_not_cover:
  point: the roster and the scheme check answer different problems, and for most of the reported missing attributes it is the roster that does the work
  detail: rule:url-bearing-attributes threat_classes
  consequence: ping, srcset, background and object data take a hostile value under an allowed scheme, so widening isURLAttribute matters there for the type gate it brings, not for the scheme test
  do_not_conclude: that the roster is therefore optional; without membership those attributes accept a plain string, which is a weaker position than the five have today
adjacent_gaps_not_scoped:
  style_attribute:
    what: validateInsertion permits a plain string in style, and CSS gets HTML-text escaping
    severity: lower, since the CSS payloads that executed script are gone from current browsers
one_acceptance_is_not_held:
  which: the redraw decode path and the render path apply the same allowlist, so neither is a way around the other
  measured_2026_08_08: the redraw and action entries call htmlbind.Render with no options, so a caller's WithURLSchemes never reaches them; an app scheme renders on the page and substitutes the blocked marker on the redraw of the same component
  direction: the redraw is stricter, not looser, so nothing hostile renders and this is a correctness divergence rather than a hole
  still_a_gap: an app whose own scheme is legitimate cannot redraw a component that renders it, and the two paths were meant to agree in both directions
  fixed_by: requirement:fragment-render-options
acceptance:
  - a url parameter holding javascript:alert(1) renders an inert attribute, in every attribute on the roster
  - the same value supplied to a redraw endpoint renders inert rather than executing
  - url.URL{Opaque: "javascript:alert(1)"} with an empty Scheme field is rejected, not treated as relative
  - a relative URL, a scheme-relative URL, and a fragment still render unchanged
  - a srcset with one hostile entry and two good ones keeps the two
  - an attribute added to the roster fails a template that supplies it a plain string, as the five do today
  - Escape keeps its current behavior for text and non-URL attributes, so no existing output moves
related:
  - rule:url-bearing-attributes
  - decision:url-context-escaper
  - rule:template-context-safety
  - rule:redraw-input-trust
  - requirement:explicit-output-control
  - requirement:html-template-v1
```
