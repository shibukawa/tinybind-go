# htmlbind User Guide

`htmlbind` compiles typed `.tb.html` templates into Go render plans. Templates are not parsed at runtime; value types and HTML insertion contexts are checked during generation.

Each component becomes an immutable instruction list typed by its parameter struct, and the shared `htmlbind` runtime walks it. Generated code owns rendering only: it never touches `net/http`, sets no headers, and negotiates no content encoding. Your handler owns the response.

## What is automated

- Discovering `.tb.html` files
- Generating Go declarations for template types, enums, and exported components
- Generating one render plan per component
- Checking text, attribute, URL, script, and style contexts
- Escaping ordinary strings for HTML
- Omitting optional attributes
- Rendering component composition, `if`, and `for`
- Filling named and unnamed slots
- Scoping component styles and merging head contributions into the document
- Running `await` boundaries concurrently and streaming them as they settle
- Re-rendering one region per delivery when a boundary binds a `live` source
- Reusing the output of `@cache` components through a store you supply
- Reporting type and unsafe-context errors with file, line, and column

You do not need to understand generated implementation details. Application code binds the `export component` declarations to their parameters and renders the result.

## What you provide

1. `.tb.html` files directly inside a Go package directory
2. A `package` declaration and the required `type`, `enum`, and `component` declarations
3. Same-package Go implementations for any declared external functions
4. Handlers or other code that calls exported components
5. A code-generation command

## Setup and generation

```go
package pages

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir .
```

Place `profile.tb.html` in the same directory, then run:

```bash
go generate ./...
```

The generator discovers `.tb.html` and `.tb.sql` files directly inside the target directory and combines them in `tinybind_templates_gen.go`. It does not descend into child package directories; generate each package separately.

To use another naming convention, pass base-name globs with
`-html-template-pattern` and `-sql-template-pattern`, for example:

```go
//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -html-template-pattern "*.page.html" -sql-template-pattern "*.query.sql"
```

The defaults remain `*.tb.html` and `*.tb.sql`.

## Minimal component

`hello.tb.html`:

```text
package pages

export component Hello(name: string): html {
<!DOCTYPE html>
<html lang="en">
  <body>
    <h1>Hello, {name}</h1>
  </body>
</html>
}
```

Generated public API:

```go
type HelloParams struct {
	Name string
}

func Hello(params HelloParams) htmlbind.Fragment
```

Every component takes exactly one argument, a generated `{ComponentName}Params`
struct with one exported field per declared parameter, in declaration order.
The rule is the same for zero, one, and many parameters. Private components get
an unexported `render{Name}Params`.

`Hello` does not write anything. It returns a `Fragment`: the plan paired with
its parameters. Rendering is a separate step, so your handler keeps control of
status, headers, and error handling:

```go
import "github.com/shibukawa/tinybind-go/htmlbind"

func hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := htmlbind.Render(w, Hello(HelloParams{Name: r.URL.Query().Get("name")})); err != nil {
		// The response may already be partly written; log rather than rewrite it.
		log.Printf("render failed: %v", err)
	}
}
```

A `Fragment` is immutable and safe to share, so a parameterless wrapper can be
built once at startup and reused across requests.

## Declaring types

```text
package pages

type User {
  name: string
  active: bool
  nickname: string?
  profileURL: url
  tags: string[]
}

enum Tone { Primary, Secondary }

export component Profile(user: User, tone: Tone): html {
<article>
  <a href={user.profileURL}>{user.name}</a>
</article>
}
```

The application-facing declarations have these shapes:

```go
type User struct {
	Name       string
	Active     bool
	Nickname   *string
	ProfileURL url.URL
	Tags       []string
}

type Tone string

const (
	TonePrimary   Tone = "Primary"
	ToneSecondary Tone = "Secondary"
)

type ProfileParams struct {
	User User
	Tone Tone
}

func Profile(params ProfileParams) htmlbind.Fragment
```

A type declared in a template becomes a type in the generated Go package, and the caller constructs that generated type to call the component. Nothing converts between a hand-written struct and a template type, so a shape that drifts out of agreement is a Go compile error.

### Type mapping

| Template type | Go type passed by the caller |
| --- | --- |
| `string` / `decimal` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `float` | `float64` |
| `bytes` | `[]byte` |
| `datetime` / `date` / `time` | `time.Time` |
| `url` | `url.URL` |
| `T[]` | `[]T` |
| `T?` | `*T` |
| `html` | `htmlbind.Fragment`, the value a slot accepts |

## Conditions

```text
export component Status(active: bool): html {
{if active}
  <span class="active">active</span>
{else}
  <span class="inactive">inactive</span>
{/if}
}
```

`else if` is also supported:

```text
{if score >= 80}
  <strong>A</strong>
{else if score >= 60}
  <strong>B</strong>
{else}
  <strong>C</strong>
{/if}
```

The condition must have type `bool`.

## Loops

```text
type User { name: string }

export component UserList(users: User[]): html {
<ul>
{for user, index in users}
  <li data-index={index}>{user.name}</li>
{/for}
</ul>
}
```

Omit the index when it is not needed:

```text
{for user in users}
  <p>{user.name}</p>
{/for}
```

## Composing components

A component without `export` is private to template composition.

```text
type User { name: string }

component Badge(label: string, children: html): html {
<span class="badge"><strong>{label}</strong>{children}</span>
}

export component Card(user: User): html {
<Badge label={user.name}>
  <em>member</em>
</Badge>
}
```

The application-facing API is only the exported component:

```go
func Card(params CardParams) htmlbind.Fragment
```

A component with a `children: html` parameter receives the content between its start and end tags. Components without children can be called with self-closing syntax:

```text
<Avatar user={user} compact={true} />
```

## Slots

`<slot>` marks where a component inserts content its caller supplies. The slot
element itself is never emitted; only the supplied content or the declared
default is.

```text
component Panel(title: string, header: html?, children: html, footer: html?): html {
<section class="panel">
  <div class="head"><slot name="header"><b>{title}</b></slot></div>
  <div class="body"><slot required /></div>
  <slot name="footer" />
</section>
}
```

- `<slot />` is the unnamed slot and binds the reserved `children` parameter.
- `<slot name="header" />` binds the `header` parameter of type `html`.
- Children of the slot element are the default content, rendered when the
  argument is absent.
- `required` marks a mandatory slot and must agree with the declared type:
  `required` needs `html`, its absence needs `html?`.
- An absent optional slot with no default leaves nothing behind: no element, no
  wrapper, no marker.

The caller fills a named slot with a `template` element carrying the same
`name`, and the unnamed slot with the remaining content:

```text
export component Page(caption: string): html {
<Panel title={caption}>
  <template name="header"><em>Guide</em></template>
  <p>body text</p>
</Panel>
}
```

Whitespace between fill blocks does not count as unnamed content. A `template`
element without a `name` attribute is ordinary markup and is emitted as written.

A slot may sit inside an `if`, so a component can legitimately drop its
children. It may appear in both branches of an `if`, because only one branch
runs. It may not appear inside a `for` body, and it may not render twice on the
same path.

Slot arguments are not values: a slot parameter cannot be read in an
expression, so it cannot be tested for presence, forwarded, or inserted twice.
Use default content instead of testing whether the caller supplied something.

## Composing whole documents

A document shell, any number of layouts, and a page are separate components,
often in separate files. `RenderChain` composes them, outermost first, each
filling the next into its unnamed slot:

```go
wrappers := []htmlbind.Wrapper{
	BindDocument(DocumentParams{Title: "Docs"}),
	BindLayout(LayoutParams{}),
}
err := htmlbind.RenderChain(w, wrappers, Page(PageParams{Body: "hello"}))
```

The wrapper list is variable length; an empty list renders the page alone,
which is what `htmlbind.Render` does.

The two generated shapes make misuse a compile error: only a component with an
unnamed slot gets a `Bind<Name>` returning `Wrapper`, so a leaf cannot be used
as a wrapper. Assembly is validated before anything is written, so a chain
missing its leaf fails while the status code can still be changed.

## Component styles and scripts

A component may declare a `head` element outside the document shell. Its
contents are hoisted into the document head instead of being emitted where they
appear:

```text
export component Card(label: string): html {
<head>
<link rel="stylesheet" href="/shared.css" />
<style>
.box { color: red; animation: fade 1s }
.box .label { font-weight: bold }
@keyframes fade { from { opacity: 0 } }
</style>
</head>
<div class="box shadow"><span class="label">{label}</span></div>
}
```

Inside such a `head`, `style` and `script` bodies are raw text, so CSS and
JavaScript braces are not template syntax. Contributions must be static markup;
the merged head is written before the first body byte, so it cannot depend on
request data.

A contribution may be a `link`, `meta`, `style`, `script`, `title`, or
`noscript`. `noscript` is the only one that may hold elements — `link`, `style`,
and `meta` — because everything else there would be body content:

```html
<head>
<noscript><meta http-equiv="refresh" content="0; url=/no-script"></noscript>
</head>
```

A contribution written this way is unconditional, which is right for a page that
always wants it. A tag that should appear only for some responses is supplied at
the render call instead, through `htmlbind.WithHead`.

Every component reachable from the rendered chain contributes, including
components called from a body, and identical tags are emitted once. Identity is
per tag, so two components that both link `/shared.css` and then declare their
own styles emit one link and two style blocks.

Those contributions need somewhere to land. The document shell is the component
that owns `html`, `head`, and `body`, and its `head` element is the destination.

#### Inspecting contributions

A bound component reports its own contributions, one entry per tag, together with
the component that declared each one:

```go
fragment := Badge(BadgeParams{Label: "new"})

fragment.Head()
// []string{`<link rel="stylesheet" href="/shared.css">`, `<style>.badge_uvkb9m { color: red }</style>`}

fragment.HeadSources()
// []string{"Badge (components.tb.html:5:1)", "Badge (components.tb.html:6:1)"}
```

`Head` and `HeadSources` are two views of one list: same length, same order, so
index `i` of either describes the same tag. `Wrapper` carries the same pair, so a
chain member can be inspected without knowing which form it is.

This is what a caller uses when it cannot deliver a contribution. A response with
no document shell — a partial rendered for a swap library, say — has no head to
merge into, and dropping a contribution would swap in an unstyled region with
nothing in any log. Rejecting it is the safe choice, and `HeadSources` is how the
report names the component to change rather than printing head markup the reader
then has to grep for:

```go
if head := fragment.Head(); len(head) > 0 {
	// component Badge (components.tb.html:5:1) declares <link rel="stylesheet" ...>
	return fmt.Errorf("component %s declares %s", fragment.HeadSources()[0], head[0])
}
```

`MergeHead` returns the merged tags with duplicates dropped, so it has no
matching source list. Ask a chain member for its own.

### Scoped styles

A component's style block is scoped by renaming the class names it declares and
rewriting the matching `class` attributes in the same component:

```css
.box_dwu687 { color: red; animation: fade_dwu687 1s }
.box_dwu687 .label_dwu687 { font-weight: bold }
@keyframes fade_dwu687 { from { opacity: 0 } }
```

- Classes the style block does not declare pass through unchanged, so utility
  classes from an external framework keep working.
- `@keyframes` names are renamed too, along with their `animation` and
  `animation-name` references, because those names are only referenced from
  within CSS.
- `font-family` names and CSS custom properties stay global, so `@font-face`
  and theming still work across components.
- `:global(...)` opts a selector out of scoping.
- A bare element selector such as `p { ... }` is a generation error: it carries
  no name to rename, and leaking it to every page would defeat scoping. Qualify
  it with a class, as in `.card p { ... }`.
- A class supplied through an expression cannot be rewritten and is a
  generation error.

The suffix is derived from the template path and component name, so unrelated
edits do not change generated class names.

### Extracted static files

A `style` block and a `script` block carrying inline content never reach the
response. Generation writes them as files and puts a reference tag in the merged
head instead, so the bytes are cached by the client and a Content Security
Policy may forbid inline script:

```html
<link rel="stylesheet" href="/public/generated/card.style.1f0a3c9d4b21.css">
<script src="/public/generated/card.script.7c62e0b1d938.js" defer></script>
```

- Style blocks of one template file bundle into one stylesheet; each component
  script becomes its own file, so `defer`, `async`, `type`, and any other author
  attribute survive on its tag.
- The file name carries a hash of the content, so the URL is immutably
  cacheable and an unchanged project regenerates identical names.
- A `script` or `link` that already names an external URL contributes its tag
  unchanged and produces no file.
- Extraction happens at generation time. Nothing is assembled per request, and
  the reference tag needs no per-request collection because the composition
  cannot change it.

Two generator options decide where the files go and how they are named:

| Option | Default | Meaning |
| --- | --- | --- |
| `PublicDir` | `public/generated` | directory receiving the generated files |
| `PublicURLBase` | `/public/generated` | prefix of the reference URL |

The generate command exposes them as `-public-dir` and `-public-url-base`.
Neither is derived from the other: the file path is `PublicDir` joined with the
file name, the reference is `PublicURLBase` joined with the same name, and no
path segment is ever added, stripped, or inferred. `PublicURLBase` is used
verbatim, so a full URL such as `https://cdn.example.com/assets` emits absolute
references without changing where files are written. Setting one option
requires setting the other; configuring only one fails generation.

## Attributes

### Ordinary attributes

```text
<p title={user.nickname}>{user.name}</p>
<p class="user {user.active ? 'active' : 'inactive'}">...</p>
```

When a `string?` supplies the entire attribute value, a nil value omits the whole attribute:

```text
<p title={user.nickname}>...</p>
```

An optional value cannot be mixed with static text in one attribute:

```text
<!-- Invalid when nickname is optional -->
<p title="User: {user.nickname}">...</p>
```

### Boolean attributes

```text
<article hidden={not user.active}>...</article>
```

The attribute is emitted only when the value is true. Static boolean attributes are also supported:

```text
<input disabled>
```

### URL attributes

URL attributes such as `href` and `src` require `url`, not `string`:

```text
type Link {
  label: string
  destination: url
}

export component LinkView(link: Link): html {
<a href={link.destination}>{link.label}</a>
}
```

The Go caller supplies a `url.URL`.

## Whitespace

The generator does not ship your indentation. At generation time every run of
whitespace in static markup collapses to a single space, which is exactly what a
browser renders it as, so the page is unchanged while the generated Go, the
binary, and every response lose one run per authored line.

```text
export component Card(): html {
<div class="card">
    <h1>Title</h1>
</div>
}
```

emits `" <div class=\"card\"> <h1>Title</h1> </div> "`, not the source with its
newlines and four-space indents.

A run is collapsed to one space rather than deleted, because whitespace between
two inline boxes is visible:

```text
<span>a</span>
<span>b</span>
```

still renders `a b`. Deleting the newline would render `ab`, and whether an
element is inline is a CSS question the generator cannot answer.

These keep their bytes exactly as written:

- `<pre>` and `<textarea>`, including everything nested inside them
- `<script>` and `<style>` bodies, where a newline ends a line comment and drives
  automatic semicolon insertion
- any subtree marked `preserve-whitespace`

Reach for the marker when a stylesheet — not the markup — made an element
whitespace-significant, which the generator cannot see:

```text
<div id="log" preserve-whitespace>
  first line
  second line
</div>
```

The attribute is reserved and never appears in the output. It is a bare
attribute; `preserve-whitespace="false"` is a generation error rather than a
silent no-op.

Whitespace-only runs are removed outright only where the HTML parser itself
discards them: directly inside `<html>`, `<head>`, and the table elements, and
around the doctype of a component that renders a whole document.

To keep the authoring whitespace byte for byte across a whole run — when
comparing generated markup against pre-existing golden files, for instance —
pass `PreserveTemplateWhitespace` in the generator options.

## Escaping and trusted content

Ordinary strings are automatically escaped in HTML text and attribute contexts:

```text
export component Safe(message: string): html {
<p title={message}>{message}</p>
}
```

A string containing `<script>` is therefore not executed as HTML.

Use an explicit intrinsic only when HTML, CSS, or JavaScript must intentionally be inserted without escaping:

```text
type Payload {
  message: string
  count: int
  enabled: bool
}

export component Document(
  markup: string,
  css: string,
  javascript: string,
  payload: Payload
): html {
{RawHTML(markup)}
<style>{RawCSS(css)}</style>
<script>{RawJavaScript(javascript)}</script>
<script>window.payload = {JsonForScript(payload)};</script>
}
```

| Intrinsic | Allowed context | Meaning |
| --- | --- | --- |
| `RawHTML(string)` | HTML child position | Emit trusted HTML unchanged |
| `RawCSS(string)` | Inside `<style>` | Emit trusted CSS unchanged |
| `RawJavaScript(string)` | Inside `<script>` | Emit trusted JavaScript unchanged |
| `JsonForScript(value)` | Inside `<script>` | Convert typed data to script-safe JSON |

`Raw*` is not a sanitizer. Never pass arbitrary external input; restrict it to fixed or previously validated trusted content. Use `JsonForScript`, not `RawJavaScript`, when passing data to JavaScript.

### Braces inside `<script>` and `<style>`

A `<script>` or `<style>` body is authored JavaScript or CSS, where `{` is
ordinary syntax of that language. Inside those two elements a brace opens a
template insertion only when it is written tight against its content and takes
one of these shapes:

```text
{name}                    a bare value
{record.field}            a member access
{JsonForScript(payload)}  a call
{(active ? on : off)}     a parenthesized expression
{if ready} ... {/if}      a control block
```

Every other brace is content. The leading space is what separates the two, so
`{ name }` is content while `{name}` is an insertion.

#### What stays content

None of this needs escaping. Each line is emitted byte for byte:

```text
export component Widget(): html {
<script>
class X {}                                  // empty block
function f() {
  return 1
}                                           // block on its own line
function g(){return 1}                      // minified function
const o = { a: 1 };                         // object literal
const n = {0: 'a'};                         // numeric key
const p = {a, b};                           // multiple shorthands
class C { m() { this.v = 1; } }             // method body assigning this
if (x) { render() }                         // single statement block
const s = `hi ${name}`;                     // template literal placeholder
</script>
<style>
.a { color: red; }                          /* declaration block */
.b{color:red}                               /* minified declaration */
@media print {
  .c { color: #000; }
}                                           /* nested at-rule */
</style>
<script type="speculationrules">
{"prerender": [{"where": {"href_matches": "/*"}}]}
</script>
}
```

The `${name}` line is the one worth calling out. It is left alone, so a
JavaScript template literal keeps its browser-side meaning. Before these rules it
was read as an insertion, and with a `trusted_javascript` parameter named `name`
in scope it compiled without a diagnostic and substituted a server value into
what the author wrote as client code.

#### What is an insertion

Each of these reaches the expression grammar and is type-checked against its
context, exactly as an insertion in child position is:

```text
type Config { js: trusted_javascript }

export component Widget(
  js: trusted_javascript,
  cfg: Config,
  css: string,
  payload: Payload,
  ready: bool,
  on: trusted_javascript,
  off: trusted_javascript
): html {
<script>{js}</script>                          <!-- bare value -->
<script>{cfg.js}</script>                      <!-- member access -->
<script>{JsonForScript(payload)}</script>      <!-- call -->
<script>{(ready ? on : off)}</script>          <!-- parenthesized expression -->
<style>{RawCSS(css)}</style>                   <!-- call in style content -->
<script>{if ready}console.log(1){/if}</script> <!-- control block -->
}
```

Anything an insertion cannot express in those shapes is parenthesized:
`{(items[0])}`, not `{items[0]}`; `{(ready ? on : off)}`, not
`{ready ? on : off}`.

#### When content and a shape collide

Two authored forms do match a shape, because they are written tight. Both are
caught rather than silently substituted:

```text
<script>const o = {name};</script>     ⟶  unknown identifier name
<script>if(x){render()}</script>       ⟶  unknown function render
```

`{{` ... `}}` is the way out. It is the literal-brace escape everywhere in a
template, not only here: it emits a single `{` ... `}` pair and nothing inside is
parsed.

```text
<script>const o = {{name}};</script>
```

emits `const o = {name};`.

One case stays silent: a tight shorthand whose name matches a parameter of an
insertable type. `const o = {payload};` with `payload: script_json` in scope
compiles and substitutes. The escape above is the remedy, and it is why the
spaced form `const o = { payload };` — which authored code writes far more often
— is content.

A `<head>` element declared outside `<html>` is a
[head contribution](#component-styles-and-scripts), and its `<script>` and
`<style>` bodies are verbatim — no shape rules apply there at all. The rules
above are for the document shell's own head and for element bodies.

When a brace does get read as an insertion by mistake, the diagnostic names the
element and the ways out:

```text
tasks.tb.html:13:65: unknown identifier name; this is inside <script> content,
where {...} is a template insertion. Write {{...}} to keep a literal brace,
insert a value with RawJavaScript or JsonForScript, or move the script to a file
under the public asset directory
```

For anything longer than a few lines, the last suggestion is usually the right
one: put the script in a file the application serves and reference it.

## External functions

Declare an `external` function when display-specific conversion is implemented in Go:

```text
enum Tone { Primary, Secondary }

external Decorate(value: string, tone: Tone): string

export component Label(value: string, tone: Tone): html {
<span>{Decorate(value, tone)}</span>
}
```

Implement the corresponding function in the same Go package:

```go
func Decorate(value string, tone Tone) string {
	if tone == TonePrimary {
		return "★ " + value
	}
	return value
}
```

### Reading the request

Declare a leading `context.Context` and your function receives the context the
page is rendering under — the `ctx` you passed to an async entry, or the one
`WithContext` supplied to a synchronous one:

```go
func RequestID(ctx context.Context) string { return traceFrom(ctx) }
```

The template declaration is unchanged — `external RequestID(): string` either way
— so this is a decision for whoever writes the Go, function by function. Leave
the parameter out and the function is called plainly, exactly as before.

This is how a value that belongs to the request rather than to the page — a
request id, a nonce, a locale — reaches markup without travelling through the
parameter struct of every page that needs it. It is a read: a function called
this way must not write the response.

An external declared `: html` returns an `htmlbind.Fragment` and renders as a
subtree, so a whole element can come back instead of a bare value.

> [!NOTE]
> **A CSRF token is not one of these.** It used to be the example here, and it is
> now emitted for you: every unsafe form carries a hidden field automatically,
> and the token arrives as `htmlbind.WithCSRFToken(token)` on the render call
> rather than through the context. See
> [htmlbind_frameworkowner.md](htmlbind_frameworkowner.md#own-the-csrf-tokens-lifecycle).

## Async components

An `external async` function runs concurrently while the page renders. Your Go
implementation stays an ordinary blocking function; it just gains an error
result, because a boundary needs something to recover from:

```text
external async LoadUser(id: string): User
external async LoadPosts(id: string): Post[]
```

```go
func LoadUser(id string) (User, error)
func LoadPosts(id string) ([]Post, error)
```

Nothing in that code knows about concurrency. The runtime runs each binding in
its own goroutine and joins the results, so two slow calls in one `await` take
as long as the slower one rather than their sum.

### Taking a context

Declare a leading `context.Context` and the boundary's context is passed to it:

```go
func LoadPosts(ctx context.Context, id string) ([]Post, error)
```

The template declaration does not change. Generation reads the Go sources in the
package and passes the context to the functions that accept one, so the choice
belongs to whoever writes the implementation, function by function. A function
without the parameter is called plainly.

Take the context when the call can genuinely abort — a database query or an
outbound request. Leave it out when it cannot; the wait is bounded either way.

An async result exists only inside the boundary that waits for it, so an async
function can only be called in an `await` binding. Calling one anywhere else is
a generation error.

### await, fallback, recover

An `await` block has three clauses:

```text
export component Profile(id: string): html {
<section>
{await user = LoadUser(id), posts = LoadPosts(id)}
  <h1>{user.name}</h1>
  <ul>{for post in posts}<li>{post.title}</li>{/for}</ul>
{fallback}
  <p class="pending">loading…</p>
{recover err}
  <p class="failed">{err.message}</p>
{/await}
</section>
}
```

- The bindings after `await` start together. Each names one async call, and the
  bound value is an ordinary typed identifier in the primary subtree.
- `fallback` is required. It is what commits to the response first, so a slow
  dependency does not delay the rest of the page.
- `recover` is optional and binds a safe `error` value with the fields `code`,
  `message`, `retryable`, and `timeout`.

When a block that omitted `recover` fails, the failure leaves the template and
becomes the whole page's. `Render` returns a `*htmlbind.UnrecoveredError` and
writes no fallback in its place; `RenderAsync` yields it and ends the sequence.
On the streaming path the fallback is already in the response, so replacing what
is on screen is the caller's job — usually the framework above you. Write a
`recover` clause when you want one part of the page to fail on its own. Either
way, no loading state is left to sit there forever.

The bindings are visible only in the primary subtree, and the error name only in
`recover`, so no clause can read a value that does not exist when it renders.

A `<slot>` may not appear inside an `await` block: the fallback and the
replacement would both render it.

### Values the caller starts

An `external async` call starts when the boundary reaches it, so it cannot
overlap anything that ran earlier: not request parsing, not the layout above it.
Declare the parameter `async` instead and the work starts wherever you start it,
leaving the template only the wait:

```text
type Customer {
  name: string
  orders: async Order[]
}

export component Profile(customer: Customer, headline: async string?): html {
<h1>{customer.name}</h1>
{await orders = customer.orders}
  <ul>{for order in orders}<li>{order.id}</li>{/for}</ul>
{fallback}
  <p>loading {customer.name}…</p>
{/await}
}
```

```go
customer := Customer{
	Name:   "ada",
	Orders: htmlbind.Go(ctx, func(ctx context.Context) ([]Order, error) {
		return store.Orders(ctx, id)
	}),
}
err := htmlbind.Render(w, Profile(ProfileParams{Customer: customer}))
```

`async T` is a prefix modifier on any parameter or record field, and it becomes
`htmlbind.Pending[T]` in Go. It is not a function and not callable. The one
place it may be read is an `await` binding, where it sits beside async calls in
the same clause and settles with them.

The modifier covers the whole type, so `async Order[]` is a single pending slice
rather than a slice of pending values. When each row has to arrive on its own,
give the row type its own `async` field and await it inside the `for` body.

A record may carry settled and pending members together. That is what lets the
example above render `customer.name` in the `fallback`, while the orders it is
still waiting for stay behind the boundary.

Three constructors produce a handle:

| Constructor | Use |
| --- | --- |
| `htmlbind.Go(ctx, work)` | start the work in its own goroutine |
| `htmlbind.Resolved(v)` | a value you already have, and tests |
| `htmlbind.Failed(err)` | a failure you already know about |

There is no channel-taking constructor. A service that already returns a channel
is adopted by receiving from it inside the `Go` closure, so every handle belongs
to a goroutine this package started — and a panic in one of those becomes the
handle's error instead of the process's exit.

A handle settles once and stays readable. A layout and the page inside it may
therefore hold the same value: both boundaries see the same result, and the work
behind it runs once. A channel would have delivered it to whichever boundary
received first.

The context you pass to `Go` bounds the work, and that work stays yours to
cancel. A render bounds only how long it waits.

Where the awaited type is optional, an unset handle is a legal value. It settles
immediately as absent, opens no boundary, and never reaches `recover`, because
absence is data rather than failure.

Leaving a required one unset is a caller bug instead, and it surfaces as an
error rather than as a wait that never ends. A value reachable from a chain
member's own parameters is checked before any byte is written, so that response
can still carry an error status; a value reached through a loop item is checked
when the loop arrives at it. Either way the error names the value as the
template declared it:

```go
var unset *htmlbind.UnsetPendingError
if errors.As(err, &unset) {
	log.Printf("%s was never set", unset.Path)
}
```

A cached component cannot declare an `async` parameter, or a record that reaches
an `async` field. Stored bytes stand in for a fresh render, and a pending value
belongs to the one request that started it.

### Live sources

An `external async` function answers once. A gauge, a chat log, and a feed all
want the opposite: the value changes on the server's clock, and the region that
shows it should change with it. Declare the source `live` and it returns a
sequence instead of a value:

```text
external live WatchMetrics(id: string): Point
```

```go
func WatchMetrics(ctx context.Context, id string) iter.Seq2[Point, error]
```

Bind it in an ordinary `await` block. There is no second clause keyword, because
the clause never constrained a boundary to one change — it says which values the
subtree waits for, and the declaration says how often each arrives:

```text
{await point = WatchMetrics(id)}
  <p class="value">{point.label}: {point.value}</p>
{fallback}
  <p class="pending">waiting…</p>
{recover err}
  <p class="failed">{err.code}</p>
{/await}
```

The fallback commits first, exactly as it would for a settle-once binding. After
that the primary subtree is rendered again for every value the source yields,
and each render replaces the same region.

The leading `context.Context` is required here rather than optional. An endless
source has to be stoppable, and without a context there is nothing to make it
return when the subscription ends.

Every value carries the whole state of the region, not an increment. A source
watching a chat channel yields the current message list; a source sampling a
gauge yields the current window. That is what keeps a render repeatable and lets
a missed value cost nothing, and it means whoever owns the source holds the
accumulated state, because the template holds nothing between deliveries.

Delivery is pull-based, so the source blocks in its own `yield` until the
boundary is ready for the next value. A source producing faster than the screen
can use it misses ticks rather than filling a queue, and there is no buffer to
size. Breaking out of the range is the unsubscribe.

A yielded error is a delivery of a failure, not the end of the source. The
boundary renders `recover` and a later value replaces it, so a transient fault
shows and then clears. The sequence returning is the only terminal signal. A
block that declared no `recover` clause ends its subscription on the first
failure, the same rule a settle-once binding follows.

One clause may bind both kinds:

```text
{await title = LoadTitle(id), point = WatchMetrics(id)}
  <h1>{title}</h1>
  <p>{point.label}: {point.value}</p>
{fallback}
  <p>waiting…</p>
{/await}
```

`LoadTitle` settles once and `WatchMetrics` keeps delivering. Every render reads
both, which is why nothing has to say which source moved. The first render waits
until every binding has a value — the subtree reads them all, so rendering
earlier would show a zero one — and values that arrive while another binding is
still pending are coalesced, since the newest one is sufficient by construction.

A live region renders output, not input. A `<form>`, `<input>`, `<textarea>`, or
`<select>` inside the primary subtree is a generation error: a delivery arrives
while the user is typing, and replacing the subtree discards what they typed with
nothing to signal it. Put the control outside the boundary and let the live data
inside it change. The rule follows the boundary, so one live binding is enough to
apply it to the whole block, and `fallback` and `recover` are exempt because no
delivery replaces them.

Focus, scroll position, and media playback have the same exposure and no static
rule can forbid them, so keep them out of a region that re-renders on a timer.

### Rendering an async component

`Render` blocks on the bindings and writes the settled subtree in place, so a
template with `await` still produces a correct, complete document with no client
JavaScript involved:

```go
err := htmlbind.Render(w, Profile(ProfileParams{Id: id}))
```

Streaming the fallbacks first is the alternative, and picking it means knowing
beforehand whether anything in the composition can open a boundary at all.
`HasAwaitBlock` answers that on `Fragment` and `Wrapper`, and as a chain form
that unions the members:

```go
if htmlbind.HasAwaitBlock(wrappers, page) {
	// this response will stream
}
```

The flag is transitive, so a component that only calls an async one reports
`true`. Reading it renders nothing. A fragment you passed in through a parameter
is not counted, so union it with the values you built yourself.

`RenderAsync` sends the fallbacks first and yields each settled boundary after.
It returns a sequence, and your handler writes each item:

```go
func profile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := Profile(ProfileParams{Id: r.PathValue("id")})
	for content, err := range htmlbind.RenderAsync(r.Context(), w, page,
		htmlbind.WithAsyncTimeout(3*time.Second),
		htmlbind.WithErrorReporter(func(err error) { log.Printf("boundary failed: %v", err) }),
	) {
		if err != nil {
			// The response is already committed; log rather than rewrite it. A
			// boundary that failed with no recover clause arrives here too, as
			// an *htmlbind.UnrecoveredError.
			log.Printf("render failed: %v", err)
			break
		}
		if err := writeCompletion(w, content); err != nil {
			break
		}
		htmlbind.Flush(w)
	}
}
```

`writeCompletion` is yours, not the module's; the next section writes one.

`RenderChainAsync` is the same for a wrapper chain. There is no variant that
hides this loop: how many boundaries a render produces is not knowable up front,
least of all for a chain assembled at request time, so a streaming handler has
to be written against the sequence anyway.

A live boundary on either of those entries takes its first delivery and
unsubscribes. That is what puts real content on the first paint instead of a
loading state while still letting the response finish, and it is what a crawler
or a browser with no JavaScript receives. `RenderLive` and `RenderChainLive` are
the entries that keep the subscription open:

```go
for content, err := range htmlbind.RenderLive(ctx, io.Discard, page) {
	if err != nil {
		log.Printf("delivery failed: %v", err)
		break
	}
	// deliver content to the client
}
```

The sequence ends when every source has ended, when you stop ranging, or when
the context is cancelled — not when the boundaries have settled, because a live
boundary never does. Pass `io.Discard` when you want the deliveries alone: the
document bytes are still produced, since evaluating a source's arguments means
executing the component that holds them, but nothing is transferred. Boundary
ids are allocated by position, so the same chain rendered again produces the same
ids and a client can address the placeholders already on its screen.

`WithAsyncTimeout` bounds how long a boundary may show nothing on the entries
that must produce a response. Running out is not a failure of the source, so the
fallback stays rather than being replaced by `recover` content: a fallback is
honest about a value that has not arrived. `RenderLive` applies no such bound,
because a live source is allowed to be quiet for as long as its data is quiet.

Only the ranging caller writes the response, and stopping the range early ends
the render without waiting for the outstanding boundaries. The render flushes
after the initial pass; `htmlbind.Flush` is how you do the same after each
chunk, and it is a no-op for a writer that cannot flush.

### Framing a completion

A progressive render writes each pending boundary as `<tb-boundary id="...">`
holding the fallback. That is the module's half. The other half is yours: a
yielded `Content` is the settled fragment plus the id of the placeholder it
belongs to, and `WriteTo` emits that fragment and nothing else — no wrapper, no
marker, no script. htmlbind injects nothing into the head either, on either
entry point.

The split is deliberate. How a completion travels and the client code that acts
on it are one design, so they belong to one owner: the framework you are
building, or the handler itself.

The recipe below is a good default. Wrap each fragment in an inert template and
follow it with a marker element:

```go
func writeCompletion(w io.Writer, content htmlbind.Content) error {
	if _, err := io.WriteString(w, `<template data-tb-boundary="`+content.BoundaryID+`">`); err != nil {
		return err
	}
	if _, err := content.WriteTo(w); err != nil {
		return err
	}
	_, err := io.WriteString(w, `</template><tb-apply for="`+content.BoundaryID+`"></tb-apply>`)
	return err
}
```

which puts this on the wire:

```html
<template data-tb-boundary="tb-1">…</template><tb-apply for="tb-1"></tb-apply>
```

Define `tb-apply` once, in the runtime script your pages already load, and do
the swap from its connected callback:

```js
customElements.define("tb-apply", class extends HTMLElement {
	connectedCallback() {
		const id = this.getAttribute("for");
		this.remove();
		const template = document.querySelector(`template[data-tb-boundary="${id}"]`);
		if (!template) return;
		const placeholder = document.getElementById(id);
		if (placeholder) placeholder.replaceWith(template.content);
		template.remove();
	}
});
```

Because it lives in a file you already serve, no completion carries a script of
its own, and the page needs neither a CSP nonce nor `unsafe-inline`.

The marker is what makes the swap safe. An HTML parser inserts an element when
it reads the *start* tag, so a runtime that reacted to the template's appearance
could read one whose content had not arrived yet and replace the placeholder
with nothing — losing the fallback along with the result. Because `<tb-apply>`
comes after `</template>` in the byte stream, the template is complete by the
time it exists, however a proxy, TLS record, or compressing encoder split the
bytes. A completion whose marker never arrives is simply not applied, and the
fallback stays. Trigger on the marker, never on the template.

A response no script ever applies keeps its fallbacks, which is also what a
client with JavaScript disabled sees. `Render` settles every boundary in place
instead, so a route that must work without a runtime has an entry that needs
none.

### Cancellation bounds the wait

A cancelled request or an expired `WithAsyncTimeout` makes the runtime stop
waiting. A cancelled boundary produces no completion at all, because nobody is
left to read one. An expired deadline is a failure like any other: it renders
`recover` with `code: "timeout"`, or fails the page when the clause declared no
`recover`.

Whether the work itself stops is up to the external. One that takes a context
sees the cancellation and can return early. One that does not cannot be
interrupted, so it is abandoned: it finishes on its own and its result is
discarded.

### Errors stay server-side

A recover subtree never sees a raw Go error. By default a failure becomes
`code: "internal"` with no message, and a timeout becomes `code: "timeout"`. To
publish something more specific, give the error its own safe projection:

```go
type UpstreamError struct{ Service string }

func (e UpstreamError) Error() string { return "upstream " + e.Service + " unreachable" }

func (e UpstreamError) PublicError() htmlbind.AsyncError {
	return htmlbind.AsyncError{Code: "upstream", Message: "Please try again.", Retryable: true}
}
```

`WithErrorReporter` receives the original error either way, including when a
`recover` clause handled it, so logging and metrics still see everything.

## Cached components

Annotate a component with `@cache` to reuse its rendered output for equal
parameters:

```text
@cache(ttl: "5m")
export component Sidebar(userId: string, tone: Tone): html {
<aside>...</aside>
}
```

The `ttl` argument is required and is parsed at generation time, so a malformed
duration fails the build rather than a request.

Caching is a deployment choice, not a template rewrite: nothing is stored until a
caller supplies a store.

```go
var pageCache = htmlbind.NewMemoryCache(1024)

err := htmlbind.Render(w, Page(params), htmlbind.WithCache(pageCache))
```

`WithCache` works on `RenderChain`, `RenderAsync`, and `RenderChainAsync` too. Without it,
an annotated component renders exactly as it would without the annotation.

### What the key covers

A key holds the component's package and file, a fingerprint of its generated
plan, and a canonical encoding of every declared parameter. Changing a
parameter, editing the template, or editing anything the component renders all
produce a different key, so regenerated code can never read stale output.

Anything that is not a declared parameter is invisible to the key. Request
identity, authorization, locale, and headers must be passed in as parameters, or
the component must not be cached.

### Supplying your own store

`CacheStore` is a two-method interface, so a Redis or memcached adapter is
ordinary application code:

```go
type CacheStore interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration)
}
```

`Set` returns nothing, because a cache write failure must not fail a response
that already rendered correctly; an implementation reports its own failures. A
store is used from several goroutines during one render and must be safe for
concurrent use. Keys are plain strings and may be long, so a store is free to
hash them.

### Restrictions

Generation rejects a cached component that could not be replayed from stored
bytes:

- It cannot declare an `html` parameter, because a slot argument is a bound
  continuation rather than a value that can enter the key.
- It cannot reach an `await` boundary, directly or through a component it calls.
  A boundary is emitted as a placeholder now and a replacement later, so it is
  not one byte range that can be stored.
- It cannot own the document `head`, because the merged head depends on the
  chain rather than on its parameters.

## Generated API shapes

### Exported component

Template:

```text
export component Name(p1: T1, p2: T2): html { ... }
```

Public API:

```go
type NameParams struct {
	P1 T1
	P2 T2
}

func Name(params NameParams) htmlbind.Fragment
```

### No parameters

```text
export component Layout(): html { ... }
```

```go
type LayoutParams struct{}

func Layout(params LayoutParams) htmlbind.Fragment
```

### Exported component with an unnamed slot

A component with a `children: html` parameter also gets a chain binder:

```go
func BindName(params NameParams) htmlbind.Wrapper
```

Use `Name` when you supply the children yourself, and `BindName` when the chain
supplies them. See "Composing whole documents".

### Private component

A component without `export` does not create an application-facing public API. Call it as a component tag from another template.

### External function

An `external` declaration does not generate the function. You implement a Go function with the declared mapped signature in the same package.

## Multiple template files

Templates in one directory are combined into one Go file.

- Use the same Go package name in every file
- Do not duplicate exported component, type, enum, or external names
- Give private components distinct names as well, because their generated declarations share a package

The `package` declaration may be omitted in some cases. Write it anyway: `package pages` states which Go package the generated file joins, and leaves nothing for a reader to infer from the directory name.

## Reading diagnostics

Generation errors include the template position:

```text
profile.tb.html:12:8: html:url requires url, got string
```

Common causes include:

- Passing `string` to `href` or `src`
- Inserting an ordinary `string` into `<script>`
- Using an optional value as part of a mixed attribute
- Passing a non-boolean expression to `if`
- Referring to an undeclared field, function, or component
- Using `RawHTML` or another trusted intrinsic in the wrong context
- Declaring a slot whose `required` marker disagrees with its parameter type
- Filling a slot the target component does not declare
- Writing a bare element selector in a scoped style block
- Calling an `external async` function outside an `await` binding
- Reading an `async` parameter or field anywhere but an `await` binding
- Writing an `await` block with no `fallback` clause
- Annotating a component with `@cache` when it declares an `html` or `async`
  parameter, or reaches an `await` boundary
- Writing JavaScript or CSS that collides with a template insertion shape; see
  [Braces inside `<script>` and `<style>`](#braces-inside-script-and-style)

A diagnostic raised inside `<script>` or `<style>` content names the element and
the ways out, because a brace there is usually authored content rather than
template syntax:

```text
tasks.tb.html:13:65: unknown identifier name; this is inside <script> content,
where {...} is a template insertion. Write {{...}} to keep a literal brace,
insert a value with RawJavaScript or JsonForScript, or move the script to a file
under the public asset directory
```

Inside the document shell's own `<head>` it adds one more clause, because a
`<head>` declared outside `<html>` reads the same markup verbatim:

```text
. A <head> declared outside <html> is a contribution, whose script and style
bodies are verbatim
```

Run `go generate ./...` after every template change, before building or testing. Until you do, the Go build still sees the previous plan — including the diagnostic you may have already fixed in the template.
