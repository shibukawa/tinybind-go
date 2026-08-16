package htmlbind_test

import (
	"strings"
	"testing"

	htmlbind "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const bindingHead = "package pages\n\n" +
	"type Record {\n  title: string\n  summary: string\n}\n\n" +
	"external LoadData(id: string): Record\n" +
	"external async LoadSlow(id: string): Record\n" +
	"external Fragment(): html\n" +
	"external Norm(s: string): string\n\n"

func bindingSource(body string) string {
	return bindingHead + "export component Card(id: string): html {\n" + body + "\n}\n"
}

// The whole point of the construct: four reads of one loaded record are one
// call. Without a binding each mention compiles to its own closure, which is
// correct and is exactly what makes a component that fetches unaffordable.
func TestValueBindingCallsItsExternalOnce(t *testing.T) {
	generated := generateWith(t, bindingSource("{val record = LoadData(id)}\n<h1>{record.title}</h1>\n<p>{record.summary}</p>"), htmlbind.GenerateOptions{})
	if calls := strings.Count(generated, "LoadData("); calls != 1 {
		t.Fatalf("want one LoadData call, got %d:\n%s", calls, generated)
	}
	for _, want := range []string{"htmlbind.Val(", "p.Record.Title", "p.Record.Summary"} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated code is missing %q:\n%s", want, generated)
		}
	}
}

// The same template without the binding is the behaviour that has to stay
// unchanged, so the repeat is still a repeat when nobody asked for a name.
func TestWithoutABindingEveryMentionIsStillItsOwnCall(t *testing.T) {
	generated := generateWith(t, bindingSource("<h1>{LoadData(id).title}</h1>\n<p>{LoadData(id).summary}</p>"), htmlbind.GenerateOptions{})
	if calls := strings.Count(generated, "LoadData("); calls != 2 {
		t.Fatalf("want two LoadData calls, got %d:\n%s", calls, generated)
	}
}

// The bindings of one directive are independent, so one cannot read another.
// Go reads a comma-separated declaration the same way and an await clause has
// to, because its bindings settle concurrently; letting this comma mean
// something else would be one spelling with two meanings.
func TestBindingsOfOneDirectiveCannotDependOnEachOther(t *testing.T) {
	message := generateError(t, bindingSource("{val raw = Norm(id), key = Norm(raw)}\n<p>{key}</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "the bindings of one directive are independent") {
		t.Fatalf("want the independence diagnostic, got %q", message)
	}
	if !strings.Contains(message, "write key as its own {val}") {
		t.Fatalf("the diagnostic does not say what to do instead: %q", message)
	}
}

// Written as two directives it is an ordinary enclosing binding, and the
// generated scopes chain through Outer.
func TestADependentBindingIsWrittenAsTwoDirectives(t *testing.T) {
	generated := generateWith(t, bindingSource("{val raw = Norm(id)}\n{val key = Norm(raw)}\n<p>{raw}/{key}</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "Norm(p.Raw)") {
		t.Fatalf("the second binding never read the first:\n%s", generated)
	}
	if !strings.Contains(generated, "p.Outer.Raw") {
		t.Fatalf("the body never reached the outer binding:\n%s", generated)
	}
}

// Independent bindings in one directive still work, and both have to count as
// read: the lowering nests them, so the scan has to walk into a binding's body
// to find the reader of the outer one.
func TestIndependentBindingsShareOneDirective(t *testing.T) {
	generated := generateWith(t, bindingSource("{val a = Norm(id), b = Norm(id)}\n<p>{a}/{b}</p>"), htmlbind.GenerateOptions{})
	if calls := strings.Count(generated, "Norm("); calls != 2 {
		t.Fatalf("want one call per binding, got %d:\n%s", calls, generated)
	}
}

// A value binding may not take a name that is already visible. It is stricter
// than the shadowing every other binder allows, and it is what
// decision:value-binding-hoisting buys with: a binding whose name is unique
// cannot, when its evaluation moves to the top of its block, pass a node that
// reads the same name meaning something else.
//
// One check covers every source of a visible name, because the lowering nests.
func TestBindingCannotReuseAVisibleName(t *testing.T) {
	for name, body := range map[string]string{
		"an earlier binding": "{val a = Norm(id)}\n{val a = Norm(id)}\n<p>{a}</p>",
		"a sibling binding":  "{val a = Norm(id), a = Norm(id)}\n<p>{a}</p>",
		"an enclosing block": "{val a = Norm(id)}\n<p>{a}</p>\n{if true}{val a = Norm(id)}<p>{a}</p>{/if}",
		"a parameter":        "{val id = Norm(id)}\n<p>{id}</p>",
		"a loop variable":    "{for x in ids}{val x = Norm(id)}<p>{x}</p>{/for}",
	} {
		t.Run(name, func(t *testing.T) {
			source := bindingHead + "export component Card(id: string, ids: string[]): html {\n" + body + "\n}\n"
			_, err := htmlbind.Generate("page.tb.html", []byte(source), htmlbind.GenerateOptions{})
			if err == nil || !strings.Contains(err.Error(), "reuses a name that is already visible here") {
				t.Fatalf("want the shadowing diagnostic, got %v", err)
			}
		})
	}
}

// A for variable and an await binding may still shadow. Neither hoists, so
// neither can move past a read, which is the whole reason the value binding is
// the one that may not.
func TestOtherBindersMayStillShadow(t *testing.T) {
	for name, body := range map[string]string{
		"a loop variable":  "{val a = Norm(id)}\n<p>{a}</p>\n{for a in ids}<p>{a}</p>{/for}",
		"an await binding": "{val s = Norm(id)}\n<p>{s}</p>\n{await s = LoadSlow(id)}<p>{s.title}</p>{fallback}...{/await}",
	} {
		t.Run(name, func(t *testing.T) {
			source := bindingHead + "export component Card(id: string, ids: string[]): html {\n" + body + "\n}\n"
			if _, err := htmlbind.Generate("page.tb.html", []byte(source), htmlbind.GenerateOptions{}); err != nil {
				t.Fatalf("a permitted shadow was refused: %v", err)
			}
		})
	}
}

// A binding nothing reads still calls its external on every render, and an
// external may only answer a query, so the call is paid for and discarded.
// Generated Go accepts it, because the value becomes a struct field rather than
// a local — which is exactly why the diagnostic has to come from here.
func TestUnreadBindingIsRefused(t *testing.T) {
	message := generateError(t, bindingSource("{val a = Norm(id)}\n<p>hi</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "val binding a is never read") {
		t.Fatalf("want the unread diagnostic, got %q", message)
	}
}

// One unread binding beside a read one is still unread, which is the case a
// comma list makes easy to write by accident.
func TestUnreadBindingBesideAReadOneIsRefused(t *testing.T) {
	message := generateError(t, bindingSource("{val a = Norm(id), b = Norm(id)}\n<p>{b}</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "val binding a is never read") {
		t.Fatalf("want the unread diagnostic for the first binding, got %q", message)
	}
}

// A read inside a subtree that rebinds the name resolves to the inner binding,
// so the outer one is unread and the shadow was pointless. Only the binders that
// may still shadow can reach this; a value binding rebinding a name is refused
// before the scan runs.
func TestShadowedBindingCountsAsUnread(t *testing.T) {
	for name, body := range map[string]string{
		"a loop variable":  "{val a = Norm(id)}\n{for a in ids}<p>{a}</p>{/for}",
		"an await binding": "{val s = Norm(id)}\n{await s = LoadSlow(id)}<p>{s.title}</p>{fallback}...{/await}",
	} {
		t.Run(name, func(t *testing.T) {
			source := bindingHead + "export component Card(id: string, ids: string[]): html {\n" + body + "\n}\n"
			if _, err := htmlbind.Generate("page.tb.html", []byte(source), htmlbind.GenerateOptions{}); err == nil ||
				!strings.Contains(err.Error(), "is never read") {
				t.Fatalf("want the shadowed binding reported unread, got %v", err)
			}
		})
	}
}

// Every position a bound name can be read from has to count as a read, or a
// working template is refused. These are the ones the walk had to learn.
func TestABindingIsReadFromEveryValuePosition(t *testing.T) {
	for name, body := range map[string]string{
		"text":              "{val a = Norm(id)}\n<p>{a}</p>",
		"bare attribute":    "{val a = Norm(id)}\n<p class={a}>hi</p>",
		"quoted attribute":  "{val a = Norm(id)}\n<p class=\"x {a}\">hi</p>",
		"if condition":      "{val a = Norm(id)}\n{if a == \"x\"}<p>y</p>{/if}",
		"for iterable":      "{val a = ids}\n{for x in a}<p>{x}</p>{/for}",
		"for body":          "{val a = Norm(id)}\n{for x in ids}<p>{a}{x}</p>{/for}",
		"await binding":     "{val a = Norm(id)}\n{await s = LoadSlow(a)}<p>{s.title}</p>{fallback}...{/await}",
		"await primary":     "{val a = Norm(id)}\n{await s = LoadSlow(id)}<p>{a}{s.title}</p>{fallback}...{/await}",
		"await fallback":    "{val a = Norm(id)}\n{await s = LoadSlow(id)}<p>{s.title}</p>{fallback}{a}{/await}",
		"a later directive": "{val a = Norm(id)}\n{val b = Norm(a)}\n<p>{b}</p>",
		"a nested element":  "{val a = Norm(id)}\n<div><section><p>{a}</p></section></div>",
		"a component child": "{val a = Norm(id)}\n<Panel label=\"x\"><p>{a}</p></Panel>",
		"a component arg":   "{val a = Norm(id)}\n<Panel label={a}/>",
	} {
		t.Run(name, func(t *testing.T) {
			source := bindingHead + "component Panel(label: string, children: html?): html {\n<i>{label}</i><slot/>\n}\n" +
				"export component Card(id: string, ids: string[]): html {\n" + body + "\n}\n"
			if _, err := htmlbind.Generate("page.tb.html", []byte(source), htmlbind.GenerateOptions{}); err != nil {
				t.Fatalf("a read binding was refused: %v", err)
			}
		})
	}
}

// A block is a control construct, not a tag. Markup structure carries no scope,
// so a binding written inside a div reaches past the div's closing tag — which
// is also what lets decision:value-binding-hoisting evaluate it before the div
// opens.
func TestMarkupNestingIsNotABlock(t *testing.T) {
	generateWith(t, bindingSource("<div>{val a = Norm(id)}<p>{a}</p></div>\n<p>{a}</p>"), htmlbind.GenerateOptions{})
}

// A control construct is a block, so a binding inside one is unresolved after it.
func TestBindingDoesNotEscapeAControlBlock(t *testing.T) {
	for name, body := range map[string]string{
		"an if branch": "{if true}{val a = Norm(id)}<p>{a}</p>{/if}\n<p>{a}</p>",
		"a for body":   "{for x in ids}{val a = Norm(x)}<p>{a}</p>{/for}\n<p>{a}</p>",
	} {
		t.Run(name, func(t *testing.T) {
			source := bindingHead + "export component Card(id: string, ids: string[]): html {\n" + body + "\n}\n"
			_, err := htmlbind.Generate("page.tb.html", []byte(source), htmlbind.GenerateOptions{})
			if err == nil || !strings.Contains(err.Error(), "unknown identifier a") {
				t.Fatalf("want the trailing read to be unresolved, got %v", err)
			}
		})
	}
}

// The evaluation hoists but the name does not, so a read written before the
// directive is still a mistake even though the lowering has put it inside the
// binding's subtree.
func TestReadingABindingBeforeItIsWrittenIsRefused(t *testing.T) {
	message := generateError(t, bindingSource("<p>{a}</p>\n{val a = Norm(id)}\n<p>{a}</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "is read before its val binding") {
		t.Fatalf("want the read-before diagnostic, got %q", message)
	}
}

// The point of the hoist: a binding written after markup is evaluated before it,
// so a chain member's loader can still choose the response status.
func TestBindingIsEvaluatedAtTheTopOfItsBlock(t *testing.T) {
	generated := generateWith(t, bindingSource("<div><p>hello</p></div>\n{val a = Norm(id)}\n<p>{a}</p>"), htmlbind.GenerateOptions{})
	call := strings.Index(generated, "Norm(p.Id)")
	markup := strings.Index(generated, "hello")
	if call < 0 || markup < 0 || call > markup {
		t.Fatalf("the binding was not hoisted above the markup written before it:\n%s", generated)
	}
	// Only the computation moves. The markup keeps its written order, which is
	// what makes the hoist unobservable in the output.
	if opening := strings.Index(generated, "<div>"); opening < 0 || opening > markup {
		t.Fatalf("the markup was reordered:\n%s", generated)
	}
}

// A binding written inside an element is hoisted out of it, and the element is
// left whole: hoisting moves the evaluation, never the markup.
func TestHoistingOutOfAnElementLeavesItIntact(t *testing.T) {
	generated := generateWith(t, bindingSource("<div>{val a = Norm(id)}<p>{a}</p></div>\n<p>{a}</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "<div><p>") || !strings.Contains(generated, "</p></div>") {
		t.Fatalf("the element was split by the hoist:\n%s", generated)
	}
	if calls := strings.Count(generated, "Norm("); calls != 1 {
		t.Fatalf("want one call for two reads, got %d:\n%s", calls, generated)
	}
}

// An await clause is the only place an async external can be called, so naming
// one here points at that clause rather than reporting an unknown function.
func TestAsyncExternalCannotBeBound(t *testing.T) {
	message := generateError(t, bindingSource("{val r = LoadSlow(id)}\n<p>{r.title}</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "await") {
		t.Fatalf("want the diagnostic to name the await clause, got %q", message)
	}
}

// An html result is a subtree rendered where it is written, not a value, so
// binding it would promise an operand position it cannot fill.
func TestHTMLResultCannotBeBound(t *testing.T) {
	message := generateError(t, bindingSource("{val f = Fragment()}\n<p>x</p>"), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "renders where it is written") {
		t.Fatalf("want the html diagnostic, got %q", message)
	}
}

// A binding has a body even without a closer, and an attribute value has no
// later siblings for it to reach.
func TestBindingIsRefusedInAnAttribute(t *testing.T) {
	message := generateError(t, bindingSource(`<p class="{val a = Norm(id)}">x</p>`), htmlbind.GenerateOptions{})
	if !strings.Contains(message, "forbidden in attributes") {
		t.Fatalf("want the attribute diagnostic, got %q", message)
	}
}

// A binding is scoped by whatever block encloses it, and every construct that
// owns a body is such a block. These are the traversals that had to learn the
// node, so a miss shows up here rather than as a subtree quietly dropped.
func TestBindingWorksInsideEveryBodyBearingBlock(t *testing.T) {
	for name, body := range map[string]string{
		"for body":       "{for x in ids}{val r = LoadData(x)}<p>{r.title}{r.summary}</p>{/for}",
		"if branch":      "{if flag}{val r = LoadData(id)}<p>{r.title}{r.summary}</p>{else}<p>no</p>{/if}",
		"await primary":  "{await s = LoadSlow(id)}{val r = LoadData(s.title)}<p>{r.title}{r.summary}</p>{fallback}...{/await}",
		"await fallback": "{await s = LoadSlow(id)}<p>{s.title}</p>{fallback}{val r = LoadData(id)}<p>{r.title}{r.summary}</p>{/await}",
	} {
		t.Run(name, func(t *testing.T) {
			source := bindingHead + "export component Card(id: string, ids: string[], flag: bool): html {\n" + body + "\n}\n"
			generated := generateWith(t, source, htmlbind.GenerateOptions{})
			if calls := strings.Count(generated, "LoadData("); calls != 1 {
				t.Fatalf("want one LoadData call, got %d:\n%s", calls, generated)
			}
		})
	}
}

// The case the whole request was made for: a component takes a primary key,
// loads its own data, and one @cache covers the load and the render together.
// The loader sits inside the cached subtree and the key is the declared
// parameter, so a hit skips the fetch as well as the markup — and none of that
// needed a change to the cache.
func TestCachedComponentCanLoadItsOwnData(t *testing.T) {
	source := bindingHead + "@cache(ttl: \"5m\")\nexport component Card(id: string): html {\n" +
		"{val record = LoadData(id)}\n<h1>{record.title}</h1>\n<p>{record.summary}</p>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if calls := strings.Count(generated, "LoadData("); calls != 1 {
		t.Fatalf("want one LoadData call, got %d:\n%s", calls, generated)
	}
	if !strings.Contains(generated, "CachePolicy[CardParams]") {
		t.Fatalf("the component lost its cache policy:\n%s", generated)
	}
	// The key is the declared parameter, never the loaded value: a key built
	// from what the lookup exists to avoid fetching would be no lookup at all.
	if !strings.Contains(generated, "KeyString[string](p.Id)") {
		t.Fatalf("the cache key is not the declared parameter:\n%s", generated)
	}
	policy := strings.Index(generated, "planCardCache")
	loader := strings.Index(generated, "LoadData(p.Id)")
	if policy < 0 || loader < 0 || loader < policy {
		t.Fatalf("the loader is not inside the cached plan:\n%s", generated)
	}
}

// Inside a script body the gate decides whether a brace is an insertion at all.
// Without the keyword the shapes read `{val a = f()}` as content, because an
// identifier followed by another one is neither a bare value nor a call.
func TestBindingIsRecognizedInsideAScriptBody(t *testing.T) {
	source := bindingHead + "export component Card(id: string): html {\n" +
		"<script>{val a = Norm(id)}const x = {JsonForScript(a)};</script>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "htmlbind.Val(") {
		t.Fatalf("the binding never became an instruction:\n%s", generated)
	}
	if calls := strings.Count(generated, "Norm("); calls != 1 {
		t.Fatalf("want one Norm call, got %d:\n%s", calls, generated)
	}
}

// A synchronous external is otherwise total. Declaring a trailing error in the
// Go implementation gives a call that can fail somewhere to say so, and the
// binding is where the failure has a place to go: nothing else in the lowering
// can carry an error out of a value expression.
func TestFailingExternalIsBoundAsAWholeValue(t *testing.T) {
	generated := generateWith(t, bindingSource("{val record = LoadData(id)}\n<h1>{record.title}</h1>"),
		htmlbind.GenerateOptions{ErrorExternals: map[string]bool{"LoadData": true}})
	if !strings.Contains(generated, "htmlbind.ValErr(") {
		t.Fatalf("the failing call did not become an error-carrying instruction:\n%s", generated)
	}
	if !strings.Contains(generated, "(Record, error) { return LoadData(p.Id) }") {
		t.Fatalf("the value closure does not hand back the error:\n%s", generated)
	}
}

// The context and error variants compose, and generation names the instruction
// by appending the suffix, so the runtime has to spell it the same way.
func TestFailingExternalTakingTheContextComposes(t *testing.T) {
	source := "package pages\n\nexternal Token(): string\n\nexport component Card(): html {\n{val t = Token()}\n<p>{t}</p>\n}\n"
	generated := generateWith(t, source, htmlbind.GenerateOptions{
		ErrorExternals:   map[string]bool{"Token": true},
		ContextExternals: map[string]bool{"Token": true},
	})
	if !strings.Contains(generated, "htmlbind.ValErrCtx(") {
		t.Fatalf("want the context-carrying error instruction:\n%s", generated)
	}
	if !strings.Contains(generated, "(string, error) { return Token(ctx) }") {
		t.Fatalf("the closure takes neither the context nor the error:\n%s", generated)
	}
}

// Every other position is refused, for the same reason an async external is
// confined to an await clause: there is nowhere for the failure to go. The
// diagnostic says what to write instead.
func TestFailingExternalIsRefusedOutsideABinding(t *testing.T) {
	for name, body := range map[string]string{
		"interpolated":           "<h1>{LoadData(id).title}</h1>",
		"nested in a bound call": "{val a = Norm(LoadData(id).title)}\n<p>{a}</p>",
		"an if condition":        "{if LoadData(id).title == \"x\"}<p>y</p>{/if}",
		"an attribute":           "<p class={LoadData(id).title}>x</p>",
	} {
		t.Run(name, func(t *testing.T) {
			message := generateError(t, bindingSource(body),
				htmlbind.GenerateOptions{ErrorExternals: map[string]bool{"LoadData": true}})
			if !strings.Contains(message, "returns an error, so it can only be the whole value of a val binding") {
				t.Fatalf("want the placement diagnostic, got %q", message)
			}
		})
	}
}

// An async external already returns an error and its failure is the boundary's,
// recoverable at the clause, so the scan naming it changes nothing about it.
func TestAsyncExternalIsUnaffectedByTheErrorScan(t *testing.T) {
	source := bindingHead + "export component Card(id: string): html {\n" +
		"{await s = LoadSlow(id)}<p>{s.title}</p>{fallback}...{/await}\n}\n"
	if _, err := htmlbind.Generate("page.tb.html", []byte(source),
		htmlbind.GenerateOptions{ErrorExternals: map[string]bool{"LoadSlow": true}}); err != nil {
		t.Fatalf("the error scan disturbed an async external: %v", err)
	}
}

// A project whose externals declare no error generates exactly what it
// generated before this existed.
func TestATotalExternalIsUnchanged(t *testing.T) {
	body := "{val record = LoadData(id)}\n<h1>{record.title}</h1>"
	with := generateWith(t, bindingSource(body), htmlbind.GenerateOptions{ErrorExternals: map[string]bool{}})
	without := generateWith(t, bindingSource(body), htmlbind.GenerateOptions{})
	if with != without {
		t.Fatal("an empty error set changed the output")
	}
	if strings.Contains(with, "ValErr") {
		t.Fatalf("a total external became an error-carrying instruction:\n%s", with)
	}
}

// Reported by the framework 2026-08-14 against v0.5.11, held adoption.
//
// Hoisting puts every binding of a block in front of that block's markup, so a
// component that binds anything presents a value binding where its root element
// used to be. Three things ask a component for that root, and this one asks
// silently: a component that loads its own data kept rendering and stopped
// being an update boundary, with no diagnostic. That is the shape the retired
// typed page rung leaves behind, so it landed on every discovered page at once.
func TestAValueBindingLeavesTheComponentItsBoundaryRoot(t *testing.T) {
	plain := generateWith(t, bindingSource("<section><h1>{id}</h1></section>"), htmlbind.GenerateOptions{})
	if !strings.Contains(plain, "BoundaryAttr()") {
		t.Fatalf("the component is not a boundary before a binding is added, so this test proves nothing:\n%s", plain)
	}
	// Written inside the element, which is where an author puts it and which is
	// what makes the binding hoist out past the root.
	bound := generateWith(t, bindingSource("<section>{val record = LoadData(id)}<h1>{record.title}</h1></section>"), htmlbind.GenerateOptions{})
	for _, want := range []string{"BoundaryAttr()", "htmlbind.Boundary[", "Boundary:"} {
		if !strings.Contains(bound, want) {
			t.Fatalf("the binding dropped %q from the generated boundary:\n%s", want, bound)
		}
	}
}

// One binding and two are the same question, because normalization nests them:
// a fix that steps over exactly one level would leave the second binding
// failing the way the first one did.
func TestTwoValueBindingsLeaveTheComponentItsBoundaryRoot(t *testing.T) {
	generated := generateWith(t, bindingSource(
		"{val one = Norm(id)}\n{val record = LoadData(one)}\n<section><h1>{record.title}</h1></section>"), htmlbind.GenerateOptions{})
	if !strings.Contains(generated, "BoundaryAttr()") {
		t.Fatalf("two bindings dropped the boundary:\n%s", generated)
	}
}

// The rule the binding must not disable. A component rendering two elements has
// no root to carry the attribute, binding or no binding, so seeing through the
// binding must not turn into inventing a root.
func TestAValueBindingDoesNotInventARootTheComponentLacks(t *testing.T) {
	generated := generateWith(t, bindingSource(
		"{val record = LoadData(id)}\n<h1>{record.title}</h1>\n<p>{record.summary}</p>"), htmlbind.GenerateOptions{})
	if strings.Contains(generated, "BoundaryAttr()") {
		t.Fatalf("a two-element component became a boundary:\n%s", generated)
	}
}

// The loud half of the same defect: the script block's marker lives on the root
// element, so the same nil reads as "no single root" and refuses a component
// that has one.
func TestAValueBindingLeavesAScriptBlockItsRoot(t *testing.T) {
	source := bindingHead + "export component Card(id: string): html {\n" +
		"<script component>\nexport function setup(el) { return () => {} }\n</script>\n" +
		"<section>{val record = LoadData(id)}<h1>{record.title}</h1></section>\n}\n"
	if _, err := htmlbind.Generate("card.tb.html", []byte(source), htmlbind.GenerateOptions{}); err != nil {
		t.Fatalf("a binding cost a component with one root element its script block: %v", err)
	}
}

// The third caller, which the report did not name: a reloadable component
// carries its id and kind on that same root, and refuses generation without it.
func TestAValueBindingLeavesAReloadableComponentItsRoot(t *testing.T) {
	source := bindingHead + "@reloadable\nexport component Card(id: string): html {\n" +
		"<section>{val record = LoadData(id)}<h1>{record.title}</h1></section>\n}\n"
	if _, err := htmlbind.Generate("card.tb.html", []byte(source), htmlbind.GenerateOptions{}); err != nil {
		t.Fatalf("a binding cost a reloadable component its root: %v", err)
	}
}
