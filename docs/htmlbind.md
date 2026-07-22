# htmlbind User Guide

`htmlbind` compiles typed `.tb.html` templates into Go functions that render to an `io.Writer`. Templates are not parsed at runtime; value types and HTML insertion contexts are checked during generation.

## What is automated

- Discovering `.tb.html` files
- Generating Go declarations for template types, enums, and exported components
- Generating one rendering function per component
- Checking text, attribute, URL, script, and style contexts
- Escaping ordinary strings for HTML
- Omitting optional attributes
- Rendering component composition, `if`, and `for`
- Reporting type and unsafe-context errors with file, line, and column

You do not need to understand generated implementation details. Application code calls the functions corresponding to `export component` declarations.

## What you provide

1. `.tb.html` files directly inside a Go package directory
2. A `package` declaration and the required `type`, `enum`, and `component` declarations
3. Same-package Go implementations for any declared external functions
4. Handlers or other code that calls exported components
5. A code-generation command

## Setup and generation

```go
package pages

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen -dir .
```

Place `profile.tb.html` in the same directory, then run:

```bash
go generate ./...
```

The generator discovers `.tb.html` and `.tb.sql` files directly inside the target directory and combines them in `tinybind_templates_gen.go`. It does not descend into child package directories; generate each package separately.

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

Generated public signature:

```go
func Hello(w io.Writer, name string) error
```

An `http.ResponseWriter` already implements `io.Writer`:

```go
func hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := Hello(w, r.URL.Query().Get("name")); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}
```

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

func Profile(w io.Writer, user User, tone Tone) error
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
| `html` | `HTML`, primarily for component children |

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

The application-facing signature is only the exported component:

```go
func Card(w io.Writer, user User) error
```

A component with a `children: html` parameter receives the content between its start and end tags. Components without children can be called with self-closing syntax:

```text
<Avatar user={user} compact={true} />
```

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

## Generated function signatures

### Exported component

Template:

```text
export component Name(p1: T1, p2: T2): html { ... }
```

Public API:

```go
func Name(w io.Writer, p1 T1, p2 T2) error
```

### No parameters

```text
export component Layout(): html { ... }
```

```go
func Layout(w io.Writer) error
```

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

Run `go generate ./...` after changing templates, before building and testing the application.
