---
id: decision:message-reference-syntax
type: decision
title: Message Reference Is A Contextual Directive
---
Spell a message reference `{t <id>}`, recognized as a directive only when the whole brace body is a message reference, so `t` stays an ordinary identifier and the value stays a string.

```yaml
source:
  - concept:template-message-surface, request item B
  - reporter's open contract 1, the one it said could force a redesign
review_gate: approved 2026-08-16 by the owner
as_built:
  status: implemented 2026-08-16, parse through emission
  parser:
    where: templates/internal/syntax/body.go, parseMessage beside parseVal, reached from the default arm of ParseEmbedded
    recognizer: the whole trimmed body must read as a reference; the first top-level comma part has to be an id, and anything else falls through to ParseExpressionAt
    commit_point: a valid id, after which a malformed argument is reported as a bad reference rather than as a confusing expression
    keyword: the messageKeyword constant, so the fallback spelling is one edit
  representation:
    chosen: MessageExpr, an Expr rather than a Node
    why: an attribute value holds an Expr and not a Node, so a node-shaped reference would have needed its own path through AttributePart, analysis and emission
    consequence: text, attribute, condition, component argument and val positions all work with no rule of their own, which is the string_valued claim above holding in code rather than in prose
    recognition_is_still_brace_level: `{f(t title)}` is not a reference and never becomes one; representation and recognition are separate
  id_form: messageID, dot-separated segments of letters, digits, underscores and hyphens, with no segment starting or ending in a hyphen
  escaping: unchanged and verified in the emitted output; a reference in a text position goes through the Text op and one in an attribute through htmlbind.Escape, exactly as any other string
  printing: MessageString, printing the authored id so a qualified reference is not rewritten
  tests: templates/htmlbind/message_test.go, and templates/sqlbind/message_scope_test.go for the dialect scope
  walkers_that_had_to_learn_the_new_expression:
    found: 2026-08-16, by walking every expression type switch rather than by a failing test
    syntax_ExprReads: missed a message's arguments, so a val binding read only from inside a reference would have been invisible to the binding-scope analysis
    exprPos_three_copies: htmlbind, sqlbind and the shared one each returned line 1 column 1 for a message, so any diagnostic anchored on one pointed at the top of the file
    emitter_walkExpr: recorded under requirement:message-symbol-resolution; the instruction would not have carried a render context an argument needed
    reading: adding an Expr kind is not additive; every switch over Expr is a place that silently does the wrong thing rather than failing to compile, because the interface is satisfied either way
verified_in_code_2026_08_16:
  the_compatibility_proof_is_now_a_test: TestMessageKeywordIsContextual, covering `{t}`, `{t.Year}`, `{t.Format(layout)}`, `{t .Year}`, `{t == other}`, `{if t > other}`, `{t - other}`, `{t -other}` and `{t ? a : b}`, each against a component declaring a parameter named t
  the_hyphen_trap_is_real_and_closed: `{t -other}` lexes as subtraction, so a segment may not begin with a hyphen; without that rule this form would have silently changed meaning
requested_form:
  text: "<h1>{t title}</h1>"
  attribute: "<input placeholder={t name_field}>"
  arguments: a message taking arguments names them at the reference; requirement:message-symbol-resolution checks them
must_keep_working:
  bare: "{t} interpolates a parameter named t"
  field: "{t.Year}"
  call: "{t(x)}"
  why_it_matters: `component PostCard(t time.Time)` is legitimate, and a single-letter identifier is not worth spending permanently
verified_2026_08_16:
  where: templates/internal/syntax/body.go ParseEmbedded
  finding: every existing directive is already contextual; the dispatch is a prefix test on the keyword plus one space, and the default arm hands the whole trimmed body to ParseExpressionAt
  members: else, else if, /if, /for, fallback, recover, /await, await, val, if, for
  consequence: this request asks for the mechanism the language already uses rather than a new parser category, so the reserved-word question the reporter feared does not arise
  cost: one case beside the `val ` case, plus the recognizer below
  compatibility_proof: `t title` is not a valid expression, because the expression grammar in requirement:template-language-core has no juxtaposition; every brace body this form would newly claim is a parse error today, so no existing template changes meaning
recognizer:
  correction: the request's rule, an identifier followed by another identifier, is not sufficient
  why: the existing dispatch tests a prefix, and `{t == "x"}`, `{t > n}`, and `{t ?? d}` all carry the prefix `t ` while meaning the parameter
  rule: recognize the directive only when the entire brace body parses as a message reference — a dotted id, then optional comma-separated named arguments — and otherwise fall through to the expression arm
  latent_elsewhere: the same shape is a live defect for the existing keywords; `{val == 1}` and `{for > 2}` misparse today for a parameter named val or for, which is unreported because those names are implausible and the failure is a syntax error rather than a wrong result
  disposition: fixing the general case is not required by this request, but the recognizer written here should be the shared one rather than a second mechanism beside the prefix tests
id_lexical_form:
  decided: 2026-08-16 by the owner; a hyphen is permitted, so `{t item-count}` is written as the request wrote it
  why_it_was_a_question: the expression lexer reads `item-count` as subtraction, so an id is not an expression identifier
  what_makes_it_safe: the recognizer above consumes the whole brace body, and an id token contains no whitespace; `{t x - y}` has spaces around the hyphen, fails the id form, and falls through to the expression arm, while `{t x-y}` is unambiguous because juxtaposition is invalid in every expression
  form: dot-separated segments, each of unreserved word characters and hyphens, with no whitespace anywhere in the token
  scanner: the id is scanned by this recognizer rather than by ParseExpressionAt, which is already true of every directive header
  consequence_downstream: an id is not a Go identifier, so the mapping from id to generated symbol is a table requirement:message-symbol-resolution is given rather than a transform it computes
  cost_accepted: the id shape is now catalog policy this module cannot constrain, which is the price of not making the reporter's slugs adapt to Go's lexer
string_valued:
  rule: a reference evaluates to a plain string
  escaping: unchanged; the value flows through the existing rule:template-context-safety path and is escaped once for its position
  worked_example: a message text carrying `&` and an argument carrying `<b>` are concatenated by the generated function and escaped together by the template, so text, attribute, and URL positions need no new rule
  why_this_is_the_load_bearing_choice: it is the reason the whole change stays small; a markup-valued message would need a trust type, a new insertion class, and a rule for translator-supplied tags
  translators_never_carry_markup: requirement:message-hole-binding is the one form with internal structure, and it supplies the markup from the template rather than the catalog
  url_position_caveat: a string reaching a URL attribute is refused by the type gate today; see requirement:embedder-implicit-bindings, which is where that gate is actually contested
documentation_cost:
  what: a reader must learn that `{t x}` and `{t} x` differ
  accepted_by_the_reporter: yes, as cheaper than losing the identifier
  our_note: the same cost already exists unremarked for `{val x = 1}` against `{val}`, so this is a documentation gap the language has rather than one this feature opens
fallback_spelling:
  when: only if the recognizer above is refused
  form: `{msg <id>}`
  what_changes: nothing downstream except documentation, which is why the reporter asks to settle it before writing any
  observation: the reporter treats reserving a word as the fallback's cost, and the verification above removes that cost from both options; `msg` would be no more reserved than `t`
formatter:
  obligation: requirement:template-source-formatting must print the new node, and rule:template-format-fidelity applies unchanged
  not_mentioned_in_the_request: the printer is a second consumer of every syntax addition, and requirement:template-parse-introspection makes it a third
```
