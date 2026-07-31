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

Types declared in a template become types in the generated Go package. Application code constructs those generated types when calling components.

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

Every component reachable from the rendered chain contributes, including
components called from a body, and identical tags are emitted once.

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

The document shell is the component that owns `html`, `head`, and `body`. Its
`head` element is where merged contributions land.

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

A package declaration can be omitted in some cases, but explicitly using the matching declaration, such as `package pages`, makes the intent clear.

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

Run `go generate ./...` after changing templates, before building and testing the application.
