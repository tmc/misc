package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/google/go-cmp/cmp"
	"github.com/tmc/misc/mcptools/internal/mcp"
)

var (
	serverCmd = flag.String("s", "", "server command to run")
	inFile    = flag.String("f", "", "recording file to verify against")
)

func main() {
	flag.Parse()
	if *serverCmd == "" || *inFile == "" {
		log.Fatal("must specify both -s and -f")
	}

	f, err := os.Open(*inFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	entries, err := mcp.LoadRecording(f)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command("sh", "-c", *serverCmd)
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
	defer cmd.Process.Kill()

	for _, e := range entries {
		switch e.Dir {
		case "in":
			if _, err := fmt.Fprintf(stdin, "%s\n", e.Data); err != nil {
				log.Fatal(err)
			}
		case "out":
			var buf bytes.Buffer
			if _, err := io.CopyN(&buf, stdout, int64(len(e.Data)+1)); err != nil {
				log.Fatal(err)
			}
			got := bytes.TrimSpace(buf.Bytes())
			want := e.Data

			// Compare as JSON to ignore formatting differences
			var gotJSON, wantJSON interface{}
			if err := json.Unmarshal(got, &gotJSON); err != nil {
				log.Fatalf("invalid JSON response: %v", err)
			}
			if err := json.Unmarshal(want, &wantJSON); err != nil {
				log.Fatalf("invalid JSON in recording: %v", err)
			}
			if diff := cmp.Diff(wantJSON, gotJSON); diff != "" {
				log.Fatalf("response mismatch (-want +got):\n%s", diff)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("verification successful")
}
