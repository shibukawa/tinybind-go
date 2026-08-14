---
id: decision:value-binding-form
type: decision
title: Value Binding Is A Statement
---
Spell requirement:template-value-binding as one closerless directive scoped to the end of its enclosing block, and desugar it to a subtree before analysis.

```yaml
source:
  - requirement:template-value-binding
  - downstream framework change request 2026-08-14, which named this as the one design question it could not answer
  - owner decision 2026-08-14
review_gate: approved 2026-08-14 by the owner, against the reporter's stated preference and against this file's first reading
candidates:
  statement:
    shape: "{val a = f()}" with no closer, written {var ...} in the request
    scope: the rest of the enclosing block
    benefit: no nesting, and much less noise at the call site for two or three bindings in one element
  block:
    shape: "{val a = f()} subtree {/val}", written {let ...} in the request
    scope: the subtree, like for and await
    claimed_benefit: every analysis already run over this tree sees a shape it understands
  keyword_is_a_separate_question: the request spelled the statement var and the block let, but shape and spelling are independent; both are compared here at one keyword so only the shape is weighed
decision:
  chosen: the statement
  owner_reason: nesting depth; a subtree that binds three values would indent three levels for no structural reason, and an author pays that on every read while the implementer pays the desugaring once
  what_it_overrides: the reporter preferred the block, and this file's first reading recommended it, both weighting one-time implementer cost over recurring author cost
  what_survives_from_that_reading: the emission finding below, which no longer forbids the statement but does decide where the desugaring goes
verified_2026_08_14:
  parser:
    finding: BodyContext.ParseEmbedded returns either a node or a terminator, and a control node recurses through format.ParseBody for its body
    block_cost: one terminator kind and one case, structurally identical to parseFor
    statement_cost: also cheap as parsing; a statement is just a node the enclosing ParseBody appends
    verdict: parsing does not decide this
  analysis:
    finding: analyzeNodes carries scope as a map, so a statement could add to it mid-loop
    but: containment is currently free from the tree shape rather than enforced, and element children are analyzed with the enclosing scope itself rather than a copy
    consequence_if_left_flat: every recursion site becomes load-bearing, because a site that forgot to copy would leak a binding out of the element that declared it
  emission:
    finding: emitScope is per instruction list, not per node, and emitOps lowers one list against one Go receiver type
    consequence: a binding changes the receiver type for everything after it, so the flat form cannot be lowered directly
    what_it_actually_decides: not the spelling, but that the tree reaching analysis must already be the nested one; the statement is a source spelling and the subtree is its meaning
desugaring:
  rule: rewrite each body list so a binding directive owns the siblings that follow it, before analysis and before emission
  effect: the tree every downstream traversal sees is the block-shaped one, so containment stops being an invariant anyone has to maintain and goes back to being a property of the tree
  needed_by: the HTML format only, because its lowering is a plan of closures and emitScope is per instruction list
  not_needed_by: the SQL format, whose emitter writes straight-line Go where an if node is already a Go if block, so requirement:template-value-binding lowers a binding to a Go local and the target's own block scoping does the work
  placement:
    chosen: a normalization pass over each body list inside the HTML compiler, beside the existing normalizeWhitespace call
    why: it touches no existing contract, is one function, and leaves the flat tree in place for everything that reads Parse output directly
    per_format: the shared parser produces the flat node either way and each format decides whether to rewrite it, so neither format pays for the other's lowering
    rejected: desugaring inside the parse function itself
    why_not:
      contract: ParseEmbedded's callers all treat a node and a terminator as exclusive, returning early on a terminator and dropping the node
      spread: the binding would have to return its own node and the terminator that ended the enclosing body together, changing five call sites across templates/htmlbind, templates/internal/rawparse, and templates/sqlbind
      failure_mode: a site left unconverted drops the node silently, so the binding and everything after it vanish from the output with no diagnostic
  order: it runs before whitespace normalization, which walks the same lists and gains the new node either way
  what_it_destroys: the source-level distinction between a sibling and a nested binding, since after the rewrite everything following a binding is nested inside it
same_level_check:
  rule: requirement:template-value-binding forbids two bindings of one name in the same source-level block while allowing every shadow from further out
  problem_the_statement_form_creates: after desugaring, two consecutive directives nest, so an analysis-time scope-occupancy test cannot tell a redeclaration from a legal shadow inside a nested element
  where_it_belongs: a shared check over the flat list, since that is the last place the source level exists and the check is set membership over the names bound while walking one list
  shared_not_per_format: only the HTML format desugars, so the check cannot live inside that rewrite; the HTML normalization pass and the SQL compiler each call the same function over the same flat list
  sql_needs_it_more: two bindings of one name in one SQL block would emit two short variable declarations in one Go block, so without it the diagnostic is a Go compile error against generated code
  expressible_but_awkward_later: on the rewritten tree the condition is "a binding node appearing directly in another binding node's body list", which encodes a source fact as a structural coincidence
  block_form_did_not_have_this: with a closer, siblings stay siblings and the check is a plain duplicate scan over one list
  cost_ledger: this is the third thing the statement form pays for, beside the isControl entry and the flat print variant
  comma_form_covered_too: one directive binding a name twice is the same error, and the existing await duplicate-binding diagnostic is the model for its wording
print_fidelity:
  requirement: rule:template-format-fidelity, which the htmlbind fidelity test enforces
  free_here: the parse function keeps the flat tree, so the printer prints the directive and its following siblings at one indentation level with no closer
  no_flag_needed: only one spelling exists, so no node has to remember which shape it was written as
  printer_note: the existing control writer indents a body, so a binding needs a flat variant beside it rather than a reuse
attribute_context:
  fact: requirement:template-v1-scope excludes block control inside attribute values, and htmlbind isControl guards both attribute call sites before ParseEmbedded is reached
  requirement: the keyword joins isControl, or a binding written in an attribute reaches ParseEmbedded in html:attribute context instead of being refused by name
  block_would_not_have_needed_this: with a closer the construct cannot be written in an attribute at all, so this check is a cost the statement form adds
keyword:
  chosen: val, decided 2026-08-14 by the owner
  working_name_was: let, taken from the requirement:template-v1-scope deferred entry "immutable let bindings"
  why_val:
    semantics: the binding is immutable and requirement:template-v1-scope excludes mutable variables, so the keyword should be the immutable half of an immutable/mutable pair
    audience: JavaScript is the language written beside this markup, and there let is the reassignable declaration and const is the immutable one, so let names the wrong half for the reader most likely to be looking at it
    let_is_not_wrong_everywhere: Rust and Swift bind immutably with let, which is why the name survived as long as it did; the collision is with the adjacent language rather than with the concept
    val_var_pair: Scala and Kotlin spell the pair val and var, and only the immutable half is reachable here, so the pair is legible without var ever existing
  why_not_var: it is Go's and Kotlin's mutable spelling, so it names exactly the thing the language excludes
  casing: lowercase, per rule:template-name-casing dsl_keywords
raw_text_consequence:
  fact: the keyword must join htmlbind insertionKeywords, or the gate reads "{val a = f()}" in a script or style body as content and the construct silently does nothing there
  measured_2026_08_14:
    valid_javascript: "{let x = 1}" and "if(a){let b=1;f(b)}", both a block statement holding a let declaration
    not_javascript: "{val x = 1}", a syntax error on the juxtaposed identifiers
  consequence: val adds no residual ambiguity to this gate, because no valid JavaScript opens a tight brace with it
  what_let_would_have_cost: block-scoped declaration inside a block is what let exists for, so it is the spelling a minifier emits most often, and it would have been the highest-collision choice available
  policy_unchanged: rule:raw-text-insertion-gate residual_ambiguity still accepts collisions resolving toward insertion; this keyword simply does not reach it
  still_true_for_others: if and for collide with authored code on those terms already
rejected:
  block_form:
    what: a binding with a closer, scoped to its own subtree
    why_not: it indents every bound subtree for a reason the markup does not have, and the author pays that on every read
    what_it_would_have_saved: the normalization pass, the isControl entry, and the flat print variant
    revisit_if: the flat spelling turns out to read worse than expected at three or more bindings, which is the case it was chosen for
  automatic_call_collapsing:
    what: recognizing two identical external calls and evaluating one
    why_not: it would make the call count depend on an optimizer, and rule:render-external-query-semantics leaves the count open precisely so nothing depends on it
    who_rejected_it: the reporter, in the same request that asked for the binding
consequence:
  - after normalization the construct reuses the generated-scope lowering for and await already use, so no new lowering concept enters decision:generated-render-plan
  - the source is flat and the tree is nested, so authors get no nesting and analyses get containment
  - the two shapes must not diverge; anything reading Parse output sees the flat one and anything reading the compiler's tree sees the nested one
as_built_2026_08_14:
  held: the desugaring placement, the shared same-level check, the isControl entry, the insertionKeywords entry, and the flat printer path all landed as written here
  printer_was_cheaper_than_predicted:
    predicted: a flat variant beside the existing control writer
    actual: no variant at all; the parsed node has no body, so both formats print it as a leaf beside an expression node and the nodes it scopes are already where they belong
    why: this file treated the flat print as a cost of the statement form, when it is the statement form's own shape doing the work
  cost_ledger_settled: two of the three costs were real — the isControl entry and the same-level check — and the third, the flat print variant, did not exist
  one_thing_the_block_form_would_not_have_needed_either: nothing further; the normalization pass is the whole remaining difference
  verified: full module test suite and go vet clean, with the construct exercised in a for body, an if branch, both await subtrees, a script body, and a cached component
```
