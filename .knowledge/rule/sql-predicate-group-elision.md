---
id: rule:sql-predicate-group-elision
type: rule
title: SQL Predicate Group Elision
---
A clause keyword, a grouping paren, and a joiner are written the moment a fragment inside them actually emits, so a group that stays empty was never opened and nothing has to be taken back.

```yaml
priority: must
serves: requirement:sql-conditional-predicate-composition
rationale: decision:sql-boundary-joiner-inference
detection: rule:sql-top-level-keyword-scan
status: implemented 2026-08-17, boolean clauses first and comma clauses in the same day's second change
comma_groups:
  rule: a comma at the item depth of an open comma group is a joiner, on the same frame protocol as AND and OR
  openers:
    clause: SET, ORDER BY, GROUP BY, and VALUES
    two_token: ORDER BY and GROUP BY need one token of lookahead, so the opener spans both words
    value_list_paren:
      which: the tuple after VALUES, and the column list after INSERT INTO its target
      why_it_inverts_the_boolean_test: a boolean group's paren must not follow a word, because rule:sql-template-layout calls a parenthesized list data; a comma group's paren is that list, so following a word is exactly what identifies it
      insert_column_list: one bit of state, set by INSERT INTO through its target name and consumed by the next paren at that depth
  left_as_text: SELECT, RETURNING, FROM, WITH, WINDOW, USING, and PARTITION BY
  why_select_and_returning_are_excluded: a conditional item there is already refused by validateStaticResultShape before elision could see it, so a group there would carry no case
  group_by_included_unasked: the request named SET, ORDER BY, and VALUES; GROUP BY is the same two-token opener with the same empty-clause failure, so excluding it would read as an oversight rather than a decision
  empty_clause: an ORDER BY, GROUP BY, or SET whose every item is conditional drops its own keyword, which is what requirement:sql-template-v1 asks for with 'manage commas and empty clause'
  set_and_the_mutation_proof: a withheld comma fills nothing, so an UPDATE whose SET items are all conditional stays the generation error rule:sql-static-mutation-safety already makes it
  insert_pairing_hazard:
    what: a conditional column and its conditional value sit in two independent groups, so guarding them with different conditions renders a column count that disagrees with its value count
    status: not checked; the mismatch reaches the database as a runtime error
    why_not_refused: the per-path counts are decidable and a check is tractable on the walkClause machinery, but it is a new refusal nobody asked for and it could reject templates in use
    what_the_author_owes: guard a column and its value with the same condition
joiner_recognition:
  rule: every AND or OR at the item depth of the innermost open group is a joiner
  no_adjacency_test:
    superseded: the drafted rule, which asked for an operator adjacent to a node whose alwaysEmits is false
    why_the_narrow_rule_was_wrong: it misses the operator that joins a parenthesis group to its sibling, as in 'WHERE ({if a}x{/if}) AND y', where the closing paren stands between the operator and the elidable node; that AND has to be withheld when the group is empty and the narrow rule leaves it as text
    why_the_broad_rule_is_safe: Joiner and WriteString have the same effect once the group holds an item, because the separator is flushed by the next item at the position it was recorded; the rule can only differ where an operand can vanish, and there it is the fix
    consequence: the four accepted positions and the elidable-node test are no longer needed as separate cases; alwaysEmits is still what decides whether a predicate call may skip its Item
  exclusions: an operator is text where no group is open at its depth, inside a CASE region, and where it closes a BETWEEN
  elsewhere: 'WHERE a = {a} AND b = {b}' has no group at all, because a body with nothing elidable in it is planned as before
group_openers:
  read_from: the SQL the author already wrote; no opener is invented
  clause: the boolean and comma classification matchClauseHead already computes at templates/sqlbind/fmtclause.go
  boolean_clauses: WHERE, HAVING, QUALIFY, and a bare join ON
  paren: a grouping paren opening inside a region a clause already established
  not_a_group: a paren preceded by a word is a call, function argument, or IN list, which rule:sql-template-layout already calls data rather than a statement; sqlLexer tracks depth but does not yet draw this line and must learn it
  closes_at: the matching paren, or the clause terminator sets at templates/sqlbind/mutation.go:26-33
frame_protocol:
  where: a stack of frames on sqlbind.Builder at sqlbind/statement.go, one per open group, each holding its opener text, whether that opener was written, and a pending joiner
  OpenGroup: push; write nothing
  Joiner: record as the frame's pending joiner; a no-op when the frame has written no item, which is what makes a leading operator vanish
  Item: emitted immediately before every fragment that can write a token or bind a value; walk from the outermost unwritten ancestor inward, writing each ancestor's opener after flushing that ancestor's parent's pending joiner, then flush this frame's pending joiner, then mark the frame written
  CloseGroup: takes the closer text, written only if the opener was written; a clause group passes an empty closer because it ends where the next clause begins; a pending joiner nothing followed is dropped, which is what makes a trailing operator vanish
  Space:
    added_in_implementation: a fifth method the drafted protocol did not carry
    what_it_does: writes a whitespace run that separates two items rather than being one; it appends to a pending joiner so it travels after the separator, is dropped in a group holding no item, and is written otherwise
    why_it_is_needed: a whitespace-only run between two conditions is neither an item nor a joiner, and writing it eagerly puts it in front of a separator that has not flushed yet, which emits 'x AND' with no space before the next item; the failure is only visible in the trailing-operator position, where the operator ends one branch and the space follows it
    why_it_cannot_fill: the scanner does not tokenize whitespace and alwaysEmits does not count it, so a run of it must never open the group it merely separates
  nesting: a nested group registers in its parent only by having filled, so an empty one takes its parens and the joiner that attached it; that falls out rather than needing its own rule
  generator_decision: which call a sliced token becomes; the slices come from the byte offsets sqlToken already reports
text_split:
  opener: the whitespace run preceding the keyword or paren, through that token
  item: begins immediately after
  why_this_way: keeping leading whitespace with the opener leaves no double space when the group survives and no missing space when it vanishes; the reverse split fails both ways
  evidence: in generate_test.go the space before an if block is the only thing separating ')' from 'AND'
emitter_obligation:
  invariant: Item precedes every fragment that writes a token or binds a value
  covers: a filling text chunk, Builder.Arg, AppendValues, and the paren a RelationNode writes
  one_exception: a sql.predicate call whose body is not provably non-empty gets no Item, because the callee writes into the same Builder and calls Item where its own fragments emit; an Item at the call site would open a group the callee must leave shut
  always_emitted_even_with_no_local_group: a predicate fragment is embedded into a caller's group, so its body calls Item even when it opened none of its own; Item on an empty stack is a no-op, and this is what makes composition work
  hazard: Builder embeds strings.Builder, so WriteString is a promoted public method and nothing in the type enforces the invariant; an emitter arm added later that writes without calling Item breaks elision silently rather than failing
  mitigation: the obligation is on emitText and emitNodes in templates/sqlbind/generate.go, and every write path there is covered by the branch-combination matrix
planning_gate:
  rule: a body with nothing elidable in it is planned as nil and emitted exactly as before, so not one group call reaches its generated code
  why: such a body cannot leave an operator dangling, so the whole mechanism is unnecessary there
  what_it_buys: the byte-identity claim becomes structural for every non-conditional template rather than something a test has to establish
  where: containsElidable in templates/sqlbind/groups.go
exactness:
  between:
    rule: BETWEEN closes with an AND that is never a joiner
    mechanism: one bit of scanner state, opened by BETWEEN and closed by the next AND at that depth; this is grammar, not a heuristic
    case: 'WHERE x BETWEEN {lo} {if hi}AND {hi}{/if}' is already broken on the false branch and is reported rather than silently made worse
  on_conflict:
    rule: ON CONFLICT opens no group; only a bare ON does
    found_in_implementation: not named by the request
    why_it_matters: ON is the one boolean opener with a non-boolean homograph, so the ON frame would be closed by CONFLICT with nothing having filled it and the keyword would vanish with its own group, rendering CONFLICT (id) DO UPDATE with no ON
    mechanism: one token of lookahead, which is the same test fmtclause.go already applies to classify ON CONFLICT as absorbing
    general_hazard_it_names: a keyword treated as a group opener where it is not one is silently deleted rather than diagnosed, because withholding is invisible by design
  case_expression:
    rule: inside a CASE region an AND or OR is text and a parenthesis opens no group, so CASE is excluded by construction rather than diagnosed
    changed_from_the_draft: the drafted rule made a conditional boundary inside CASE an error
    why_exclusion_instead: CASE WHEN opens a boolean region that is neither a clause keyword nor a paren, so nothing ever pushes a frame there and no opener or joiner can be withheld; the region is therefore emitted exactly as today, and an error would refuse templates that work
    what_it_costs: a vanishing WHEN condition still leaves 'CASE WHEN THEN', unchanged and unimproved, which is the outcome the request said it would also accept
  clause_opened_inside_a_branch:
    rule: a group opened inside a branch closes at that branch's end, so both paths leave the same stack
    why_not_an_error: '{if a}WHERE x = {x}{/if}' is the pre-existing idiom for a wholly conditional clause and is legal for a SELECT today; closing the frame at the branch end keeps it working and renders it unchanged
  unbalanced_branch:
    rule: branches leaving different parenthesis nesting are a generation error, not a paired guess
    precedent: walkClause at templates/sqlbind/mutation.go:97-104 already treats them as not valid SQL on both paths
  unresolvable_is_an_error: where inference cannot resolve a construct it reports, per requirement:analysis-diagnostics, on the ground rule:sql-static-mutation-safety already states; BETWEEN split across a branch and an unbalanced branch paren are the two cases, and CASE is excluded rather than reported
placeholder_invariance:
  claim: numbering and Args are unchanged in every branch combination
  proof: the only elided text is an opener, a closer, and a joiner, none of which binds; a vanished group registered no item, and Item precedes anything that can bind, so it bound nothing
  keeps: the rule:sql-placeholder-emission invariant that appending the argument and writing its placeholder are one operation, at sqlbind/statement.go:115
mutation_safety:
  unrelaxed: rule:sql-static-mutation-safety keeps its full strength
  why: dropping an empty WHERE under an UPDATE or DELETE turns one false branch into a full-table mutation, which is what the proof exists to prevent
  two_changes_only:
    - proveClause runs against the group rather than against the keyword
    - a token recognized as a joiner counts as filling nothing
  consequence_both_ways: an empty clause vanishes for a SELECT, HAVING, subquery, and join ON only, which is what requirement:sql-template-v1 already says with 'omit when empty for SELECT'; a conditional-only mutation predicate still fails
no_runtime_reparse: nothing scans, trims, or rewrites the assembled string; a runtime scan cannot tell a dangling operator from one inside a literal without redoing sqlLexer on every call, and requirement:sql-template-v1 forbids a generated runtime check for a generation-time condition
acceptance:
  verified: 2026-08-17 in templates/sqlbind/groups_test.go, every case below passing
  - all sixteen combinations of the four-condition predicate the request opens with, asserting SQL text and Args together
  - a nested group whose every condition is false takes its parens and its joiner
  - all conditions false on a SELECT emits no WHERE
  - all three accepted operator positions render identically across all four branch combinations
  - 'IN ({names})' keeps its parens in every branch, because the paren is preceded by a word
  - a whole BETWEEN inside a condition renders unchanged; one split across a branch reports and names BETWEEN
  - a branch opening a paren it does not close reports
  - a sql.predicate that may emit nothing fills the caller's group only when it emits
  - HAVING and a join ON elide the same way as WHERE
  - 'DELETE FROM users WHERE {if flag}id = {id}{/if}' still fails the mutation proof, as do the paren-wrapped and two-condition forms of it
  - 'DELETE FROM users WHERE id = {id} {if flag}AND flag{/if}' still generates
  - the multi-line in-branch form docs/sqlbind.md teaches renders unchanged with the condition true and loses the operator with it when false
  - a body with no condition emits no OpenGroup, Joiner, Item, or CloseGroup call at all
not_done:
  comma_clauses_left_as_text: SELECT, RETURNING, FROM, WITH, WINDOW, USING, and PARTITION BY, per comma_groups.left_as_text
  insert_pairing: per comma_groups.insert_pairing_hazard
  case_regions: excluded rather than modelled, per exactness.case_expression
  whitespace_around_a_dropped_leading_operator: an operator leading a branch takes its own preceding whitespace but not the space that followed it, so that one position can leave a double space; it occurs only in a branch combination that renders invalid SQL today
related:
  - requirement:sql-conditional-predicate-composition
  - decision:sql-boundary-joiner-inference
  - rule:sql-static-mutation-safety
  - rule:sql-top-level-keyword-scan
  - rule:sql-placeholder-emission
  - rule:sql-template-layout
  - rule:template-format-fidelity
  - requirement:sql-template-v1
  - requirement:analysis-diagnostics
  - data:sql-statement
```
