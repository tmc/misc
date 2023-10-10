package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	templateDir := flags.String("templates", "", "path to custom templates")
	generator := newGenerator(*templateDir)
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	opts.Run(generator.Generate)
}
