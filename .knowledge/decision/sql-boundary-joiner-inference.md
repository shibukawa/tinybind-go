---
id: decision:sql-boundary-joiner-inference
type: decision
title: Infer the Joiner From the Operator Beside the Block
---
An AND, OR, or comma adjacent to a conditional node becomes a joiner the builder may withhold, rather than an explicit and/or marker the author must write; accept the round on that basis, because inference can only fix a source or leave it alone.

```yaml
source: downstream framework change request 2026-08-17, against v0.5.14, Popcorn Wave at github.com/shibukawa/popcornwave
review_gate: implemented 2026-08-17 for boolean clauses, with the refinements below
round:
  when: 2026-08-17, following the 2026-08-16 message-reference and implicit-binding rounds
  shape: one ask in three parts that only work together, three exactness rules offered unprompted, and three open questions
  reporter_position: the parts do not decompose; joiner recognition without group elision writes a joiner into a group that never opened
  what_is_new_in_its_shape: the reporter drafted the ask with explicit markers first and then argued itself out of them, so the round arrives carrying its own rejected alternative and the reason it lost
decision: infer from adjacency; add no syntax
serves: requirement:sql-conditional-predicate-composition
mechanism: rule:sql-predicate-group-elision
rejected_alternative:
  option: explicit and/or markers, or a group block form
  proposed_by: the reporter's own first draft
  why_rejected:
    - a marker requires editing every existing conditional predicate, and until edited the old spelling stays quietly wrong
    - it adds a node kind and a scope name, which the reporter pays for in a tokenizer snapshot over every template in its repository
    - it would leave the template shape docs/sqlbind.md teaches still broken
why_inference_is_safe_here:
  bounded_blast_radius:
    claim: this is a boundary, not a body
    detail: one token, one side of one node, one depth, and only when that node is not provably non-empty; every other token stays text
    contrast: the removed _tinybindSafeMutation runtime guard scanned whole statements with whitespace splitting for tokens that might indicate a safe mutation
  proof_not_guess: alwaysEmits is the same proof checkMutationSafety already trusts to decide whether a DELETE may run
  no_third_outcome:
    - an operator that is not dangling renders identically, so inference leaves the source alone
    - an operator that is dangling renders SQL the engine rejects, so inference only fixes it
    - therefore no branch combination exists in which a template working today stops working
accepted_positions:
  canonical: the operator in the enclosing text between the two nodes, which is where it sits in the finished statement
  also_accepted: leading a branch body, and trailing a branch body
  why_accepted: they are what docs/sqlbind.md and the existing generate and mutation tests contain, so rejecting them would leave the documented shape broken and reintroduce the migration this design avoids
  cost: one comparison per boundary, since the analysis reads the token on each side of the node either way
  documentation_stance: teach the canonical form; an operator inside a branch reads as part of that condition, which is the wrong shape for something joining two
formatter_cannot_normalize:
  finding: moving a token across an if boundary changes the AST
  forbidden_by: rule:template-format-fidelity parse_stability and no_semantic_edits
  consequence: the canonical form is a documentation recommendation, not a tool-enforced shape; the invariant is not relaxed and the reporter did not ask for it to be
verification:
  method: every cited position read against the worktree at v0.5.14
  alwaysEmits: confirmed at templates/sqlbind/mutation.go:148; recursive through if/else requiring both branches, and through a sql.predicate body, and true for a value expression and a relation call
  walkClause_group_imbalance: confirmed at templates/sqlbind/mutation.go:97-104, with the comment the request paraphrases
  clause_terminators: confirmed at templates/sqlbind/mutation.go:26-33, whereTerminators and setTerminators
  sqlLexer_carries_depth: confirmed at templates/sqlbind/sqlscan.go; depth across text nodes, literals and quoted identifiers and comments and dollar-quoted regions skipped, and start and end byte offsets recorded, which is what makes the token slice available
  clause_classification: confirmed at templates/sqlbind/fmtclause.go; boolean and comma are fields of clauseKind and matchClauseHead returns them
  flat_text_emission: confirmed at templates/sqlbind/generate.go:511; a TextNode becomes one b.WriteString inside a plain Go if, with no separator logic
  arg_atomicity: confirmed at sqlbind/statement.go:115; the append and the placeholder write are one operation in both placeholder styles
  conditional_result_columns_already_refused: confirmed at templates/sqlbind/compiler.go:695
  tests_that_must_not_move: confirmed; the conditional-only DELETE predicate still fails the proof and the trailing-AND-in-branch DELETE still generates
  line_drift: minor and immaterial; analyzeNodes is at compiler.go:256 not 378, validateStaticResultShape at 669 not 693, and the two mutation_test cases at 57 and 61 not 56 and 60
corrections_to_the_reporter_s_reading:
  join_on_is_already_classified_boolean:
    reported: as an open question, that fmtclause.go does not classify ON as boolean today, so it is the one opener not already computed
    actual: a bare ON already returns clauseKind{boolean: true, indented: true}; only ON CONFLICT branches away as absorbing
    consequence: the premise of the open question is wrong in the reporter's favour, and join ON lands in the first phase at no classification cost
    resolved: yes, in the same change
  the_builder_cannot_enforce_the_item_invariant:
    not_in_the_report: Ask 3 states correctly that Item precedes anything that can write or bind, and does not price what guarantees it
    finding: Builder embeds strings.Builder, so WriteString is a promoted public method; the elision invariant is an emitter obligation with no type-level enforcement
    consequence: an emitter arm added later that writes without calling Item breaks elision silently rather than failing, and elision is the half of this design that cannot be caught by reading the generated SQL of a passing branch
    disposition: recorded in rule:sql-predicate-group-elision emitter_obligation; the write paths in emitNodes are enumerable and every one must appear in the branch-combination matrix
  a_paren_is_already_generated_that_the_author_did_not_write:
    reported: the builder should only withhold a paren the author did write, and never invent punctuation
    actual: a RelationNode already emits b.WriteByte('(') at templates/sqlbind/generate.go:546
    why_it_does_not_contradict_the_ask: that is a subquery paren in data position rather than a grouping paren, so the principle holds as stated about grouping parens
    consequence: the rule must say grouping paren precisely, and a relation call must count as an item
open_questions_resolved:
  join_on: yes, in the same change; the opener is already computed, per the correction above
  comma_clauses:
    confirmed_no_interaction: conditional SELECT and RETURNING items are refused before elision could see them, at templates/sqlbind/compiler.go:695
    scope: boolean clauses first; comma clauses second
    set_last: a SET list additionally carries the mutation proof, so it lands after WHERE rather than beside it
  unresolvable_dangling_operator:
    decided: a generation diagnostic naming the construct, over leaving today's verbatim text
    reason: the template is already broken on some branch, and rule:sql-static-mutation-safety already states that a condition decidable at generation time must fail the build rather than a request
    covers: BETWEEN, CASE, and a branch-unbalanced paren pair
compatibility:
  byte_identity: a template whose operators never dangle renders byte-identical SQL; whitespace around an elided joiner is the only possible difference, and only in branches broken today
  invalid_to_valid: a dangling operator changes from invalid SQL to valid SQL, which is a fix carrying a release note rather than a behaviour change
  no_grammar_change: analyzeNodes rejects unknown node kinds and needs no new arm; no node kind, no directive, no scope name
  api_surface: OpenGroup, Joiner, Item, and CloseGroup are new methods on a Builder only generated code drives; generated bodies gain calls and no signature changes
  walkers_to_admit_the_distinction: walkClause and alwaysEmits, isReadOnly and topLevelVerb, validateStaticResultShape and staticSQL, and the formatter, which already treats a comma and AND or OR as the item separators of a classified clause
what_implementation_changed_about_the_design:
  the_adjacency_test_was_dropped_as_too_narrow:
    drafted: an operator is a joiner only when adjacent to a node whose alwaysEmits is false
    implemented: every AND or OR at the innermost open group's item depth is a joiner
    what_the_narrow_rule_missed: 'WHERE ({if a}x{/if}) AND y' puts the closing paren between the operator and the elidable node, so the operator joining a parenthesis group to its sibling is not adjacent to anything elidable and stays text; it then survives an empty group
    why_the_broad_rule_costs_nothing: a recorded separator is flushed by the next item at the position it was recorded, so Joiner and WriteString have identical output once the group holds an item; the two rules can only differ where an operand can vanish, and there the broad rule is the fix
    what_the_reporter_gains: the four accepted operator positions stop being separate cases, so the part of the design carrying the most detail turned out not to be needed
  a_fifth_builder_method_was_needed:
    finding: a whitespace-only run between two conditions is neither an item nor a joiner, and writing it eagerly places it before a separator that has not flushed, emitting 'x AND' with no space after the operator
    added: Space, which appends to a pending joiner, is dropped in a group holding no item, and is written otherwise
    where_it_showed_up: only the trailing-operator position, where the operator ends one branch and the whitespace follows it; the canonical and leading forms never exercise it
  CASE_is_excluded_rather_than_diagnosed:
    drafted: an error naming CASE
    implemented: inside a CASE region an operator is text and a parenthesis opens no group, so nothing is ever withheld there
    why: no frame is ever pushed in a CASE region, so the region is emitted exactly as today; an error would refuse templates that currently work, and the request said leaving it as verbatim text was also acceptable
  a_clause_opened_inside_a_branch_is_not_an_error:
    found: '{if a}WHERE x = {x}{/if}' is the pre-existing idiom for a wholly conditional clause and is legal for a SELECT today
    implemented: a group opened inside a branch closes at that branch's end, so both paths leave the same stack and the template renders unchanged
    consequence: the unbalanced-branch error narrows to parenthesis nesting, which is the case the request actually named
  the_byte_identity_claim_became_structural:
    implemented: a body with nothing elidable in it is planned as nil and emitted exactly as before, so no group call reaches its generated code
    why_it_matters: the compatibility claim no longer rests on the frame protocol being output-neutral; for every non-conditional template the mechanism is not present at all
reporter_contribution_offered:
  - implementation against a prerelease, reporting where inference is under-determined
  - the branch-combination matrix over a two- and a three-condition predicate, asserting SQL and Args together, plus nested-group, all-absent, BETWEEN, and call-paren cases
  - byte-identity evidence, since every existing conditional-SQL test is a regression test for this change
related:
  - requirement:sql-conditional-predicate-composition
  - rule:sql-predicate-group-elision
  - rule:sql-static-mutation-safety
  - rule:template-format-fidelity
  - rule:sql-top-level-keyword-scan
  - rule:sql-template-layout
  - requirement:sql-template-v1
  - requirement:analysis-diagnostics
```
