# Two render outputs: assembled HTML, and the update record sequence

**Audience:** someone building a framework on `tinybind-go`.
**Status:** partly built.

> **Shipped:** the boundary decomposition — a fragment per boundary, a hole where each nested boundary sits, and the `boundaries` list that separates a hole to fill from one to retain — plus the `children` operation, which says a boundary's own markup is unchanged and its nested boundaries are now these, in this order. It is what a navigation delta and a live delivery already write.
>
> Measured on a hundred-row message panel, against a full render of 15,328 bytes: editing one row costs 76, appending one costs 365, and changing the panel's own markup costs 7,240 — the last being what slot spans would compress, since almost all of it is the list of holes.
>
> **Also shipped:** the redraw body is now that same shape — `ops`, `head`, and `manifest` — so one client applies every update path, the head has left its header, and a redraw returns the validator it used to leave stale.
>
> **Also shipped:** slot spans and the static sequences. Set the `-Sequences` request header and a fragment arrives as an address plus its values wherever that is smaller than its markup; ask `Options.Sequence` for the tree behind an address, public and immutable because a sequence derives from the template rather than from a request.

## The shape in one paragraph

A render has two output forms. The first is assembled HTML — a complete document, or one component's subtree. The second is a sequence of records: the HTML of each update boundary, the tree that composes them, and the static and variable parts of each fragment separated. The module produces both from the same compiled plan. What you do with the second one — which records you transfer, in what wire format, over what transport — is yours.

There is no third form and no negotiation between them inside the module. You pick an entry.

## What we decide, and what you decide

**We decide** how a render decomposes, what is static, where a boundary's identity comes from, and every escaping and URL-safety question. Those are properties of the template and of the language, and they cannot be re-derived correctly outside it.

**You decide** the wire format and its versioning, the transport, which records are worth sending for a given request, and the browser runtime that applies them. That split is unchanged from `decision:caller-owned-wire-versioning` and `decision:client-runtime-ownership`; this output form does not move it.

## Output form 1 — assembled HTML

Three entries exist today and are unchanged:

| entry | writes | use |
| --- | --- | --- |
| `Render` / `RenderChain` | complete HTML, no markers | a page for a client with no runtime, a static export, a mail body |
| `RenderAsync` / `RenderChainAsync` | complete HTML progressively, yielding boundary completions | `requirement:suspense-html-streaming` |
| `CollectChain` | complete HTML plus instance attributes and a manifest | a page a later partial update will address |

Only the collecting entry emits instance attributes, so the other two stay byte-identical to what they produce now.

## Output form 2 — the update record sequence

An ordered sequence of two record kinds, yielded as an iterator rather than collected, because a fragment may contain an `await` boundary and the fragments of one render therefore do not all exist at one moment.

**A boundary list** names the update boundaries appearing inside one fragment, for that fragment's scope.

**A fragment** carries one boundary's HTML with its id. Where a nested boundary sits, the HTML carries an empty placeholder element bearing that boundary's id — so a parent is structurally complete and installable without its children.

Nesting is expressed by ordinary nesting: a child fragment declares its own boundary list. An ancestor always precedes its descendants, so a client can install a parent and fill each hole as its fragment arrives.

### Worked example

```
<Parent>
  <h1>Title</h1>
  <Child item="1" />   // reloadable
  <Child item="2" />   // reloadable
</Parent>
```

Assembled: one complete HTML subtree.

Decomposed:

```
[
  { boundaries under Parent: [child-1, child-2] },
  { Parent's HTML, with an empty placeholder at each of child-1 and child-2 },
  { child-1's HTML, id child-1 },
  { child-2's HTML, id child-2 },
]
```

And if `Child` itself contains a reloadable boundary:

```
[
  { boundaries under Parent: [child-1, child-2] },
  { Parent's HTML, with placeholders },
  { child-1's HTML, id child-1, with a placeholder at grandchild-1 },
  { boundaries under child-1: [grandchild-1] },
  { grandchild-1's HTML, id grandchild-1 },
  { child-2's HTML, id child-2 },
]
```

### Why the boundary list is not redundant

The ids are readable from the placeholders, so a separate list looks like duplication. It is not, because a hole has two meanings:

- **retain** — the client already holds that node and moves its live DOM in, preserving whatever state lives inside it
- **install** — a fragment for it arrives in this response and replaces the placeholder

Nothing in the HTML distinguishes them. The list declares the complete structure while the fragments present declare your selection, so **a hole with no fragment reads as a retain rather than as a truncated response**. That is also what lets you omit fragments without the response becoming ambiguous.

## Two tiers, and they are independent

| tier | what it is | what it buys | what it needs |
| --- | --- | --- | --- |
| boundary holes | `reloadable`, `@cache`, and chain members become their own fragments | DOM retain, independent addressing | one root element and an id |
| slot spans | inside one fragment, every dynamic op's output is reported as its own span | transfer only | an ordinal position, nothing more |

The upper tier preserves browser-owned state across an update. The lower tier removes bytes. Neither depends on the other, and you can consume one without the other.

### Slot spans

A compiled plan is an ordered instruction list, and each instruction writes one contiguous byte range. Recording each non-static instruction's start and end makes the static parts of a fragment the gaps between them. A slot is named by its position in instruction order — a fixed instruction path cannot reorder or omit one — so **no element, no comment marker, and no id is minted for a slot**.

**A sequence is a tree, and there is one per component.** An `if` or a `for` changes which instructions run, but enumerating a sequence per path would be exponential in a component's conditionals, and assembling one from whatever a render happened to produce would make it unfetchable — the server cannot serve a sequence back from its address unless it can reproduce it, and a data-dependent one it could only reproduce by having stored it. So a sequence carries conditional nodes and repeat nodes, and which branch ran and how many times a loop repeated travel with the values. A five-row list and a six-row list share one address.

The address is the plan fingerprint, computed at generation time — which is also why no request-time hashing is involved.

A slot is one whole unit of output, not a bare value. An optional attribute's slot is ` title="…"` or the empty string. A boolean attribute's is ` disabled` or empty. A `href` is the value after our scheme policy ran, or the neutralization marker. There is no case where you have to decide what a slot means.

## Escaping never leaves this module

**Slot values arrive already escaped**, exactly as we write them into HTML today. You concatenate the static parts and the slot values and parse the result once. You apply no escaper, and you make no decision about any value.

This is stronger than telling you which escaper each position needs. It works because reassembly ends in a parse: an escaped value is what a parser wants, and it is what we already produce. If instead you assigned values to nodes directly you would need them unescaped, and then the escaping rules — including the URL scheme allowlist and its data-URL media-type policy — would have to be reproduced on your side. They are not, and they will not be.

## What is static, and how we know

A boundary whose entire subtree contains no dynamic instruction is knowable at generation time, over the same call-graph walk that already computes a component's await, live, asset, and vary properties.

This is stronger than a validator match:

- a validator requires you to have sent one and us to digest the render, and omits only when they agree
- a statically-known boundary is settled at build time, needs no digest and no hint from you, and is keyed by the build identity alone

So a static boundary is never transferred again under one build — on a same-page redraw or on any other request.

The limit: a static subtree only helps where it is already a boundary. A navigation bar inside a layout is covered, because a layout is a chain member. The same markup written as a plain component inside a page is not.

## The static sequences are fetched, not sent

A static sequence is one per fragment shape, not per delivery, per chunk, or per row — a five-hundred-row list shares the sequence a five-row list uses. A page has perhaps five to thirty of them, a few kilobytes in total, immutable until a template changes.

They travel as their own request, addressed by content hash, and not inside the responses that name them. The reason is a difference in lifetime:

| | lifetime | cache policy | ends by |
| --- | --- | --- | --- |
| static sequence | until a template changes | `public`, `immutable` | one response |
| document or decomposed response | one request | `private` or `no-store` | termination marker |
| live delivery stream | the subscription | `private` | lifetime bound, then reconnect |

A static sequence derives from the template rather than from a request, which makes it **the only response on this wire that can be public and served from an edge**. Riding it inside a private response forfeits exactly the property it uniquely has, and makes you re-send every sequence on every page load, every lifetime rollover, and every reconnect.

The request for a sequence is a render mode on a URL you choose. This package mounts no path for it.

### You do not tell us which sequences you hold

There is no client-to-server list of held addresses. The choice between sending a fragment assembled and sending its spans is a **heuristic, not a contract** — spans you cannot resolve cost you one fetch, and an assembled fragment where spans would have done costs a few bytes. Neither is wrong.

What we already have is enough to tune it: the manifest you return distinguishes a fresh navigation from a same-page re-render by which chain validators match. A fresh navigation can send assembled fragments and omit the layout the outgoing page shared; a same-page re-render can send spans and trim further. That policy is yours.

The consequence is that **not waiting on a fetch before first paint is your property to hold**, by sending assembled to a cold client, rather than ours to guarantee.

## All three update paths use the same output

There is one decomposition, and the three paths differ only in which records you choose to send.

**Redraw** — a JavaScript-triggered re-render of one registered component. Returns that component's fragment, plus its nested boundaries as holes.

**Page refresh** — a navigation or a search where the page re-executes. Returns the decomposition of the new composition. A boundary whose validator you returned and which rendered identically is a hole with no fragment.

**Live delivery** — one delivery per source value, scoped to the boundary that produced it, on a connection you opened.

That the three share one shape is the point. Your runtime applies records; it does not need to know which path produced them.

Two things stay path-specific. A redraw carries no live boundary — declaring one on a `reloadable` component is a generation error, because a subscription is reconstructed by executing the page and a redraw's arguments came from you rather than from the page. And a live subscription is always its own request; a delivery never rides on the document or the refresh response.

## Selection is yours

The module decomposes; you choose. Some choices the shape makes available:

- a same-page redraw sends no static fragment at all
- a fresh page sends only what is below the layout boundary you and the client already share
- a fragment whose validator you returned unchanged becomes a hole with no fragment

None of these needs a mode of its own, and none of them makes the response ambiguous, because the boundary list is what says the structure is complete.

## What this does not give you

**Application still parses.** Every fragment is reassembled into a string and installed by parsing it. Focus, selection, form-control state, playing media, and running animations *inside* a fragment are lost when it is replaced. Retain protects state in a nested boundary; it does not protect state in the fragment being replaced.

Applying a value to a node without reparsing — setting `textContent`, calling `setAttribute`, touching nothing else — is a further step, designed and deferred. It costs a second rendering path, per-slot kinds crossing the wire, and the escaping guarantees above becoming your problem instead of ours. Whether it is worth that is a question measurement should answer, and the decomposed form is what makes the measurement possible.

## A cached component is one opaque unit

`CacheStore` is yours to implement, so it is worth saying plainly: **its interface does not change.** `Get(ctx, key) ([]byte, bool)` and `Set` stay as they are.

A `@cache` component's output is never decomposed internally. The annotation already declares that output reusable as a unit, and separating its statics from its variables would contradict its own declaration. The clearest case is a rendered markdown body: it arrives from an external function as one trusted string, has no template structure to separate, and recording spans inside it would find exactly one. Applying it is the same story — the client installs the unit whole, with no slot mapping to carry.

It still participates in the decomposition as a **hole in its parent**: the parent's fragment carries a placeholder for it, and the parent's own render computes the structure around it. What the cache supplies is the child's bytes, which is what it supplies today.

Two consequences:

- **A cached component may not contain a nested `reloadable` component.** That is a generation error, with the declaration position, for the reason `await` already is. A nested boundary would need its own fragment and id and a hole in the cached output, which a stored byte range cannot express — and it is the only thing that would have forced structure into the store.
- **`@cache` gains a single-root requirement**, because a hole needs an element to hold the place. A cached component rendering several roots will stop generating.

Omitting an unchanged cached fragment needs nothing new: the parent's boundary list names the hole, you return the validator you hold, and an unchanged one becomes a hole with no fragment. That is the ordinary comparison path.

One thing never to do, because it looks like a shortcut: **do not expect a cache key to reach the browser.** A cache key frames every declared parameter in plaintext and the runtime hashes nothing, so a key carries parameter values — a user id among them. Fragment identity on the wire is a validator, never a cache key.

## What we need from you

**One root element per boundary.** A placeholder has to be a node. `reloadable` already requires it; `@cache` gains the requirement, as the section above notes.

Nothing from your `CacheStore`, if you have one — see the section above; that interface is unchanged.

**Client obligations.** Four rules, none of which the record shape can enforce:

1. Do not set `If-None-Match` yourself. Issue an ordinary fetch and let the browser revalidate under the response's own cache policy — it reconstructs the full body from its store, so head, live marker, and manifest always reach you. A conditional you issue returns a bodiless 304 and loses all three.
2. On a same-document history navigation, send the manifest describing **the DOM currently on screen**, not the one stored with the entry you are moving to. Going A → B → A and returning A's manifest while the screen shows B makes every boundary compare equal, and nothing is sent.
3. Abort a pending redraw for an instance before issuing another for it, and abort every pending redraw before applying navigation records. A response whose request you aborted is never applied, even if its bytes arrived.
4. Across a navigation into a page that marks itself live: abort the outgoing page's live request, apply the navigation records, then open a new live request. In that order.

**Measurements, if you have them.** Transfer size per delivery, before and after, on real pages. That number should decide whether the deferred step above gets built.

## Related concepts

`requirement:boundary-decomposed-render`, `requirement:structured-render-output`, `data:component-delta-response`, `requirement:component-delta-rendering`, `requirement:component-redraw-endpoint`, `requirement:partial-update-boundaries`, `requirement:component-output-cache`, `api:cache-store`, `decision:cache-component-declaration`, `decision:cache-key-derivation`, `decision:dom-application-strategy`, `decision:live-transport-boundary`, `requirement:update-wire-contract`, `rule:preserved-client-subtree`, `rule:dynamic-slot-kinds`
