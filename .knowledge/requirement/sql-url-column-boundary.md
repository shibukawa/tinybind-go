---
id: requirement:sql-url-column-boundary
type: requirement
title: URL Column Boundary Conversion
---
A url template field crosses the database boundary as text in both directions, because database/sql accepts net/url.URL at neither end.

```yaml
source:
  - requirement:sql-generated-api-layers
  - user design discussion 2026-07-30
defect:
  symptom: any statement with a url parameter or a url result column failed at runtime on every driver and every dialect
  bind_cause: driver.DefaultParameterConverter rejects a struct; it reported 'unsupported type url.URL, a struct'
  scan_cause: Rows.Scan cannot assign a text column into a struct that is not a sql.Scanner
  reach: unrelated to data:sql-dialect; the type mapping in the generator names url.URL and nothing converted it
bind:
  location: sqlbind.Builder.Arg, so one conversion covers parameters, optional parameters, and expanded value lists
  url.URL: bound as its String form
  '*url.URL': nil binds SQL NULL; otherwise the String form
scan:
  reason: Scan receives an address and cannot be handed a conversion, so the target itself must convert
  required: sqlbind.ScanURL(*url.URL); NULL is an error, matching a required string column
  optional: sqlbind.ScanOptionalURL(**url.URL); NULL leaves a nil pointer
  driver_text: string and []byte both accepted, because drivers disagree on which one a text column yields
  parse: net/url.Parse, the inverse of the String form used to bind
emission:
  rule: the generated scan list passes the adapter in place of the field address for a url field only
  unchanged: the generated struct field stays url.URL, so calling code is unaffected
related_driver_dependency:
  datetime: a MySQL DSN needs parseTime=true or the driver returns bytes and the scan fails
  treatment: documentation, per rule:sql-dialect-syntax-rejection driver_layer; no generated conversion
acceptance:
  - a url parameter reaches the driver as its text form
  - a nil optional url parameter binds NULL
  - a url column scans back into an equal url.URL through database/sql
  - an optional url column scans nil for NULL
  - a required url column reports an error for NULL and for a non-text value
  - scanning a text column into a bare url.URL still fails, proving the adapter is what makes it work
```
