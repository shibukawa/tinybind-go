---
id: decision:firestorebind-runtime-package
type: decision
title: firestorebind Runtime Package And Its Name
---
Add a Firestore Datastore binding runtime as its own package named after the product rather than after the driver package, and never let the driver import tinybind-go.

```yaml
status: implemented
proposed: 2026-08-03
implemented: 2026-08-04; runtime in firestorebind/, generator mode in generator/firestorebind*.go, fixture in internal/firestorefixture
extends: decision:runtime-package-boundaries
package:
  name: firestorebind
  path: github.com/shibukawa/tinybind-go/firestorebind
name_choice:
  house_rule_would_say: datastorebind, since dynamobind is named after nosql/dynamodb and sqlbind after database/sql
  chosen_instead: firestorebind
  why: "datastore" is a common noun and names no product; a reader seeing datastorebind in an import list learns nothing about which service it reaches, while firestorebind does
  cost: the one place in this catalog where the binding name and the driver package name differ, so system:tinygodriver-firestore records the pairing explicitly
  what_it_does_not_mean: Firestore native mode is out of scope; the name is the product, and requirement:firestorebind-product-goals states the mode
  tag_follows_the_package: the tag is firestore, not datastore, so a reader who found the package by name finds the tag by the same word, per rule:firestore-tag-options
owns:
  - api:firestorebind-operations
  - the codec interfaces EntityEncoder, EntityDecoder, Keyer, Kinder and Versioner
  - Kinder and Versioner were not in the first draft: a kind has to be reachable from a value for a key builder to use it, and a version has to be reachable for a write to become conditional, and an interface assertion is not a call the generator can discover
  - the Context client and namespace resolution of decision:firestore-context-client-api
  - the typed transaction wrapper of decision:firestore-transaction-scope
  - decode field errors, reusing the jsonbind FieldError shape where it fits, as dynamobind does
imports:
  - github.com/shibukawa/tinygodriver/nosql/datastore
driver_version:
  minimum: v1.1.5, not v1.1.4 which introduced nosql/datastore
  why_the_later_one: v1.1.5 exports the service limits api:firestorebind-operations chunks against and the Index type decision:firestore-no-schema-artifact emits, and neither has a local substitute worth writing
  effect: a second external driver dependency enters the module, and the module requirement rises from the v1.1.3 minimum decision:dynamobind-runtime-package set for nosql/dynamodb
  one_module: both packages live in tinygodriver, so this raises one requirement rather than adding a second
  side_effect_for_dynamobind: v1.1.5 also exports the DynamoDB batch limits, which api:dynamobind-operations currently declares itself; raising the minimum makes that duplication removable
excludes:
  - net/http beyond what the driver itself pulls in
  - database/sql
dependency_direction:
  - user package -> firestorebind -> tinygodriver/nosql/datastore
  - user package -> tinygodriver/nosql/datastore, because generated code names driver types
forbidden:
  - tinygodriver importing tinybind-go, in any package, example or test
  - a firestorebind example living in tinygodriver
  - firestorebind importing dynamobind, or the reverse; the two share a shape and no code
what_the_two_bindings_share:
  today: nothing but the pattern, since Item and Entity are unrelated types with unrelated codecs
  tempting_and_declined: a common generic Load and Store over an abstract backend, which would have to erase the key model difference decision:firestore-key-identity exists to keep visible
  actually_shared: the generator's discovery, usage direction and diagnostics machinery, which is not runtime code
generated_code_placement:
  location: the user package
  may_import: both firestorebind and the driver
  declares: only the methods of its own declared types, per decision:generated-runtime-in-module
size_expectation:
  unmeasured: no counterpart to requirement:dynamobind-verification exists yet
  prediction_to_test: the Context client costs about what decision:dynamo-context-client-api measured, since it is the same context.WithValue and type assertion
  what_differs: there is no reflection mapper to compare against, so the honest baseline is the same codec written by hand
related:
  - requirement:firestorebind-product-goals
  - system:tinygodriver-firestore
  - decision:dynamobind-runtime-package
  - decision:runtime-package-boundaries
  - requirement:tinygo-wasm
  - system:tinybind
```
