---
id: decision:update-manifest-transport
type: decision
title: Update Manifest Transport
---
Carry instance identity and validators as boundary-element data attributes, and everything else in one inert document payload.

```yaml
source:
  - data:component-update-manifest
  - user data-attribute proposal 2026-07-26
review_gate: proposed markup convention requires user approval
options:
  attributes_only:
    shape: every manifest field as data attributes on the boundary element
    problem: capability descriptors and continuations bloat markup and sit in easily scraped attributes
  payload_only:
    shape: one inert JSON payload per document
    problem: a streamed delta fragment must be merged into a separate structure before its own DOM exists, and any client DOM mutation desynchronizes it
  hybrid: attributes for what locating and comparing a boundary needs; payload for document-level and capability data
decision:
  chosen: hybrid
  on_element:
    - rule:component-instance-identity instance ID
    - parent boundary ID and ordering token when structural operations need them
    - boundary revision
  never_on_element:
    validators:
      reason: a start tag is written before the content it would describe exists, so a digest of that content cannot be an attribute without buffering the boundary and back-patching the tag
      consequence: streaming and attribute-carried validators are mutually exclusive; identity is the only thing knowable at start-tag time
      where_instead: the delta response, held by the client in memory
    seeding:
      rule: a freshly loaded page holds no validators and simply sends none
      effect: its first update returns every boundary, and every update after that is a true delta
      benefit: no inert payload, no document shell change, and no second render pass are needed until requirement:delta-head-sync forces a payload anyway
  in_payload:
    - render version, route identity, and protocol version
    - redraw registration descriptors, when a page carries any
    - data:html-client-bootstrap fields including the CSRF token
rationale:
  - the runtime must find the target element anyway, so identity belongs where the target is
  - the DOM stays the single source of truth after arbitrary client-side DOM movement
  - a requirement:streaming-delta-response fragment arrives self-describing, needing no separate merge step to become addressable
  - keeping continuations out of attributes limits what a scraping script harvests cheaply
single_root_constraint:
  rule: an update-enabled component must render exactly one root element; that element may be any element the surrounding content model allows, so no wrapper is implied
  scope: update-enabled components only; an ordinary component still returns a multi-node fragment
  reason:
    - a multi-node boundary is a DOM range rather than a node, which turns replace, insert, and move into range operations
    - retain holes and rule:preserved-client-subtree markers both need a node reference
    - a fragment has no element to carry decision:update-manifest-transport attributes
  static_check: exactly one top-level element, not nested inside a conditional or loop; a top-level loop always fails
  ignored_nodes: doctype, comment, whitespace, and a hoisted head declaration, none of which can receive an attribute
  enforcement: an eligibility rule while boundaries are automatic; a diagnostic once requirement:client-update-rollout m3 makes the flag explicit
  diagnostic: generation error listing the top-level nodes and offering a root element or moving the update flag to the parent
  asymmetry: relaxing to range boundaries later only adds operation kinds and markers behind a protocol version bump, while starting permissive cannot be tightened
  revisit:
    at: requirement:client-update-rollout m3, when arbitrary components become update-enabled
    trigger: real components that must emit sibling rows or list items as one boundary
    alternative: comment-pair range markers with identity held in the inert payload instead of attributes
naming:
  default: 'tb', producing attributes of the form data-tb-<field>
  option: data:generator-options DataAttributePrefix overrides it, for a project whose markup already uses that prefix
  validation: lowercase letters and digits only, no leading or trailing hyphen, non-empty; an invalid value is a generation error
  runtime_binding: the browser runtime hardcodes the prefix; it is framework-owned implementation, not discovered at runtime, so no bootstrap field carries it
  consistency: the generator substitutes the configured prefix when it emits the runtime asset, so an overridden prefix and its runtime always agree
  hand_written_runtime: a framework shipping its own runtime build must build it for the prefix that project configures; a mismatch is a project configuration error, not a protocol negotiation
  challenged_2026_08_01: both lines above assume the module ships no runtime; v0.3.0 ships the only one, so no deployment can build for its own prefix and the option configures the server half alone, per requirement:update-protocol-naming-ownership
  incomplete_reach: the substitution covers 'data-<prefix>-id' but not the '<tb-boundary>' placeholder element or the 'tb-' boundary id allocation, so an overridden prefix yields two naming systems in one document
  stability: field names after the prefix are protocol surface and change with the protocol version
  scope: the prefix names update-protocol attributes only; requirement:suspense-html-streaming markers and other generated markup follow the same prefix for consistency
safety:
  - validators are keyed per rule:update-validator-computation, because any page script can read an attribute
  - attribute values are opaque and escaped
  - the runtime owns these attributes; application code mutating them is unsupported, per api:client-component-update
  - a server render never trusts attribute values returned by the client as authority
acceptance:
  - a delta fragment inserted into the DOM is immediately addressable by a later delta
  - moving a boundary element with client code does not corrupt update state
  - a component emitting sibling root nodes fails generation with an actionable message
open_questions:
  - whether the input validator is emitted at all
  - payload form as inert JSON script versus head metadata
```
