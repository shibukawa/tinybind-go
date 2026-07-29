---
id: requirement:client-managed-head
type: requirement
title: Client Managed Head
---
Return the merged head as a separate tag list for navigation responses, and define how the browser retires the outgoing page's tags without disturbing tags it did not own.

```yaml
priority: should
source:
  - requirement:component-delta-rendering
  - decision:client-runtime-ownership
  - user SPA discussion 2026-07-27
review_gate: proposed
problem:
  server_side: on a full document the shell writes the merged head into the byte stream, so nothing needs to hand it back; a navigation response has no shell and no head position
  client_side: after navigation the live document still holds the previous page's title, meta, link, and script tags, and no parser will replace them
  ownership: the document also holds tags the application added itself, which a navigation must not touch
existing_pieces:
  merge_head: MergeHead already returns the merged tags as ready-to-write HTML without rendering, so the list is already derivable
  body_only: omitting decision:html-document-shell from the chain already renders body content with no head emission, because only the shell reads the merged head
  gap: the two are separate calls over the same chain, so they can drift, and only the render path validates the chain
server_api:
  need: one entry that validates the chain once and returns both the merged head list and the body output
  sync_shape: a render entry writing body output to the writer and returning the merged tag list
  async_shape: the same, returning the tag list up front beside the boundary sequence, which is possible because assembly computes the head before rendering starts
  return_form:
    shape: the merged tag list stays a slice of ready-to-write HTML strings, matching MergeHead
    tokens_from_values: a caller reads per-member ids off the values it already holds, rather than from an enriched return type
    accessor:
      name: HeadIDs, the same name on Fragment and on Wrapper so a caller treats chain members uniformly
      plural: one id per contributed tag, because identity is per tag and a member usually contributes several
      pairs_with: the existing Head accessor returning tag markup, so Head and HeadIDs are the two views of one list in the same order
      rejected_name: HeadChildrenIDs, because children is already the reserved unnamed-slot parameter name, so it reads as the ids of slot content rather than of head tags
    wrapper_gap: closed 2026-07-30; Wrapper already exposes Head, and requirement:head-contribution-provenance added HeadSources to both forms, so HeadIDs follows an established pair rather than introducing one
    granularity_settled: Head is now one entry per contributed tag, which this requirement's per-tag id assumed; requirement:head-contribution-provenance made the change
    why_this_shape: it follows the principle already applied to the await flag, that a caller decides from the values in its own hand, and it keeps the render entry from growing a struct
    filtering: with member tokens in hand, a caller drops the markup of an unchanged member from the merged list by token, so the module needs no notion of which member changed
  transport: the module returns data and never serializes it; the framework places it in its navigation response, per the decision:client-runtime-ownership split
  no_shell: this entry is for a chain without a document shell; including one is a usage error because the shell would emit the head it is also returning
client_lifecycle:
  identity: the marker token carried by the tag, which encodes the exact merged string requirement:head-merging already deduplicates on
  diff_not_replace:
    rule: apply the difference between the previous and next tag sets, never clear and rebuild
    reason: a stylesheet present in both sets must not be removed and re-added, which would refetch it and flash unstyled content
    add: tags in the next set and not the previous one
    remove: tags in the previous set and not the next one
    keep: tags in both, untouched, preserving their loaded state
  ownership_marking:
    rule: the module marks every tag it manages, and the client only ever removes marked tags
    approved: 2026-07-27
    reason: an application or a third-party script may add head tags at runtime, and a navigation must not evict them
    form: a data attribute on each emitted tag, so the marking survives serialization and costs no separate manifest
    managed_set: tags that arrive as merged requirement:head-merging contributions
    unmanaged_set: markup the decision:html-document-shell writes literally inside its own head, which is shell output rather than a contribution, so the boundary is already structural
  marker_configuration:
    owner: data:generator-options, set by the framework generate command like the other generation-time settings
    default: a module-owned data attribute name; a framework may substitute its own
    timing: applied when the tag string is generated, not per response
    reason: a merged contribution is already a fixed string on the plan, so adding the attribute at render time would mean rewriting strings on every request
    scope: one attribute name per generation unit
    name_only: the framework chooses the attribute name; the value is module-generated, decided 2026-07-27
  marker_value:
    produced_by: the generator, because only it holds the canonical tag string
    form: the first 12 hex characters of a SHA-256 digest of the canonical tag string, decided 2026-07-27
    algorithm_reuse: SHA-256 is already the generator's digest for the rule:generated-source-self-contained input stamp, so no second algorithm enters the build
    width_rationale: 48 bits keeps collision probability around one in a billion at a few thousand distinct tags, while staying short enough to read in a diff
    purpose: give the client a token to diff on, so it never compares its own DOM serialization against a server-produced string
    why_not_bare_flag: a presence flag would force the client to identify a tag by its serialized markup, which browsers normalize in attribute order, quoting, boolean attributes, and entity form
    why_content_hash:
      generation_scope: generation runs one package directory per invocation, so any per-run counter or emission index would repeat across packages in the same build
      independence: a content hash needs no coordination between invocations, which is the property a per-directory generator actually has
      stability: an unchanged tag keeps its token across builds, so a document served by one build and a navigation response from the next still agree
    rejected_global_sequence:
      shape: a second generation pass over every package directory, sorting distinct tags and assigning short ordinal tokens
      viable: it would work; distinct tag strings rather than plans must be numbered, or two components declaring one stylesheet would stop deduplicating
      cross_build: data:component-delta-response render_version already forces full navigation on mismatch, so build-to-build token drift would not actually break a session
      why_rejected:
        - it buys shorter tokens only, which a truncated hash already delivers
        - it needs a project-wide generation phase, so a single directory could no longer be regenerated correctly on its own through go generate
        - it must rewrite packages that incremental generation skipped, because adding one package shifts every later token
      revisit_if: a project-wide identity space is needed anyway, for example document-lifetime boundary identifiers or rule:component-instance-identity, since one pass would then serve several purposes
    identity_alignment: requirement:head-merging already deduplicates on the exact tag string, so the token is a compact encoding of an identity the module already uses
    collision: the encoded width must make a collision between two distinct tags implausible, because a collision silently merges two identities in the diff
    not_the_asset_hash: requirement:static-asset-extraction hashes file content for cache-busting; this hashes the tag markup and the two are unrelated
  token_transport:
    rule: the token travels inside the tag string, so a tag delivered as markup carries its own identity
    client: parse the incoming tag strings first, read their tokens, then diff that set against the tokens of the marked tags already in the live head
    benefit: no parallel token array to keep in step with the markup
token_only_retention:
  purpose: a navigation response keeps unchanged outer members' tags installed without resending their markup
  shape: the response carries markup for the changed member's tags and bare tokens for the tags it deliberately omitted
  client_rule: the next tag set is the union of parsed markup tokens and the bare token list; a token present in the live head and in that union is kept untouched
  works_because: the token is content derived, so it identifies an installed tag without its markup; an owner or plan derived identifier could not, since the client tracks no owners
  ordering_safe: the omitted members are the outer ones, whose tags already sit earlier in the live head, and new tags append after survivors, so the merged cascade order is preserved
  ordering_caveat: omitting a middle member while sending an outer and an inner one would not preserve relative order by appending alone
  dedup: a tag contributed by both an omitted member and the changed member appears once, because both routes yield the same token
  missing_markup:
    case: a bare token names a tag that is not installed, for example after a third party removed it or the client arrived by a path whose chain differed
    rule: the client cannot synthesize the tag, so it requests the full head or falls back to full navigation
    detection: the union contains a token with neither installed element nor supplied markup
  payload_note: a head is small, so this is an optimization rather than a correctness requirement; the full-markup form must keep working
  initial_document:
    rule: a full document render marks its contributed head tags exactly as a navigation response does
    reason: the first navigation must find the initial page's tags in the live DOM, which is only possible if they were marked when the document was served
    consequence: marking is not a navigation-only concern, so it cannot live in the navigation entry point
  previous_set:
    rule: the client reads the current marked tags from the live document each navigation
    approved: 2026-07-27
    reason: the DOM is the authority on what is installed; a remembered set can drift after a manual insertion, a bfcache restore, or a failed navigation
    consequence: the runtime keeps no head state between navigations
  excluded_from_management:
    charset: reapplying it after parsing is meaningless, so it stays a document-shell concern
    viewport: unchanged across navigations in practice; churning it can retrigger layout
    reason: requirement:head-merging already treats both as singletons decided once
  title: an ordinary managed tag; the innermost contributor still wins, so the diff swaps it like any other
  ordering: the added tags keep their merged order, appended after the surviving marked tags, so cascade-sensitive stylesheets stay in their relative order
script_tags:
  reexecution: re-adding a script element that was previously removed executes it again, which the diff avoids for a tag present in both sets
  moved_module: a module already evaluated is not re-evaluated on re-insertion, so a removed and re-added module silently does nothing
  guidance: a contribution needed across navigations belongs in the always-included set of requirement:framework-script-contribution rather than in a per-page head
delta_response_gap:
  finding: data:component-delta-response carries operations and the next manifest but no head payload
  consequence: a navigation delta cannot currently express the head change at all
  resolution: the head tag list becomes a field of that response, or the framework carries it alongside; either way the module supplies the list and not the encoding
constraints:
  - the module never emits client script for this; the diff is implemented by the caller-owned runtime per decision:client-runtime-ownership
  - merged tags keep rule:template-context-safety escaping from their declaring context
  - the list is computed before the first body byte, exactly as requirement:head-merging already requires
  - a full document response is unaffected and keeps writing the head through the shell
acceptance:
  - a navigation response returns the merged tag list and body output from one call over one validated chain
  - a stylesheet shared by the old and new page is not removed, not refetched, and does not flash
  - a tag the application added by hand survives navigation
  - the title changes on navigation
  - a page-specific stylesheet is removed when navigating away
  - an unchanged tag keeps its token after an unrelated part of the project is regenerated, so a rolling deploy does not evict it
  - two packages generated by separate invocations give one shared tag the same token
  - two components declaring the same stylesheet still emit one link, because their tokens match
  - a caller reads the tokens of the document and layout members from the values it holds, with no extra render call
  - a navigation sending page markup plus document and layout tokens leaves those tags untouched and installs only the page's
  - a bare token naming a tag that is not installed does not silently drop it; the client recovers instead
  - a page swap under an unchanged document and layout preserves their tags even without the token-only form, because the merged sets share those tokens
  - the client decides the diff without reading serialized markup out of the live DOM
  - rendering a full document through the shell produces byte-identical output to today
related:
  - requirement:head-merging
  - requirement:framework-script-contribution
  - api:render-html-chain
  - requirement:fragment-capability-introspection
navigation_with_await:
  decision: a navigation response may carry await boundaries, and the client composes them, approved 2026-07-27
  sequence: the same data:async-boundary-content items the streaming mode yields
  framing_differs:
    streaming: template plus commit marker, applied by the parser-driven marker callback
    navigation: id and html as data, applied by the client after it installed the delta HTML
    reason: no parser runs over a navigation response body, so there is no marker to connect; decision:client-runtime-ownership already keeps framing out of the module for exactly this reason
  order: the client installs the delta HTML containing placeholders and fallbacks first, then applies completions by boundary id as they arrive
  head: the head diff applies with the delta HTML, before completions, because a completion may reference a stylesheet the new page contributed
boundary_id_scope:
  finding: the coordinator numbers boundaries from a per-render counter, so two renders in one document produce the same identifiers
  problem: a navigation inserts boundaries into a live document that may still hold boundaries from an earlier render, such as one inside a layout the navigation did not replace
  consequence: applying a completion by id could target the wrong placeholder
  requirement: an identifier must be unique within the document over its lifetime, not only within one response
  note: harmless today because a full document load starts an empty document, so this surfaces only once navigation ships
vocabulary:
  id: the marker attribute value, a content hash of the canonical tag string; token and hash were used interchangeably while designing and mean this
  marker: the attribute carrying that id, whose name is a generation-time setting
open_questions:
  - the default marking attribute name
  - how a document-unique boundary identifier is derived without a server-side session counter
```
