---
id: decision:websocket-message-typing
type: decision
title: Two Type Arguments, Variants By Discriminator Field
---
Type a socket by an inbound and an outbound type and let each carry its variants in a discriminator field, rather than one symmetric type or a library-owned dispatch over many types.

```yaml
status: implemented 2026-08-10
decided_by: recommendation accepted 2026-08-10; the owner asked what real protocols need rather than choosing a shape
two_axes_not_one:
  direction: whether inbound and outbound carry the same type
  variety: whether one direction carries several kinds of message
  finding: they are orthogonal, and conflating them is why the choice looked hard
direction_answer:
  chosen: "Socket[In, Out]"
  evidence: real protocols are asymmetric — chat sends send/typing/join and receives message/presence/error; a dashboard sends subscribe and receives only data; realtime LLM APIs declare client events and server events as two separate unions
  symmetric_case: echo, and little else
  rejected: "Socket[T], which forces one struct to carry both directions' fields and makes every unused field a question at every call site"
variety_answer:
  chosen: a discriminator field on the direction's own struct, read by the application
  example: |
    type ClientMsg struct {
        Type string `json:"type"`           // "start" | "message" | "end"
        Room string `json:"room,omitempty"`
        Text string `json:"text,omitempty"`
    }
    type ServerMsg struct {
        Type string `json:"type"`           // "ready" | "message" | "error"
        Text string `json:"text,omitempty"`
        Code string `json:"code,omitempty"`
    }
  house_style: concept:streaming already spells variants this way, as ChatEvent{Type:"delta"} and ChatEvent{Type:"done"}; a socket spelled differently would put two idioms in one application
  library_names_nothing: the discriminator's spelling — type, event, op — stays the application's, because the protocol is the application's
rejected_library_dispatch:
  shape: "OnMessage[T](func(T) error) registered per variant, with the runtime reading a discriminator and routing"
  why_rejected:
    - the library would have to fix the discriminator's field name and its position in the document, which is a protocol decision taken from the author
    - it needs a two-pass decode, or a partial parse, on every inbound message
    - it costs one generated codec per variant instead of one per direction
    - it is buildable on top of the chosen shape as an application switch; the chosen shape is not buildable on top of it
  reconsider_if: a downstream framework wants one canonical envelope, in which case it owns the convention rather than this library
admitted_cost:
  what: a variant's required fields are not required at compile time, and an absent field is indistinguishable from a zero value
  same_cost_as: concept:streaming, which has carried it since the stream shipped
  escape_when_variants_diverge:
    shape: an envelope with the payload left raw, decoded per variant on demand
    feasible: jsonbind already parses objects and hands back raw members, so the pieces exist
    status: not designed; recorded as the way out if a real protocol makes the flat struct untenable
codec_binding:
  in: needs a generated decoder only
  out: needs a generated encoder only
  asymmetry_is_new: a stream's one type argument needed an encoder alone; see requirement:websocket-codec-generation
related:
  - concept:typed-websocket
  - concept:streaming
  - api:socket-read-write
  - requirement:websocket-codec-generation
  - concept:standalone-json-codec
```
