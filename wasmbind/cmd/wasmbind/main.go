// Wasmbind generates browser WebAssembly bindings for annotated Go packages.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/tmc/misc/wasmbind/internal/generate"
	"github.com/tmc/misc/wasmbind/internal/schema"
)

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: wasmbind {check|generate|build} [-tags list] [-go-out dir -out dir] package")
		return 2
	}
	command := args[0]
	if command != "check" && command != "generate" && command != "build" {
		fmt.Fprintf(stderr, "unknown command %q\n", command)
		return 2
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	tags := fs.String("tags", "", "build tags")
	var goOut, out string
	if command != "check" {
		fs.StringVar(&goOut, "go-out", "", "generated main directory")
		fs.StringVar(&out, "out", "", "binding output directory")
	}
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 || (command != "check" && (goOut == "" || out == "")) {
		fmt.Fprintln(stderr, "expected one package and, for generation, -go-out and -out")
		return 2
	}
	api, err := schema.Load(ctx, fs.Arg(0), *tags)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if command == "check" {
		return 0
	}
	path, err := generate.Write(ctx, api, goOut, out, *tags, command == "build")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
