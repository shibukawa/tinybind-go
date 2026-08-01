---
id: api:generator-main
type: api
title: Configurable Generator CLI Main
---
Package generator exposes a composable command dispatcher and a generate command backed by api:generator-execution.

```yaml
status: required
public_shape:
  - "func GenerateCommand(options Options) Command"
  - "func NewCommandSet(commands ...Command) (CommandSet, error)"
  - "func (set CommandSet) Run(ctx context.Context, args []string, io CommandIO) int"
  - "func Main(set CommandSet)"
Command:
  fields: [Name, Summary, Run]
CommandIO:
  injected: [Stdin, Stdout, Stderr, WorkingDirectory, Environment]
main_behavior:
  - build CommandIO from process state
  - pass os.Args[1:] to CommandSet.Run
  - terminate with dispatcher exit code
generate_flags:
  - dir
  - out
  - name
  - openapi
  - openapi-name
  - templates-name
  - sql-context-api
  - check
  - generate-all
  - force
option_behavior:
  - normalize data:generator-options before package loading
  - use one normalized option for mapping generation, route parsing, checks, and OpenAPI
  - command-line flags may disable configured artifacts but cannot re-enable a disabled feature
  - print artifact paths on stdout whether the run generated or skipped, and report a skip separately
stdlib_command: "func main() { generator.Main(generator.MustCommandSet(generator.GenerateCommand(generator.DefaultOptions()))) }"
custom_command: |
  func main() {
      commands := generator.MustCommandSet(
          generator.GenerateCommand(frameworkGeneratorOptions()),
          framework.InitCommand(),
          framework.BuildCommand(),
          framework.WatchCommand(),
      )
      generator.Main(commands)
  }
related:
  - data:generator-options
  - rule:generation-input-hash
  - requirement:configurable-generator-discovery
  - rule:generator-feature-disable
  - flow:code-generation
  - api:generator-execution
  - requirement:extensible-generator-command
  - api:generator-call-registration
```
