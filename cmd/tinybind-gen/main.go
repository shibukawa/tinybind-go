package main

import "github.com/shibukawa/tinybind-go/generator"

func main() {
	options := generator.DefaultOptions()
	generator.Main(generator.MustCommandSet(
		generator.GenerateCommand(options),
		generator.FormatCommand(options),
	))
}
