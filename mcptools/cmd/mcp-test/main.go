package main

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"
)

func main() {
    flag.Usage = func() {
        name := filepath.Base(os.Args[0])
        fmt.Printf("Usage: %s [options]", name)
        if name == "mcp-replay" || name == "mcp-verify" {
            fmt.Printf(" recording")
        }
        fmt.Printf("\n")
    }
    flag.Parse()

    if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
        flag.Usage()
        os.Exit(0)
    }
}
