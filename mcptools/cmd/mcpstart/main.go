package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

var (
	outFile = flag.String("f", "", "output recording file")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mcpstart [flags] command [args...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	var out io.Writer = os.Stdout
	if *outFile != "" {
		f, err := os.Create(*outFile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		out = io.MultiWriter(os.Stdout, f)
	}

	cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, os.Stdin)
	}()

	go func() {
		io.Copy(out, stdout)
	}()

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}
