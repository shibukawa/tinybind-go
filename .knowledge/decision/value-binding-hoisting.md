---
id: decision:value-binding-hoisting
type: decision
title: A Value Binding Is Evaluated At The Top Of Its Block
---
Evaluate every value binding at the top of the block it is written in, and run a chain member's top-level bindings during assembly, so a loader that fails can still choose the response status.

```yaml
source:
  - requirement:template-value-binding failing_external
  - requirement:redirect-error
  - owner decision 2026-08-14
review_gate: approved and implemented 2026-08-14
problem:
  what_works_already: a failing external's error reaches the caller unwrapped, so an error value carrying HTTP intent — a redirect target, a not-found, a forbidden — is recognizable by api:write-error exactly as requirement:redirect-error already defines for a rung 2 page function
  verified: no wrapping anywhere in the render path, so errors.As reaches the value
  what_does_not: position; a binding runs at its instruction's place, and by then the chain's document shell has written bytes, so the status is already committed
  consequence_without_this: the mechanism is expressive but only usable on a buffered render, which gives up streaming for the whole page to keep one status free
rule:
  evaluation: a value binding is evaluated at the top of its block, in written order among the bindings of that block
  block: an if branch, a for body, an await subtree, or the declaration body; markup nesting is not one, per requirement:template-value-binding what_a_block_is
  uniform: every block, so a for body evaluates at the top of each iteration and an if branch at the top of the branch; one sentence rather than a rule with an exception
  past_markup_nesting: a binding written inside a div is evaluated before the div's opening tag, since the div opens no block
  chain_members: the leaf's declaration-body bindings run during assemble, which validates every member before anything is written
  leaf_only:
    found: on implementation 2026-08-14
    why: a wrapper's parameters are not complete until the chain installs the child fragment, so a scope built during assembly would carry an unset slot and the layout would render around nothing
    principled_rather_than_arbitrary: the leaf's parameters are final at Bind and a wrapper's are not, which is the property that decides it
    consequence: a layout that loads its own data computes it where it runs, and its failure lands after the shell has written
  what_it_buys: any binding of a chain member that is not inside a control block can answer 404, 403, or a redirect while the rest of the page still streams, wherever in the markup it happens to be written
scope_does_not_hoist:
  rule: the name is visible from the directive onward, exactly as it is today; only the evaluation moves
  why: hoisting the name too is JavaScript's var wart, and reading a binding written later is a mistake worth keeping as an error
  implementable: a node preceding the binding references only outer names, and the generated scope struct resolves those through Outer, so the lowering carries it with no special case
hoisting_needs_no_exception:
  hazard_that_existed: while a binding could shadow, one hoisted above a read of the same name from an enclosing block would change which value that read sees, which is an output change rather than an ordering one
  not_expressible_either_way: the reading node would need Outer.A while the following one needs A, so one instruction list would carry two scopes, which is the property that decided decision:value-binding-form in the first place
  how_it_was_closed: requirement:template-value-binding shadowing now refuses a binding whose name is already visible, so no read's meaning can change and the hoist needs no stop rule
  order_of_reasoning: the exception was found first and the refusal second, which is why the refusal is recorded as bought rather than as tidiness
why_reordering_is_safe_otherwise:
  external_purity: rule:render-external-query-semantics forbids externally visible mutation, so evaluation order among bindings, and between a binding and the output around it, is unobservable
  output_identical: only which failure surfaces first, and when relative to the bytes written, can differ — which is the point of the change
  not_the_call_collapsing_objection:
    that_one: requirement:template-value-binding refused an optimizer deciding the call count, because authors would rely on a number the rule leaves open
    this_one: there is no order to rely on, because the rule that makes a binding cheap is the same rule that makes its position unobservable
    checked_rather_than_assumed: the two look alike and are not, which is why the objection was raised and then withdrawn
no_diagnostic:
  decided: 2026-08-14 by the owner
  why: it follows from the rule rather than being a second choice — with hoisting there is no wrong place to write a binding, so there is nothing for a diagnostic to say
  what_it_would_have_been_under_the_alternative: a leading-only rule needs a way to tell an author their binding cannot set the status, which is a diagnostic the author then has to act on
rejected:
  leading_only:
    what: no hoisting; a binding sets the status only when written before any output in its component's top-level block
    argued_for: position stays in the source, matching how requirement:template-value-binding keeps the call count in the source
    why_not: the analogy does not hold, per why_reordering_is_safe_otherwise; and it constrains where a loader may be written for a property the author cannot see in the source either way
not_a_copy_of_check:
  check_is_pure: Plan.Check is a predicate over the parameters, deliberately run twice — at assembly and again before the component's first byte — because re-running a pure predicate is cheaper than tracking which values were already seen
  a_loader_is_not: running it twice is a duplicate fetch, which is the cost requirement:template-value-binding exists to remove
  therefore: the value is computed once and carried, a different mechanism from Check rather than another user of it
lowering:
  today: normalization splits a block at the binding's position and nests the rest into its body
  under_this_rule: collect the block's bindings in written order, nest them at the front, and put everything else as the innermost body, which is simpler than splitting at a position
  order_preserved: the non-binding nodes keep their written order inside the innermost body, so output is byte-identical
  the_shape_is_already_right: a component whose block holds one binding compiles to leading static whitespace plus one instruction wrapping the rest, and hoisting is running that instruction's value early and its body at render
carry_mechanism:
  chosen: the prologue returns a prepared instruction list, and assembly swaps the fragment's render for one that executes it
  rejected: a slot on the Fragment written by the pre-pass and read by the render, which was this file's first design
  why_rejected: one Fragment rendered twice would share the slot, and the value belongs to a render rather than to a binding
  prepared_op: a value binding whose value is computed becomes an instruction holding the built scope, so nothing about it is left to do when it runs
  leading_only_in_practice: preparation walks the leading instructions and stops at the first that is not preparable, stepping over static output, because anything after that point runs once something is written and has nothing to gain
  not_prepared_is_safe: a fragment that is not a chain member has no prologue run, and its bindings compute where they stand
scope:
  synchronous_only: an await binding opens a boundary by definition, and its failure is what a recover subtree is for; nothing about it wants hoisting
  chain_members_for_the_status: a leaf's declaration-body bindings run before the first byte, wherever in the markup they are written; one inside an if, for, or await block runs when that block does
  not_a_storing_cached_component:
    found: on review 2026-08-14, after the prologue shipped
    bug: the prologue runs during assembly and the store is consulted during the render, so a cached leaf's loader ran on every request and its value was discarded on a hit — the one thing requirement:component-output-cache exists to stop
    rule: a plan carrying a storing cache policy is not prepared
    what_it_gives_up: the status choice on a miss, and only there; on a hit the stored bytes are the answer and there is no failure to report
    why_that_trade: paying a fetch per hit to keep a choice half the requests cannot use is the wrong way round
consequence:
  - a page whose loader fails can answer 404, 403, or a redirect while still streaming everything else
  - requirement:redirect-error widens from a rung 2 return value to any chain member's top-level binding, with its value and its api:write-error behaviour unchanged
  - the unread-binding scan gains the hoist rule's boundary, since a node preceding the binding is inside its body after normalization but cannot read it
as_built:
  when: 2026-08-14
  scope_rule: the compiler records each visible binding's source position and refuses a read that precedes it, checked on the identifier rather than on the node, because an element opening before the binding holds children that come after it
  prologue: prepareOps walks the leading instructions, steps over static output, and stops at the first that cannot be prepared; a prepared binding holds its built scope
  per_entry_context: every render entry passes its own render's context, which the collecting entry did not until review caught it assembling with a background one
  found_on_review:
    cached_components_excluded: a storing cache is not prepared, per not_a_storing_cached_component; the prologue runs during assembly and the store is consulted during the render, so hoisting made a cached loader fetch on every hit
    wrapper_excluded: leaf_only above, discovered while wiring assembly rather than while designing it
  the_shape_held: the lowering already produced leading static plus one instruction wrapping the rest, exactly as this file predicted, so hoisting is running that instruction's value early and its body at render
open_questions:
  - whether a failure after the first byte should be distinguishable to the caller from one before it, so a framework can tell a status it could have set from one it could not
```
