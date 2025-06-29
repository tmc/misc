// Command langmodel-example demonstrates using Chrome's LanguageModel API via chromedp.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/langmodel"
)

func main() {
	var (
		prompt       = flag.String("prompt", "Explain quantum computing in simple terms.", "Prompt for the language model")
		checkOnly    = flag.Bool("check", false, "Only check availability, don't generate")
		waitDownload = flag.Bool("wait", false, "Wait for model download if needed")
		verbose      = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	ctx := context.Background()

	// Create browser instance with required Chrome flags for LanguageModel API
	browserOpts := []browser.Option{
		browser.WithHeadless(false), // Must be visible for LanguageModel API
		browser.WithVerbose(*verbose),
		browser.WithTimeout(180),
		browser.WithNavigationTimeout(60),
		browser.WithChromePath("/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"),
		browser.WithChromeFlags([]string{
			"--enable-features=Gemini,AILanguageModelService",
			"--enable-ai-language-model-service",
			"--optimization-guide-on-device-model=enabled",
			"--prompt-api-for-gemini-nano=enabled",
			"--prompt-api-for-gemini-nano-multimodal-input=enabled",
		}),
	}

	b, err := browser.New(ctx, nil, browserOpts...)
	if err != nil {
		log.Fatalf("Failed to create browser: %v", err)
	}
	defer b.Close()

	if err := b.Launch(ctx); err != nil {
		log.Fatalf("Failed to launch browser: %v", err)
	}

	// Navigate to a blank page to initialize the context
	if err := b.Navigate("about:blank"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	// Check LanguageModel availability
	fmt.Println("Checking LanguageModel availability...")
	availability, err := langmodel.CheckAvailability(b.Context())
	if err != nil {
		log.Fatalf("Failed to check availability: %v", err)
	}

	fmt.Printf("LanguageModel status: %s\n", availability)

	if *checkOnly {
		return
	}

	// Handle different availability states
	switch availability {
	case langmodel.AvailabilityDownloadable:
		if *waitDownload {
			fmt.Println("Model is downloadable. Starting download...")
			_, err := langmodel.Create(b.Context(), &langmodel.Options{
				InferenceMode: langmodel.InferenceModeOnlyOnDevice,
			})
			if err != nil {
				log.Fatalf("Failed to initiate download: %v", err)
			}

			fmt.Println("Waiting for download to complete...")
			if err := langmodel.WaitForDownload(b.Context(), 10*time.Minute); err != nil {
				log.Fatalf("Download failed or timed out: %v", err)
			}
			fmt.Println("Download completed!")
		} else {
			fmt.Println("Model needs to be downloaded. Use --wait flag to download automatically.")
			return
		}

	case langmodel.AvailabilityDownloading:
		if *waitDownload {
			fmt.Println("Model is currently downloading. Waiting...")
			if err := langmodel.WaitForDownload(b.Context(), 10*time.Minute); err != nil {
				log.Fatalf("Download failed or timed out: %v", err)
			}
			fmt.Println("Download completed!")
		} else {
			fmt.Println("Model is currently downloading. Use --wait flag to wait for completion.")
			return
		}

	case langmodel.AvailabilityAvailable:
		fmt.Println("Model is available!")

	default:
		log.Fatalf("Unexpected availability status: %s", availability)
	}

	// Create model and generate text
	fmt.Printf("Generating response to: %s\n", *prompt)
	fmt.Println("---")

	model, err := langmodel.Create(b.Context(), &langmodel.Options{
		InferenceMode: langmodel.InferenceModeOnlyOnDevice,
		Temperature:   0.7,
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	response, err := model.Generate(*prompt)
	if err != nil {
		log.Fatalf("Failed to generate response: %v", err)
	}

	fmt.Println(response)
	fmt.Println("---")
	fmt.Println("Generation completed successfully!")
}
