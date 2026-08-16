package htmlbind_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/internal/syntax"
)

// findMessages walks a parsed module and collects every message reference, so a
// test can assert on what the parser decided rather than on rendered output.
func findMessages(t *testing.T, module *htmlbind.Module) []*syntax.MessageExpr {
	t.Helper()
	var out []*syntax.MessageExpr
	collect := func(e syntax.Expr) {
		if message, ok := e.(*syntax.MessageExpr); ok {
			out = append(out, message)
		}
	}
	var walk func(nodes []syntax.Node)
	walk = func(nodes []syntax.Node) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *syntax.ExpressionNode:
				collect(n.Expression)
			case *htmlbind.ElementNode:
				walk(n.Children)
				for _, attribute := range n.Attributes {
					for _, part := range attribute.Value {
						if part.Expression != nil {
							collect(part.Expression)
						}
					}
				}
			case *htmlbind.HeadNode:
				walk(n.Children)
			case *syntax.IfNode:
				collect(n.Condition)
				walk(n.Then)
				walk(n.Else)
			case *syntax.ForNode:
				walk(n.Body)
			}
		}
	}
	for _, decl := range module.Declarations {
		if d, ok := decl.(*syntax.TemplateDecl); ok {
			if body, ok := d.Body.([]syntax.Node); ok {
				walk(body)
			}
		}
	}
	return out
}

func parseComponent(t *testing.T, body string) *htmlbind.Module {
	t.Helper()
	module, err := htmlbind.Parse("message.txt", []byte("component W(t: string, other: string, a: string, b: string, count: int, layout: string, user: string, sep: string): html {"+body+"}"))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return module
}

// TestMessageKeywordIsContextual is the compatibility proof. Every shape below
// mentions t and means the parameter, and the message form must not claim any
// of them. See .knowledge decision:message-reference-syntax recognizer.
func TestMessageKeywordIsContextual(t *testing.T) {
	expressions := []struct {
		name string
		body string
	}{
		{"bare value", `<p>{t}</p>`},
		{"member access", `<p>{t.Year}</p>`},
		{"call", `<p>{t.Format(layout)}</p>`},
		{"call with a space", `<p>{t .Year}</p>`},
		{"comparison", `<p>{t == other}</p>`},
		{"greater than", `{if t > other}<p>x</p>{/if}`},
		{"subtraction with spaces", `<p>{t - other}</p>`},
		{"subtraction without a leading space", `<p>{t -other}</p>`},
		{"ternary", `<p>{t ? a : b}</p>`},
	}
	for _, test := range expressions {
		t.Run(test.name, func(t *testing.T) {
			module := parseComponent(t, test.body)
			if found := findMessages(t, module); len(found) != 0 {
				t.Fatalf("%s was read as a message reference %q, but it means the parameter", test.body, found[0].Written)
			}
		})
	}
}

func TestMessageReferenceForms(t *testing.T) {
	references := []struct {
		name string
		body string
		want string
		args []string
	}{
		{"bare id", `<p>{t title}</p>`, "title", nil},
		{"qualified id", `<p>{t common.save}</p>`, "common.save", nil},
		{"deeply qualified id", `<p>{t checkout.payment.title}</p>`, "checkout.payment.title", nil},
		{"hyphenated id", `<p>{t item-count}</p>`, "item-count", nil},
		{"hyphen and dot", `<p>{t common.item-count}</p>`, "common.item-count", nil},
		{"underscored id", `<p>{t name_field}</p>`, "name_field", nil},
		{"one argument", `<p>{t item-count, n: count}</p>`, "item-count", []string{"n"}},
		{"two arguments", `<p>{t greeting, name: user.Name, when: t}</p>`, "greeting", []string{"name", "when"}},
		{"argument holding a call", `<p>{t greeting, name: Format(user, sep)}</p>`, "greeting", []string{"name"}},
		{"in an attribute value", `<input placeholder={t name_field}>`, "name_field", nil},
	}
	for _, test := range references {
		t.Run(test.name, func(t *testing.T) {
			module := parseComponent(t, test.body)
			found := findMessages(t, module)
			if len(found) != 1 {
				t.Fatalf("found %d message references in %s, want 1", len(found), test.body)
			}
			if found[0].Written != test.want {
				t.Fatalf("id = %q, want %q", found[0].Written, test.want)
			}
			if len(found[0].Args) != len(test.args) {
				t.Fatalf("got %d arguments, want %d", len(found[0].Args), len(test.args))
			}
			for i, name := range test.args {
				if found[0].Args[i].Name != name {
					t.Fatalf("argument %d = %q, want %q", i, found[0].Args[i].Name, name)
				}
			}
		})
	}
}

// TestMessageArgumentDiagnostics covers the commit rule: once the id is valid
// the reference is committed, so a bad argument is reported as what it is.
func TestMessageArgumentDiagnostics(t *testing.T) {
	bad := []struct {
		name string
		body string
		want string
	}{
		{"argument without a value", `<p>{t greeting, name}</p>`, "message argument syntax"},
		{"empty value", `<p>{t greeting, name: }</p>`, "has no value"},
		{"capitalized name", `<p>{t greeting, Name: x}</p>`, "lowerCamelCase"},
		{"duplicate name", `<p>{t greeting, name: a, name: b}</p>`, "duplicate message argument"},
	}
	for _, test := range bad {
		t.Run(test.name, func(t *testing.T) {
			_, err := htmlbind.Parse("message.txt", []byte("component W(): html {"+test.body+"}"))
			if err == nil {
				t.Fatalf("%s parsed without error", test.body)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestMessagesHeaderDeclaration(t *testing.T) {
	module, err := htmlbind.Parse("message.txt", []byte("messages about\n\ncomponent W(): html {<p>{t title}</p>}"))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if module.Messages == nil {
		t.Fatal("messages declaration was not recorded")
	}
	if module.Messages.Name != "about" {
		t.Fatalf("scope = %q, want about", module.Messages.Name)
	}
}

func TestMessagesHeaderAcceptsADottedName(t *testing.T) {
	module, err := htmlbind.Parse("message.txt", []byte("messages checkout.payment\n\ncomponent W(): html {<p>x</p>}"))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if module.Messages.Name != "checkout.payment" {
		t.Fatalf("scope = %q, want checkout.payment", module.Messages.Name)
	}
}

func TestSecondMessagesDeclarationIsAnError(t *testing.T) {
	_, err := htmlbind.Parse("message.txt", []byte("messages about\nmessages other\n\ncomponent W(): html {<p>x</p>}"))
	if err == nil {
		t.Fatal("a second messages declaration parsed without error")
	}
	if !strings.Contains(err.Error(), "only be declared once") {
		t.Fatalf("error = %v, want it to name the repetition", err)
	}
}

// TestMessageReferencePrintsBackAsWritten keeps requirement:template-source-formatting
// honest for the new form, including the id the author chose to qualify.
func TestMessageReferencePrintsBackAsWritten(t *testing.T) {
	sources := []string{
		"messages about\n\ncomponent W(): html {\n  <p>{t title}</p>\n}\n",
		"messages about\n\ncomponent W(): html {\n  <p>{t common.item-count, n: count}</p>\n}\n",
	}
	for _, source := range sources {
		module, err := htmlbind.Parse("message.txt", []byte(source))
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		printed, err := syntax.PrintModule(module, []syntax.RootPrinter{htmlbind.RootPrinter()}, syntax.PrintOptions{})
		if err != nil {
			t.Fatalf("format failed: %v", err)
		}
		if _, err := htmlbind.Parse("message.txt", []byte(printed)); err != nil {
			t.Fatalf("printed output does not parse: %v\n%s", err, printed)
		}
		if !strings.Contains(string(printed), "messages "+module.Messages.Name) {
			t.Fatalf("printed output lost the messages declaration:\n%s", printed)
		}
		for _, message := range findMessages(t, module) {
			if !strings.Contains(string(printed), message.Written) {
				t.Fatalf("printed output lost the id %q:\n%s", message.Written, printed)
			}
		}
	}
}

// TestMessageResolution covers requirement:message-scope-declaration: a bare id
// takes the file's scope, a dotted one leaves it, and a file with neither is an
// error rather than something derived from the file name.
func TestMessageResolution(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"bare id takes the scope",
			"messages about\n\ncomponent W(): html {<p>{t title}</p>}",
			"about.title",
		},
		{
			"dotted id leaves the scope",
			"messages about\n\ncomponent W(): html {<p>{t common.save}</p>}",
			"common.save",
		},
		{
			"a dotted scope composes",
			"messages checkout.payment\n\ncomponent W(): html {<p>{t title}</p>}",
			"checkout.payment.title",
		},
		{
			"the declaration may follow the component that uses it",
			"component W(): html {<p>{t title}</p>}\n\nmessages about",
			"about.title",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			module, err := htmlbind.Parse("message.txt", []byte(test.source))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if _, err := htmlbind.Generate("message.txt", []byte(test.source), messageOptions()); err != nil {
				t.Fatalf("generate failed: %v", err)
			}
			found := findMessages(t, module)
			if len(found) != 1 {
				t.Fatalf("found %d references, want 1", len(found))
			}
		})
	}
}

func TestMessageWithoutAScopeIsAnError(t *testing.T) {
	_, err := htmlbind.Generate("message.txt", []byte("component W(): html {<p>{t title}</p>}"), messageOptions())
	if err == nil {
		t.Fatal("a bare reference compiled with no messages declaration")
	}
	if !strings.Contains(err.Error(), "needs a messages declaration") {
		t.Fatalf("error = %v, want it to name the missing declaration", err)
	}
}

// TestQualifiedMessageNeedsNoScope is the other half: a dotted id is absolute,
// so it compiles in a file that declares nothing.
func TestQualifiedMessageNeedsNoScope(t *testing.T) {
	if _, err := htmlbind.Generate("message.txt", []byte("component W(): html {<p>{t common.save}</p>}"), messageOptions()); err != nil {
		t.Fatalf("a qualified reference needs no scope, but: %v", err)
	}
}

// messageOptions is the symbol table the tests generate against. It stands in
// for what a catalog generator supplies.
func messageOptions() htmlbind.GenerateOptions {
	return htmlbind.GenerateOptions{
		Messages: map[string]htmlbind.MessageSymbol{
			"about.title":            {Package: "example.com/app/messages", Name: "AboutTitle"},
			"checkout.payment.title": {Package: "example.com/app/messages", Name: "CheckoutPaymentTitle"},
			"common.save":            {Package: "example.com/app/messages", Name: "CommonSave"},
			"about.item-count":       {Package: "example.com/app/messages", Name: "AboutItemCount", Params: []string{"n"}},
			"about.local":            {Name: "LocalMessage"},
		},
		ImplicitBindings: []htmlbind.ImplicitBinding{{
			Name:     "locale",
			Provider: htmlbind.BindingProvider{Package: "example.com/app/framework", Name: "Locale", Result: "framework.Locale"},
			VaryAxis: "Accept-Language",
		}},
		MessageContextBinding: "locale",
	}
}

func TestMessageEmission(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
		absent []string
	}{
		{
			"qualified call and its import",
			"messages about\n\ncomponent W(): html {<p>{t title}</p>}",
			[]string{`"example.com/app/messages"`, "messages.AboutTitle(framework.Locale(ctx))"},
			nil,
		},
		{
			"arguments are ordered by the declared parameters",
			"messages about\n\ncomponent W(count: int): html {<p>{t item-count, n: count}</p>}",
			[]string{"messages.AboutItemCount(framework.Locale(ctx), "},
			nil,
		},
		{
			"a symbol in the generated package is called unqualified",
			"messages about\n\ncomponent W(): html {<p>{t local}</p>}",
			[]string{"LocalMessage(framework.Locale(ctx))"},
			[]string{`"example.com/app/messages"`},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			out, err := htmlbind.Generate("message.txt", []byte(test.source), messageOptions())
			if err != nil {
				t.Fatalf("generate failed: %v", err)
			}
			for _, want := range test.want {
				if !strings.Contains(string(out), want) {
					t.Fatalf("generated output does not contain %q:\n%s", want, out)
				}
			}
			for _, absent := range test.absent {
				if strings.Contains(string(out), absent) {
					t.Fatalf("generated output should not contain %q:\n%s", absent, out)
				}
			}
		})
	}
}

func TestUnresolvedMessageIsAnError(t *testing.T) {
	_, err := htmlbind.Generate("message.txt", []byte("messages about\n\ncomponent W(): html {<p>{t missing}</p>}"), messageOptions())
	if err == nil {
		t.Fatal("an unresolved message generated without error")
	}
	if !strings.Contains(err.Error(), "unknown message about.missing") {
		t.Fatalf("error = %v, want it to name the resolved id", err)
	}
}

func TestMessageArgumentChecking(t *testing.T) {
	bad := []struct {
		name   string
		source string
		want   string
	}{
		{
			"missing argument",
			"messages about\n\ncomponent W(): html {<p>{t item-count}</p>}",
			"needs argument n",
		},
		{
			"unknown argument",
			"messages about\n\ncomponent W(count: int): html {<p>{t item-count, n: count, extra: count}</p>}",
			"has no argument extra",
		},
		{
			"argument on a message taking none",
			"messages about\n\ncomponent W(count: int): html {<p>{t title, n: count}</p>}",
			"has no argument n",
		},
	}
	for _, test := range bad {
		t.Run(test.name, func(t *testing.T) {
			_, err := htmlbind.Generate("message.txt", []byte(test.source), messageOptions())
			if err == nil {
				t.Fatalf("%s generated without error", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to mention %q", err, test.want)
			}
		})
	}
}

// TestMessageRefsReportsBeforeAnySymbolTable covers the reference half of
// requirement:template-parse-introspection: a caller has to be able to ask what
// a template needs before it can supply the table.
func TestMessageRefsReportsBeforeAnySymbolTable(t *testing.T) {
	source := "messages about\n\ncomponent W(count: int): html {<p>{t title}</p><p>{t common.save}</p><p>{t item-count, n: count}</p>}"
	refs, err := htmlbind.MessageRefs("message.txt", []byte(source))
	if err != nil {
		t.Fatalf("MessageRefs failed: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("got %d references, want 3", len(refs))
	}
	want := []struct {
		written string
		id      string
		args    int
	}{
		{"title", "about.title", 0},
		{"common.save", "common.save", 0},
		{"item-count", "about.item-count", 1},
	}
	for i, expected := range want {
		if refs[i].Written != expected.written || refs[i].ID != expected.id {
			t.Fatalf("reference %d = %q/%q, want %q/%q", i, refs[i].Written, refs[i].ID, expected.written, expected.id)
		}
		if len(refs[i].Args) != expected.args {
			t.Fatalf("reference %d has %d arguments, want %d", i, len(refs[i].Args), expected.args)
		}
		if refs[i].Scope != "about" {
			t.Fatalf("reference %d scope = %q, want about", i, refs[i].Scope)
		}
	}
}

// TestMessageRefsReportsAnUnresolvableReference keeps the report usable on a
// tree that is not finished yet.
func TestMessageRefsReportsAnUnresolvableReference(t *testing.T) {
	refs, err := htmlbind.MessageRefs("message.txt", []byte("component W(): html {<p>{t title}</p>}"))
	if err != nil {
		t.Fatalf("MessageRefs failed: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("got %d references, want 1", len(refs))
	}
	if refs[0].ID != "" || refs[0].Written != "title" {
		t.Fatalf("reference = %q/%q, want an unresolved title", refs[0].Written, refs[0].ID)
	}
}

// TestMessageInAURLAttributeIsRefused pins the interaction
// requirement:embedder-implicit-bindings is about: a message is a string, and
// the url type gate refuses any non-url part of a URL attribute. It is recorded
// as a test so the downstream framework meets a diagnostic rather than a
// surprise, and so the gate's amendment has something to change.
func TestMessageInAURLAttributeIsRefused(t *testing.T) {
	_, err := htmlbind.Generate("message.txt",
		[]byte(`messages about`+"\n\n"+`component W(): html {<a href="/{t title}/x">go</a>}`), messageOptions())
	if err == nil {
		t.Fatal("a string-valued message reached a url attribute")
	}
	if !strings.Contains(err.Error(), "requires url") {
		t.Fatalf("error = %v, want the url type gate", err)
	}
}

// TestMessageInAnOrdinaryAttributeIsEscapedHere is the other half of the same
// rule: outside the URL roster a message is ordinary text and this module
// escapes it, which is what decision:message-reference-syntax claims.
func TestMessageInAnOrdinaryAttributeIsEscapedHere(t *testing.T) {
	out, err := htmlbind.Generate("message.txt",
		[]byte(`messages about`+"\n\n"+`component W(): html {<p title={t title}>x</p>}`), messageOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if !strings.Contains(string(out), "htmlbind.Escape(messages.AboutTitle(framework.Locale(ctx)))") {
		t.Fatalf("a message in an attribute is not escaped by this module:\n%s", out)
	}
}

// TestTextNodeByteRanges covers the offsets half of
// requirement:template-parse-introspection: the range a rewriting tool replaces
// must be the source that produced the text, and replacing it must leave a file
// that parses.
func TestTextNodeByteRanges(t *testing.T) {
	source := "component W(): html {\n  <p>Welcome home</p>\n}\n"
	module, err := htmlbind.Parse("range.txt", []byte(source))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var found *htmlbind.TextNode
	var walk func(nodes []htmlbind.Node)
	walk = func(nodes []htmlbind.Node) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *htmlbind.TextNode:
				if strings.TrimSpace(n.Text) == "Welcome home" {
					found = n
				}
			case *htmlbind.ElementNode:
				walk(n.Children)
			}
		}
	}
	for _, decl := range module.Declarations {
		if d, ok := decl.(*htmlbind.TemplateDecl); ok {
			if body, ok := d.Body.([]htmlbind.Node); ok {
				walk(body)
			}
		}
	}
	if found == nil {
		t.Fatal("the text node was not found")
	}
	if got := source[found.Start:found.End]; got != found.Text {
		t.Fatalf("source[%d:%d] = %q, want the node's text %q", found.Start, found.End, got, found.Text)
	}
	rewritten := source[:found.Start] + "{t welcome}" + source[found.End:]
	if _, err := htmlbind.Parse("range.txt", []byte(rewritten)); err != nil {
		t.Fatalf("the rewritten source does not parse: %v\n%s", err, rewritten)
	}
	refs, err := htmlbind.MessageRefs("range.txt", []byte(rewritten))
	if err != nil {
		t.Fatalf("MessageRefs failed: %v", err)
	}
	if len(refs) != 1 || refs[0].Written != "welcome" {
		t.Fatalf("the rewrite did not produce one reference: %+v", refs)
	}
}

// TestAttributeValueByteRanges is the attribute half of the same rule, which is
// the case `pw i18n extract` needs for a placeholder.
func TestAttributeValueByteRanges(t *testing.T) {
	source := `component W(): html {<input placeholder="Your name">}`
	module, err := htmlbind.Parse("range.txt", []byte(source))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var part *htmlbind.AttributePart
	var walk func(nodes []htmlbind.Node)
	walk = func(nodes []htmlbind.Node) {
		for _, node := range nodes {
			element, ok := node.(*htmlbind.ElementNode)
			if !ok {
				continue
			}
			for _, attribute := range element.Attributes {
				if attribute.Name != "placeholder" {
					continue
				}
				for i := range attribute.Value {
					part = &attribute.Value[i]
				}
			}
			walk(element.Children)
		}
	}
	for _, decl := range module.Declarations {
		if d, ok := decl.(*htmlbind.TemplateDecl); ok {
			if body, ok := d.Body.([]htmlbind.Node); ok {
				walk(body)
			}
		}
	}
	if part == nil {
		t.Fatal("the attribute value was not found")
	}
	if got := source[part.Start:part.End]; got != part.Text {
		t.Fatalf("source[%d:%d] = %q, want %q", part.Start, part.End, got, part.Text)
	}
	rewritten := source[:part.Start] + "{t name_field}" + source[part.End:]
	refs, err := htmlbind.MessageRefs("range.txt", []byte(rewritten))
	if err != nil {
		t.Fatalf("the rewritten source does not parse: %v\n%s", err, rewritten)
	}
	if len(refs) != 1 || refs[0].Written != "name_field" {
		t.Fatalf("the rewrite did not produce one reference: %+v", refs)
	}
}

// TestMessageArgumentReachingAContextExternal covers the instruction selection:
// a message argument is an ordinary expression, so an argument that reaches a
// context-taking external has to make its instruction carry the context.
func TestMessageArgumentReachingAContextExternal(t *testing.T) {
	source := "messages about\n\nexternal Count(): int\n\ncomponent W(): html {<p>{t item-count, n: Count()}</p>}"
	options := messageOptions()
	options.ContextExternals = map[string]bool{"Count": true}
	out, err := htmlbind.Generate("message.txt", []byte(source), options)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if !strings.Contains(string(out), "TextCtx(") {
		t.Fatalf("the instruction does not carry the render context:\n%s", out)
	}
	if !strings.Contains(string(out), "Count(ctx)") {
		t.Fatalf("the external was not given the context:\n%s", out)
	}
}

// TestMessageReferenceKeysThroughItsContextBinding is the property routing the
// leading argument through a binding buys: a cached component carrying a message
// is distinguished per binding value, with no rule about messages in the cache
// at all.
//
// See .knowledge decision:implicit-binding-cache-identity.
func TestMessageReferenceKeysThroughItsContextBinding(t *testing.T) {
	source := "messages about\n\n@cache(ttl: \"5m\")\ncomponent Page(): html {<p>{t title}</p>}"
	out, err := htmlbind.Generate("message.txt", []byte(source), messageOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	rendered := string(out)
	if !strings.Contains(rendered, "Bindings: func(ctx context.Context) string") {
		t.Fatalf("a cached component carrying a message does not key on the binding:\n%s", rendered)
	}
	if !strings.Contains(rendered, "htmlbind.KeyString(framework.Locale(ctx))") {
		t.Fatalf("the context binding is not framed into the key:\n%s", rendered)
	}
}

// TestMessageReferenceFoldsTheVaryAxis is the outside-the-component half of the
// same property.
func TestMessageReferenceFoldsTheVaryAxis(t *testing.T) {
	source := "messages about\n\ncomponent Page(): html {<p>{t title}</p>}"
	out, err := htmlbind.Generate("message.txt", []byte(source), messageOptions())
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if !strings.Contains(string(out), `"Accept-Language"`) {
		t.Fatalf("a message reference did not fold its binding's vary axis:\n%s", out)
	}
}

// TestMessageContextBindingMustBeDeclared keeps the two options in step.
func TestMessageContextBindingMustBeDeclared(t *testing.T) {
	options := messageOptions()
	options.MessageContextBinding = "missing"
	_, err := htmlbind.Generate("message.txt",
		[]byte("messages about\n\ncomponent Page(): html {<p>{t title}</p>}"), options)
	if err == nil {
		t.Fatal("an undeclared context binding was accepted")
	}
	if !strings.Contains(err.Error(), "not a declared implicit binding") {
		t.Fatalf("error = %v, want it to name the missing declaration", err)
	}
}

// TestATypedBindingCannotBeWrittenIntoMarkup is what lets a catalog take its own
// locale type: the binding carries a value this module has no escaping rule for,
// so it is usable as a message context and nowhere else.
func TestATypedBindingCannotBeWrittenIntoMarkup(t *testing.T) {
	_, err := htmlbind.Generate("message.txt",
		[]byte("messages about\n\ncomponent Page(): html {<p>{locale}</p>}"), messageOptions())
	if err == nil {
		t.Fatal("a typed binding was written into markup")
	}
	if !strings.Contains(err.Error(), "cannot be written into markup") {
		t.Fatalf("error = %v, want the markup refusal", err)
	}
}
