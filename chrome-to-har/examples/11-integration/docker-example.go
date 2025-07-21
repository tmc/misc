// Docker integration example
// This example shows how to use chrome-to-har in Docker containers
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
		fmt.Println("Usage: go run docker-example.go <url>")
		os.Exit(1)
	}

	url := os.Args[1]
	
	// Docker-optimized Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("remote-debugging-port", "9222"),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	rec := recorder.New()

	err := chromedp.Run(chromeCtx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		rec.Stop(),
	)

	if err != nil {
		log.Fatal(err)
	}

	harData, err := rec.HAR()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(harData)
}