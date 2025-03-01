package main

import (
    "flag"
    "log"
    "net/http"

    "github.com/tmc/misc/ant-proxy/internal/proxy"
)

var (
    addr = flag.String("addr", ":8080", "HTTP service address")
    dir  = flag.String("dir", "recordings", "Directory to store recordings")
)

func main() {
    flag.Parse()

    // Create handlers
    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("OK"))
    })
    mux.Handle("/v1/messages", proxy.NewRecorder(*dir))

    // Start server
    log.Printf("Starting server on %s", *addr)
    log.Fatal(http.ListenAndServe(*addr, mux))
}

