---
id: rule:url-bearing-attributes
type: rule
title: URL Bearing Attributes
---
The roster of attributes whose value a browser resolves as a URL, and the shape each one holds; membership decides which escaping runs, so the roster is the check rather than a convenience list.

```yaml
source: security review 2026-08-06
review_gate: proposed
consumer: requirement:url-attribute-scheme-safety, and templates/htmlbind/compiler.go isURLAttribute which today names only the first five of single_url
shape_matters: a scheme check that assumes one URL per attribute is wrong for three of these, so the shape is part of the roster and not a footnote
single_url:
  in_force_today: [href, src, action, formaction, poster]
  missing_and_reported:
    xlink:href: SVG, the legacy namespaced form; SVG2 also accepts a plain href on the same elements, which the href row already covers
    data: object; loads and can execute the referenced resource
    cite: blockquote, q, ins, del; not navigated by default but resolved, and it is markup an app fills from user data
  missing_and_found_here:
    background: body, table and friends; deprecated in the standard and still honored
    longdesc: img; deprecated, still resolved where implemented
    manifest: html; deprecated
    classid, codebase, archive: object and applet; legacy plugin loading
    profile: head; obsolete
  note_on_deprecated: a deprecated attribute is still a sink, because the rule that matters is what a browser resolves and not what the standard recommends
url_list:
  srcset:
    on: img, source
    grammar: comma-separated entries, each a URL then an optional width or density descriptor
    reported: yes
    per_entry: the check runs per entry; one rejected entry drops that entry and keeps the rest
  imagesrcset:
    on: link rel=preload
    grammar: the srcset grammar
    found_here: yes
  ping:
    on: a, area
    grammar: space-separated URL list
    reported: yes
    per_entry: same rule as srcset
embedded:
  meta_refresh:
    attribute: content on meta http-equiv=refresh
    grammar: seconds, then optional ';url=' and a URL
    why_it_is_here: no attribute-name match finds it, because the name is content and the meaning comes from a sibling attribute
    consequence: matching on the attribute name alone cannot cover this one; it needs the element and the http-equiv value
not_urls_but_adjacent_sinks:
  purpose: these are named so a reader does not assume the URL roster covers them; each needs its own decision
  srcdoc: iframe; a whole HTML document rather than a URL
  style: a CSS context that currently takes a plain string
  on_star: every event handler attribute, a JavaScript context; owned by rule:event-attribute-context and fixed in the same pass by a different mechanism
  usemap: resolved but fragment-only, so it carries no scheme and is out of scope here
not_urls:
  - target and formtarget, which name a browsing context
  - rel, type, and media, which are metadata
  - download, whose value is a filename
matching_rules:
  case: HTML attribute names are ASCII case-insensitive; the template language already requires lowercase or kebab-case names, so a lowercase comparison is exact for authored templates
  namespaced: xlink:href must match by its full name, since the colon form is what SVG carries
  element_context: the roster is by attribute name except for meta_refresh, which is the one entry needing the element and a sibling attribute
  data_and_aria: data-* and aria-* are never URL-bearing and must not be swept in by a prefix rule
threat_classes:
  why_this_section: the roster is not one risk repeated; a scheme allowlist answers only the first class, and for most of the missing attributes it is the type gate rather than the allowlist that does the work
  classification_basis: what the standard says a browser does with the attribute; the per-browser notes below are marked where current behavior differs from what the standard permits and were not measured here
  a_script_execution:
    what: a hostile value runs script in the page's own origin
    members: [href, src, xlink:href]
    href_and_src: already on the roster; this is where the confirmed Escape hole actually bites, which is why that fix stands alone and does not wait for the roster
    xlink_href: SVG a; SVG2 also accepts a plain href, which the href row covers when matching is by attribute name regardless of element
    browser_note: current Chrome and Firefox restrict javascript: navigation from an SVG anchor, so this is a hardening item rather than a demonstrated hole; it has been a recurring bypass source and is worth closing regardless
    allowlist_helps: yes, this is the class the scheme check exists for
  b_attacker_chosen_embedded_document:
    what: the attribute opens a nested browsing context showing a document the attacker picked
    members: [data]
    impact: phishing and UI redressing inside the page; the embedded document is a separate origin, so it does not reach the parent DOM
    no_scheme_trick_needed: an ordinary https URL is enough, so the allowlist does not close this; the type gate is what stops a plain string from choosing the destination
    browser_note: javascript: and text/html data URLs in object data are additionally restricted in current browsers
  c_forced_outbound_request:
    what: the browser issues a request to a destination the attacker chose, with the user's cookies; no script runs
    members: [ping, srcset, imagesrcset, background, longdesc]
    ping: designed to POST to every listed URL on click, so an attacker-controlled ping is a forced cross-site POST from an authenticated browser
    image_loads: srcset, imagesrcset and background choose which resource loads; the effect is content spoofing plus a beacon that deanonymizes the reader
    javascript_scheme_is_inert_here: none of these execute a javascript: value, so the allowlist buys nothing
    what_actually_helps: the type gate, because it stops a plain string from reaching the attribute at all
    parsing_note: srcset and imagesrcset are comma-separated, so a value carrying a comma injects extra candidates; this is a grammar bug as much as a scheme one
  d_metadata_only:
    what: the browser initiates nothing; the value is readable from the DOM
    members: [cite, usemap, profile]
    cite: not fetched and not navigated; exposed as the DOM cite property, so only a script or an extension reading it is affected
    impact: low; included because membership is nearly free and the attribute is one an app fills from user data
  e_effectively_dead:
    what: the mechanism the attribute drove has been removed from browsers
    members: [manifest, classid, codebase, archive]
    manifest: AppCache, removed
    plugin_attributes: NPAPI and Java plugin loading, gone
    disposition: include for completeness, expect nothing from it, and do not let it inflate the priority of the roster work
  special_meta_refresh:
    what: content on meta http-equiv=refresh navigates the page with no user interaction
    javascript_scheme: blocked by current browsers
    the_live_risk: an ordinary https target, which is a forced navigation to an attacker's site with no click; open redirect and phishing
    rank: the highest-impact item in the missing set after the confirmed Escape hole, and the only one needing element and sibling-attribute matching rather than a name match
    allowlist_helps: no; the scheme is legitimate. What helps is refusing a dynamic value in this position at all, or requiring the URL part to come from a url-typed expression
priority_order:
  basis: impact first, then whether the fix is the allowlist or the type gate
  1: the Escape hole on href and src, which is the only confirmed script execution and is independent of this roster
  2: meta_refresh content, forced navigation with no interaction
  3: ping, forced authenticated POST on click
  4: xlink:href, script execution candidate pending per-browser verification
  5: data on object, attacker-chosen embedded document
  6: srcset, imagesrcset, background, resource choice and beaconing
  7: cite, longdesc and the dead group, cheap membership and little else
milestone:
  v1: the whole roster, by owner decision 2026-08-06; single_url and url_list including the deprecated and the dead groups, because membership is cheap and omission is the bug being fixed
  v1_alongside: rule:event-attribute-context, the on-prefixed attributes, by the same decision
  v1_decide: meta_refresh, still open only on the matching shape, which needs the element and a sibling attribute rather than a name
  post_v1: srcdoc and style, each on its own decision
related:
  - requirement:url-attribute-scheme-safety
  - decision:url-context-escaper
  - rule:template-context-safety
```
