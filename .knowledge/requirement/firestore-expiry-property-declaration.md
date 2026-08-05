---
id: requirement:firestore-expiry-property-declaration
type: requirement
title: Declare Which Property A TTL Policy Targets
---
A ttl tag option that applies nothing and encodes nothing, emitting one generated fact so a framework can publish which property each kind's expiry policy points at instead of hand-maintaining that list beside the types.

```yaml
status: implemented
proposed: 2026-08-05
implemented: 2026-08-05, in generator/firestorebind.go, generator/firestorebind_emit.go and firestorebind/firestorebind.go
approved_by: the maintainer, 2026-08-05, which is the decision the requirement was gated on
source: decision:firestore-framework-requests, the one ask needing a decision rather than an implementation
reverses: rule:firestore-tag-options what_has_no_counterpart_here ttl, which concluded that no tag is needed
tag: "`firestore:\"expires_at,ttl\"`"
generated: "func (r T) ExpiryProperty() (string, bool)"
what_it_does_not_do:
  write_path: nothing; the property is written as an ordinary timestamp, exactly as today
  encoding: nothing is implied, in deliberate contrast to requirement:dynamo-ttl-attribute where ttl implies the epoch-second encoding DynamoDB expires on
  apply: nothing; a TTL policy stays gcloud firestore fields ttls update, per system:tinygodriver-firestore ttl
  index: nothing is excluded or added
the_argument_the_recorded_position_missed:
  what_was_answered: TTL is not expressible on this wire, and a plain timestamp property is all a policy consumes, so nothing has to be produced
  still_true: every clause of it
  the_third_duty: a framework declares, applies, and publishes; the recorded position weighed declaring against applying and did not reach publishing
  what_publishing_is: telling a deployment, for every kind it owns, which property to point the policy at
  the_drift: that list is hand-maintained beside the types today, so renaming the property leaves the guide and the generated codec disagreeing with no compile error and no run-time error - a policy aimed at a property that no longer exists, and records that never expire
  why_this_module: silent drift between a tag and a name written elsewhere is the failure requirement:firestorebind-product-goals exists to remove, and this is that failure with the second copy outside the repository
why_the_DynamoDB_answer_does_not_transfer:
  there: requirement:dynamo-ttl-attribute is blocked on applying a setting, which needs a driver call that does not exist
  here: nothing is being applied, so nothing is blocked; the tag produces a string
generation_checks:
  - at most one ttl field per type
  - the field is a timestamp type, since a policy over anything else expires nothing
  - ttl on a field that is not stored, which is a contradiction rather than a no-op, matching the existing noindex check
  - the accessor name colliding with a method the type already declares, as every other generated method is checked
  source: each is the shape rule:firestore-tag-options already applies to another option
unresolved:
  ttl_with_noindex:
    question: a TTL policy reads the property from an index, so a ttl field also marked noindex may never expire
    what_is_not_known_here: whether Datastore mode requires the property indexed, which neither catalog records
    position: do not guess; a generation error on an unconfirmed rule refuses a working declaration, and a silent pass ships a policy that never fires
    action: confirm against the service or upstream, then make it an error if confirmed and leave it alone if not
  usage_direction:
    question: whether the accessor is emitted from the tag alone or only when something calls it
    position: from the tag, for the reason the version accessor is - the caller is a publishing tool outside the package, which rule:usage-directed-generation cannot discover
alternative_if_the_tag_is_declined:
  what: the same ExpiryProperty accessor from some other declaration, such as a clause in the .tb.firestore file
  serves_equally: the consumer says so
  why_the_tag_is_preferred: it puts the fact on the field it describes, which is where every other property fact lives
decided:
  question: whether publishing is a duty this module takes on, given that it neither applies nor encodes anything
  answer: yes, 2026-08-05, by the maintainer; the tag shipped rather than the alternative below
  what_it_cost: one option, four checks, one accessor, one runtime interface, and the rule clause that no longer says no tag is needed
as_built:
  interface: "firestorebind.Expirer, satisfied only by a type carrying the tag, so the assertion succeeding is the declaration"
  boolean: always true for a generated type, kept because the consumer asked for this signature and a caller reaching it through the interface should not have to know that
  emission: from the tag, alongside the version accessor and for the same reason - the caller is a tool outside the package
  proof_it_changes_no_bytes: internal/firestorefixture asserts the stored Session carries expires_at as an ordinary timestampValue and that no "ttl" appears on the wire
acceptance:
  - a tagged field emits ExpiryProperty returning the property name and true
  - a type with no ttl field emits nothing, and the accessor is absent rather than returning false
  - the encoded entity is byte-identical to what the same struct produces without the tag
  - a second ttl field, a non-timestamp field, or a skipped field is a generation error naming the struct and the field
related:
  - rule:firestore-tag-options
  - requirement:firestorebind-generated-entity-codec
  - requirement:dynamo-ttl-attribute
  - system:tinygodriver-firestore
  - decision:firestore-framework-requests
  - requirement:firestorebind-product-goals
```
