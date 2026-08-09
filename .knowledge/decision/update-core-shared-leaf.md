---
id: decision:update-core-shared-leaf
type: decision
title: The Update Surface Splits Into A Shared Core And Two Shells
---
Move the transport-free half of htmlupdate into internal/updatecore over a six-method Reader, and let each transport shell redeclare only Options and Response.

```yaml
status: implemented 2026-08-10
raised_by: downstream framework report, 2026-08-09, asking for the read-only update entries on the fasthttp backend
shape_chosen: the shell, not the interface
two_shapes_offered:
  a_reader_interface: 'Options.WantsUpdate(r RequestReader), one implementation and no duplication'
  b_mirror_package: the entries redeclared over the other transport, as the bind and write halves already are
why_a_was_refused:
  measured: '*http.Request does not satisfy such an interface; Header, Method and URL are fields, not methods'
  consequence: every existing caller would wrap at the call site, or the package would carry both spellings of all twelve entries
  precedent: decision:fasthttpbind-no-transport-interface refused the same shape on the bind path, for moving the author shape; it moves it here too
what_the_report_missed:
  reads_beyond_headers: 'r.Context() in the action and redraw paths, r.PostFormValue in CSRFToken, and r.URL.RawQuery for the redraw bound, which is measured before parsing'
  transport_in_fields_not_only_parameters:
    - 'Options.OnFailure(r *http.Request, Failure)'
    - 'Reloadable.Render(r *http.Request, string, url.Values)'
    - 'Response.WriteTo(http.ResponseWriter), which the group needs to send what it returns'
  resolution: the first two were retyped to context.Context before the split, because no implementation in the module read the request; see requirement:component-redraw-endpoint and requirement:update-error-hook
as_built:
  core: internal/updatecore, holding Options, the wire types, the manifest codec, the query decoders, the runtime asset, and the twelve read-only entries over a Reader
  reader: six methods, Header, Method, RawQuery, Query, FormValue and Context, each present because an entry calls it
  shells: htmlupdate over *http.Request, fasthttpupdate over *fasthttp.RequestCtx
  duplicated: Options and Response only, because each needs methods and a method cannot be declared on another package's type
  drift_guard: 'each shell converts with updatecore.Options(o) and Response(resp), so a field added on one side and not the other stops compiling'
  aliased: every other type, following the internal/bindcore precedent that the root already sets for Problem, FieldError, HTTPError and File
  consequence_worth_noting: a Registry, a Reloadable and an Update are one type across both backends, so a helper building one needs no build tag and generated registrations stay out of the tagged half
net_http_in_the_core:
  fact: updatecore imports net/http for http.Header, and so does the fasthttp shell
  why_acceptable: rule:transport-dead-code-elimination already found that the fasthttp fork imports net/http itself, so no arrangement keeps it out of the graph; the property to defend is that no net/http server path is reachable, and a map type reaches none
  rejected_alternative: a private Header map type, which would stop Response from being convertible and buy nothing measurable
verification:
  parity_suite: fasthttpupdate/parity_test.go compares status, headers, body and failure kind between the two shells for redraw, action, negotiation, headers, CSRF, the runtime asset and WriteTo
  mutation_checked: 'the CSRF test fails when the reader is switched to fasthttp FormValue, which would accept a token from the query string'
  end_to_end: testdata/transform_rewrite carries a handler using the entries, and the emitted fasthttp source is compiled against the real shell
related:
  - requirement:transport-port-surface
  - requirement:fasthttpbind-parity-scope
  - decision:fasthttpbind-no-transport-interface
  - decision:backend-build-tag-mode
  - rule:transform-rewrite-table
  - rule:transport-dead-code-elimination
```
