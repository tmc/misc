package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	templateDir := flags.String("templates", "", "path to custom templates")
	defaultExposeQueries := flags.Bool("default_expose_queries", false, "expose all query fields by default")
	defaultExposeMutations := flags.Bool("default_expose_mutations", false, "expose all mutation fields by default")
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	opts.Run(func(p *protogen.Plugin) error {
		generator := newGenerator(*templateDir, *defaultExposeQueries, *defaultExposeMutations)
		return generator.Generate(p)
	})
}
