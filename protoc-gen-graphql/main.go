package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	templateDir := flags.String("templates", "", "path to custom templates")
	defaultExpose := flags.Bool("default_expose", false, "expose all fields by default")
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	opts.Run(func(p *protogen.Plugin) error {
		generator := newGenerator(*templateDir, *defaultExpose)
		return generator.Generate(p)
	})
}
