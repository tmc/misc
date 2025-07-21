// Simple HAR capture example
// This example shows the most basic usage of chrome-to-har
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run simple-capture.go <URL>")
		os.Exit(1)
	}

	url := os.Args[1]

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Chrome context
	chromeCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	// Create recorder
	rec := recorder.New()

	// Navigate to URL and capture HAR
	err := chromedp.Run(chromeCtx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		rec.Stop(),
	)

	if err != nil {
		log.Fatal(err)
	}

	// Output HAR to stdout
	harData, err := rec.HAR()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(harData)
}
