# Proposal: a structured render output, and what verification changed

**From:** `github.com/shibukawa/tinybind-go`
**To:** Popcorn Wave (`github.com/shibukawa/popcornwave`)
**Against:** your change request of 2026-08-08, raised against v0.4.2
**Status:** superseded, and kept as the record of the round rather than as a current design.

> The verification in Part 1 stands and is what moved the priorities. The design in Part 2 does not: a later round of the same day scoped the work down to decomposing a render at its boundaries, with the split inside a dynamic region deferred and then built the same day. **Read [`httpbind_update_surface.md`](httpbind_update_surface.md) for how to use what was built, and [`httpbind_render_modes.md`](httpbind_render_modes.md) for why it is shaped that way.** The two differ on how a client applies a delivery — this document assembles values into the DOM, and the shipping design reassembles a fragment and reparses it.

## Summary

All three asks are accepted. One is larger than you described, one is smaller, and one has a blocker that is not the one its open question names.

Before the design, three things verification changed. We read every claim against v0.4.2 and rendered two of them rather than reasoning about them, and the result moves your priorities:

1. **Ask 2 is the defect in this round, not the footnote.** The redraw entry does not merely miss the cache store — it passes no render option at all, and neither does the action path you listed as unaffected. Two of the absences fail rather than default.
2. **Your record sketch does not survive contact with the plan.** The separation is there and your reasoning about it is right, but the statics you drew cannot be read off a plan, and an attribute is not the shape your table assumes.
3. **Ask 1 is our own unnamed half.** `decision:dom-application-strategy` chose the static-dynamic split as the end of its staging in 2026-08-01 and named no module output to install from. You are asking for a half this catalog left unwritten.

Sequencing differs from yours: **render options, then the update flag, then the structured output.** You ranked by value and put the structured output first. We agree on value. The two items ahead of it are a defect and an opt-in flag, and neither needs a design round.

---

## Part 1 — What verification found

### 1.1 The redraw and action entries pass no options at all

`htmlupdate/redraw.go` `writeRedraw` and `htmlupdate/action.go` `WriteUpdateStatus` both call `htmlbind.Render(&out, fragment)` with nothing else. Thirteen render options exist. Both entries pass zero.

**Correction to your report.** You wrote that the document, navigation delta, and action paths all reach `htmlbind` through an entry taking `[]htmlbind.Option`. The action path does not: `WriteUpdate` and `WriteUpdateStatus` take no options either, and carry the worse instance of the gap.

Two absences are wrong rather than merely default. Both measured against v0.4.2:

**A component containing an unsafe form cannot render.** `CSRFField` requires `WithCSRFToken` or `WithoutCSRFToken`, and with neither supplied it returns `ErrNoCSRFToken`. Rendering a plan of `Static` / `CSRFField` / `Static` with no options returns:

```
htmlbind: form needs a CSRF token: this render supplied none;
pass WithCSRFToken, or WithoutCSRFToken for a render with no session behind it
```

On the redraw path that becomes `FailureRenderFailed` and a 500. On the action path `WriteUpdateStatus` returns the error before writing. `WriteUpdateStatus` is documented as the way a failed validation returns 422 and rewrites the form region with its errors — and a form region is exactly what cannot render.

**A configured URL scheme allowlist does not reach either path.** A `url` of `myapp://open/42` in an `href`:

```
no options                          -> <a href="#tb-blocked-url">x</a>
WithURLSchemes("http","https","myapp") -> <a href="myapp://open/42">x</a>
```

So a component renders one way inside its page and another in the response that replaces it. The divergence is stricter, not looser, so nothing hostile renders — this is a correctness defect, not a hole. It does mean `requirement:url-attribute-scheme-safety`'s acceptance that "the redraw decode path and the render path apply the same allowlist" is not held in the direction that costs an application its own scheme.

Two more follow from the same cause: a redrawn component owning an await or live boundary writes placeholders under the default boundary prefix rather than the configured one — the same defect shape as the hardcoded `data-tb-kind` the wire contract round found, in generated output this time — and both entries render under `context.Background()`, so request cancellation reaches neither a context-taking external nor a shared store.

**The fix is smaller than the finding.** `Options.renderOptions(caller)` already exists, already carries the boundary prefix and the build-identity validator tag, and already appends caller options after its own so a caller can override. The document and stream paths use it. The redraw and action paths simply do not call it.

**One thing cannot be auto-wired.** `Options.CSRFToken(r)` reads the token the *request claimed*, from the header or the form field. The session's token is the caller's to produce — that is what `VerifyCSRF`'s `expected` parameter is for. Echoing the claimed token back into a rendered form would put an attacker-supplied value in it. So the token has to arrive from the caller, which is what the variadic is for.

### 1.2 The statics are not there to expose

Your reading of the mechanism is right: `Op` is one `Exec` method, `staticOp` is a string, `textOp` holds a function, both are unexported, and `execOps` concatenates before any caller sees output. Diffing two renders downstream would indeed give you no slot kinds. We are not asking you to build that.

What measurement adds is that the statics are not recoverable *inside* the module either. A generated plan for `<article hidden={...} title={...}><a href={...}>` emits:

```go
Static(" <article"),
BoundaryAttr(),
BoolAttr("hidden", …),
Attr("title", …),
Static("> <a"),
URLAttr("href", …),
Static(">"),
Text(…),
Static("</a> "),
```

A static run ends mid-tag. The ` title="` prefix and the closing quote are state inside the attribute op, not `Static` values. So producing your `"s"` array is emitter work that changes what is emitted — which you say, and this is how far.

Three consequences your table does not cover:

**An optional attribute is structure, not a value.** `Attr` returns `(string, bool)`, and an absent value omits the attribute name and its quotes along with the value. A unit holding one has no fixed skeleton: presence changes the static runs themselves. Your boolean row covers `BoolAttr`; your attribute row assumes a value always exists.

**An attribute value is already assembled, and partly already escaped.** The emitter builds a mixed value as `"card " + htmlbind.Escape(status)` — author literals verbatim, expressions escaped, joined inside one closure. A raw attribute value does not exist at runtime. `class` additionally passes its literal through scoped-class rewriting. More on this below; it is the one place the split does not reach cleanly.

**A child unit is opaque.** `Component` binds to a `Fragment` carrying `render func(*Renderer) error` and nothing else — no plan pointer, no params, no identity.

### 1.3 The identity ask half holds

You asked us to name units with an identity that already exists, so a client's skeleton cache, a server's output cache, and a boundary validator invalidate together on a regeneration. **The rule is right**, and right for the same reason `decision:cache-key-derivation` adopted it.

The assumption underneath it is not. `CachePolicy.ID` is emitted only for a component carrying a cache annotation. `Boundary.ComponentID` is emitted only for a component that can be a chain member. `Plan` carries no identity field at all. A message row — your own example — has neither. So this is one rule and new emission, not one rule and no emission.

### 1.4 What is already the right shape

Two things work in our favour, and they are why this is tractable at all:

- A `For` body is generated as a **separate typed op list under its own scope struct**, executed once per item. One shared skeleton with one value set per item is what generation already emits, not a form it has to invent.
- An `If` branch is its own op list.
- A `Text` closure returns the value **unescaped**; `textOp` escapes at `Exec`. The text kind is available as data today.

Minor: `examples/live_render` does not exist in this repository. The live template shape is `testdata/templates/htmlbind/live`. Nothing in your argument depends on it.

---

## Part 2 — Design direction for Ask 1

### 2.1 A unit is a component

Not a loop body, not a conditional branch, not an arbitrary region.

This is what makes the identity rule you asked for hold without inventing a second identity kind: `decision:cache-key-derivation` is already component-scoped, and nothing in it contemplates naming a loop body or a branch. It also collapses nesting to one op — `Component` — so the child-unit problem in §1.2 has to be solved once, in one place, rather than three times.

**What it asks of you.** A loop body you want as a shared skeleton should be a component call:

```
{#for m in messages}<MessageRow m={m}/>{/for}
```

rather than a `<li>` written inline. This is also what `decision:list-item-key` will want, since a match target has to be a node with a single root, and it is what your own record sketch already assumes when it gives the row its own template identity.

**What it costs us.** `decision:generated-render-plan` inlines a private component that uses no phase-dependent capability. Being a unit becomes a fifth phase-dependent capability, alongside await ownership, slot declaration, partial-update boundary, and output cache. Which components are units therefore has to be decided rather than defaulted to "all", because "all" would stop every private helper from inlining and make the ordinary render path pay for the structured one.

### 2.2 Skeleton identity is a content address

The identity is a hash of **the emitted skeleton**, not of the component declaration.

This dissolves two of the three structural problems in §1.2:

- A conditional changes structure rather than values — so each branch is simply its own address.
- An optional attribute changes the static runs — so present and absent are two addresses.

A client caches by address; a server sends an address it has not sent on this connection. The count is bounded by the optional elements in one component, each skeleton is small, and each travels once. `decision:dom-application-strategy` already described this property — "the component kind hash is a content address, so a skeleton is immutable, permanently cacheable, and invalidated by a deploy" — and this is that sentence taken literally.

It gives you **one rule, several addresses** — not one identity. The addresses do not coincide: `decision:cache-key-derivation` is scoped per component, while a skeleton address is per emitted skeleton, so a component with a conditional or an optional attribute has several. What is shared is the derivation: all of them come from the same generation-time fingerprint over the same emitted plan, so a template edit invalidates a client's skeleton cache, a server's output cache, and a boundary validator together. That invalidation property is what you actually needed, and there is one rule to explain rather than three.

### 2.3 A unit is not an update boundary

We are keeping these on separate axes, and the reason is that their costs scale differently.

| | scope | when it travels | cost with instance count |
| --- | --- | --- | --- |
| update boundary | instance | manifest entry and validator, returned by the client on **every request** | linear |
| structured unit | template | skeleton, sent **once per connection** | flat |

A five-hundred-row message list is exactly the case that needs a row to be a unit and needs it not to be a boundary. `decision:manifest-state-ownership` already says to place boundaries at meaningful regions rather than per list row, for the linear cost above. If the two were one declaration, a row would be either too expensive to declare or invisible to the split — and the split exists for the row.

So your Ask 3 and Ask 1 stay separate declarations, and a component may be either, both, or neither.

### 2.4 Slot kinds

Your escaping table is accepted, with one rule stated under it and one kind added.

**The rule.** An *encoding* is fixed by the output position and can be known without looking at the value; a *policy* is read from the value and from a render option. An encoding may be named to a client, because naming it moves no judgement. A policy never travels, because moving it moves the judgement. This is the same split `decision:url-context-escaper` drew when the scheme check moved out of the value closure into an op that can read the render options.

| kind | what the module sends | what a client does |
| --- | --- | --- |
| text | the value before escaping | assign to a text node; escape only if splicing into markup |
| attribute | the assembled attribute value before escaping | `setAttribute` as is |
| optional attribute | presence, plus the value when present | absence removes the attribute, not just its value |
| boolean attribute | presence alone | add or remove |
| URL attribute | the value after this render's scheme policy, or the neutralization marker | set it; test nothing |
| URL list attribute | the surviving entries, one bad entry dropped | set it |
| raw | the value unchanged | exactly as trusted as today |
| module owned | nothing addressable | it is fixed output for that render |

Values travel **before** escaping, because `setAttribute` and a text node take unescaped values and an escaped one would be visibly wrong there. A client splicing into markup applies the escaper the kind names. What never travels is the scheme allowlist and the data media-type policy — they are render options a caller sets and a judgement over the value, and your reasoning that the URL check must not move is accepted unchanged.

`module owned` is the kind you did not have: the boundary identity attribute, the CSRF field, and the merged head are written from render state rather than from parameters. A client cannot supply them and must not be able to replace them.

An unrecognized kind makes a client take the assembled form for that unit rather than guess, so adding a kind later degrades instead of breaking.

### 2.5 The one place the split does not reach cleanly

A mixed attribute value — `class="card {status}"` — is one closure returning `"card " + Escape(status)`. The literal is verbatim; only the expression is escaped.

**v1 rule: an attribute value is one dynamic carrying its whole assembled value, literal parts included.** `card active` travels, not `active`. The cost is bounded and small, and it keeps the attribute a single slot the client can `setAttribute`.

Making that value travel unescaped means the emitter stops escaping inside the closure and the op escapes at write time — precisely the move `URLAttr` already made, and for the same structural reason. The compatibility question it opens is measurable: it changes output for any mixed attribute value whose **literal** part contains a character the escaper touches. We can measure that across the fixture tree before committing, and we will report the number rather than our expectation of it.

Splitting an attribute value into its literal and expression parts — a mini-skeleton inside the attribute — is possible and is not proposed for v1.

### 2.6 What this changes in your record shape

- `"s"` cannot be a flat array of strings. Attributes carry static text inside their ops, and an optional attribute changes the runs themselves. A skeleton is an element description.
- `"t"` becomes a content address rather than a component name plus a plan hash. It still changes on a template edit, by the same rule.
- The loop record is unchanged from your sketch, **provided the body is a component**.
- A mixed attribute value is one entry in `"d"`, not several.

We would rather settle the element-description shape with you before emitting anything. Writing `docs/httpbind_update_wire_contract.md` found a defect that was invisible to the only implementation that existed; the same argument applies here, earlier.

---

## Part 3 — Ask 2 and Ask 3

### Ask 2 — render options on the update entries

Accepted, and **raised to must** on the measurement rather than on the report. The ask widens from a cache store on `Redraw` to a `[]htmlbind.Option` variadic on `Redraw`, `WriteUpdate`, and `WriteUpdateStatus`, matching `RenderStreamAsync`, with both entries routing through the existing `renderOptions` helper so the boundary prefix and the build-identity tag stop being absent.

Agreed and not contested: the store belongs to the caller per render, not on `Options`. `WithCache`'s own documentation says so.

Open on our side, both small:

- whether a render reaching `CSRFField` with no token supplied stays a failure. We think it should — the diagnostic already names both ways out — but it is a behaviour change on a released path and it is a decision rather than a default.
- whether `WriteUpdate` gains a request parameter so it can supply the context itself, or whether the caller passes `WithContext`. `Redraw` already holds the request and can supply it.

### Ask 3 — priority for the explicit update flag

Accepted as scheduling, as you framed it. Its recorded open question is the flag's syntax; that is not what is blocking it.

`reloadable` already ships — exported, single-rooted, carrying a kind id and an instance id, published as an endpoint. Nothing in our catalog states what an update flag adds that `reloadable` does not. Our reading is that `reloadable` is client-addressed re-rendering and the update flag is participation in server-discovered navigation deltas, but `requirement:partial-update-boundaries` lists them adjacently without saying so. Three things settle it:

1. whether one component can be both, and whether `reloadable` should imply the flag;
2. whether a flagged component carries an author-written DOM id, as `decision:author-declared-boundary-id` requires of explicit boundaries, or the automatic positional identity chain members get — the manifest entry shape follows from this;
3. the exported-only rule, which forces a component to be exported to become updatable and which we said we would revisit.

Your framing that finer boundaries decide *what* is sent and statics and dynamics decide *how much each costs* is right, and it is why this is sequenced ahead of Ask 1 rather than behind it: it is designed, it is opt-in, and it blocks on nothing.

---

## What we would like from you

1. **The measurements you offered.** Transfer size per delivery on real pages, before and after. That number should decide whether the emitter work is worth doing, and we would rather have yours than our estimate.
2. **A read on the element-description skeleton shape**, before it is emitted.
3. **Whether component-per-row is acceptable authoring cost** on your side. §2.1 is the one place this design asks something of your templates rather than of our emitter.

## What is not changing

- The wire format, the record shape, and protocol versioning stay yours.
- The browser runtime stays yours.
- Boundary identity, the validators, the manifest codec, and the delta operation kinds are unchanged.
- `requirement:live-mode-plan-slice` remains accepted, unbuilt, and separate — you did not make this request depend on it and neither do we. It shares a home with the structured output, since both take a generated plan and use part of it rather than all of it.
