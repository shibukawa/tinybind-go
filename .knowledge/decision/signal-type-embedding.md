---
id: decision:signal-type-embedding
type: decision
title: A Signal Is Recognized By An Embedded Module Type
---
Let an application define its own signal types by embedding a module-provided struct, and detect one through the unexported method that embedding promotes, so recognition is a plain type assertion and a forged signal is not expressible.

```yaml
source:
  - concept:signal-channel
  - decision:signal-in-the-error-slot
  - user design discussion 2026-08-11
review_gate: proposed
status: shipped 2026-08-11 in htmlbind/signal.go, with the payload deviation below
statement: the module exports one Signal struct; an application type that embeds it is a signal, and nothing else can be
as_built:
  confirmed: the promoted unexported accessor identifies a signal through a plain type assertion, the promoted Error makes an embedding type an error with no boilerplate, and the private field catches an embed that was never constructed
  reflect_free: verified by the htmlbind suite passing under TinyGo, which is where a linked reflect would show
  deviation_payload_encoding:
    designed: a constructor generic over the payload type, resolving the jsonbind codec at the emit call site
    built: a SignalPayload interface with one AppendJSON method, which the payload implements
    why: htmlbind imports stdlib alone, and reaching jsonbind for the generic form would put the codec registry in every htmlbind binary, which is a TinyGo size cost paid by every project whether or not it emits a signal
    property_kept: the encoding still happens at the emit site with the concrete type in hand, so nothing at the seam reflects and the value is immutable once yielded, which is what the generic form was for
    idiom: AppendJSON is what Content, the update records, and the generated encoders already use, so this is the repo's existing shape rather than a new one
    cost: a payload type writes or generates one method, where the generic form would have found a registered codec
  escape_hatch: NewRawSignal takes bytes that are already JSON, for a payload the application has encoded elsewhere
  fault_handling: a construction fault travels with the value and is turned into a plain error at the pump, per requirement:live-signal-emission found_while_building
module_side:
  type: an exported struct carrying an unexported name, unexported encoded payload bytes, and an unexported initialized marker
  sealed_accessor: an unexported method with a value receiver, returning the embedded value
  error_method: Error() on the same type, so embedding alone makes an application type satisfy error with no boilerplate, which is what lets it ride the slot decision:signal-in-the-error-slot chose
  constructors: a generic one taking a name and a typed payload, and a bare one taking a name alone
detection:
  interface: an unexported single-method interface the module declares and asserts against
  why_it_seals: an unexported method name is qualified by its defining package, so only a type embedding this struct can satisfy the interface; an application cannot declare a method with that name that counts
  reflect_free: an ordinary type assertion, walked through Unwrap the way htmlbind/async.go findPublicError already walks for PublicError
  no_registration: nothing has to be registered, listed, or generated for a new signal type to be recognized
the_private_field_is_not_what_is_read:
  correction: a private field cannot be read from outside its package without reflect, so it is not the thing the module looks at
  what_reads: the promoted unexported method, which is the only cross-package way to ask "was this type built from ours"
  what_the_field_is_for: telling an initialized embed from a zero one, since an application writing `Toast{}` embeds a zero Signal, satisfies the interface, and carries no name
  rule: a signal whose initialized marker is unset is rejected at emit as a construction error, not forwarded as an unnamed signal
  reading: the field seals construction and the method seals identification; both are wanted and they do different jobs
receivers:
  chosen: value receiver on the accessor, and the application embeds by value
  why: a value receiver puts the method in the method set of both the application type and its pointer, so a source may yield either
  pointer_embed_rejected: embedding the pointer form makes a zero application value carry a nil Signal, so the accessor panics where the value form merely reports uninitialized
payload_shape:
  chosen: the application type wraps a signal the module constructed, and the payload is a separate typed value handed to the constructor
  example: |
    type Toast struct{ htmlbind.Signal }
    func NewToast(text string) Toast {
        return Toast{htmlbind.NewSignal("app.toast", toastPayload{Text: text})}
    }
  why: the generic constructor resolves the jsonbind codec at its own call site, where the concrete type is still known, which is the one place a codec can be found without reflect
  gains: a named Go type per signal, so an emit site is type-checked and the payload shape is documented by a struct rather than by a name
  rejected_alternative:
    shape: let the application struct itself be the payload, with the embedded Signal skipped during encoding
    reads_better: one struct rather than two
    why_not: the module holds the value as an interface at the seam, so its concrete type is erased and its generated codec cannot be looked up without reflect
    and: jsonbind's generator would have to learn to skip an embedded module type, which is a generator change the wrapper shape does not need
    reconsider_if: a signal payload turns out to want its own methods, at which point the generator rule is worth the cost
what_the_application_writes:
  most_of_it: its own signal types, its constructors, its payload structs, and every client handler
  none_of_it: classification, forwarding, ordering, or framing
  boilerplate: one embed and one constructor per signal type
constraints:
  - the module never inspects an application field, so a signal type may hold anything beside the embed
  - a signal value is immutable after construction, because the payload is encoded there
  - the accessor is unexported and stays unexported; exporting it would let an unrelated type claim to be a signal
related:
  - data:signal
  - requirement:live-signal-emission
open_questions:
  - whether the bare constructor is worth having, or a payload-free signal should pass an empty struct so there is one constructor
  - whether an application may embed the type in a struct it also uses for something else, and whether the module should care
```
