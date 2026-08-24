---
id: decision:byte-slices-are-base64
type: decision
title: Byte Slices Are Base64
---
A byte sequence field is a base64 string in JSON and a byte string in CBOR, whether it was spelled []byte, []uint8 or [N]byte, because a blob is binary rather than a list of small numbers.

```yaml
decided: 2026-08-24, by the maintainer
status: implemented 2026-08-24
question: >
  requirement:byte-and-rune-field-kinds makes []byte reachable for the first
  time. The uint8 element kind would carry it as a JSON array of numbers, which
  is what a slice of any other admitted width already does. That is a wire
  format, so it is a decision rather than a consequence.
chosen:
  json: a base64 string, padded standard encoding
  cbor: a byte string, major type 2
  covers: '[]byte, []uint8 and [N]byte alike, since Go makes the first two one type and a fixed-length blob is the case base64 is most wanted for'
why:
  interoperability: encoding/json and encoding/json/v2 both write a byte slice as base64, so a client already speaks it and one written against another Go service needs no special case
  size: 'a 1 MiB blob is 1.33 MiB as base64 and roughly 4 MiB as a list of numbers; in CBOR the byte string is the payload plus a header'
  cbor_has_the_type: a byte string says binary on the wire, which an array of one-byte integers does not, and it costs one byte rather than one byte an element
  nothing_was_pinned: no model struct in the repo declared a byte slice when this was decided, so the change had no wire to break
alternatives:
  numbers:
    what: the array of numbers the uint8 element kind produces
    for_it: consistent with every other sized-integer slice requirement:sized-integer-field-kinds admits, and needs no runtime at all
    against_it: three times the bytes, and no client sending base64 can talk to it
  refuse_until_decided:
    what: a generation error on a byte sequence
    against_it: '[]uint8 is the same type and was already accepted, so refusing one spelling and not the other is incoherent'
what_it_costs:
  no_number_array_of_bytes: a fixed-length array of small numbers has to be declared at another width, since [N]uint8 now means a blob
  and_that_is_a_behaviour_change: '[]uint8 encoded as a number array before this; no field in the repo declared one, so nothing regenerates differently, but a downstream project that did would see its wire change'
  runtime_added: jsonbind AppendBase64, ParseBase64 and ParseBase64Into
  cbor_needed_nothing: the driver already has AppendBytes and Reader.ReadBytes
divergences_from_encoding_json:
  fixed_length: 'encoding/json applies its base64 rule to a slice only and writes a [N]byte as a list of numbers, refusing to read a string back into one; this codec treats both spellings alike, which is the point of the decision'
  nil: 'encoding/json writes null for a nil byte slice; this codec writes "", the same rule it already applies to a nil slice and a nil map'
  both_are_pinned_by_a_test: generator/byte_field_test.go compares member for member against encoding/json, so a divergence has to be chosen rather than drifted into
as_built:
  kind: 'generator/plan.go KindBytes, one kind rather than an element kind because the wire form has nothing to do with the uint8 it is made of; ArrayLen tells the two spellings apart as it does for KindArray'
  fixed_length_contract: 'the same two ends requirement:fixed-length-array-fields set: a short blob zeroes the tail, a long one is an error naming the field'
  cbor_copies_what_it_reads: Reader.ReadBytes borrows the body buffer, so a field keeping the slice would alias input the caller may reuse; the copy is what the JSON half pays anyway, since a base64 decode allocates its result
  openapi: 'string with format byte, which is the OpenAPI spelling of base64 and better than the untyped string the default arm gave'
  the_span_is_not_copied_first: decoding reads the encoded payload straight out of the parser buffer, so a large blob costs the one allocation its decoded form needs
value_sources_carry_it_too:
  decided: 2026-08-24, by the maintainer, immediately after the wire form
  what: 'a byte field tagged query, path, header or cookie binds from that value as base64'
  why_it_is_not_body_only: a byte sequence is the one composite with a spelling outside a document, so the reason every other composite is body-only does not reach it
  untagged_stays_body_only: 'a blob''s home is the body, and reading one off a URL is asked for by name; that also keeps the query-then-body precedence machinery out of it'
  method_is_still_refused: an HTTP method has no value to carry
  both_alphabets_accepted:
    what: standard and URL-safe, padded or not
    why: 'a + in a standard-alphabet query value arrives as a space unless the client percent-encoded it, and a client putting a blob in a URL will have reached for the URL-safe alphabet; refusing either would mean the same blob binds from a body and 400s from a query'
    a_raw_plus_is_still_an_error: a space is not base64 in either alphabet, so it is a loud 400 rather than a blob quietly missing a byte
    asymmetric_on_purpose: what is accepted widens, what is produced does not -- a byte field is written as padded standard base64 wherever it is written
  fixed_length_is_the_same_contract: counted in bytes, so a short value zeroes the tail and one byte too many is a 400
  runtime: 'internal/bindcore ParseBytes, forwarded by httpbind and fasthttpbind; the logic is shared rather than duplicated because it is longer than the one-line parsers beside it, and decision:fasthttpbind-runtime-package requires the pair either way'
  parity_is_tested_directly: fasthttpbind/parity_test.go compares the two forwarders answer for answer
  openapi: a byte parameter is documented string/format byte, which is what the binder accepts

related:
  - requirement:byte-and-rune-field-kinds
  - requirement:fixed-length-array-fields
  - requirement:sized-integer-field-kinds
  - decision:cbor-shape-is-the-only-axis
  - decision:fasthttpbind-runtime-package
  - rule:openapi-query-fields
  - concept:standalone-json-codec
open_questions:
  - whether a multipart form field should carry a byte sequence, which is the one binder source left out
```
