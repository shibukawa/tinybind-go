---
id: api:configbind-env-files
type: api
title: LoadOptions EnvFiles
---
LoadOptions takes dotenv paths, LoadResult reports which were read, and two helpers decode the file-carrying place.

```yaml
option:
  field: 'EnvFiles []EnvFile'
  entry: 'EnvFile{Path string, Secret bool}'
  since: v0.5.31; v0.5.29 and v0.5.30 took []string, replaced without compatibility per decision:env-secret-source-shape
  doc: >
    dotenv files read in slice order and laid under Environ: a later file wins
    over an earlier one on the same name, and Environ, or os.Environ() when
    Environ is nil, wins over every file
  secret_flag: marks the entry's values secret by origin per rule:secret-origin-masking; reading and order are unaffected
  paths: used as given; no directory search, no token-derived names
  missing_file: skipped
  unreadable_present_file: load error
  rejected_line: 'load error, configbind: read env file %q line %d: %v'
result:
  field: 'EnvFiles []EnvFile'
  meaning: the entries actually read, in order and with their Secret flag, the way ConfigPath and FoundFile report the TOML
place:
  const: 'PlaceEnvFile Place = "file_env:"'
  form: PlaceEnvFile + file name
  helper: 'func EnvFileOf(place Place) (string, bool)'
  helper_meaning: the file a PlaceEnvFile place names, and whether place is one
helper:
  signature: 'func EnvVariable(key string) string'
  meaning: the environment variable that sets key in the registered definitions, or empty for env:"-" or a repeated-table element
  why: the key-to-variable table lives in Definition.FlagMetas through cliparser DefFromField and EnvName, and is otherwise unreachable
  status: shipped alongside, although the request rated it optional
unchanged:
  - Environ and every other LoadOptions field
  - api:configbind-provenance shape and ordering
  - ScaffoldEnv and ScaffoldTOML
semantics: rule:env-file-composition
parser: decision:env-file-parser
related:
  - requirement:env-file-input
  - api:configbind-env-secret-sources
  - api:configbind-bind
  - term:config-source
  - data:provenance-event
  - flow:config-load
  - system:configbind
```
