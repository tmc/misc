package main

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/tmc/misc/mcptools/internal/mcp"
)

var (
	outFile = flag.String("f", "", "output recording file")
)

func main() {
	flag.Parse()
	if *outFile == "" {
		log.Fatal("must specify -f")
	}

	f, err := os.Create(*outFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	tee := &teeRecorder{
		r:   os.Stdin,
		w:   os.Stdout,
		log: f,
	}

	_, err = io.Copy(tee, tee)
	if err != nil {
		log.Fatal(err)
	}
}

type teeRecorder struct {
	r   io.Reader
	w   io.Writer
	log io.Writer
}

func (t *teeRecorder) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		entry := mcp.Entry{Dir: "in", Data: p[:n]}
		entry.WriteTo(t.log)
	}
	return
}

func (t *teeRecorder) Write(p []byte) (n int, err error) {
	n, err = t.w.Write(p)
	if n > 0 {
		entry := mcp.Entry{Dir: "out", Data: p[:n]}
		entry.WriteTo(t.log)
	}
	return
}
