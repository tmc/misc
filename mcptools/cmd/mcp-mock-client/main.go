package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

var (
	dryRun = flag.Bool("n", false, "print requests instead of sending them")
)

func main() {
	log.SetPrefix("mcp-mock-client: ")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mcp-mock-client [flags] recording\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(os.Stdout, flag.Arg(0)); err != nil {
		log.Fatal(err)
	}
}

func run(out io.Writer, recordingFile string) error {
	f, err := os.Open(recordingFile)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Bytes()
		if !bytes.HasPrefix(line, []byte("mcp-in ")) {
			continue
		}
		req := line[7:] // skip "mcp-in "
		if *dryRun {
			fmt.Printf("would send: %s\n", req)
			continue
		}
		if _, err := fmt.Fprintf(out, "%s\n", req); err != nil {
			return err
		}
	}
	return s.Err()
}

