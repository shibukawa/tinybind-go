# Client behaviour: server actions, client handlers, component parameters

A page carries two kinds of behaviour, and they are different things that happen
to sit on the same elements.

A **server action** is a destination. `server-action="Rename"` names an exported
Go function, generation resolves it to an address, and activating the element
performs a mutation. There is one per element and its trigger follows from the
element: a form submits, a button is clicked.

A **client handler** is a listener. `on-click="increment"` names a function the
component's script block produced, and your runtime binds it when the component
mounts. An element may carry several, and most of the useful events are not the
activation event.

This module reserves the attributes, resolves what can be resolved at
generation, and lowers each to markup. Everything that happens in a browser is
yours. In particular **the module reads no JavaScript**: where a decision needs
to know what is inside a script block, you parse it and hand back the answer as a
compile option.

Two packages are called `htmlbind`. In this document `htmlbind.Generate`,
`htmlbind.ComponentScripts` and the option types are the compiler at
`tinybind-go/templates/htmlbind`; `htmlbind.WithCSRFToken` and the render
options are the runtime at `tinybind-go/htmlbind`. Import one under an alias.

---

## Do these three things first

They are not optional. Skipping any of them breaks a page rather than degrading
it.

### 1. Supply a CSRF token on every render of a page that has a form action

A `<form server-action="…">` now carries a hidden token, which makes it an
unsafe form, and a render that reaches one with no token **fails** rather than
writing an empty value:

```go
htmlbind.WithCSRFToken(sessionToken)  // a render with a session behind it
htmlbind.WithoutCSRFToken()           // a render with none, stated deliberately
```

Without either, the render returns `htmlbind.ErrNoCSRFToken`. One form makes this
true for **every** render of that page, including ones nobody will submit.

### 2. Verify the token yourself

The module writes the field. It does not check it — the session, the cookie, and
the verifying middleware are yours. The default field name is `_csrf`
(`GenerateOptions.CSRFFieldName` renames it).

### 3. Check for a POST collision on your own routes

A page whose template declares a form action registers `POST` on its own path
beside its `GET`. If you already hand-register `POST` at that address,
`ServeMux` panics at startup on the duplicate pattern.

A `server-action` on a bare button registers nothing extra, because a button has
no native submit to serve.

---

## Server actions

### What a button emits

Unchanged. The attribute lowers to the handler's direct endpoint and every other
attribute survives unread:

```html
<button server-action="Rename" data-target="#name">rename</button>
```

```html
<button data-tb-action="/_action/00369cf962b6/Rename" data-target="#name">rename</button>
```

### What a form emits

A form gets that attribute **and** the markup a browser needs on its own, from
one build:

```html
<form server-action="Retire"> … </form>
```

```html
<form data-tb-action="/_action/d71506d06c1e/Retire" method="post">
  <input type="hidden" name="_action" value="d71506d06c1e/Retire" />
  <input type="hidden" name="_csrf" value="…" /> …
</form>
```

There is **no `action` attribute**, deliberately. A form declaring none submits
to the document URL, which is already this page with its path parameters filled
in, and a `POST` keeps that URL's query rather than replacing it. So the page
registers `POST` alongside its `GET` and the generated dispatcher branches on the
hidden selector.

One build therefore serves a client with a runtime and one without: your runtime
intercepts the submit, and its absence leaves a working form.

### What your runtime must change

**Your GET-form interception no longer matches.** The emitted form declares
`method="post"`, so anything keyed on "a form with no method" will not see it.
Key submit interception on the presence of the action attribute instead:

```js
document.addEventListener('submit', (e) => {
  const form = e.target.closest('form[data-tb-action]');
  if (!form) return;
  e.preventDefault();
  // …
});
```

**Two extra fields ride along.** A body you collect with `new FormData(form)`
carries `_action` and `_csrf` in addition to the author's fields. Nothing in the
binder rejects a field the input type does not declare, so this is harmless.

### Which entry point to post to

Each handler has two addresses, and they differ in what the handler can read and
in what its silence means.

| | direct endpoint | the page's own POST |
|---|---|---|
| address | `/_action/<hash>/<Name>` | the page pattern |
| path parameters | not carried | carried, already filled in |
| registered for | every exported handler | only a handler a form names |
| handler writes nothing | that empty response stands | `303` back to the page |
| CSRF transport | your request header | the hidden field |

For a **form**, post to the form's own target. It already carries the selector,
and the absent `action` already resolves to the page URL, so nothing needs a
header:

```js
fetch(form.action, { method: 'POST', body: new FormData(form) });
```

For a **bare button**, use the `data-tb-action` URL. Posting to `location.pathname`
instead will 405 unless that page also declares a form action, because the page
POST is registered only for handlers a form names.

If you want a delta rather than a re-rendered page, note that the page POST
answers `303` by default and `fetch` follows redirects. Either use the direct
endpoint, or have the handler write its own response.

### Reading the selector

The selector is the tail of the direct endpoint: `<hash>/<Name>`. The generated
route table publishes both halves:

```go
for _, info := range pages.Actions {
    selector := info.Hash + "/" + info.Handler
}
```

---

## Reading a component's script block

This is the seam the next two features consume, and the reason neither needs a
JavaScript parser in this module.

```go
scripts, err := htmlbind.ComponentScripts("page.tb.html", source)
// scripts[0] == ComponentScript{
//     Component:  "Counter",
//     Script:     "export function setup({ label }) { … }",  // as authored
//     Pos:        …,
//     Handlers:   []string{"increment"},   // names the markup referenced
//     Parameters: []string{"label", "count"},  // declared, in order
// }
```

It runs the same analysis `Generate` does, so a module that would not compile
fails here with the same diagnostic instead of yielding a partial answer. A
component declaring no block is absent from the report rather than present and
empty. A compile that already ran reports the same value on `Result.ComponentScripts`.

`Script` is the block as authored. Reading it from the extracted asset instead is
not available to you: that file exists only after the compile that needs the
answer.

### Through the tree generator

```go
routetree.GenerateOptions{
    ScriptResolver: func(path string, scripts []htmlbind.ComponentScript) (routetree.ScriptAnswers, error) {
        // your JavaScript parser reads scripts[i].Script here
        return routetree.ScriptAnswers{
            Handlers:   map[string]htmlbind.ClientHandlerSet{…},
            Parameters: map[string][]string{…},
        }, nil
    },
}
```

Both maps are keyed by component declaration name, which is unique within one
template module.

Configuring a resolver costs **one extra parse per template carrying a block**,
because the blocks have to be reported before the compile that consumes the
answers can run. A tree configuring none parses once, as it always has.

---

## Client handlers

```html
<button on-click="increment" on-blur="validate" data-id={row.ID}>+1</button>
```

```html
<button data-tb-on="click:increment,blur:validate" data-id="7">+1</button>
```

One marker per element, comma between entries and colon within one, so a runtime
finds every bound element with a single indexed query:

```js
for (const el of root.querySelectorAll('[data-tb-on]')) {
  for (const entry of el.dataset.tbOn.split(',')) {
    const [event, handler] = entry.split(':');
    el.addEventListener(event, (e) => namespace[handler](e));
  }
}
```

### What is reserved, and what is not

The reservation applies **only inside a component declaring a `<script
component>` block**. Everywhere else an `on-`prefixed hyphenated attribute keeps
its ordinary meaning and is emitted unread, so do not expect the marker outside
such a component.

- `onclick` is unchanged: still inline JavaScript, still requiring
  `trusted_javascript`. It is not reinterpreted.
- `on-my-event` — a second hyphen — is **not** matched, and stays the ordinary
  custom-element attribute it was.
- The value must be a static JavaScript identifier. A computed value is a
  generation error, because it could not be checked.
- Two `on-click` on one element is an error; the second would be lost.

### Answering with the resolved set

```go
htmlbind.ClientHandlerSet{
    Resolved:   []string{"increment", "validate"},
    Unresolved: map[string]string{"validate": "setup returned it conditionally"},
}
```

A name in `Unresolved` fails generation at the attribute that referenced it,
carrying your reason and our template position.

**Report a refusal explicitly rather than by omission.** An omission cannot be
told from a map that was never populated, so a block your parser mis-read would
report every one of its names as unknown.

A component with **no entry at all** is unchecked: every name it references is
accepted and lowered. That is what lets the reporting pass run before you have
anything to answer with.

---

## Component parameters

A handler frequently needs a value the component was called with. The script
block is extracted to one content-hashed file shared by every instance and every
render, so there is nothing per-render to interpolate into it, and reading a
rendered attribute back loses the type.

Name the parameters to emit, per component:

```go
ComponentParameters: map[string][]string{"Card": {"label", "count", "row"}}
```

```html
<div data-tb-component="templates.card.Card" data-tb-props="{&#34;label&#34;:&#34;hi&#34;,&#34;row&#34;:{&#34;id&#34;:&#34;7&#34;}}" class="card">
```

```js
const props = JSON.parse(el.dataset.tbProps);
```

The object rides the component's single root element, beside the declaration
marker, which is why the component must declare a script block: nothing else
would consume it, and the single-root invariant it needs exists for that marker.

### Types

The rule is the one `JsonForScript` already applies, not the one a reloadable
component carries. A reloadable component refuses a record and a slice because a
**query string** must carry them deterministically; an attribute holding JSON is
not a query string.

- Accepted: `string`, `bool`, `int`, `float`, `decimal`, an enum, an array of an
  accepted type, and a record whose fields are all accepted, recursively.
- Refused: `html`, and a value that has not settled.

A refusal is a generation error rather than a silent omission, for the reason the
reloadable diagnostics already give: the author asked for it by naming it in code
that uses it.

### Absence

An absent optional **omits its key**. It does not write `null`, so JavaScript has
one absence to test rather than two — which is what the attribute context already
does when it omits a whole attribute.

```js
if ('count' in props) { … }
```

### Disclosure

An emitted parameter is in the DOM, where it is readable and editable by the
client. Deriving the set from what `setup` destructured means **the disclosure
boundary is decided by whether a name was destructured in JavaScript** — which
reads fine until someone pulls out `{price}` for a display string. Say so where
you document this.

Treat anything that comes back from the client as untrusted input, and sign
anything that must not be edited.

---

## Attribute names you can change

| setting | default | on |
|---|---|---|
| `ActionAttr` | `data-tb-action` | `Emitter`, `GenerateOptions.ServerActionAttr` |
| `ActionSelectorField` | `_action` | `Emitter`, `GenerateOptions.ServerActionSelectorField` |
| `ClientHandlerAttr` | `data-tb-on` | `Emitter`, `GenerateOptions` |
| `ComponentParameterAttr` | `data-tb-props` | `Emitter`, `GenerateOptions` |
| `CSRFFieldName` | `_csrf` | `GenerateOptions` |
| `ActionPrefix` | `/_action` | `Emitter` |

**These do not follow `DataAttributePrefix`.** That option renames the boundary
and declaration markers — `data-tb-id`, `data-tb-component` — and the four
attribute names above are literals it does not reach. A project setting
`DataAttributePrefix: "pw"` gets `data-pw-component` next to `data-tb-on` unless
it sets each of these too. This has always been true of `ActionAttr`; there are
now four of them.

---

## What is not there

- **CSRF verification.** The field is written; checking it is middleware's.
- **Renaming the CSRF field through `routetree`.** `GenerateOptions.CSRFFieldName`
  is a `templates/htmlbind` option and the tree generator forwards neither it nor
  `CSRFMode`, so a tree always emits `_csrf` and always emits it. That gap
  predates this feature — an author-written `<form method="post">` already hit it
  — but a server action form now carries a token where it did not, so a framework
  whose middleware expects another name will meet it. Compile those templates
  directly, or ask and it becomes an emitter setting.
- **A template outside the route tree still emits a GET form.** When you resolve
  an address through `ActionResolver`, no selector exists, so the form keeps the
  scripted attribute alone and carries no method and no token. This module
  registers no route for that address and cannot know what method yours accepts.
  Supply a selector through `ServerActionSelectors`, or own the fallback.
- **The script-free mode** — suppressing the runtime, the boundary markers and
  the async streaming for a crawler or a mail body — is designed and unbuilt.
  Actions no longer wait on it.
- **Carrying the selector in the element attribute** instead of the URL is not
  built. It cannot be the default, because lowering to `hx-post` needs an address
  there; ask if you want it as a profile setting.
- **A trigger model.** The event in emitted markup follows from the element kind.
  A template cannot pick an arbitrary event to fire a server action on, because
  that would only work with a runtime and would reopen the split the native form
  markup exists to close.
