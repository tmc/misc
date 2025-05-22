package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tmc/misc/vim-jsonrpc/pkg/client"
	"github.com/tmc/misc/vim-jsonrpc/pkg/server"
)

func main() {
	var (
		mode     = flag.String("mode", "client", "Mode: client or server")
		addr     = flag.String("addr", "localhost:8080", "Address for TCP transport")
		transport = flag.String("transport", "stdio", "Transport: stdio, tcp, or unix")
		socket   = flag.String("socket", "/tmp/vim-jsonrpc.sock", "Unix socket path")
	)
	flag.Parse()

	switch *mode {
	case "client":
		runClient(*transport, *addr, *socket)
	case "server":
		runServer(*transport, *addr, *socket)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func runClient(transport, addr, socket string) {
	c, err := client.New(transport, addr, socket)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	if err := c.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	fmt.Println("Client connected successfully")
}

func runServer(transport, addr, socket string) {
	s, err := server.New(transport, addr, socket)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	fmt.Printf("Starting server on %s\n", transport)
	if err := s.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}