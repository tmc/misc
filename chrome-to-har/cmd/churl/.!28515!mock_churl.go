// +build nomock

// Command churl is like curl but runs through Chrome and can handle JavaScript/SPAs.
// This is a mock implementation for demo purposes since Chrome is not available.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
)

// mock implementation that doesn't use Chrome
// we use build tags to ensure this doesn't conflict with the real implementation

type options struct {
	// Output options
	outputFile   string
	outputFormat string // html, har, text, json

	// Chrome options
	profileDir string
	headless   bool
	debugPort  int
	timeout    int
	chromePath string
	verbose    bool

	// Wait options
	waitNetworkIdle bool
	waitSelector    string
	stableTimeout   int

	// Request options
	headers        headerSlice
	method         string
	data           string
	followRedirect bool

	// Authentication
	username string
	password string
}

// headerSlice allows multiple -H flags
type headerSlice []string

func (h *headerSlice) String() string {
	return strings.Join(*h, ", ")
}

func (h *headerSlice) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func main() {
