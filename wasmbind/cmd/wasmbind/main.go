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

	"github.com/tmc/misc/wasmbind/internal/docsite"
	"github.com/tmc/misc/wasmbind/internal/generate"
	"github.com/tmc/misc/wasmbind/internal/pack"
	"github.com/tmc/misc/wasmbind/internal/qualify"
	"github.com/tmc/misc/wasmbind/internal/schema"
)

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: wasmbind {check|generate|build|pack|docs|check-examples|adapters|qualify} [-tags list] [-go-out dir -out dir] package")
		return 2
	}
	command := args[0]
	if command == "qualify" {
		return qualifyCommand(ctx, args[1:], stdout, stderr)
	}
	if command == "adapters" {
		return adaptersCommand(args[1:], stdout, stderr)
	}
	if command == "docs" || command == "check-examples" {
		return docsCommand(ctx, command, args[1:], stdout, stderr)
	}
	if command == "pack" {
		return packCommand(ctx, args[1:], stdout, stderr)
	}
	if command != "check" && command != "generate" && command != "build" {
		fmt.Fprintf(stderr, "unknown command %q\n", command)
		return 2
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	tags := fs.String("tags", "", "build tags")
	features := fs.String("features", "", "enabled extensions")
	backend := fs.String("backend", "syscalljs", "binding backend")
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
	api, err := schema.LoadOptions(ctx, fs.Arg(0), *tags, *features, *backend)
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

func packCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opt pack.Options
	fs.StringVar(&opt.Bundle, "bundle", "", "completed build bundle")
	fs.StringVar(&opt.Out, "out", "", "package directory")
	fs.StringVar(&opt.Name, "name", "", "npm name")
	fs.StringVar(&opt.Version, "version", "", "package version")
	fs.StringVar(&opt.ReadmeFile, "readme-file", "", "optional package README")
	fs.StringVar(&opt.LicenseFile, "license-file", "", "explicit license file")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || opt.Bundle == "" || opt.Out == "" || opt.Name == "" || opt.Version == "" || opt.LicenseFile == "" {
		fmt.Fprintln(stderr, "pack requires -bundle -out -name -version -license-file")
		return 2
	}
	path, err := pack.Write(ctx, opt)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}

func docsCommand(ctx context.Context, command string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	bundle := fs.String("bundle", "", "generated bundle directory")
	out := fs.String("out", "", "documentation output directory")
	tsc := fs.String("tsc", "", "explicit TypeScript compiler executable")
	lib := fs.String("lib", "ES2022,DOM", "TypeScript library set")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	var err error
	if command == "docs" {
		if *bundle == "" || *out == "" || fs.NArg() != 0 {
			fmt.Fprintln(stderr, "docs requires -bundle and -out")
			return 2
		}
		err = docsite.Write(*bundle, *out)
	} else {
		err = docsite.CheckExamples(ctx, *tsc, *lib, fs.Args())
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "ok")
	return 0
}

func adaptersCommand(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("adapters", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "generated bindadapter package directory")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *out == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "adapters requires -out")
		return 2
	}
	path, err := generate.WriteAdapters(*out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}

func qualifyCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("qualify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o qualify.Options
	fs.StringVar(&o.Package, "package", "", "local npm package directory")
	fs.StringVar(&o.Out, "out", "", "new owned qualification directory")
	fs.StringVar(&o.TSC, "tsc", "", "local TypeScript executable")
	fs.StringVar(&o.NPM, "npm", "", "local npm executable")
	fs.StringVar(&o.Node, "node", "", "local Node executable")
	fs.StringVar(&o.Playwright, "playwright", "", "local Playwright ESM entrypoint")
	fs.IntVar(&o.TimeoutMS, "timeout", 30000, "browser lifecycle timeout in milliseconds")
	fs.StringVar(&o.Fixtures, "fixtures", "", "optional scoped fixture tree preserving relative imports")
	fs.StringVar(&o.Consumer, "consumer", "", "self-contained consumer TypeScript fixture")
	fs.StringVar(&o.Hook, "hook", "", "optional local Node application hook")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	o.Examples = fs.Args()
	if err := qualify.Run(ctx, o); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "ok")
	return 0
}
