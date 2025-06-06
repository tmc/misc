package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tmc/misc/macgo"
	"github.com/tmc/misc/macgo/debug"
)

func init() {
	debug.Init()
	macgo.EnableDebug()
	macgo.Start()
}

func main() {
	fmt.Printf("=== CHILD PROCESS OUTPUT ===\n")
	fmt.Printf("PID: %d\n", os.Getpid())
	fmt.Printf("In app bundle: %t\n", macgo.IsInAppBundle())
	fmt.Printf("Args: %v\n", os.Args)
	fmt.Printf("Working dir: %s\n", getWorkingDir())

	// Test stdout
	fmt.Println("This should appear on stdout")

	// Test stderr
	fmt.Fprintln(os.Stderr, "This should appear on stderr")

	// Test with explicit flush
	fmt.Print("Flushing stdout...")
	os.Stdout.Sync()
	fmt.Println(" done")

	fmt.Print("Flushing stderr...")
	os.Stderr.Sync()
	fmt.Fprintln(os.Stderr, " done")

	fmt.Println("=== END CHILD OUTPUT ===")

	// Short wait to ensure output is flushed
	time.Sleep(100 * time.Millisecond)
}

func getWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return dir
}
