package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tmc/misc/mcptools/internal/mcp"
)

var (
	dryRun = flag.Bool("n", false, "print messages instead of sending them")
	delay  = flag.Duration("d", 0, "delay between messages")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mcpreplay [flags] recording\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	entries, err := mcp.LoadRecording(f)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {
		if e.Dir != "in" {
			continue
		}
		if *dryRun {
			fmt.Printf("would send: %s\n", e.Data)
			continue
		}
		if _, err := fmt.Printf("%s\n", e.Data); err != nil {
			log.Fatal(err)
		}
		if *delay > 0 {
			time.Sleep(*delay)
		}
	}
}
