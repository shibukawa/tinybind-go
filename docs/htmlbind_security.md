# htmlbind Security Reference

Escaping a string for HTML is not enough to make it safe in every position. `javascript:alert(1)` contains no `&`, `<`, `>`, `"`, or `'`, so an HTML escaper hands it back untouched — correct behavior for text, and a script execution when the same value lands in `href`.

`htmlbind` therefore decides how to treat a value by the position it is written into, not by the value alone. This document lists what each position accepts, what it rewrites, and where the coverage still ends.

For the template language itself, see [htmlbind.md](htmlbind.md).

## Positions and their rules

| Position | Accepts | Applied at render |
| --- | --- | --- |
| HTML text | any type | HTML escaping |
| Ordinary attribute | any type except `html` and the trusted types | HTML escaping |
| URL attribute | `url` only | scheme policy, then HTML escaping |
| URL list attribute | `string` | scheme policy per entry, then HTML escaping |
| Event handler attribute | `trusted_javascript` only | HTML escaping |
| `<script>` content | `trusted_javascript` or `script_json` | none |
| `<style>` content | `trusted_css` | none |

Two of these rows are enforced twice. A URL attribute rejects a `string` at generation time and filters the scheme at render time, because the type system can guarantee that a `url.URL` was parsed but not that it points anywhere sensible.

## URL attributes

A URL attribute requires `url`. Supplying a `string` fails generation:

```text
export component Bad(link: string): html {
<a href={link}>x</a>
}
```

```
attribute href requires url, got string
```

That check does real work before anything renders. `url.Parse` refuses several classic obfuscations outright — `java\tscript:alert(1)` fails on the control character, and `" javascript:alert(1)"` fails because a URL's first path segment cannot contain a colon. A value that reaches the renderer as a `url.URL` has already survived parsing.

Parsing does not judge the destination, though. `url.Parse("javascript:alert(1)")` succeeds, so the scheme is filtered again at render time.

### Which schemes render

By default a URL attribute renders these and nothing else:

| Form | Example | Rendered |
| --- | --- | --- |
| `http`, `https` | `https://example.com/a` | unchanged |
| `mailto` | `mailto:a@example.com` | unchanged |
| `tel` | `tel:+81-3-0000-0000` | unchanged |
| relative | `/images/logo.png` | unchanged |
| scheme-relative | `//cdn.example.com/x.js` | unchanged |
| fragment | `#section` | unchanged |
| raster image `data:` | `data:image/png;base64,…` | unchanged |
| anything else | `javascript:alert(1)` | `#tb-blocked-url` |

A value with no scheme is always permitted. It resolves against the document's own origin and cannot reach another protocol, so the roster does not apply to it.

A rejected value is replaced rather than dropped:

```html
<a href="#tb-blocked-url">…</a>
```

The attribute survives on purpose. A dropped `href` looks exactly like an `href` the template never wrote, so a URL rejected by mistake would leave nothing to find it by. `#tb-blocked-url` is a fragment: it resolves to the current document and reaches nothing.

### What the filter reads

The scheme is read from the text a browser will resolve, not from the `Scheme` field of the parsed URL. The distinction matters more than it looks:

```go
hostile := url.URL{Opaque: "javascript:alert(1)"}
hostile.Scheme        // ""
hostile.String()      // "javascript:alert(1)"
```

An empty `Scheme` field reads as "relative URL" to any check that trusts the struct, while `String()` still renders something a browser executes. The same gap appears in reverse with `url.URL{Scheme: "JAVASCRIPT"}`, which keeps its case because folding happens in `url.Parse` and not in the struct.

Browsers also strip tab, line feed, and carriage return from a URL before parsing it, so `java\tscript:` is a `javascript:` URL as far as the browser is concerned. The filter strips the same characters and trims leading control characters before reading the scheme.

### Inline images

`data:` is filtered by media type rather than allowed or refused as a whole scheme, because an inline image is ordinary authoring:

| Media type | Default |
| --- | --- |
| `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/avif`, `image/bmp`, `image/x-icon` | rendered |
| `image/svg+xml` | blocked |
| `text/html` and everything else | blocked |

SVG is absent from the allowed set deliberately. An SVG document can carry script, which makes `data:image/svg+xml` a script sink wearing an image's media type. Add it with `WithDataURLMediaTypes` if your application needs it and controls where those documents come from.

### The full attribute roster

Every attribute below requires `url` and passes through the scheme filter:

```
href  src  action  formaction  poster  data  xlink:href
cite  background  longdesc  manifest
classid  codebase  archive  profile
```

Only the first group is a script-execution risk. The rest earn their place for a different reason — a browser resolves them, so an application should not be able to point one at an arbitrary destination with a bare `string`. `manifest`, `classid`, `codebase`, `archive`, and `profile` drive mechanisms browsers have removed; they are on the roster because membership costs nothing and closes the question.

### List-valued attributes

`srcset`, `imagesrcset`, and `ping` hold several URLs, which no single `url.URL` can express. They keep their `string` type and are filtered entry by entry at render time:

```text
srcset={candidates}
```

```
/a.png 1x, javascript:alert(1) 2x, /b.png 3x
```

renders as:

```html
srcset="/a.png 1x, /b.png 3x"
```

One refused candidate drops that candidate alone. Refusing the whole attribute would turn a single hostile entry into a missing image.

Be aware of what this does not give you. The threat these attributes carry is usually not a hostile scheme — `ping="https://attacker.example/collect"` is a well-formed `https` URL, and the browser will POST to it when the link is clicked. The scheme filter has no opinion about that. Treat a user-supplied value in `ping` or `srcset` as a destination you are choosing to trust.

## Event handler attributes

An event handler's value is compiled as JavaScript by the browser, so an `on`-prefixed attribute accepts `trusted_javascript` and nothing else:

```text
export component Bad(value: string): html {
<button onclick={value}>x</button>
}
```

```
html:event requires trusted_javascript; wrap the value in RawJavaScript to state
that it is code, or attach the behavior with server-action instead
```

There is no scheme to filter here. The whole attribute value is the handler body, which is why the rule makes the type honest rather than rewriting anything:

```text
<button onclick={RawJavaScript(code)}>x</button>
```

`RawJavaScript` asserts trust; it does not sanitize. The recommended path for behavior remains `server-action`, which names a Go function statically and never substitutes a value into client code.

Two things are unaffected. A static handler with no insertion is authored markup and passes through verbatim, and a hyphenated name such as `on-click` belongs to a custom element rather than to the event handler vocabulary:

```text
<button onclick="doThing()">x</button>
<p on-click={value}>x</p>
```

Values that do reach a handler are still HTML-escaped. That is not a contradiction: an HTML parser decodes an attribute value before the handler body is compiled, so escaping keeps the value inside its quotes without changing the JavaScript the browser sees.

## Configuration

Both policies are render options, so one binary can serve two applications under different rules:

```go
htmlbind.Render(w, page,
    htmlbind.WithURLSchemes("https", "mailto", "sms"),
    htmlbind.WithDataURLMediaTypes("image/png", "image/svg+xml"),
)
```

Each option replaces the default set rather than adding to it. Passing no arguments permits nothing, which is a usable policy and is distinct from leaving the option unset:

```go
htmlbind.WithURLSchemes()          // only relative URLs render
htmlbind.WithDataURLMediaTypes()   // no data URL renders
```

Relative, scheme-relative, and fragment URLs render regardless of configuration.

| Symbol | Meaning |
| --- | --- |
| `htmlbind.DefaultURLSchemes` | `http`, `https`, `mailto`, `tel` |
| `htmlbind.DefaultDataURLMediaTypes` | the raster image media types listed above |
| `htmlbind.BlockedURL` | `#tb-blocked-url`, the substituted value |

Because the defaults can be widened per render, they are deliberately narrow. An application that needs `ftp` or its own registered scheme says so, which is cheaper than shipping a permissive default for everyone.

## Known gaps

### meta refresh

`<meta http-equiv="refresh" content="0;url=…">` navigates the page, and its URL is not filtered.

Nothing on the attribute roster finds it. The attribute is named `content`, and only the sibling `http-equiv="refresh"` gives it a URL meaning at all, so matching it requires reading the element and a second attribute rather than the attribute name. That work is not done yet.

The risk here is not a hostile scheme — browsers already refuse `javascript:` in a meta refresh. It is an ordinary `https` target:

```html
<meta http-equiv="refresh" content="0;url=https://attacker.example/">
```

That navigates with no click and no interaction. Until the position is covered, do not build a meta refresh `content` value from untrusted input.

### Redraw parameter decoding

`htmlupdate.QueryURL` decodes a redraw parameter with `url.Parse` and accepts any scheme it parses. Rendering is filtered, so a hostile scheme arriving this way is neutralized where it would have executed. The value still travels intact up to that point, which means surviving the decoder is not evidence that a URL is safe to use for anything else.

A reloadable component's parameters are public input; see [httpbind_reloadable_componet.md](httpbind_reloadable_componet.md).

### Positions with no context rule

Two sinks are still treated as ordinary attribute text:

- `style` accepts a `string` and receives HTML escaping rather than CSS escaping. The CSS payloads that once executed script are gone from current browsers, so this is a hardening gap rather than an open hole.
- `srcdoc` on `<iframe>` carries a whole HTML document. It is neither a URL nor a script, and it has no rule of its own yet.

## What none of this covers

The rules above constrain what a value can *become* in a given position. They say nothing about whether the value should have been there.

A filtered `href` still points wherever an allowed scheme takes it, and `ping` and `srcset` accept any destination that parses. Authorization, ownership checks, and open-redirect defenses remain the application's work.
