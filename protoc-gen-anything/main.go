package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	templateDir := flags.String("templates", "", "path to custom templates")
	verboseMode := flags.Bool("verbose", false, "enable verbose mode")
	continueOnError := flags.Bool("continue_on_error", false, "continue on error")
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	opts.Run(func(p *protogen.Plugin) error {
		return NewGenerator(Options{
			TemplateDir:     *templateDir,
			Verbose:         *verboseMode,
			ContinueOnError: *continueOnError,
		}).Generate(p)
	})
}