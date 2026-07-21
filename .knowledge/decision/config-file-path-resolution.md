---
id: decision:config-file-path-resolution
type: decision
title: Config File Path Resolution
---
Resolve a single TOML config file path; never merge user and system files. CLI --config-path wins over directory search.

```yaml
status: accepted
lookup_inputs:
  - vendor_name: required API argument for configdir vendor segment
  - tool_name: application name API argument for configdir
  - file_name: config file basename e.g. config.toml
  - optional --config-path from CLI when present
api_sketch:
  - 'ResolveConfigPath(vendor, tool, fileName, configPathFlag) (path string, ok bool, err error)'
  - vendor and tool are passed by the app through configbind public API
exclusive_file:
  - at most one file path is chosen for TOML load
  - user and system files are never content-merged
priority_high_to_low:
  - id: cli_config_path
    source: cliparser flag --config-path
    meaning: explicit filesystem path to the config file
    wins_over: all directory search results
  - id: user_config_dir
    source: system:configdir user-level folder for vendor+tool
    meaning: first existing file_name under user config dir
  - id: system_config_dir
    source: system:configdir system-level folder for vendor+tool
    meaning: used only when user path does not contain file_name
search_helper: system:configdir
rules:
  - if --config-path is set, use that path only; skip directory search
  - if --config-path is set but missing or unreadable, return error; no fallback to configdir
  - without --config-path, QueryFolderContainsFile-style search user then system
  - if no file found without --config-path, TOML layer is absent; defaults/env/CLI still apply
  - --config-path is a process-level path flag, not a Bind field under a prefix table
  - vendor_name and tool_name are always supplied by the application API caller
related:
  - requirement:config-file-discovery
  - system:configdir
  - decision:cli-flag-naming
  - concept:cli-option-codegen
  - requirement:layered-config-load
  - flow:config-load
  - system:configbind
```
