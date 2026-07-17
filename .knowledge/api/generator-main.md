---
id: api:generator-main
type: api
title: Configurable Generator CLI Main
---
Package generator owns CLI parsing and execution so a command package only selects data:generator-options.

```yaml
status: required
entrypoint: "func Main(options Options)"
testable_core: "func Run(args []string, stdout io.Writer, stderr io.Writer, options Options) int"
main_behavior:
  - pass os.Args[1:], os.Stdout, and os.Stderr to Run
  - terminate with Run exit code
owned_flags:
  - dir
  - out
  - name
  - openapi
  - openapi-name
  - check
  - generate-all
option_behavior:
  - normalize data:generator-options before package loading
  - use one normalized option for mapping generation, route parsing, checks, and OpenAPI
  - command-line flags may disable configured artifacts but cannot re-enable a disabled feature
stdlib_command: "func main() { generator.Main(generator.DefaultOptions()) }"
custom_command: |
  func main() {
      generator.Main(generator.Options{
          ServeMuxes: generator.PatternSet[generator.TypePattern]{
              Set: []generator.TypePattern{
                  {PackagePath: "net/http", Name: "ServeMux"},
                  {PackagePath: "github.com/shibukawa/petitweb-go/handler", Name: "ServeMux"},
              },
          },
          RuntimePackages: generator.PatternSet[string]{
              Set: []string{
                  "github.com/shibukawa/httpbind-go",
                  "github.com/shibukawa/petitweb-go/handler",
              },
          },
          FileTypes: generator.PatternSet[generator.TypePattern]{
              Set: []generator.TypePattern{
                  {PackagePath: "github.com/shibukawa/httpbind-go", Name: "File"},
              },
          },
      })
  }
related:
  - data:generator-options
  - requirement:configurable-generator-discovery
  - rule:generator-feature-disable
  - flow:code-generation
```
