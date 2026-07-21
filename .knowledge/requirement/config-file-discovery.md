---
id: requirement:config-file-discovery
type: requirement
title: Config File Discovery
---
Discover one TOML config file from --config-path or OS config dirs using vendor name, tool name, and file name from the API.

```yaml
priority: must
intent: locate config.toml-style files without merging multi-location contents
inputs:
  - vendor_name: required via configbind discovery API
  - tool_name: application/tool identifier via API
  - file_name: config file name under the tool config directory
  - optional --config-path from CLI
resolution: decision:config-file-path-resolution
behavior:
  - prefer --config-path over any directory search
  - if --config-path is unreadable or missing, fail with error; no silent fallback
  - else search user config dir then system config dir via system:configdir
  - user path wins when both contain the file; only that file is read
  - system path used only when user path lacks the file
  - never merge keys from user file and system file in one load
  - configdir.New(vendor_name, tool_name) receives both API arguments
cli:
  - flag long name: config-path
  - form: '--config-path <path>'
  - registered by cliparser / configbind process flags
  - maps to explicit path, not a Bind overlay config key under a prefix
tinygo:
  - system:configdir must remain TinyGo-buildable on host targets
  - requirement:configbind-tinygo
acceptance:
  - with --config-path=/tmp/app.toml, that path is used even if user/system files exist
  - with --config-path set to a missing path, load returns an error and does not search configdir
  - without --config-path, user config file is chosen over system when both exist
  - without --config-path and only system file, system file is used
  - user and system files are not deep-merged
  - discovery API accepts vendor name, tool name, and file name
related:
  - decision:config-file-path-resolution
  - system:configdir
  - requirement:layered-config-load
  - requirement:cli-option-codegen
  - flow:config-load
  - system:configbind
```
