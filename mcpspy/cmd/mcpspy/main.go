package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tmc/misc/mcpspy/internal/mcp"
)

func main() {
	var (
		inputFile  = flag.String("input-file", "", "File to read MCP messages from (default: stdin)")
		outputFile = flag.String("output-file", "", "File to write MCP messages to (default: stdout)")
		recordFile = flag.String("record-file", "recording.mcp", "File to record MCP traffic")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	var input io.Reader = os.Stdin
	if *inputFile \!= "" {
		f, err := os.Open(*inputFile)
		if err \!= nil {
			log.Fatalf("Failed to open input file: %v", err)
		}
		defer f.Close()
		input = f
	}

	var output io.Writer = os.Stdout
	if *outputFile \!= "" {
		f, err := os.Create(*outputFile)
		if err \!= nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer f.Close()
		output = f
	}

	recorder, err := os.Create(*recordFile)
	if err \!= nil {
		log.Fatalf("Failed to create record file: %v", err)
	}
	defer recorder.Close()

	spy, err := mcp.NewSpy(input, output, recorder, *verbose)
	if err \!= nil {
		log.Fatalf("Failed to create spy: %v", err)
	}

	// Handle graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("Shutting down...")
		spy.Close()
		os.Exit(0)
	}()

	if err := spy.Start(); err \!= nil {
		log.Fatalf("Spy failed: %v", err)
	}
}
