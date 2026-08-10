---
id: requirement:websocket-codec-generation
type: requirement
title: Discovery Reaches Both Socket Type Arguments
---
Generation must read both type arguments of a socket entry and emit a decoder for the inbound one and an encoder for the outbound one, because a socket with no generated codec compiles and then fails at the first message.

```yaml
status: required for the first release, decided by the owner 2026-08-10
why_not_deferred: the failure is jsonbind's missing-decoder error at runtime, on a connection that already returned 101, so the caller sees a socket that opens and then dies
precedent: concept:streaming, whose element type discovery decision:stream-callback-shape describes
what_is_new:
  two_arguments: "Socket[In, Out] needs index 0 decoded and index 1 encoded; a stream's one argument needed encoding alone"
  opposite_directions: the two roles are not the same operation applied twice
shape:
  chosen: two call patterns against the same target function, one per direction
  sketch: |
    SocketReceiveCall(Function(path, "WebSocket"), GenericType("socket-in", 0))
    SocketSendCall(Function(path, "WebSocket"), GenericType("socket-out", 1))
  why_this_one: each pattern still carries one role, so the one-role-per-operation model is entered rather than widened
verified_by_spike_2026_08_10:
  method: a throwaway fixture holding a local two-type-parameter generic called with neither argument spelled, parsed through ParsePackageWithConfig under five configurations; fixture and probe deleted after
  index_1_reads:
    result: works
    evidence: a lone pattern with TypeArgument 1 recorded ServerMsg, where TypeArgument 0 recorded ClientMsg, from a call site spelling no type arguments at all
    conclusion: "instantiatedTypeArgumentName is index-generic already; inference from the closure parameter reaches both arguments"
  two_patterns_per_target:
    result: does not work, and fails silently
    evidence: "the pair of patterns yielded only the first one's type; swapping their order swapped which type survived"
    isolated: two patterns with distinct operations writing distinct fields — Bind into request, StreamCreate into stream — behaved the same way in both orders, so the cause is the match loop rather than a first-wins field guard
    cause: "configuredCall returns the first pattern whose target matches and stops"
    severity: no error and no diagnostic; the second direction is simply never discovered
  role_model_needs_no_widening:
    finding: "toParserCallPattern already lowers TypeRoles[role].GenericArgument into the parser's single TypeArgument, so a generator pattern per direction becomes a parser pattern per direction with no change"
    consequence: the earlier fallback — widening a call pattern to carry several roles, touching every primaryTypeSource consumer — is not needed and is dropped
found_while_implementing_2026_08_10:
  one_target_one_pattern_was_enforced:
    what: "CallRegistry.Register refuses two patterns for one target unless they are deeply equal, reporting conflicting call patterns"
    why_it_exists: a second pattern on one target is normally a framework claiming a meaning the runtime already claimed
    resolution: "an explicit exception naming both socket operations, rather than relaxing the guard to any two operations, which would let the framework case back in"
    not_predicted: the spike probed the parser, and this guard is on the generator side, so it surfaced only once the patterns were registered
  feature_disable_reaches_further_than_the_socket_flag:
    what: "socket-receive is gated on FeatureDecodeJSON as well as FeatureWebSocket, and socket-send on FeatureEncodeJSON"
    why: "enabled usage is computed from the configured patterns rather than from discovered calls, so a socket pattern gated on its own flag alone leaves decoder emission enabled for every package, breaking disable decode-json everywhere"
    caught_by: TestDisabledPatternCannotBeReenabledByGenerateAll, which asserts the invariant directly
    latent_elsewhere: "OperationStreamCreate yields UsageEncodeJSON while gating on FeatureStreaming alone, so the same hole exists for encode; untouched here because widening it is a separate change"
required_changes:
  parser:
    - "configuredCall yields every matching pattern instead of the first; it has two call sites, parse.go route-registration detection and body.go analyzeBody"
    - "the route-registration site becomes any match is a registration, which is also more correct than today for a target carrying two patterns"
    - "new operations for the two directions, with their own bodyInfo fields; the existing Request, Response and Stream fields are single-valued and first-wins"
  generator:
    - "operations, requiredCallRoles entries for socket-in and socket-out, toParserCallPattern cases, and the primaryTypeSource role list"
    - "FeatureWebSocket as the flag name; a socket is neither FeatureStreaming nor a JSON codec feature, and rule:generator-feature-disable makes the flag a public surface rather than an implementation detail"
  size: contained; no model in either package has to change shape
two_lists_must_both_learn_the_name:
  fact: "the parser keeps its own DefaultConfig of runtime call names, separate from the generator's call patterns"
  history: decision:stream-callback-shape records both having to learn WriteStream, and that a name added to only one is silently undiscovered
  requirement: the socket entry is registered in both, and a test proves discovery from a call spelling no explicit type arguments
  sites_2026_08_10:
    - "parser/symbols.go, the map that gives WriteStream its CallStreamCreate operation"
    - "generator/options.go, the StreamCreateCall pattern and the name list beside Write and WriteStatus"
acceptance:
  status: implemented 2026-08-10
  met:
    - a handler calling the socket entry with types spelled only in the closure parameter yields both codecs
    - each direction gets one codec and not the other, so a socket adds no dead decoder to a TinyGo binary
    - disabling the socket feature drops both halves of the pair
  evidence:
    parser: testdata/socket_websocket, whose golden records socket_in ClientMsg and socket_out ServerMsg from a call spelling neither
    generator: TestSocketTypesGetOneCodecEachInTheRightDirection and TestDisablingTheWebSocketFeatureDropsBothDirections
  not_met: a missing codec is still a runtime error rather than a generation-time diagnostic, which is the stream's behaviour too and is left as it stands
related:
  - decision:websocket-message-typing
  - decision:stream-callback-shape
  - concept:code-generation
  - concept:standalone-json-codec
  - rule:generated-source-not-discovered
```
