---
id: requirement:template-value-binding
type: requirement
title: Template Value Binding
---
Bind a synchronous external's result to a name in a template body, so the rest of the block reads one value instead of calling once per mention.

```yaml
priority: should
source:
  - downstream framework change request 2026-08-14, against v0.5.9
  - requirement:template-v1-scope deferred entry "immutable let bindings", which this promotes rather than invents
review_gate: approved and implemented 2026-08-14
form: decision:value-binding-form
formats: both, per sql_format below; the request asked for HTML and the owner widened it 2026-08-14
problem:
  today: a synchronous external is an ordinary expression with no binding form, so every mention of its result is another call
  cheap_until_it_fetches: the results called for today are small, so the repeat costs little and is correct
  case_that_breaks_it: a component taking a primary key and loading its own record calls the loader once per field it renders; four fields, four calls
  reporter_summary: it is the reason a component cannot honestly fetch anything
  mental_model: the intended one is "cache the fetch and the render as one unit", and a grammar that cannot name a fetched value cannot express it
why_the_ask_is_small:
  rule: rule:render-external-query-semantics already requires an external to be a repeatable data query and already declines to guarantee a call count
  consequence: calling once instead of four times changes no behavior the language guarantees, so this is an efficiency and legibility change against a contract that already permits it
  not_a_semantic_change: no new rule about side effects, ordering, or retries is needed
explicitly_not_asked:
  what: automatic collapsing of identical external calls
  why_not: it would make the call count depend on an optimizer, and authors would rely on a number rule:render-external-query-semantics deliberately leaves open
  what_the_binding_does_instead: puts the count in the author's hands and in the source, where it can be read
existing_binders:
  for: the loop variable, scoped to the loop body
  await: one or more requirement:async-external-functions results, scoped to the primary subtree, per decision:async-boundary-syntax
  sync_external: nothing, which is the gap
why_await_is_not_the_workaround:
  mechanical: an async external may be called only in an await clause header, so getting a name out of one means declaring a fast local lookup async
  cost: a streamed region, a required fallback, a committed placeholder, a commit point, and the client runtime, to avoid typing a call twice
  stated_by_the_reporter: not a workaround they would recommend to anybody
  same_shape_as: requirement:render-context-externals async_is_not_the_substitute, which rejected the same substitution for the same reason
shape:
  working: |
    {val record = LoadData(id)}
    <h1>{record.title}</h1>
    <p>{record.summary}</p>
  keyword: val, per decision:value-binding-form; the binding is immutable and val is the immutable half of a val/var pair, where let is JavaScript's reassignable one
  closer: none; decision:value-binding-form chose the statement so a bound subtree gains no indentation
  bindings: one or more, comma separated, as await already accepts; one value per name
  independent_within_a_directive:
    rule: the bindings of one directive cannot read each other; a dependency is written as two directives
    corrected: 2026-08-14 by the owner, reversing a sequential reading recorded here and built before it was questioned
    why_sequential_was_wrong:
      go: `a, b := f(), g(a)` does not compile, because a short variable declaration evaluates every right side before assigning any; measured, not recalled
      await: decision:async-boundary-syntax bindings settle concurrently, so one has never been able to read another
      one_spelling_two_meanings: the same comma would be simultaneous in an await clause and sequential here, which is the objection the change request itself raised against reading `a, b = f()` as destructuring
    what_it_costs_the_author: one more directive, and the dependency becomes visible instead of hiding inside a line read left to right
    where_checked: parseVal, since it is a syntactic property of one directive; both formats inherit it from the shared parser
    lowering_unchanged: normalization still nests a comma list into one node per binding, which is lowering rather than meaning
  scope: from the directive to the end of the enclosing block, which is the enclosing element, control body, or declaration body
  meaning: the siblings that follow are the binding's subtree; decision:value-binding-form desugars to exactly that before analysis
  immutable: no reassignment; the binding is a name for one evaluation
  name_form: lowerCamelCase, per rule:template-name-casing dsl_values, matching the check parseFor and parseAwait already apply
usable_positions:
  rule: the bound name is an ordinary typed value afterwards
  list: interpolation, attribute value, boolean attribute, if condition, for iterable, component argument, and argument to another external in the same scope
  reason: requirement:render-context-externals already enumerates these as the positions an external call reaches, so the bound name inherits the same set
withdrawn_by_the_reporter:
  multi_value_return:
    what: `{var a, b = LoadData(id)}`, one call yielding two values
    why_withdrawn: it lands on the declaration grammar rather than the markup grammar, needs a tuple-ish type the language has no other use for, and raises what happens when only one of the two is read
    better_answer: a Go function returning a record holding both, because a record has a name a second caller can accept as a parameter
  record_destructuring:
    what: reading `a, b = f()` as taking two fields out of one record
    why_not: it is spelled identically to the multi-value return, means something else, and an author who guesses wrong gets a type error at a position that cannot say which they meant
typing:
  async_stays_await_only: an async or live external named in a binding is a generation error pointing at decision:async-boundary-syntax, unchanged from requirement:async-external-functions usage
  html_result_rejected: requirement:template-language-core makes html a non-value type usable only at a requirement:html-slot-syntax slot, so `external CSRFField(): html` cannot be bound; it is already rendered as a subtree at its call site
  reserved_name: outer, which the generated scope struct uses; the same rejection analyzeAwait already makes
  duplicate_name: an error within one binding construct, as await already reports
  any_expression: the right side need not be an external call; a field path or a nested call binds the same way, because the analysis is over the typed expression rather than over the callee
  shadowing:
    decided: 2026-08-14 by the owner
    allowed: shadowing a name bound further out — a parameter, a for variable, an await binding, or a binding in an enclosing block; for and await already shadow silently and a binding that did not would be the odd one
    error: two bindings of one name introduced into the same source-level block, whether by one directive's comma list or by two consecutive directives
    reason: at one level the second is a redeclaration rather than a deliberate shadow, and the source gives the author no closer to read the first one's extent from
    where_checked: decision:value-binding-form same_level_check; the desugaring makes consecutive directives nest, so this cannot be a plain scope-occupancy test at analysis time
    unchanged_for_others: a for or await clause is unaffected by this rule and keeps the silent shadow it has today
  attribute_position: refused by name, per decision:value-binding-form attribute_context; requirement:template-v1-scope excludes block control inside attribute values and a binding has a body even without a closer
  unread_binding:
    rule: a generation error in both formats, decided 2026-08-14 by the owner over two rounds — HTML first, then SQL once the asymmetry was named
    reason: the value is computed before anything reads it, so an unread binding calls its external and discards the result; rule:render-external-query-semantics allows an external to answer a query and nothing else, so there is no reading under which the call was wanted
    when_the_call_happens: once per render in HTML, once per statement build in SQL, which is the only thing the two diagnostics word differently
    why_the_language_has_to_say_it:
      html: the bound value becomes a field of a generated struct rather than a local, so Go accepts the dead call and nothing downstream would ever report it
      sql: Go would report the unused local, but against a line of emitted code; a diagnostic that cannot name the template line that caused it is not the template's diagnostic
      shared_principle: a rule of this language is reported by this language, whether or not the target happens to catch it
    what_it_retired: the SQL blank-assignment path, which existed only to make an unread local compile; no unread binding reaches emission now
    what_it_makes_of_shadowing: a shadow whose outer binding is read nowhere else is an error, so deliberate shadowing stays legal and pointless shadowing does not
    escape_hatch: none, and none is wanted; a call worth making for its result is worth reading, and one worth making for its effect is already forbidden
lowering:
  precondition: decision:value-binding-form desugaring has run, so the tree reaching analysis already nests the following siblings under the binding
  op: a Val form beside htmlbind For, minus iteration; value builds the bound value from the enclosing parameters, scope builds the child scope struct, body is the subtree's instruction list
  context_form: a ValCtx variant, per requirement:render-context-externals as_built, since the bound expression is exactly where a context-taking external is expected to appear
  scope_struct: one generated struct per binding, holding Outer and the bound field, the shape emitForOp and emitAwaitOp already emit
  comma_form_is_notation_only:
    corrected: 2026-08-14 on implementation, reversing a claim made here before it was built
    was_claimed: a comma list emits one struct where consecutive directives emit one per binding
    actual: normalization splits a comma list into nested single-binding nodes, so both spellings emit one struct per binding and differ only in what the author types
    why_that_shape_was_chosen: it leaves the analyzer and the emitter handling one binding at a time
    what_it_does_not_mean: the nesting is not a scope the author can use; independent_within_a_directive forbids reading a sibling, so the two spellings differ in nothing but line count
  call_count: one, because the value closure runs once before the body's instruction list
  sequence_splice:
    requirement: the body must be spliced inline into the requirement:structured-render-output sequence tree, contributing no value-stream marker
    why: the body runs exactly once, so unlike a loop it has no count and unlike a conditional it has no branch to record
    failure_if_missed: htmlbind sequenceOf falls through to SeqSlot for an unrecognized op, so every bound subtree would silently become an opaque hole and requirement:component-delta-rendering would stop decomposing it
    severity: a working render with a degraded delta path and no diagnostic, which is why it is stated rather than left to the implementer
sql_format:
  decided: available in SQL too, 2026-08-14 by the owner, widening the request, which asked for HTML alone
  use_case: normalize a value once in Go and use the result in several parameter positions of one statement
  same_problem_there: sqlbind's expression emitter writes an external call inline per mention, so a normalization named in a WHERE and again in an ORDER BY runs twice, exactly as in markup
  lowering:
    shape: one Go statement, emitted where the directive stands
    why_it_is_cheaper_than_html: sqlbind emits straight-line Go and an if node already emits a real Go if block, so the binding's scope maps onto Go's own block scope with nothing generated to carry it
    nothing_needed: no scope struct, no op, no closure, and no desugaring; decision:value-binding-form places the rewrite in the HTML compiler because only a closure-based lowering needs it
    name: through sqlbind goLocalName, which already prefixes a Go keyword with an underscore, so a binding named type or range emits a valid local
  emits_no_sql: the directive contributes no bytes to the statement, so the surrounding text nodes carry the spacing unchanged
  bindable_result: value types only; a predicate or relation call is not a value and is refused, as an html result is in the HTML format
  same_level_check_earns_more_here: two bindings of one name in one block would emit two short variable declarations in one Go block, so without the template-level check the author gets a Go compile error against generated code
  no_conflict_with_v1_exclusions: requirement:template-v1-scope excludes general SQL loops, identifier interpolation, and dynamic result columns; a binding names a value and touches none of them
  supersedes: the earlier reading in this file, which scoped the construct to HTML and left SQL as an open question
downstream_goal:
  wanted: a component declaring a primary key as its parameter, loading inside itself, and carrying one requirement:component-output-cache annotation over the load and the render together
  today: the render cache saves the rendering and the framework's own data cache saves the fetching, so one slow page is configured in two places for what an author thinks of as one thing
  what_the_binding_already_gives_it: the load sits inside the cached subtree, and decision:cache-key-derivation keys on the declared parameters, so a hit skips the loader as well as the markup with no further cache work
  what_it_does_not_give: a staleness policy over fetched data, which requirement:component-output-cache tracks as its own open question
  not_requested_yet: the caching half; the reporter states its key-routing question is unresolved and asks for the binding alone
constraints:
  - rule:render-external-query-semantics is unchanged; the binding relies on it rather than amending it
  - requirement:html-rendering-compatibility holds; a template with no binding generates what it generates today
  - decision:reflection-free holds; the bound value is a typed field of a generated struct in HTML and a typed Go local in SQL
acceptance:
  - a subtree binding one external call executes that call once regardless of how many times the name is read
  - the same template written without the binding is unchanged, call for call and byte for byte
  - the bound name resolves in interpolation, attribute, condition, iterable, component argument, and nested external argument positions
  - an async or live external named in a binding fails generation and names the await clause
  - an html-returning external named in a binding fails generation
  - several bindings in one directive each bind one value, and a duplicate name fails generation
  - two consecutive directives binding one name in the same block fail generation, with the same diagnostic as the one-directive duplicate
  - the same two directives separated by an enclosing element compile, because the inner one shadows from a deeper block
  - a binding shadowing a parameter, a for variable, or an await binding compiles, provided the outer name is read outside the shadowing subtree
  - a binding no position reads fails generation in both formats, including one written beside a read binding in the same directive
  - the unread diagnostic names the template file and position, never a line of generated Go
  - a SQL statement emits no blank assignment, because no unread binding reaches emission
  - every read position counts as a read: text, bare and quoted attributes, an if condition, a for iterable, a for body, an await binding expression, an await primary and fallback, a later directive's bound expression, a nested element, and a component argument or child
  - a directive whose binding reads a sibling of the same directive fails generation in both formats, and the diagnostic says to write it as its own directive
  - two independent bindings in one directive both compile and both count as read
  - a subtree under a binding still decomposes into a sequence tree rather than becoming one opaque node
  - a binding's name is unresolved after the enclosing element, control body, or declaration body ends
  - a binding written in an attribute value is refused by name rather than by the generic attribute diagnostic
  - a source written with a binding round-trips through the formatter with no closer and no added indentation
  - a SQL statement binding one external call and reading it in two parameter positions calls it once
  - a SQL binding declared inside an if body is out of scope after it, because the generated Go block ends there
  - a SQL binding named after a Go keyword generates a valid local
  - a SQL binding on a predicate or relation call fails generation
open_questions:
  - whether requirement:awaitable-parameters and this share one analysis, since both bind a name to a value the subtree reads
related:
  - requirement:template-language-core
  - requirement:render-context-externals
  - requirement:component-output-cache
answers_open_question:
  where: requirement:render-context-externals open_questions, "whether one call is shared when the same context-taking external appears several times in one render"
  answer: no sharing is inferred; the author writes the binding and the count is in the source
as_built:
  when: 2026-08-14, both formats in one change
  shared: syntax.ValNode and ValBinding, parseVal for the closerless form, ValString for printing it back, DuplicateValBinding for the same-block check both formats call, and ExprReads, which both formats' usage scans walk expressions with
  usage_scan:
    two_walks_one_expression_helper: each format walks its own node types and shares ExprReads, because the node sets have nothing in common and the expression grammar is entirely shared
    exact_not_conservative: every binder that introduces a name is recognized — a nested binding, a for variable and index, an await binding over the primary subtree alone, and a recover error name — so a shadowed outer binding is reported unread rather than guessed at
    why_exactness_is_required_now: the answer refuses a template rather than adding a line to generated code, so a name wrongly reported unread rejects working work; there is no longer a safe direction to lean
    extent: the nodes following the binding; a sibling of the same directive cannot read it, so it never has to be counted as a reader
    walk_must_enter_a_binding_body: found on implementation; the HTML scan skipped a nested binding's body and reported a read binding unread whenever two independent bindings shared one directive
  html:
    normalization: a compiler pass over each body list, run after whitespace is settled so a run is still decided on the flat list the author wrote
    runtime: htmlbind.Val and ValCtx, For minus the iteration
    sequence: valOp exposes sequenceInline, a new hook beside the existing sequenceBody; the splice is what keeps a bound subtree decomposing
    gates: the keyword joined insertionKeywords, so it is recognized in a script or style body, and isControl, so an attribute refuses it as a block rather than as a bare expression
  sql:
    lowering: one Go local per binding, emitted where the directive stands; nothing generated carries the scope
    record_binding_allowed: an interpolation still cannot place a record, but binding one and reading its fields is the point, so the record rejection stays on the expression node and is not applied to a binding
    unread_check_lives_in_analysis: not in the emitter, because it is a rule about the template rather than a property of the emitted Go; emission now assumes every local it writes has a reader
    fixed_on_the_way: the statement emitter was handed its declared parameter map directly, which a binding would have mutated; it now gets a copy
  not_reachable: a head contribution, because requirement:head-merging fixes the merged head before the first body byte and a contribution accepts static text only; a binding wrapping one fails with that existing diagnostic
  tests: value binding, redeclaration, shadowing, block extent, async and html refusal, attribute refusal, script-body recognition, every body-bearing block, the cached self-loading component, the runtime call count, ValCtx, the sequence splice, the SQL cases, and formatter round-trip in both formats
```
